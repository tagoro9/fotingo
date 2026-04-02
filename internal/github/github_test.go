package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	hub "github.com/google/go-github/v84/github"
	giturl "github.com/kubescape/go-git-url"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/config"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/jira"
)

// GithubTestSuite is the test suite for github package
type GithubTestSuite struct {
	suite.Suite
	server *httptest.Server
	client *github
}

// mockGit is a mock implementation of the Git interface for testing
type mockGit struct {
	currentBranch string
	branchErr     error
}

func (m *mockGit) GetCurrentBranch() (string, error) {
	if m.branchErr != nil {
		return "", m.branchErr
	}
	return m.currentBranch, nil
}

func (m *mockGit) GetRemote() (giturl.IGitURL, error) {
	return nil, nil
}

func (m *mockGit) GetIssueId() (string, error) {
	return "", nil
}

func (m *mockGit) CreateIssueBranch(_ *jira.Issue) (string, error) {
	return "", nil
}

func (m *mockGit) CreateIssueWorktreeBranch(_ *jira.Issue) (string, string, error) {
	return "", "", nil
}

func (m *mockGit) Push() error {
	return nil
}

func (m *mockGit) StashChanges(_ string) error {
	return nil
}

func (m *mockGit) PopStash() error {
	return nil
}

func (m *mockGit) HasUncommittedChanges() (bool, error) {
	return false, nil
}

func (m *mockGit) GetCommitsSince(_ string) ([]git.Commit, error) {
	return nil, nil
}

func (m *mockGit) DoesBranchExistInRemote(_ string) (bool, error) {
	return false, nil
}

func (m *mockGit) GetDefaultBranch() (string, error) {
	return "main", nil
}

func (m *mockGit) FetchDefaultBranch() error {
	return nil
}

func (m *mockGit) GetCommitsSinceDefaultBranch() ([]git.Commit, error) {
	return nil, nil
}

func (m *mockGit) GetIssuesFromCommits(_ []git.Commit) []string {
	return nil
}

func (m *mockGit) GetConfig() *viper.Viper {
	return nil
}

func (m *mockGit) GetConfigString(_ string) string {
	return ""
}

func (m *mockGit) SaveConfig(_ string, _ interface{}) error {
	return nil
}

// mockPullRequest creates a mock GitHub API response for a pull request
func mockPullRequest(number int, title, url, htmlURL, head, base, state string) map[string]interface{} {
	return map[string]interface{}{
		"number":   number,
		"title":    title,
		"url":      url,
		"html_url": htmlURL,
		"state":    state,
		"head": map[string]interface{}{
			"ref": head,
			"sha": "abc123",
		},
		"base": map[string]interface{}{
			"ref": base,
		},
	}
}

// mockLabel creates a mock GitHub API response for a label
func mockLabel(name, description, color string) map[string]interface{} {
	return map[string]interface{}{
		"name":        name,
		"description": description,
		"color":       color,
	}
}

// mockUser creates a mock GitHub API response for a user
func mockUser(login, name, avatarURL string) map[string]interface{} {
	return map[string]interface{}{
		"login":      login,
		"name":       name,
		"avatar_url": avatarURL,
	}
}

// mockRelease creates a mock GitHub API response for a release
func mockRelease(id int64, tagName, name, url, htmlURL string) map[string]interface{} {
	return map[string]interface{}{
		"id":       id,
		"tag_name": tagName,
		"name":     name,
		"url":      url,
		"html_url": htmlURL,
	}
}

func ptr(value string) *string {
	return &value
}

func (suite *GithubTestSuite) SetupTest() {
	// Create a new mock HTTP server for each test
	suite.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Default 404 response
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))

	// Create a GitHub client pointing to the mock server
	hubClient := hub.NewClient(nil)
	hubClient, _ = hubClient.WithEnterpriseURLs(suite.server.URL, suite.server.URL)

	suite.client = &github{
		hub:   hubClient,
		owner: "testowner",
		repo:  "testrepo",
		git:   &mockGit{currentBranch: "feature-branch"},
	}
}

func (suite *GithubTestSuite) TearDownTest() {
	if suite.server != nil {
		suite.server.Close()
	}
}

// setupMockServer creates a new mock server with the given handler
func (suite *GithubTestSuite) setupMockServer(handler http.HandlerFunc) {
	suite.server.Close()
	suite.server = httptest.NewServer(handler)

	hubClient := hub.NewClient(nil)
	hubClient, _ = hubClient.WithEnterpriseURLs(suite.server.URL, suite.server.URL)
	suite.client.hub = hubClient
}

// TestCreatePullRequest tests creating a pull request
func (suite *GithubTestSuite) TestCreatePullRequest() {
	tests := []struct {
		name           string
		opts           CreatePROptions
		mockResponse   map[string]interface{}
		mockStatusCode int
		wantErr        bool
		wantPR         *PullRequest
	}{
		{
			name: "success - create pull request",
			opts: CreatePROptions{
				Title: "Test PR",
				Body:  "This is a test pull request",
				Head:  "feature-branch",
				Base:  "main",
				Draft: false,
			},
			mockResponse:   mockPullRequest(1, "Test PR", "https://api.github.com/repos/testowner/testrepo/pulls/1", "https://github.com/testowner/testrepo/pull/1", "feature-branch", "main", "open"),
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
			wantPR: &PullRequest{
				Number:  1,
				URL:     "https://api.github.com/repos/testowner/testrepo/pulls/1",
				HTMLURL: "https://github.com/testowner/testrepo/pull/1",
			},
		},
		{
			name: "success - create draft pull request",
			opts: CreatePROptions{
				Title: "Draft PR",
				Body:  "This is a draft pull request",
				Head:  "feature-branch",
				Base:  "main",
				Draft: true,
			},
			mockResponse:   mockPullRequest(2, "Draft PR", "https://api.github.com/repos/testowner/testrepo/pulls/2", "https://github.com/testowner/testrepo/pull/2", "feature-branch", "main", "open"),
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
			wantPR: &PullRequest{
				Number:  2,
				URL:     "https://api.github.com/repos/testowner/testrepo/pulls/2",
				HTMLURL: "https://github.com/testowner/testrepo/pull/2",
			},
		},
		{
			name: "error - validation error",
			opts: CreatePROptions{
				Title: "Test PR",
				Body:  "This is a test",
				Head:  "feature-branch",
				Base:  "main",
			},
			mockResponse: map[string]interface{}{
				"message": "Validation Failed",
				"errors": []map[string]interface{}{
					{"message": "A pull request already exists for this branch"},
				},
			},
			mockStatusCode: http.StatusUnprocessableEntity,
			wantErr:        true,
			wantPR:         nil,
		},
		{
			name: "error - unauthorized",
			opts: CreatePROptions{
				Title: "Test PR",
				Body:  "This is a test",
				Head:  "feature-branch",
				Base:  "main",
			},
			mockResponse: map[string]interface{}{
				"message": "Bad credentials",
			},
			mockStatusCode: http.StatusUnauthorized,
			wantErr:        true,
			wantPR:         nil,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v3/repos/testowner/testrepo/pulls" && r.Method == http.MethodPost {
					w.WriteHeader(tt.mockStatusCode)
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			pr, err := suite.client.CreatePullRequest(tt.opts)

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), pr)
			} else {
				assert.NoError(suite.T(), err)
				assert.NotNil(suite.T(), pr)
				assert.Equal(suite.T(), tt.wantPR.Number, pr.Number)
				assert.Equal(suite.T(), tt.wantPR.URL, pr.URL)
				assert.Equal(suite.T(), tt.wantPR.HTMLURL, pr.HTMLURL)
			}
		})
	}
}

