package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/cache"
)

func writeProfileBatch(t *testing.T, w http.ResponseWriter, r *http.Request, nameFor func(string) string) int {
	t.Helper()
	var payload userProfilesRequest
	require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
	require.Contains(t, payload.Query, "query FotingoUserProfiles(")
	require.LessOrEqual(t, len(payload.Variables), userProfileBatchSize)
	data := make(map[string]User)
	for index := 0; index < len(payload.Variables); index++ {
		login := payload.Variables[fmt.Sprintf("login%d", index)]
		require.NotEmpty(t, login)
		data[fmt.Sprintf("p%d", index)] = User{Login: login, Name: nameFor(login)}
	}
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(struct {
		Data map[string]User `json:"data"`
	}{Data: data}))
	return len(data)
}

func newStatelessProfileClient(t *testing.T, server *httptest.Server) *github {
	t.Helper()
	// Simulate a stateless, read-only cache path; constructors must still deduplicate.
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blocked, nil, 0600))
	cfg := viper.New()
	cfg.Set("cache.path", filepath.Join(blocked, "cache.db"))
	cfg.Set("github.cache.userProfilesTTL", "1h")
	client, err := NewWithHTTPClientAndRepo(nil, cfg, server.Client(), server.URL+"/api/v3/", "acme", "repo")
	require.NoError(t, err)
	g := client.(*github)
	require.Nil(t, g.metadataCache)
	require.Error(t, g.cacheInitErr)
	return g
}

func TestStatelessUserProfilesBatchedAndShared(t *testing.T) {
	for _, count := range []int{1, 50, 51, 500} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			batchCalls, profileGets, listCalls := 0, 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v3/repos/acme/repo/collaborators", "/api/v3/orgs/acme/members":
					listCalls++
					assert.Equal(t, "100", r.URL.Query().Get("per_page"))
					page, _ := strconv.Atoi(r.URL.Query().Get("page"))
					if page == 0 {
						page = 1
					}
					start, end := (page-1)*100, min(page*100, count)
					users := make([]User, end-start)
					for i := range users {
						users[i].Login = fmt.Sprintf("user-%d", start+i)
					}
					if end < count {
						w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?page=%d>; rel="next"`, r.Host, r.URL.Path, page+1))
					}
					require.NoError(t, json.NewEncoder(w).Encode(users))
				case "/api/graphql":
					batchCalls++
					assert.Equal(t, http.MethodPost, r.Method)
					writeProfileBatch(t, w, r, func(login string) string { return "Name " + login })
				default:
					if strings.Contains(r.URL.Path, "/users/") {
						profileGets++
					}
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			g := newStatelessProfileClient(t, server)
			collaborators, err := g.GetCollaborators()
			require.NoError(t, err)
			require.Len(t, collaborators, count)
			for _, user := range collaborators {
				assert.Equal(t, "Name "+user.Login, user.Name)
			}
			assert.Equal(t, (count+userProfileBatchSize-1)/userProfileBatchSize, batchCalls)
			// Repeat via a different source with the same users and no persistent cache.
			members, err := g.GetOrgMembers()
			require.NoError(t, err)
			assert.Equal(t, collaborators, members)
			assert.Equal(t, (count+userProfileBatchSize-1)/userProfileBatchSize, batchCalls)
			assert.Zero(t, profileGets)
			assert.Equal(t, 2*((count+99)/100), listCalls)
		})
	}
}

func TestProfileBatchDeduplicatesAndCachesNamelessUsers(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeProfileBatch(t, w, r, func(string) string { return "" })
	}))
	defer server.Close()
	g := newStatelessProfileClient(t, server)
	users := []User{{Login: "Alice"}, {Login: " alice "}, {Login: ""}, {Login: "known", Name: "Existing Name"}}
	first, stats := g.enrichUsersWithProfileNames(users, 500, true)
	assert.Equal(t, 1, stats.NetworkLookups)
	assert.Equal(t, 1, stats.NetworkRequests)
	assert.Equal(t, "Existing Name", first[3].Name)
	_, stats = g.enrichUsersWithProfileNames([]User{{Login: "ALICE"}}, 500, true)
	assert.Equal(t, 1, stats.CacheHits)
	assert.Zero(t, stats.NetworkRequests)
	known, stats := g.enrichUsersWithProfileNames([]User{{Login: "KNOWN"}}, 500, true)
	assert.Equal(t, "Existing Name", known[0].Name)
	assert.Zero(t, stats.NetworkRequests)
	assert.Equal(t, 1, calls)
}

func TestProfileBatchFailureDoesNotFanOutOrPoisonNewClient(t *testing.T) {
	for _, mode := range []string{"http", "graphql", "invalid-json"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/api/graphql", r.URL.Path)
				calls++
				switch mode {
				case "http":
					w.WriteHeader(http.StatusForbidden)
				case "graphql":
					_, _ = w.Write([]byte(`{"errors":[{"message":"insufficient scopes"}]}`))
				default:
					_, _ = w.Write([]byte(`{invalid`))
				}
			}))
			defer server.Close()
			g := newStatelessProfileClient(t, server)
			users := make([]User, 120)
			for i := range users {
				users[i].Login = fmt.Sprintf("user-%d", i)
			}
			result, stats := g.enrichUsersWithProfileNames(users, 500, true)
			assert.Equal(t, users, result)
			assert.Equal(t, 1, stats.NetworkRequests)
			_, stats = g.enrichUsersWithProfileNames(users, 500, true)
			assert.Zero(t, stats.NetworkRequests)
			assert.Equal(t, 1, calls)
			other := newStatelessProfileClient(t, server)
			_, _ = other.enrichUsersWithProfileNames(users, 500, true)
			assert.Equal(t, 2, calls, "a new invocation can retry transient failures")
		})
	}
}

func TestProfileBatchKeepsPartialDataWithoutNegativeCachingErrors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"data":{"p0":{"login":"alice","name":"Alice"},"p1":null},"errors":[{"message":"user unavailable"}]}`))
			return
		}
		writeProfileBatch(t, w, r, func(string) string { return "Bob" })
	}))
	defer server.Close()
	g := newStatelessProfileClient(t, server)
	users, stats := g.enrichUsersWithProfileNames([]User{{Login: "alice"}, {Login: "bob"}}, 500, true)
	assert.Equal(t, "Alice", users[0].Name)
	assert.Empty(t, users[1].Name)
	assert.Equal(t, 1, stats.NetworkRequests)
	users, stats = g.enrichUsersWithProfileNames([]User{{Login: "alice"}, {Login: "bob"}}, 500, true)
	assert.Equal(t, "Alice", users[0].Name)
	assert.Equal(t, "Bob", users[1].Name)
	assert.Equal(t, 1, stats.NetworkLookups)
}

