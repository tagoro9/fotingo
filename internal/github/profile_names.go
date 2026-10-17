package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const userProfileBatchSize = 50

type userProfileMemo struct {
	lookupMu   sync.Mutex
	mu         sync.Mutex
	entries    map[string]userProfileMemoEntry
	incomplete bool // Protected by mu; do not persist an inventory after enrichment errors.
	disabled   bool // Protected by lookupMu; stop repeated failures during this command.
}

type userProfileMemoEntry struct {
	user    User
	expires time.Time
}

func newUserProfileMemo() *userProfileMemo {
	return &userProfileMemo{entries: make(map[string]userProfileMemoEntry)}
}

func (m *userProfileMemo) markIncomplete() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incomplete = true
}

func (m *userProfileMemo) canCacheLists() bool {
	if m == nil {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return !m.incomplete
}

func (m *userProfileMemo) get(login string) (User, bool) {
	if m == nil {
		return User{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := normalizeMetadataLogin(login)
	entry, ok := m.entries[key]
	if !ok {
		return User{}, false
	}
	if !time.Now().Before(entry.expires) {
		delete(m.entries, key)
		return User{}, false
	}
	return entry.user, true
}

func (m *userProfileMemo) set(user User, ttl time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[normalizeMetadataLogin(user.Login)] = userProfileMemoEntry{user: user, expires: time.Now().Add(ttl)}
}

func (g *github) enrichUsersWithProfileNames(users []User, maxLookups int, allowNetworkLookup bool) ([]User, userNameEnrichmentStats) {
	stats := userNameEnrichmentStats{}
	if maxLookups <= 0 {
		stats.RemainingEmpty = countUsersMissingNames(users)
		return users, stats
	}
	// Collaborators and organization members can overlap, even without a writable disk cache.
	if g.profiles != nil {
		g.profiles.lookupMu.Lock()
		defer g.profiles.lookupMu.Unlock()
	}
	seen := make(map[string]bool)
	pending := make([]string, 0)
	for index := range users {
		login := strings.TrimSpace(users[index].Login)
		if login == "" {
			continue
		}
		if name := strings.TrimSpace(users[index].Name); name != "" {
			g.profiles.set(User{Login: login, Name: name}, g.metadataTTL("cache.userProfilesTTL", defaultUserProfilesCacheTTL))
			continue
		}
		if name, hit := g.cachedUserProfileName(login); hit {
			stats.CacheHits++
			if name != "" {
				users[index].Name = name
				stats.Changed = true
			}
			continue
		}
		key := normalizeMetadataLogin(login)
		if !allowNetworkLookup || (g.profiles != nil && g.profiles.disabled) || seen[key] || len(pending) == maxLookups {
			continue
		}
		seen[key] = true
		pending = append(pending, login)
	}
	resolved := make(map[string]string)
	for start := 0; start < len(pending); start += userProfileBatchSize {
		end := min(start+userProfileBatchSize, len(pending))
		batch := pending[start:end]
		stats.NetworkLookups += len(batch)
		stats.NetworkRequests++
		profiles, err := g.fetchUserProfileBatch(batch)
		for login, profile := range profiles {
			resolved[login] = profile.Name
			g.cacheUserProfile(profile)
		}
		if err != nil {
			g.profiles.markIncomplete()
			if g.profiles != nil {
				g.profiles.disabled = true
			}
			g.logMetadataFetchInfo("GitHub profile-name batch lookup unavailable; keeping logins for unresolved users")
			break // Never turn a failed batch into hundreds of individual REST requests.
		}
	}
	for index := range users {
		if strings.TrimSpace(users[index].Name) != "" {
			continue
		}
		if name := resolved[normalizeMetadataLogin(users[index].Login)]; name != "" {
			users[index].Name = name
			stats.Changed = true
		}
	}
	stats.RemainingEmpty = countUsersMissingNames(users)
	return users, stats
}

type userProfilesRequest struct {
	Query     string            `json:"query"`
	Variables map[string]string `json:"variables"`
}

type userProfilesResponse struct {
	Data   map[string]*User `json:"data"`
	Errors []graphQLError   `json:"errors"`
}

func (g *github) fetchUserProfileBatch(logins []string) (map[string]User, error) {
	declarations, fields := make([]string, 0, len(logins)), make([]string, 0, len(logins))
	variables := make(map[string]string, len(logins))
	for index, login := range logins {
		key := fmt.Sprintf("login%d", index)
		declarations = append(declarations, "$"+key+":String!")
		fields = append(fields, fmt.Sprintf("p%d:user(login:$%s){login name}", index, key))
		variables[key] = login
	}
	payload := userProfilesRequest{Query: "query FotingoUserProfiles(" + strings.Join(declarations, ",") + "){" + strings.Join(fields, " ") + "}", Variables: variables}
	endpoint, err := url.Parse(g.hub.BaseURL())
	if err != nil {
		return nil, err
	}
	// GitHub Enterprise REST uses /api/v3/, while its GraphQL endpoint is /api/graphql.
	endpoint.Path = strings.TrimSuffix(strings.TrimRight(endpoint.Path, "/"), "/v3") + "/graphql"
	ctx := context.WithValue(context.Background(), githubOperationContextKey{}, "get_user_profiles")
	request, err := g.hub.NewRequest(ctx, http.MethodPost, endpoint.String(), payload)
	if err != nil {
		return nil, err
	}
	var response userProfilesResponse
	if _, err := g.hub.Do(request, &response); err != nil {
		return nil, err
	}
	if len(response.Errors) > 0 {
		g.profiles.markIncomplete()
	}
	profiles := make(map[string]User)
	for index, login := range logins {
		profile := response.Data[fmt.Sprintf("p%d", index)]
		if profile == nil || strings.TrimSpace(profile.Login) == "" {
			continue
		}
		// Cache by the requested login, also when GitHub returns its renamed canonical login.
		profiles[normalizeMetadataLogin(login)] = User{Login: login, Name: strings.TrimSpace(profile.Name)}
	}
	if len(profiles) == 0 && (len(response.Errors) > 0 || len(response.Data) == 0) {
		return profiles, fmt.Errorf("GitHub profile-name query returned no usable data")
	}
	return profiles, nil
}