func (suite *GithubTestSuite) TestUpdatePullRequest() {
	tests := []struct {
		name           string
		prNumber       int
		opts           UpdatePROptions
		mockResponse   map[string]interface{}
		mockStatusCode int
		wantErr        bool
		wantPR         *PullRequest
	}{
		{
			name:     "success - update title and body",
			prNumber: 3,
			opts: UpdatePROptions{
				Title: ptr("Updated title"),
				Body:  ptr("Updated body"),
			},
			mockResponse:   mockPullRequest(3, "Updated title", "https://api.github.com/repos/testowner/testrepo/pulls/3", "https://github.com/testowner/testrepo/pull/3", "feature-branch", "main", "open"),
			mockStatusCode: http.StatusOK,
			wantPR: &PullRequest{
				Title:   "Updated title",
				Body:    "Updated body",
				Number:  3,
				URL:     "https://api.github.com/repos/testowner/testrepo/pulls/3",
				HTMLURL: "https://github.com/testowner/testrepo/pull/3",
			},
		},
		{
			name:     "error - not found",
			prNumber: 9,
			opts: UpdatePROptions{
				Body: ptr("Updated body"),
			},
			mockResponse:   map[string]interface{}{"message": "Not Found"},
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/api/v3/repos/testowner/testrepo/pulls/%d", tt.prNumber)
				if r.URL.Path == expectedPath && r.Method == http.MethodPatch {
					w.WriteHeader(tt.mockStatusCode)
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			pr, err := suite.client.UpdatePullRequest(tt.prNumber, tt.opts)

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), pr)
				return
			}

			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), pr)
			assert.Equal(suite.T(), tt.wantPR.Title, pr.Title)
			assert.Equal(suite.T(), tt.wantPR.Number, pr.Number)
			assert.Equal(suite.T(), tt.wantPR.URL, pr.URL)
			assert.Equal(suite.T(), tt.wantPR.HTMLURL, pr.HTMLURL)
		})
	}
}

// TestGetLabels tests fetching repository labels
func (suite *GithubTestSuite) TestGetLabels() {
	tests := []struct {
		name          string
		mockResponses []struct {
			labels      []map[string]interface{}
			statusCode  int
			hasNextPage bool
		}
		wantErr    bool
		wantCount  int
		wantLabels []Label
	}{
		{
			name: "success - single page of labels",
			mockResponses: []struct {
				labels      []map[string]interface{}
				statusCode  int
				hasNextPage bool
			}{
				{
					labels: []map[string]interface{}{
						mockLabel("bug", "Something is broken", "d73a4a"),
						mockLabel("enhancement", "New feature", "a2eeef"),
						mockLabel("documentation", "Docs update", "0075ca"),
					},
					statusCode:  http.StatusOK,
					hasNextPage: false,
				},
			},
			wantErr:   false,
			wantCount: 3,
			wantLabels: []Label{
				{Name: "bug", Description: "Something is broken", Color: "d73a4a"},
				{Name: "enhancement", Description: "New feature", Color: "a2eeef"},
				{Name: "documentation", Description: "Docs update", Color: "0075ca"},
			},
		},
		{
			name: "success - paginated labels",
			mockResponses: []struct {
				labels      []map[string]interface{}
				statusCode  int
				hasNextPage bool
			}{
				{
					labels: []map[string]interface{}{
						mockLabel("bug", "Something is broken", "d73a4a"),
						mockLabel("enhancement", "New feature", "a2eeef"),
					},
					statusCode:  http.StatusOK,
					hasNextPage: true,
				},
				{
					labels: []map[string]interface{}{
						mockLabel("documentation", "Docs update", "0075ca"),
					},
					statusCode:  http.StatusOK,
					hasNextPage: false,
				},
			},
			wantErr:   false,
			wantCount: 3,
		},
		{
			name: "success - no labels",
			mockResponses: []struct {
				labels      []map[string]interface{}
				statusCode  int
				hasNextPage bool
			}{
				{
					labels:      []map[string]interface{}{},
					statusCode:  http.StatusOK,
					hasNextPage: false,
				},
			},
			wantErr:   false,
			wantCount: 0,
		},
		{
			name: "error - server error",
			mockResponses: []struct {
				labels      []map[string]interface{}
				statusCode  int
				hasNextPage bool
			}{
				{
					labels:      nil,
					statusCode:  http.StatusInternalServerError,
					hasNextPage: false,
				},
			},
			wantErr:   true,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			page := 0
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v3/repos/testowner/testrepo/labels" && r.Method == http.MethodGet {
					if page >= len(tt.mockResponses) {
						w.WriteHeader(http.StatusNotFound)
						return
					}

					resp := tt.mockResponses[page]
					if resp.hasNextPage {
						nextPage := page + 2 // GitHub pages are 1-indexed
						w.Header().Set("Link", fmt.Sprintf(`<%s/api/v3/repos/testowner/testrepo/labels?page=%d>; rel="next"`, suite.server.URL, nextPage))
					}
					w.WriteHeader(resp.statusCode)
					_ = json.NewEncoder(w).Encode(resp.labels)
					page++
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			labels, err := suite.client.GetLabels()

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), labels)
			} else {
				assert.NoError(suite.T(), err)
				assert.Len(suite.T(), labels, tt.wantCount)
				if tt.wantLabels != nil {
					for i, want := range tt.wantLabels {
						assert.Equal(suite.T(), want.Name, labels[i].Name)
						assert.Equal(suite.T(), want.Description, labels[i].Description)
						assert.Equal(suite.T(), want.Color, labels[i].Color)
					}
				}
			}
		})
	}
}

