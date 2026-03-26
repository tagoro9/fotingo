package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cli/oauth"
	hub "github.com/google/go-github/v84/github"
	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/auth"
	"github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/config"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/ui"
)

// CreatePROptions contains the options for creating a pull request
type CreatePROptions struct {
	Title string
	Body  string
	Head  string
	Base  string
	Draft bool
}

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Number  int
	URL     string
	HTMLURL string
}

// Label represents a GitHub label
type Label struct {
	Name        string
	Description string
	Color       string
}

// User represents a GitHub user
type User struct {
	Login string
	Name  string
}

// Team represents a GitHub organization team.
type Team struct {
	Organization string
	Slug         string
	Name         string
	Description  string
}

// Canonical returns the canonical team identifier (<org>/<team-slug>).
func (t Team) Canonical() string {
	org := strings.TrimSpace(t.Organization)
	slug := strings.TrimSpace(t.Slug)
	if org == "" || slug == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", org, slug)
}

// CreateReleaseOptions contains the options for creating a release
type CreateReleaseOptions struct {
	TagName         string
	TargetCommitish string
	Name            string
	Body            string
	Draft           bool
	Prerelease      bool
}

// Release represents a GitHub release
type Release struct {
	ID      int64
	TagName string
	Name    string
	URL     string
	HTMLURL string
}

type Github interface {
	auth.OauthService
	config.ConfigurableService
	// GetPullRequestUrl returns the URL of the pull request for current branch or an error if it can't be found
	GetPullRequestUrl() (string, error)
	// GetCurrentUser returns the information about the current authenticated user
	GetCurrentUser() (*hub.User, error)
	// CreatePullRequest creates a new pull request
	CreatePullRequest(opts CreatePROptions) (*PullRequest, error)
	// GetLabels returns all labels from the repository
	GetLabels() ([]Label, error)
	// AddLabelsToPR adds labels to a pull request
	AddLabelsToPR(prNumber int, labels []string) error
	// GetCollaborators returns repository collaborators for reviewer selection
	GetCollaborators() ([]User, error)
	// GetOrgMembers returns organization members for participant selection
	GetOrgMembers() ([]User, error)
	// GetTeams returns organization teams for team reviewer selection
	GetTeams() ([]Team, error)
	// RequestReviewers requests reviewers on a pull request
	RequestReviewers(prNumber int, reviewers []string, teamReviewers []string) error
	// AssignUsersToPR assigns users to a pull request
	AssignUsersToPR(prNumber int, assignees []string) error
	// DoesPRExistForBranch checks if a PR exists for a given branch
	DoesPRExistForBranch(branch string) (bool, *PullRequest, error)
	// CreateRelease creates a GitHub release
	CreateRelease(opts CreateReleaseOptions) (*Release, error)
}

var oauthClientID = ""

var ErrAuthRequired = errors.New("github authentication required; run `fotingo login` interactively")

var ErrOAuthClientIDMissing = errors.New("github oauth client id is missing in this build; use `fotingo login` with API token auth or rebuild with oauth ldflags")

const (
	githubAuthMethodOAuth = "oauth"
	githubAuthMethodToken = "token"
)

type GithubConfig struct {
	Token string
}

type github struct {
	*auth.Authenticator
	*config.ViperConfigurableService
	owner                   string
	repo                    string
	git                     git.Git
	hub                     *hub.Client
	allowPrompt             bool
	promptAuth              func() (string, error)
	promptToken             func() (string, error)
	metadataCache           cache.Store
	cacheInitErr            error
	metadataFetchInfoLogger func(string)
}

const (
	defaultLabelsCacheTTL        = 30 * time.Minute
	defaultCollaboratorsCacheTTL = 720 * time.Hour
	defaultOrgMembersCacheTTL    = 720 * time.Hour
	defaultTeamsCacheTTL         = 720 * time.Hour
	maxNameLookupPerFetch        = 500
)

var newMetadataCacheStore = func(cfg *viper.Viper) (cache.Store, error) {
	path := ""
	if cfg != nil {
		path = strings.TrimSpace(cfg.GetString("cache.path"))
	}

	if path == "" {
		store, err := cache.NewDefault()
		if err != nil {
			return nil, err
		}
		return store, nil
	}

	store, err := cache.New(cache.WithPath(path), cache.WithLogger(nil))
	if err != nil {
		return nil, err
	}
	return store, nil
}