func TestProfileMemoExpiry(t *testing.T) {
	memo := newUserProfileMemo()
	memo.set(User{Login: "alice", Name: "Old"}, -time.Second)
	_, hit := memo.get("alice")
	assert.False(t, hit)
}

func TestConcurrentProfileEnrichmentSharesOneBatch(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeProfileBatch(t, w, r, func(string) string { return "Alice" })
	}))
	defer server.Close()
	g := newStatelessProfileClient(t, server)
	var group sync.WaitGroup
	for range 5 {
		group.Go(func() {
			users, _ := g.enrichUsersWithProfileNames([]User{{Login: "alice"}}, 500, true)
			assert.Equal(t, "Alice", users[0].Name)
		})
	}
	group.Wait()
	assert.Equal(t, 1, calls)
}

func TestProfileErrorsDoNotPersistIncompleteInventories(t *testing.T) {
	for _, mode := range []string{"fatal", "partial"} {
		t.Run(mode, func(t *testing.T) {
			lists, batches := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/collaborators") {
					lists++
					require.NoError(t, json.NewEncoder(w).Encode([]User{{Login: "alice"}, {Login: "bob"}}))
					return
				}
				require.Equal(t, "/api/graphql", r.URL.Path)
				batches++
				if batches == 1 {
					if mode == "fatal" {
						w.WriteHeader(http.StatusForbidden)
					} else {
						_, _ = w.Write([]byte(`{"data":{"p0":{"login":"alice","name":"Alice"},"p1":null},"errors":[{"message":"user unavailable"}]}`))
					}
					return
				}
				writeProfileBatch(t, w, r, func(login string) string {
					if login == "alice" {
						return "Alice"
					}
					return "Bob"
				})
			}))
			defer server.Close()
			g := newStatelessProfileClient(t, server)
			store, err := cache.New(cache.WithPath(filepath.Join(t.TempDir(), "cache.db")), cache.WithLogger(nil))
			require.NoError(t, err)
			defer func() { require.NoError(t, store.Close()) }()
			g.metadataCache, g.cacheInitErr = store, nil
			users, err := g.GetCollaborators()
			require.NoError(t, err)
			require.Len(t, users, 2)
			assert.Empty(t, users[1].Name)
			var cached []User
			hit, err := store.Get(g.metadataCacheKey("collaborators"), &cached)
			require.NoError(t, err)
			assert.False(t, hit)
			// A fresh client shares the disk cache but not the failed request's memo.
			next := *g
			next.profiles = newUserProfileMemo()
			users, err = next.GetCollaborators()
			require.NoError(t, err)
			assert.Equal(t, []User{{Login: "alice", Name: "Alice"}, {Login: "bob", Name: "Bob"}}, users)
			hit, err = store.Get(g.metadataCacheKey("collaborators"), &cached)
			require.NoError(t, err)
			assert.True(t, hit)
			assert.Equal(t, 2, lists)
			assert.Equal(t, 2, batches)
		})
	}
}