// TestAddLabelsToPR tests adding labels to a pull request
func (suite *GithubTestSuite) TestAddLabelsToPR() {
	tests := []struct {
		name           string
		prNumber       int
		labels         []string
		mockResponse   []map[string]interface{}
		mockStatusCode int
		wantErr        bool
	}{
		{
			name:     "success - add single label",
			prNumber: 1,
			labels:   []string{"bug"},
			mockResponse: []map[string]interface{}{
				mockLabel("bug", "Something is broken", "d73a4a"),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:     "success - add multiple labels",
			prNumber: 1,
			labels:   []string{"bug", "enhancement", "documentation"},
			mockResponse: []map[string]interface{}{
				mockLabel("bug", "Something is broken", "d73a4a"),
				mockLabel("enhancement", "New feature", "a2eeef"),
				mockLabel("documentation", "Docs update", "0075ca"),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "error - issue not found",
			prNumber:       999,
			labels:         []string{"bug"},
			mockResponse:   nil,
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
		},
		{
			name:           "error - invalid label",
			prNumber:       1,
			labels:         []string{"nonexistent-label"},
			mockResponse:   nil,
			mockStatusCode: http.StatusUnprocessableEntity,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/api/v3/repos/testowner/testrepo/issues/%d/labels", tt.prNumber)
				if r.URL.Path == expectedPath && r.Method == http.MethodPost {
					w.WriteHeader(tt.mockStatusCode)
					if tt.mockResponse != nil {
						_ = json.NewEncoder(w).Encode(tt.mockResponse)
					} else {
						_ = json.NewEncoder(w).Encode(map[string]string{"message": "error"})
					}
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			err := suite.client.AddLabelsToPR(tt.prNumber, tt.labels)

			if tt.wantErr {
				assert.Error(suite.T(), err)
			} else {
				assert.NoError(suite.T(), err)
			}
		})
	}
}

// TestGetCollaborators tests fetching repository collaborators
func (suite *GithubTestSuite) TestGetCollaborators() {
	tests := []struct {
		name          string
		mockResponses []struct {
			collaborators []map[string]interface{}
			statusCode    int
			hasNextPage   bool
		}
		wantErr   bool
		wantCount int
		wantUsers []User
	}{
		{
			name: "success - single page of collaborators",
			mockResponses: []struct {
				collaborators []map[string]interface{}
				statusCode    int
				hasNextPage   bool
			}{
				{
					collaborators: []map[string]interface{}{
						mockUser("user1", "User One", "https://avatars.githubusercontent.com/u/1"),
						mockUser("user2", "User Two", "https://avatars.githubusercontent.com/u/2"),
					},
					statusCode:  http.StatusOK,
					hasNextPage: false,
				},
			},
			wantErr:   false,
			wantCount: 2,
			wantUsers: []User{
				{Login: "user1", Name: "User One"},
				{Login: "user2", Name: "User Two"},
			},
		},
		{
			name: "success - paginated collaborators",
			mockResponses: []struct {
				collaborators []map[string]interface{}
				statusCode    int
				hasNextPage   bool
			}{
				{
					collaborators: []map[string]interface{}{
						mockUser("user1", "User One", "https://avatars.githubusercontent.com/u/1"),
					},
					statusCode:  http.StatusOK,
					hasNextPage: true,
				},
				{
					collaborators: []map[string]interface{}{
						mockUser("user2", "User Two", "https://avatars.githubusercontent.com/u/2"),
					},
					statusCode:  http.StatusOK,
					hasNextPage: false,
				},
			},
			wantErr:   false,
			wantCount: 2,
		},
		{
			name: "success - no collaborators",
			mockResponses: []struct {
				collaborators []map[string]interface{}
				statusCode    int
				hasNextPage   bool
			}{
				{
					collaborators: []map[string]interface{}{},
					statusCode:    http.StatusOK,
					hasNextPage:   false,
				},
			},
			wantErr:   false,
			wantCount: 0,
		},
		{
			name: "error - forbidden",
			mockResponses: []struct {
				collaborators []map[string]interface{}
				statusCode    int
				hasNextPage   bool
			}{
				{
					collaborators: nil,
					statusCode:    http.StatusForbidden,
					hasNextPage:   false,
				},
			},
			wantErr:   true,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			page := 0
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v3/repos/testowner/testrepo/collaborators" && r.Method == http.MethodGet {
					if page >= len(tt.mockResponses) {
						w.WriteHeader(http.StatusNotFound)
						return
					}

					resp := tt.mockResponses[page]
					if resp.hasNextPage {
						nextPage := page + 2
						w.Header().Set("Link", fmt.Sprintf(`<%s/api/v3/repos/testowner/testrepo/collaborators?page=%d>; rel="next"`, suite.server.URL, nextPage))
					}
					w.WriteHeader(resp.statusCode)
					_ = json.NewEncoder(w).Encode(resp.collaborators)
					page++
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			collaborators, err := suite.client.GetCollaborators()

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), collaborators)
			} else {
				assert.NoError(suite.T(), err)
				assert.Len(suite.T(), collaborators, tt.wantCount)
				if tt.wantUsers != nil {
					for i, want := range tt.wantUsers {
						assert.Equal(suite.T(), want.Login, collaborators[i].Login)
						assert.Equal(suite.T(), want.Name, collaborators[i].Name)
					}
				}
			}
		})
	}
}