func (g *github) GetCurrentUser() (*hub.User, error) {
	user, _, err := g.hub.Users.Get(context.Background(), "")
	return user, err
}

func (g *github) GetPullRequestUrl() (string, error) {
	branch, err := g.git.GetCurrentBranch()
	if err != nil {
		return "", err
	}
	list, _, err := g.hub.PullRequests.List(context.TODO(), g.owner, g.repo, &hub.PullRequestListOptions{
		Head: fmt.Sprintf("%s:%s", g.owner, branch),
	})
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", fmt.Errorf("no pull request found for branch %s", branch)
	}
	return list[0].GetHTMLURL(), nil
}

// CreatePullRequest creates a new pull request with the given options
func (g *github) CreatePullRequest(opts CreatePROptions) (*PullRequest, error) {
	newPR := &hub.NewPullRequest{
		Title: &opts.Title,
		Body:  &opts.Body,
		Head:  &opts.Head,
		Base:  &opts.Base,
		Draft: &opts.Draft,
	}

	pr, _, err := g.hub.PullRequests.Create(context.Background(), g.owner, g.repo, newPR)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	return &PullRequest{
		Number:  pr.GetNumber(),
		URL:     pr.GetURL(),
		HTMLURL: pr.GetHTMLURL(),
	}, nil
}

// GetLabels returns all labels from the repository
func (g *github) GetLabels() ([]Label, error) {
	cacheKey := g.metadataCacheKey("labels")
	var cacheErr error
	if g.cacheInitErr != nil {
		cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to initialize metadata cache: %w", g.cacheInitErr))
	}

	if g.metadataCache != nil {
		var cachedLabels []Label
		hit, err := g.metadataCache.Get(cacheKey, &cachedLabels)
		if err != nil {
			cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to read labels cache: %w", err))
		} else if hit {
			return cachedLabels, nil
		}
	}

	labels, err := g.fetchLabels()
	if err != nil {
		if cacheErr != nil {
			return nil, errors.Join(cacheErr, err)
		}
		return nil, err
	}

	if g.metadataCache != nil {
		ttl := g.metadataTTL("cache.labelsTTL", defaultLabelsCacheTTL)
		_ = g.metadataCache.SetWithTTL(cacheKey, labels, ttl)
	}

	return labels, nil
}