func (suite *GithubTestSuite) TestGetCollaborators_EnrichesMissingNameFromProfile() {
	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/repos/testowner/testrepo/collaborators" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"login":      "yprk",
					"avatar_url": "https://avatars.githubusercontent.com/u/1",
				},
			})
			return
		case r.URL.Path == "/api/v3/users/yprk" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockUser("yprk", "YoungJun Park", "https://avatars.githubusercontent.com/u/1"))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	collaborators, err := suite.client.GetCollaborators()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), collaborators, 1)
	assert.Equal(suite.T(), "yprk", collaborators[0].Login)
	assert.Equal(suite.T(), "YoungJun Park", collaborators[0].Name)
}

func (suite *GithubTestSuite) TestGetCollaborators_EnrichesMissingNameFromCacheHit() {
	store, err := cache.New(cache.WithPath(filepath.Join(suite.T().TempDir(), "cache.db")), cache.WithLogger(nil))
	assert.NoError(suite.T(), err)
	defer func() { _ = store.Close() }()

	cfg := viper.New()
	cfg.Set("github.cache.collaboratorsTTL", "1h")
	cfg.Set("github.cache.userProfilesTTL", "1h")
	suite.client.ViperConfigurableService = &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	suite.client.metadataCache = store
	suite.client.cacheInitErr = nil

	cacheKey := suite.client.metadataCacheKey("collaborators")
	assert.NoError(suite.T(), store.SetWithTTL(cacheKey, []map[string]any{
		{
			"login":      "yprk",
			"name":       "",
			"avatar_url": "https://avatars.githubusercontent.com/u/1",
		},
	}, time.Hour))
	assert.NoError(suite.T(), store.SetWithTTL(suite.client.metadataUserProfileCacheKey("yprk"), map[string]any{
		"resolved": true,
		"user": map[string]any{
			"login": "yprk",
			"name":  "YoungJun Park",
		},
	}, time.Hour))

	collaboratorListCalls := 0
	profileLookupCalls := 0
	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/repos/testowner/testrepo/collaborators" && r.Method == http.MethodGet:
			collaboratorListCalls++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]any{})
			return
		case r.URL.Path == "/api/v3/users/yprk" && r.Method == http.MethodGet:
			profileLookupCalls++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockUser("yprk", "YoungJun Park", "https://avatars.githubusercontent.com/u/1"))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	collaborators, err := suite.client.GetCollaborators()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), collaborators, 1)
	assert.Equal(suite.T(), "yprk", collaborators[0].Login)
	assert.Equal(suite.T(), "YoungJun Park", collaborators[0].Name)
	assert.Equal(suite.T(), 0, collaboratorListCalls)
	assert.Equal(suite.T(), 0, profileLookupCalls)

	entries, err := store.List(cacheKey)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), entries, 1)
	assert.NotContains(suite.T(), strings.ToLower(string(entries[0].Value)), "avatar")
}

func (suite *GithubTestSuite) TestGetCollaborators_EnrichesMissingNameFromLegacyCacheHit() {
	store, err := cache.New(cache.WithPath(filepath.Join(suite.T().TempDir(), "cache.db")), cache.WithLogger(nil))
	assert.NoError(suite.T(), err)
	defer func() { _ = store.Close() }()

	cfg := viper.New()
	cfg.Set("github.cache.collaboratorsTTL", "1h")
	cfg.Set("github.cache.userProfilesTTL", "1h")
	suite.client.ViperConfigurableService = &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	suite.client.metadataCache = store
	suite.client.cacheInitErr = nil

	cacheKey := suite.client.metadataCacheKey("collaborators")
	assert.NoError(suite.T(), store.SetWithTTL(cacheKey, []map[string]any{
		{
			"login":      "yprk",
			"name":       "",
			"avatar_url": "https://avatars.githubusercontent.com/u/1",
		},
	}, time.Hour))
	assert.NoError(suite.T(), store.SetWithTTL(suite.client.metadataLegacyUserProfileNameCacheKey("yprk"), map[string]any{
		"resolved": true,
		"name":     "YoungJun Park",
	}, time.Hour))

	collaboratorListCalls := 0
	profileLookupCalls := 0
	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/repos/testowner/testrepo/collaborators" && r.Method == http.MethodGet:
			collaboratorListCalls++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]any{})
			return
		case r.URL.Path == "/api/v3/users/yprk" && r.Method == http.MethodGet:
			profileLookupCalls++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockUser("yprk", "YoungJun Park", "https://avatars.githubusercontent.com/u/1"))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	collaborators, err := suite.client.GetCollaborators()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), collaborators, 1)
	assert.Equal(suite.T(), "yprk", collaborators[0].Login)
	assert.Equal(suite.T(), "YoungJun Park", collaborators[0].Name)
	assert.Equal(suite.T(), 0, collaboratorListCalls)
	assert.Equal(suite.T(), 0, profileLookupCalls)

	var cachedProfile map[string]any
	hit, err := store.Get(suite.client.metadataUserProfileCacheKey("yprk"), &cachedProfile)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), hit)
	assert.Equal(suite.T(), true, cachedProfile["resolved"])
}

func (suite *GithubTestSuite) TestGetOrgMembers_EnrichesMissingNameFromCacheHit() {
	store, err := cache.New(cache.WithPath(filepath.Join(suite.T().TempDir(), "cache.db")), cache.WithLogger(nil))
	assert.NoError(suite.T(), err)
	defer func() { _ = store.Close() }()

	cfg := viper.New()
	cfg.Set("github.cache.orgMembersTTL", "1h")
	cfg.Set("github.cache.userProfilesTTL", "1h")
	suite.client.ViperConfigurableService = &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	suite.client.metadataCache = store
	suite.client.cacheInitErr = nil

	cacheKey := suite.client.metadataOwnerCacheKey("org-members")
	assert.NoError(suite.T(), store.SetWithTTL(cacheKey, []map[string]any{
		{
			"login":      "yprk",
			"name":       "",
			"avatar_url": "https://avatars.githubusercontent.com/u/1",
		},
	}, time.Hour))
	assert.NoError(suite.T(), store.SetWithTTL(suite.client.metadataUserProfileCacheKey("yprk"), map[string]any{
		"resolved": true,
		"user": map[string]any{
			"login": "yprk",
			"name":  "YoungJun Park",
		},
	}, time.Hour))

	orgMemberListCalls := 0
	profileLookupCalls := 0
	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/orgs/testowner/members" && r.Method == http.MethodGet:
			orgMemberListCalls++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]any{})
			return
		case r.URL.Path == "/api/v3/users/yprk" && r.Method == http.MethodGet:
			profileLookupCalls++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockUser("yprk", "YoungJun Park", "https://avatars.githubusercontent.com/u/1"))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	members, err := suite.client.GetOrgMembers()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), members, 1)
	assert.Equal(suite.T(), "yprk", members[0].Login)
	assert.Equal(suite.T(), "YoungJun Park", members[0].Name)
	assert.Equal(suite.T(), 0, orgMemberListCalls)
	assert.Equal(suite.T(), 0, profileLookupCalls)

	entries, err := store.List(cacheKey)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), entries, 1)
	assert.NotContains(suite.T(), strings.ToLower(string(entries[0].Value)), "avatar")
}

func (suite *GithubTestSuite) TestGetOrgMembers_EnrichesMissingNameFromLegacyCacheHit() {
	store, err := cache.New(cache.WithPath(filepath.Join(suite.T().TempDir(), "cache.db")), cache.WithLogger(nil))
	assert.NoError(suite.T(), err)
	defer func() { _ = store.Close() }()

	cfg := viper.New()
	cfg.Set("github.cache.orgMembersTTL", "1h")
	cfg.Set("github.cache.userProfilesTTL", "1h")
	suite.client.ViperConfigurableService = &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	suite.client.metadataCache = store
	suite.client.cacheInitErr = nil

	cacheKey := suite.client.metadataOwnerCacheKey("org-members")
	assert.NoError(suite.T(), store.SetWithTTL(cacheKey, []map[string]any{
		{
			"login":      "yprk",
			"name":       "",
			"avatar_url": "https://avatars.githubusercontent.com/u/1",
		},
	}, time.Hour))
	assert.NoError(suite.T(), store.SetWithTTL(suite.client.metadataLegacyUserProfileNameCacheKey("yprk"), map[string]any{
		"resolved": true,
		"name":     "YoungJun Park",
	}, time.Hour))

	orgMemberListCalls := 0
	profileLookupCalls := 0
	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/orgs/testowner/members" && r.Method == http.MethodGet:
			orgMemberListCalls++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]any{})
			return
		case r.URL.Path == "/api/v3/users/yprk" && r.Method == http.MethodGet:
			profileLookupCalls++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockUser("yprk", "YoungJun Park", "https://avatars.githubusercontent.com/u/1"))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	members, err := suite.client.GetOrgMembers()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), members, 1)
	assert.Equal(suite.T(), "yprk", members[0].Login)
	assert.Equal(suite.T(), "YoungJun Park", members[0].Name)
	assert.Equal(suite.T(), 0, orgMemberListCalls)
	assert.Equal(suite.T(), 0, profileLookupCalls)

	var cachedProfile map[string]any
	hit, err := store.Get(suite.client.metadataUserProfileCacheKey("yprk"), &cachedProfile)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), hit)
	assert.Equal(suite.T(), true, cachedProfile["resolved"])
}

func (suite *GithubTestSuite) TestGetOrgMembers_LimitsColdFetchProfileLookups() {
	const totalMembers = maxNameLookupPerFetch + 20

	profileLookupCalls := 0
	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/orgs/testowner/members" && r.Method == http.MethodGet:
			members := make([]map[string]any, 0, totalMembers)
			for i := 0; i < totalMembers; i++ {
				login := fmt.Sprintf("member-%d", i)
				members = append(members, map[string]any{
					"login":      login,
					"avatar_url": fmt.Sprintf("https://avatars.githubusercontent.com/u/%d", i+1),
				})
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(members)
			return
		case strings.HasPrefix(r.URL.Path, "/api/v3/users/member-") && r.Method == http.MethodGet:
			profileLookupCalls++
			login := strings.TrimPrefix(r.URL.Path, "/api/v3/users/")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockUser(login, fmt.Sprintf("Name %s", login), "https://avatars.githubusercontent.com/u/1"))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	members, err := suite.client.GetOrgMembers()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), members, totalMembers)
	assert.Equal(suite.T(), maxNameLookupPerFetch, profileLookupCalls)
	for index, member := range members {
		if index < maxNameLookupPerFetch {
			assert.Equal(suite.T(), fmt.Sprintf("Name %s", member.Login), member.Name)
			continue
		}
		assert.Empty(suite.T(), member.Name)
	}
}

func (suite *GithubTestSuite) TestGetCollaborators_DefaultCacheTTLIsThirtyDays() {
	store, err := cache.New(cache.WithPath(filepath.Join(suite.T().TempDir(), "cache.db")), cache.WithLogger(nil))
	assert.NoError(suite.T(), err)
	defer func() { _ = store.Close() }()

	cfg := viper.New()
	suite.client.ViperConfigurableService = &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	suite.client.metadataCache = store
	suite.client.cacheInitErr = nil

	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/repos/testowner/testrepo/collaborators" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]any{
				mockUser("alice", "Alice Developer", "https://avatars.githubusercontent.com/u/1"),
			})
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	start := time.Now().UTC()
	collaborators, err := suite.client.GetCollaborators()
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), collaborators, 1)

	entries, err := store.List(suite.client.metadataCacheKey("collaborators"))
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), entries, 1)
	require.NotNil(suite.T(), entries[0].ExpiresAt)

	ttl := entries[0].ExpiresAt.Sub(start)
	expectedTTL := 30 * 24 * time.Hour
	assert.GreaterOrEqual(suite.T(), ttl, expectedTTL-time.Hour)
	assert.LessOrEqual(suite.T(), ttl, expectedTTL+time.Hour)
}

func (suite *GithubTestSuite) TestGetOrgMembers_DefaultCacheTTLIsThirtyDays() {
	store, err := cache.New(cache.WithPath(filepath.Join(suite.T().TempDir(), "cache.db")), cache.WithLogger(nil))
	assert.NoError(suite.T(), err)
	defer func() { _ = store.Close() }()

	cfg := viper.New()
	suite.client.ViperConfigurableService = &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	suite.client.metadataCache = store
	suite.client.cacheInitErr = nil

	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/orgs/testowner/members" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]any{
				mockUser("member1", "Member One", "https://avatars.githubusercontent.com/u/1"),
			})
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	start := time.Now().UTC()
	members, err := suite.client.GetOrgMembers()
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), members, 1)

	entries, err := store.List(suite.client.metadataOwnerCacheKey("org-members"))
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), entries, 1)
	require.NotNil(suite.T(), entries[0].ExpiresAt)

	ttl := entries[0].ExpiresAt.Sub(start)
	expectedTTL := 30 * 24 * time.Hour
	assert.GreaterOrEqual(suite.T(), ttl, expectedTTL-time.Hour)
	assert.LessOrEqual(suite.T(), ttl, expectedTTL+time.Hour)
}