func (g *github) fetchLabels() ([]Label, error) {
	var allLabels []Label
	opts := &hub.ListOptions{PerPage: 100}

	for {
		labels, resp, err := g.hub.Issues.ListLabels(context.Background(), g.owner, g.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list labels: %w", err)
		}

		for _, label := range labels {
			allLabels = append(allLabels, Label{
				Name:        label.GetName(),
				Description: label.GetDescription(),
				Color:       label.GetColor(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allLabels, nil
}

// AddLabelsToPR adds labels to a pull request
func (g *github) AddLabelsToPR(prNumber int, labels []string) error {
	_, _, err := g.hub.Issues.AddLabelsToIssue(context.Background(), g.owner, g.repo, prNumber, labels)
	if err != nil {
		return fmt.Errorf("failed to add labels to pull request: %w", err)
	}
	return nil
}

// GetCollaborators returns repository collaborators for reviewer selection
func (g *github) GetCollaborators() ([]User, error) {
	cacheKey := g.metadataCacheKey("collaborators")
	var cacheErr error
	if g.cacheInitErr != nil {
		cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to initialize metadata cache: %w", g.cacheInitErr))
	}

	if g.metadataCache != nil {
		var cachedCollaborators []User
		hit, err := g.metadataCache.Get(cacheKey, &cachedCollaborators)
		if err != nil {
			cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to read collaborators cache: %w", err))
		} else if hit {
			enrichedCollaborators, changed := g.enrichUsersWithProfileNames(cachedCollaborators, maxNameLookupPerFetch)
			if changed {
				ttl := g.metadataTTL("cache.collaboratorsTTL", defaultCollaboratorsCacheTTL)
				_ = g.metadataCache.SetWithTTL(cacheKey, enrichedCollaborators, ttl)
			}
			return enrichedCollaborators, nil
		}
	}

	collaborators, err := g.fetchCollaborators()
	if err != nil {
		if cacheErr != nil {
			return nil, errors.Join(cacheErr, err)
		}
		return nil, err
	}

	if g.metadataCache != nil {
		ttl := g.metadataTTL("cache.collaboratorsTTL", defaultCollaboratorsCacheTTL)
		_ = g.metadataCache.SetWithTTL(cacheKey, collaborators, ttl)
	}

	return collaborators, nil
}

func (g *github) fetchCollaborators() ([]User, error) {
	g.logMetadataFetchInfo(
		fmt.Sprintf("Fetching GitHub repository collaborators for %s/%s (cache miss or expired)", g.owner, g.repo),
	)

	var allCollaborators []User
	opts := &hub.ListCollaboratorsOptions{
		ListOptions: hub.ListOptions{PerPage: 100},
	}

	for {
		collaborators, resp, err := g.hub.Repositories.ListCollaborators(context.Background(), g.owner, g.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list collaborators: %w", err)
		}

		for _, collaborator := range collaborators {
			allCollaborators = append(allCollaborators, User{
				Login: collaborator.GetLogin(),
				Name:  collaborator.GetName(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	allCollaborators, _ = g.enrichUsersWithProfileNames(allCollaborators, len(allCollaborators))
	g.logMetadataFetchInfo(
		fmt.Sprintf("Fetched %d GitHub repository collaborators for %s/%s", len(allCollaborators), g.owner, g.repo),
	)

	return allCollaborators, nil
}

// GetOrgMembers returns organization members for reviewer selection.
func (g *github) GetOrgMembers() ([]User, error) {
	cacheKey := g.metadataOwnerCacheKey("org-members")
	var cacheErr error
	if g.cacheInitErr != nil {
		cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to initialize metadata cache: %w", g.cacheInitErr))
	}

	if g.metadataCache != nil {
		var cachedMembers []User
		hit, err := g.metadataCache.Get(cacheKey, &cachedMembers)
		if err != nil {
			cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to read org members cache: %w", err))
		} else if hit {
			enrichedMembers, changed := g.enrichUsersWithProfileNames(cachedMembers, maxNameLookupPerFetch)
			if changed {
				ttl := g.metadataTTL("cache.orgMembersTTL", defaultOrgMembersCacheTTL)
				_ = g.metadataCache.SetWithTTL(cacheKey, enrichedMembers, ttl)
			}
			return enrichedMembers, nil
		}
	}

	members, err := g.fetchOrgMembers()
	if err != nil {
		if cacheErr != nil {
			return nil, errors.Join(cacheErr, err)
		}
		return nil, err
	}

	if g.metadataCache != nil {
		ttl := g.metadataTTL("cache.orgMembersTTL", defaultOrgMembersCacheTTL)
		_ = g.metadataCache.SetWithTTL(cacheKey, members, ttl)
	}

	return members, nil
}

func (g *github) fetchOrgMembers() ([]User, error) {
	g.logMetadataFetchInfo(
		fmt.Sprintf("Fetching GitHub organization members for %s (cache miss or expired)", g.owner),
	)

	var allMembers []User
	opts := &hub.ListMembersOptions{
		ListOptions: hub.ListOptions{PerPage: 100},
	}

	for {
		members, resp, err := g.hub.Organizations.ListMembers(context.Background(), g.owner, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list organization members: %w", err)
		}

		for _, member := range members {
			allMembers = append(allMembers, User{
				Login: member.GetLogin(),
				Name:  member.GetName(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	allMembers, _ = g.enrichUsersWithProfileNames(allMembers, len(allMembers))
	g.logMetadataFetchInfo(
		fmt.Sprintf("Fetched %d GitHub organization members for %s", len(allMembers), g.owner),
	)

	return allMembers, nil
}

// SetMetadataFetchInfoLogger configures an optional info logger for long-running metadata fetches.
func (g *github) SetMetadataFetchInfoLogger(logf func(string)) {
	g.metadataFetchInfoLogger = logf
}

func (g *github) logMetadataFetchInfo(message string) {
	if g == nil || g.metadataFetchInfoLogger == nil {
		return
	}
	g.metadataFetchInfoLogger(strings.TrimSpace(message))
}

func (g *github) enrichUsersWithProfileNames(users []User, maxLookups int) ([]User, bool) {
	if maxLookups <= 0 {
		return users, false
	}

	enriched := false
	remainingLookups := maxLookups
	for index := range users {
		if remainingLookups == 0 {
			break
		}

		login := strings.TrimSpace(users[index].Login)
		if login == "" || strings.TrimSpace(users[index].Name) != "" {
			continue
		}

		remainingLookups--
		resolvedName, err := g.fetchUserProfileName(login)
		if err != nil {
			continue
		}
		resolvedName = strings.TrimSpace(resolvedName)
		if resolvedName == "" {
			continue
		}

		users[index].Name = resolvedName
		enriched = true
	}

	return users, enriched
}

func (g *github) fetchUserProfileName(login string) (string, error) {
	user, _, err := g.hub.Users.Get(context.Background(), login)
	if err != nil {
		return "", err
	}
	return user.GetName(), nil
}

// GetTeams returns organization teams for team reviewer selection.
func (g *github) GetTeams() ([]Team, error) {
	cacheKey := g.metadataOwnerCacheKey("teams")
	var cacheErr error
	if g.cacheInitErr != nil {
		cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to initialize metadata cache: %w", g.cacheInitErr))
	}

	if g.metadataCache != nil {
		var cachedTeams []Team
		hit, err := g.metadataCache.Get(cacheKey, &cachedTeams)
		if err != nil {
			cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to read teams cache: %w", err))
		} else if hit {
			return cachedTeams, nil
		}
	}

	teams, err := g.fetchTeams()
	if err != nil {
		if cacheErr != nil {
			return nil, errors.Join(cacheErr, err)
		}
		return nil, err
	}

	if g.metadataCache != nil {
		ttl := g.metadataTTL("cache.teamsTTL", defaultTeamsCacheTTL)
		_ = g.metadataCache.SetWithTTL(cacheKey, teams, ttl)
	}

	return teams, nil
}

func (g *github) fetchTeams() ([]Team, error) {
	var allTeams []Team
	opts := &hub.ListOptions{PerPage: 100}

	for {
		teams, resp, err := g.hub.Teams.ListTeams(context.Background(), g.owner, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list organization teams: %w", err)
		}

		for _, team := range teams {
			allTeams = append(allTeams, Team{
				Organization: g.owner,
				Slug:         team.GetSlug(),
				Name:         team.GetName(),
				Description:  team.GetDescription(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allTeams, nil
}

// SupportsOrganizationMetadata reports whether the current repository owner is
// a GitHub organization, which determines whether organization members and
// teams can be queried for participant resolution.
func (g *github) SupportsOrganizationMetadata() (bool, error) {
	cacheKey := g.metadataOwnerCacheKey("org-metadata-supported")
	var cacheErr error
	if g.cacheInitErr != nil {
		cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to initialize metadata cache: %w", g.cacheInitErr))
	}

	if g.metadataCache != nil {
		var cachedSupported bool
		hit, err := g.metadataCache.Get(cacheKey, &cachedSupported)
		if err != nil {
			cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to read owner metadata support cache: %w", err))
		} else if hit {
			return cachedSupported, nil
		}
	}

	supported, err := g.fetchOrganizationMetadataSupport()
	if err != nil {
		if cacheErr != nil {
			return false, errors.Join(cacheErr, err)
		}
		return false, err
	}

	if g.metadataCache != nil {
		ttl := g.metadataTTL("cache.orgMembersTTL", defaultOrgMembersCacheTTL)
		_ = g.metadataCache.SetWithTTL(cacheKey, supported, ttl)
	}

	return supported, nil
}

func (g *github) fetchOrganizationMetadataSupport() (bool, error) {
	repository, _, err := g.hub.Repositories.Get(context.Background(), g.owner, g.repo)
	if err != nil {
		return false, fmt.Errorf("failed to inspect repository owner type: %w", err)
	}

	owner := repository.GetOwner()
	ownerType := strings.TrimSpace(owner.GetType())
	if ownerType == "" {
		return false, fmt.Errorf("failed to determine repository owner type for %s/%s", g.owner, g.repo)
	}

	return strings.EqualFold(ownerType, "Organization"), nil
}

func (g *github) metadataCacheKey(kind string) string {
	return fmt.Sprintf("github:%s:%s/%s", kind, g.owner, g.repo)
}

func (g *github) metadataOwnerCacheKey(kind string) string {
	return fmt.Sprintf("github:%s:%s", kind, g.owner)
}

func (g *github) metadataTTL(configKey string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(g.GetConfigString(configKey))
	if raw == "" {
		return fallback
	}

	ttl, err := time.ParseDuration(raw)
	if err != nil || ttl <= 0 {
		return fallback
	}

	return ttl
}

type latestReleaseResponse struct {
	TagName string `json:"tag_name"`
}

var latestReleaseAPIBaseURL = "https://api.github.com"

// FetchLatestReleaseTag fetches the latest release tag for a public repository
// without requiring authentication.
func FetchLatestReleaseTag(ctx context.Context, client *http.Client, owner string, repo string) (string, error) {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(repo) == "" {
		return "", fmt.Errorf("owner and repo are required")
	}

	httpClient := client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 3 * time.Second}
	}
	httpClient = wrapGitHubHTTPClient(httpClient)

	baseURL := strings.TrimRight(strings.TrimSpace(latestReleaseAPIBaseURL), "/")
	endpoint := fmt.Sprintf("%s/repos/%s/%s/releases/latest", baseURL, strings.TrimSpace(owner), strings.TrimSpace(repo))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "fotingo/version-checker")

	response, err := httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("latest release request failed with %s", response.Status)
	}

	var payload latestReleaseResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.TagName) == "" {
		return "", fmt.Errorf("latest release tag is empty")
	}

	return payload.TagName, nil
}

// RequestReviewers requests reviewers on a pull request
func (g *github) RequestReviewers(prNumber int, reviewers []string, teamReviewers []string) error {
	if len(reviewers) == 0 && len(teamReviewers) == 0 {
		return nil
	}

	reviewersRequest := hub.ReviewersRequest{
		Reviewers:     reviewers,
		TeamReviewers: teamReviewers,
	}

	_, _, err := g.hub.PullRequests.RequestReviewers(context.Background(), g.owner, g.repo, prNumber, reviewersRequest)
	if err != nil {
		return fmt.Errorf("failed to request reviewers: %w", err)
	}
	return nil
}

// AssignUsersToPR assigns users to a pull request.
func (g *github) AssignUsersToPR(prNumber int, assignees []string) error {
	if len(assignees) == 0 {
		return nil
	}

	_, _, err := g.hub.Issues.AddAssignees(context.Background(), g.owner, g.repo, prNumber, assignees)
	if err != nil {
		return fmt.Errorf("failed to assign users to pull request: %w", err)
	}
	return nil
}

// DoesPRExistForBranch checks if a PR exists for a given branch
func (g *github) DoesPRExistForBranch(branch string) (bool, *PullRequest, error) {
	list, _, err := g.hub.PullRequests.List(context.Background(), g.owner, g.repo, &hub.PullRequestListOptions{
		Head:  fmt.Sprintf("%s:%s", g.owner, branch),
		State: "open",
	})
	if err != nil {
		return false, nil, fmt.Errorf("failed to check for existing pull request: %w", err)
	}

	if len(list) == 0 {
		return false, nil, nil
	}

	pr := list[0]
	return true, &PullRequest{
		Number:  pr.GetNumber(),
		URL:     pr.GetURL(),
		HTMLURL: pr.GetHTMLURL(),
	}, nil
}

// CreateRelease creates a GitHub release
func (g *github) CreateRelease(opts CreateReleaseOptions) (*Release, error) {
	releaseRequest := &hub.RepositoryRelease{
		TagName:         &opts.TagName,
		TargetCommitish: &opts.TargetCommitish,
		Name:            &opts.Name,
		Body:            &opts.Body,
		Draft:           &opts.Draft,
		Prerelease:      &opts.Prerelease,
	}

	release, _, err := g.hub.Repositories.CreateRelease(context.Background(), g.owner, g.repo, releaseRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create release: %w", err)
	}

	return &Release{
		ID:      release.GetID(),
		TagName: release.GetTagName(),
		Name:    release.GetName(),
		URL:     release.GetURL(),
		HTMLURL: release.GetHTMLURL(),
	}, nil
}

// NewWithHTTPClient returns a new GitHub client using the provided HTTP client and base URL.
// This bypasses OAuth authentication and is intended for testing with mock servers.
func NewWithHTTPClient(g git.Git, cfg *viper.Viper, httpClient *http.Client, baseURL string) (Github, error) {
	remote, err := g.GetRemote()
	if err != nil {
		return nil, err
	}
	configurableService := &config.ViperConfigurableService{Config: cfg, Prefix: "github"}

	client, err := hub.NewClient(wrapGitHubHTTPClient(httpClient)).WithEnterpriseURLs(baseURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	metadataCache, cacheInitErr := newMetadataCacheStore(cfg)
	if cacheInitErr != nil {
		metadataCache = nil
	}

	return &github{
		ViperConfigurableService: configurableService,
		git:                      g,
		hub:                      client,
		owner:                    remote.GetOwnerName(),
		repo:                     remote.GetRepoName(),
		metadataCache:            metadataCache,
		cacheInitErr:             cacheInitErr,
	}, nil
}

// NewWithHTTPClientAndRepo creates a GitHub client with explicit owner/repo, bypassing remote URL parsing.
func NewWithHTTPClientAndRepo(g git.Git, cfg *viper.Viper, httpClient *http.Client, baseURL, owner, repo string) (Github, error) {
	configurableService := &config.ViperConfigurableService{Config: cfg, Prefix: "github"}

	client, err := hub.NewClient(wrapGitHubHTTPClient(httpClient)).WithEnterpriseURLs(baseURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	metadataCache, cacheInitErr := newMetadataCacheStore(cfg)
	if cacheInitErr != nil {
		metadataCache = nil
	}

	return &github{
		ViperConfigurableService: configurableService,
		git:                      g,
		hub:                      client,
		owner:                    owner,
		repo:                     repo,
		metadataCache:            metadataCache,
		cacheInitErr:             cacheInitErr,
	}, nil
}

func New(git git.Git, cfg *viper.Viper) (Github, error) {
	return NewWithOptions(git, cfg, true)
}

func NewAuthOnly(cfg *viper.Viper) (Github, error) {
	return NewAuthOnlyWithOptions(cfg, true)
}

func NewAuthOnlyWithOptions(cfg *viper.Viper, allowPrompt bool) (Github, error) {
	configurableService := &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	oauthHost, err := oauth.NewGitHubHost("https://github.com")
	if err != nil {
		return nil, fmt.Errorf("failed to configure github oauth host: %w", err)
	}

	gh := &github{
		ViperConfigurableService: configurableService,
		Authenticator: auth.NewAuthenticator(
			oauthClientID,
			"",
			[]string{"user", "repo"},
			oauthHost,
			func() string {
				localConfig := configurableService.GetConfig()
				if localConfig == nil {
					return ""
				}
				return localConfig.GetString("token")
			},
			func(token string) error {
				return configurableService.SaveConfig("token", token)
			},
		),
		git:         nil,
		hub:         nil,
		owner:       "",
		repo:        "",
		allowPrompt: allowPrompt,
		promptAuth:  promptGitHubAuthMethod,
		promptToken: promptGitHubToken,
	}
	metadataCache, cacheInitErr := newMetadataCacheStore(cfg)
	if cacheInitErr != nil {
		metadataCache = nil
	}
	gh.metadataCache = metadataCache
	gh.cacheInitErr = cacheInitErr

	token, err := gh.resolveToken()
	if err != nil {
		return nil, err
	}
	gh.hub = hub.NewClient(wrapGitHubHTTPClient(nil)).WithAuthToken(token)
	return gh, nil
}

func NewWithOptions(git git.Git, cfg *viper.Viper, allowPrompt bool) (Github, error) {
	remote, err := git.GetRemote()
	if err != nil {
		return nil, err
	}
	configurableService := &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	oauthHost, err := oauth.NewGitHubHost("https://github.com")
	if err != nil {
		return nil, fmt.Errorf("failed to configure github oauth host: %w", err)
	}
	gh := &github{
		ViperConfigurableService: configurableService,
		Authenticator: auth.NewAuthenticator(oauthClientID, "", []string{"user", "repo"}, oauthHost,
			func() string {
				localConfig := configurableService.GetConfig()
				if localConfig == nil {
					return ""
				}
				return localConfig.GetString("token")
			}, func(token string) error {
				return configurableService.SaveConfig("token", token)
			}),
		git:         git,
		hub:         nil,
		owner:       remote.GetOwnerName(),
		repo:        remote.GetRepoName(),
		allowPrompt: allowPrompt,
		promptAuth:  promptGitHubAuthMethod,
		promptToken: promptGitHubToken,
	}
	metadataCache, cacheInitErr := newMetadataCacheStore(cfg)
	if cacheInitErr != nil {
		metadataCache = nil
	}
	gh.metadataCache = metadataCache
	gh.cacheInitErr = cacheInitErr

	token, err := gh.resolveToken()
	if err != nil {
		return nil, err
	}
	gh.hub = hub.NewClient(wrapGitHubHTTPClient(nil)).WithAuthToken(token)

	return gh, nil
}

func (g *github) resolveToken() (string, error) {
	configToken := strings.TrimSpace(g.GetConfigString("token"))
	if configToken != "" {
		if oauthToken, ok := parseStoredOAuthToken(configToken); ok {
			return oauthToken, nil
		}
		return configToken, nil
	}

	if envToken := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); envToken != "" {
		return envToken, nil
	}

	if !g.allowPrompt || !isInputTerminal() {
		return "", ErrAuthRequired
	}

	methodPrompt := g.promptAuth
	if methodPrompt == nil {
		methodPrompt = promptGitHubAuthMethod
	}

	method, err := methodPrompt()
	if err != nil {
		return "", err
	}

	switch strings.ToLower(strings.TrimSpace(method)) {
	case "", githubAuthMethodOAuth:
		if err := validateOAuthClientID(); err != nil {
			return "", err
		}
		token, authErr := g.Authenticate()
		if authErr != nil {
			return "", authErr
		}
		return token.Token, nil
	case githubAuthMethodToken:
		tokenPrompt := g.promptToken
		if tokenPrompt == nil {
			tokenPrompt = promptGitHubToken
		}
		token, tokenErr := tokenPrompt()
		if tokenErr != nil {
			return "", tokenErr
		}
		if saveErr := g.SaveConfig("token", token); saveErr != nil {
			return "", saveErr
		}
		return token, nil
	default:
		return "", fmt.Errorf("unsupported github auth method: %s", method)
	}
}

func validateOAuthClientID() error {
	if strings.TrimSpace(oauthClientID) == "" {
		return ErrOAuthClientIDMissing
	}
	return nil
}

func parseStoredOAuthToken(raw string) (string, bool) {
	var token auth.AccessToken
	if err := json.Unmarshal([]byte(raw), &token); err != nil {
		return "", false
	}
	if strings.TrimSpace(token.Token) == "" {
		return "", false
	}
	return token.Token, true
}

func promptGitHubAuthMethod() (string, error) {
	useOAuth, err := ui.Confirm("Authenticate GitHub with OAuth? (No to use classic PAT)", true)
	if err != nil {
		return "", err
	}
	if useOAuth {
		return githubAuthMethodOAuth, nil
	}
	return githubAuthMethodToken, nil
}

func promptGitHubToken() (string, error) {
	input := ui.NewInputProgram(
		ui.WithPrompt("GitHub classic PAT (repo scope required)"),
		ui.WithPlaceholder("Create at https://github.com/settings/tokens"),
		ui.WithValidation(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("github api token is required")
			}
			return nil
		}),
	)

	value, cancelled, err := input.RunWithCancel()
	if err != nil {
		return "", err
	}
	if cancelled || strings.TrimSpace(value) == "" {
		return "", ErrAuthRequired
	}

	return strings.TrimSpace(value), nil
}

func isInputTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