func (suite *GithubTestSuite) TestGetCollaborators_CachesFetchedUserProfilesWithConfiguredTTL() {
	store, err := cache.New(cache.WithPath(filepath.Join(suite.T().TempDir(), "cache.db")), cache.WithLogger(nil))
	assert.NoError(suite.T(), err)
	defer func() { _ = store.Close() }()

	cfg := viper.New()
	cfg.Set("github.cache.collaboratorsTTL", "1h")
	cfg.Set("github.cache.userProfilesTTL", "2h")
	suite.client.ViperConfigurableService = &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	suite.client.metadataCache = store
	suite.client.cacheInitErr = nil

	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/repos/testowner/testrepo/collaborators" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"login":      "yprk",
					"name":       "",
					"avatar_url": "https://avatars.githubusercontent.com/u/1",
				},
			})
			return
		case r.URL.Path == "/api/v3/users/yprk" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockUser("yprk", "YoungJun Park", "https://avatars.githubusercontent.com/u/1"))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	start := time.Now().UTC()
	collaborators, err := suite.client.GetCollaborators()
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), collaborators, 1)

	entries, err := store.List(suite.client.metadataUserProfileCacheKey("yprk"))
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), entries, 1)
	require.NotNil(suite.T(), entries[0].ExpiresAt)

	ttl := entries[0].ExpiresAt.Sub(start)
	expectedTTL := 2 * time.Hour
	assert.GreaterOrEqual(suite.T(), ttl, expectedTTL-time.Minute)
	assert.LessOrEqual(suite.T(), ttl, expectedTTL+time.Minute)
}

func (suite *GithubTestSuite) TestGetOrgMembers_ReusesOwnerScopedCacheAcrossRepos() {
	store, err := cache.New(cache.WithPath(filepath.Join(suite.T().TempDir(), "cache.db")), cache.WithLogger(nil))
	assert.NoError(suite.T(), err)
	defer func() { _ = store.Close() }()

	cfg := viper.New()
	cfg.Set("github.cache.orgMembersTTL", "720h")
	suite.client.ViperConfigurableService = &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	suite.client.metadataCache = store
	suite.client.cacheInitErr = nil

	orgMemberListCalls := 0
	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/orgs/testowner/members" && r.Method == http.MethodGet:
			orgMemberListCalls++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]any{
				mockUser("member1", "Member One", "https://avatars.githubusercontent.com/u/1"),
			})
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	otherRepoClient := *suite.client
	otherRepoClient.repo = "otherrepo"

	first, err := suite.client.GetOrgMembers()
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), first, 1)

	second, err := otherRepoClient.GetOrgMembers()
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), second, 1)

	assert.Equal(suite.T(), 1, orgMemberListCalls)
	entries, err := store.List(suite.client.metadataOwnerCacheKey("org-members"))
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), entries, 1)
	assert.Equal(suite.T(), suite.client.metadataOwnerCacheKey("org-members"), entries[0].Key)
}

func (suite *GithubTestSuite) TestGetCollaborators_EmitsFetchLogsOnlyOnCacheMiss() {
	store, err := cache.New(cache.WithPath(filepath.Join(suite.T().TempDir(), "cache.db")), cache.WithLogger(nil))
	assert.NoError(suite.T(), err)
	defer func() { _ = store.Close() }()

	cfg := viper.New()
	cfg.Set("github.cache.collaboratorsTTL", "720h")
	suite.client.ViperConfigurableService = &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	suite.client.metadataCache = store
	suite.client.cacheInitErr = nil

	logs := make([]string, 0)
	suite.client.SetMetadataFetchInfoLogger(func(message string) {
		logs = append(logs, message)
	})

	collaboratorListCalls := 0
	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/repos/testowner/testrepo/collaborators" && r.Method == http.MethodGet:
			collaboratorListCalls++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]any{
				mockUser("alice", "Alice Developer", "https://avatars.githubusercontent.com/u/1"),
			})
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	first, err := suite.client.GetCollaborators()
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), first, 1)

	second, err := suite.client.GetCollaborators()
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), second, 1)

	assert.Equal(suite.T(), 1, collaboratorListCalls)
	require.Len(suite.T(), logs, 3)
	assert.Contains(suite.T(), logs[0], "Fetching GitHub repository collaborators for testowner/testrepo")
	assert.Contains(suite.T(), logs[1], "Fetched 1 GitHub repository collaborators for testowner/testrepo")
	assert.Contains(suite.T(), logs[2], "Loaded 1 GitHub repository collaborators for testowner/testrepo from cache")
}

func (suite *GithubTestSuite) TestGetOrgMembers_EmitsFetchLogsOnlyOnCacheMiss() {
	store, err := cache.New(cache.WithPath(filepath.Join(suite.T().TempDir(), "cache.db")), cache.WithLogger(nil))
	assert.NoError(suite.T(), err)
	defer func() { _ = store.Close() }()

	cfg := viper.New()
	cfg.Set("github.cache.orgMembersTTL", "720h")
	suite.client.ViperConfigurableService = &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	suite.client.metadataCache = store
	suite.client.cacheInitErr = nil

	logs := make([]string, 0)
	suite.client.SetMetadataFetchInfoLogger(func(message string) {
		logs = append(logs, message)
	})

	orgMemberListCalls := 0
	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/orgs/testowner/members" && r.Method == http.MethodGet:
			orgMemberListCalls++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]any{
				mockUser("member1", "Member One", "https://avatars.githubusercontent.com/u/1"),
			})
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	first, err := suite.client.GetOrgMembers()
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), first, 1)

	second, err := suite.client.GetOrgMembers()
	assert.NoError(suite.T(), err)
	require.Len(suite.T(), second, 1)

	assert.Equal(suite.T(), 1, orgMemberListCalls)
	require.Len(suite.T(), logs, 3)
	assert.Contains(suite.T(), logs[0], "Fetching GitHub organization members for testowner")
	assert.Contains(suite.T(), logs[1], "Fetched 1 GitHub organization members for testowner")
	assert.Contains(suite.T(), logs[2], "Loaded 1 GitHub organization members for testowner from cache")
}

// TestRequestReviewers tests requesting reviewers on a pull request
func (suite *GithubTestSuite) TestRequestReviewers() {
	tests := []struct {
		name           string
		prNumber       int
		reviewers      []string
		teamReviewers  []string
		mockResponse   map[string]interface{}
		mockStatusCode int
		wantErr        bool
	}{
		{
			name:      "success - request single reviewer",
			prNumber:  1,
			reviewers: []string{"reviewer1"},
			mockResponse: map[string]interface{}{
				"requested_reviewers": []map[string]interface{}{
					mockUser("reviewer1", "Reviewer One", "https://avatars.githubusercontent.com/u/1"),
				},
			},
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
		},
		{
			name:      "success - request multiple reviewers",
			prNumber:  1,
			reviewers: []string{"reviewer1", "reviewer2"},
			mockResponse: map[string]interface{}{
				"requested_reviewers": []map[string]interface{}{
					mockUser("reviewer1", "Reviewer One", "https://avatars.githubusercontent.com/u/1"),
					mockUser("reviewer2", "Reviewer Two", "https://avatars.githubusercontent.com/u/2"),
				},
			},
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
		},
		{
			name:          "success - request team reviewers",
			prNumber:      1,
			teamReviewers: []string{"platform"},
			mockResponse: map[string]interface{}{
				"requested_teams": []map[string]interface{}{
					{
						"slug": "platform",
						"name": "Platform",
						"organization": map[string]interface{}{
							"login": "testowner",
						},
					},
				},
			},
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
		},
		{
			name:      "error - pull request not found",
			prNumber:  999,
			reviewers: []string{"reviewer1"},
			mockResponse: map[string]interface{}{
				"message": "Not Found",
			},
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
		},
		{
			name:      "error - invalid reviewer",
			prNumber:  1,
			reviewers: []string{"invalid-user"},
			mockResponse: map[string]interface{}{
				"message": "Reviews may only be requested from collaborators",
			},
			mockStatusCode: http.StatusUnprocessableEntity,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/api/v3/repos/testowner/testrepo/pulls/%d/requested_reviewers", tt.prNumber)
				if r.URL.Path == expectedPath && r.Method == http.MethodPost {
					w.WriteHeader(tt.mockStatusCode)
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			err := suite.client.RequestReviewers(tt.prNumber, tt.reviewers, tt.teamReviewers)

			if tt.wantErr {
				assert.Error(suite.T(), err)
			} else {
				assert.NoError(suite.T(), err)
			}
		})
	}
}

// TestDoesPRExistForBranch tests checking if a PR exists for a branch
func (suite *GithubTestSuite) TestDoesPRExistForBranch() {
	tests := []struct {
		name           string
		branch         string
		mockResponse   []map[string]interface{}
		mockStatusCode int
		wantErr        bool
		wantExists     bool
		wantPR         *PullRequest
	}{
		{
			name:   "success - PR exists",
			branch: "feature-branch",
			mockResponse: []map[string]interface{}{
				mockPullRequest(1, "Feature PR", "https://api.github.com/repos/testowner/testrepo/pulls/1", "https://github.com/testowner/testrepo/pull/1", "feature-branch", "main", "open"),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantExists:     true,
			wantPR: &PullRequest{
				Number:  1,
				URL:     "https://api.github.com/repos/testowner/testrepo/pulls/1",
				HTMLURL: "https://github.com/testowner/testrepo/pull/1",
			},
		},
		{
			name:           "success - no PR exists",
			branch:         "no-pr-branch",
			mockResponse:   []map[string]interface{}{},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantExists:     false,
			wantPR:         nil,
		},
		{
			name:           "error - API error",
			branch:         "error-branch",
			mockResponse:   nil,
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
			wantExists:     false,
			wantPR:         nil,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v3/repos/testowner/testrepo/pulls" && r.Method == http.MethodGet {
					// Verify query parameters
					query := r.URL.Query()
					expectedHead := fmt.Sprintf("testowner:%s", tt.branch)
					if query.Get("head") == expectedHead && query.Get("state") == "open" {
						w.WriteHeader(tt.mockStatusCode)
						if tt.mockResponse != nil {
							_ = json.NewEncoder(w).Encode(tt.mockResponse)
						} else {
							_ = json.NewEncoder(w).Encode(map[string]string{"message": "error"})
						}
						return
					}
				}
				w.WriteHeader(http.StatusNotFound)
			})

			exists, pr, err := suite.client.DoesPRExistForBranch(tt.branch)

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.False(suite.T(), exists)
				assert.Nil(suite.T(), pr)
			} else {
				assert.NoError(suite.T(), err)
				assert.Equal(suite.T(), tt.wantExists, exists)
				if tt.wantPR != nil {
					assert.NotNil(suite.T(), pr)
					assert.Equal(suite.T(), tt.wantPR.Number, pr.Number)
					assert.Equal(suite.T(), tt.wantPR.URL, pr.URL)
					assert.Equal(suite.T(), tt.wantPR.HTMLURL, pr.HTMLURL)
				} else {
					assert.Nil(suite.T(), pr)
				}
			}
		})
	}
}

// TestCreateRelease tests creating a GitHub release
func (suite *GithubTestSuite) TestCreateRelease() {
	tests := []struct {
		name           string
		opts           CreateReleaseOptions
		mockResponse   map[string]interface{}
		mockStatusCode int
		wantErr        bool
		wantRelease    *Release
	}{
		{
			name: "success - create release",
			opts: CreateReleaseOptions{
				TagName:         "v1.0.0",
				TargetCommitish: "main",
				Name:            "Release v1.0.0",
				Body:            "First release",
				Draft:           false,
				Prerelease:      false,
			},
			mockResponse:   mockRelease(1, "v1.0.0", "Release v1.0.0", "https://api.github.com/repos/testowner/testrepo/releases/1", "https://github.com/testowner/testrepo/releases/tag/v1.0.0"),
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
			wantRelease: &Release{
				ID:      1,
				TagName: "v1.0.0",
				Name:    "Release v1.0.0",
				URL:     "https://api.github.com/repos/testowner/testrepo/releases/1",
				HTMLURL: "https://github.com/testowner/testrepo/releases/tag/v1.0.0",
			},
		},
		{
			name: "success - create draft release",
			opts: CreateReleaseOptions{
				TagName:         "v2.0.0-beta",
				TargetCommitish: "develop",
				Name:            "Release v2.0.0-beta",
				Body:            "Beta release",
				Draft:           true,
				Prerelease:      true,
			},
			mockResponse:   mockRelease(2, "v2.0.0-beta", "Release v2.0.0-beta", "https://api.github.com/repos/testowner/testrepo/releases/2", "https://github.com/testowner/testrepo/releases/tag/v2.0.0-beta"),
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
			wantRelease: &Release{
				ID:      2,
				TagName: "v2.0.0-beta",
				Name:    "Release v2.0.0-beta",
				URL:     "https://api.github.com/repos/testowner/testrepo/releases/2",
				HTMLURL: "https://github.com/testowner/testrepo/releases/tag/v2.0.0-beta",
			},
		},
		{
			name: "error - tag already exists",
			opts: CreateReleaseOptions{
				TagName:         "v1.0.0",
				TargetCommitish: "main",
				Name:            "Release v1.0.0",
				Body:            "Duplicate release",
			},
			mockResponse: map[string]interface{}{
				"message": "Validation Failed",
				"errors": []map[string]interface{}{
					{"code": "already_exists", "field": "tag_name"},
				},
			},
			mockStatusCode: http.StatusUnprocessableEntity,
			wantErr:        true,
			wantRelease:    nil,
		},
		{
			name: "error - unauthorized",
			opts: CreateReleaseOptions{
				TagName: "v1.0.0",
				Name:    "Release v1.0.0",
			},
			mockResponse: map[string]interface{}{
				"message": "Bad credentials",
			},
			mockStatusCode: http.StatusUnauthorized,
			wantErr:        true,
			wantRelease:    nil,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v3/repos/testowner/testrepo/releases" && r.Method == http.MethodPost {
					w.WriteHeader(tt.mockStatusCode)
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			release, err := suite.client.CreateRelease(tt.opts)

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), release)
			} else {
				assert.NoError(suite.T(), err)
				assert.NotNil(suite.T(), release)
				assert.Equal(suite.T(), tt.wantRelease.ID, release.ID)
				assert.Equal(suite.T(), tt.wantRelease.TagName, release.TagName)
				assert.Equal(suite.T(), tt.wantRelease.Name, release.Name)
				assert.Equal(suite.T(), tt.wantRelease.URL, release.URL)
				assert.Equal(suite.T(), tt.wantRelease.HTMLURL, release.HTMLURL)
			}
		})
	}
}

// TestGetCurrentUser tests fetching the current authenticated user
func (suite *GithubTestSuite) TestGetCurrentUser() {
	tests := []struct {
		name           string
		mockResponse   map[string]interface{}
		mockStatusCode int
		wantErr        bool
		wantLogin      string
		wantName       string
	}{
		{
			name:           "success - get current user",
			mockResponse:   mockUser("testuser", "Test User", "https://avatars.githubusercontent.com/u/123"),
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantLogin:      "testuser",
			wantName:       "Test User",
		},
		{
			name: "error - unauthorized",
			mockResponse: map[string]interface{}{
				"message": "Bad credentials",
			},
			mockStatusCode: http.StatusUnauthorized,
			wantErr:        true,
		},
		{
			name: "error - server error",
			mockResponse: map[string]interface{}{
				"message": "Internal Server Error",
			},
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v3/user" && r.Method == http.MethodGet {
					w.WriteHeader(tt.mockStatusCode)
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			user, err := suite.client.GetCurrentUser()

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), user)
			} else {
				assert.NoError(suite.T(), err)
				assert.NotNil(suite.T(), user)
				assert.Equal(suite.T(), tt.wantLogin, user.GetLogin())
				assert.Equal(suite.T(), tt.wantName, user.GetName())
			}
		})
	}
}

// TestGetPullRequestUrl tests getting the PR URL for the current branch
func (suite *GithubTestSuite) TestGetPullRequestUrl() {
	tests := []struct {
		name           string
		currentBranch  string
		branchErr      error
		mockResponse   []map[string]interface{}
		mockStatusCode int
		wantErr        bool
		wantURL        string
	}{
		{
			name:          "success - PR exists for branch",
			currentBranch: "feature-branch",
			mockResponse: []map[string]interface{}{
				mockPullRequest(1, "Feature PR", "https://api.github.com/repos/testowner/testrepo/pulls/1", "https://github.com/testowner/testrepo/pull/1", "feature-branch", "main", "open"),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantURL:        "https://github.com/testowner/testrepo/pull/1",
		},
		{
			name:           "error - no PR for branch",
			currentBranch:  "no-pr-branch",
			mockResponse:   []map[string]interface{}{},
			mockStatusCode: http.StatusOK,
			wantErr:        true,
			wantURL:        "",
		},
		{
			name:          "error - git error getting branch",
			currentBranch: "",
			branchErr:     fmt.Errorf("not a git repository"),
			wantErr:       true,
			wantURL:       "",
		},
		{
			name:           "error - API error",
			currentBranch:  "feature-branch",
			mockResponse:   nil,
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
			wantURL:        "",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Set up mock git
			suite.client.git = &mockGit{
				currentBranch: tt.currentBranch,
				branchErr:     tt.branchErr,
			}

			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v3/repos/testowner/testrepo/pulls" && r.Method == http.MethodGet {
					expectedHead := fmt.Sprintf("testowner:%s", tt.currentBranch)
					if r.URL.Query().Get("head") == expectedHead {
						w.WriteHeader(tt.mockStatusCode)
						if tt.mockResponse != nil {
							_ = json.NewEncoder(w).Encode(tt.mockResponse)
						} else {
							_ = json.NewEncoder(w).Encode(map[string]string{"message": "error"})
						}
						return
					}
				}
				w.WriteHeader(http.StatusNotFound)
			})

			url, err := suite.client.GetPullRequestUrl()

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Empty(suite.T(), url)
			} else {
				assert.NoError(suite.T(), err)
				assert.Equal(suite.T(), tt.wantURL, url)
			}
		})
	}
}

func TestGithubSuite(t *testing.T) {
	suite.Run(t, new(GithubTestSuite))
}
