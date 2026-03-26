package github

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	giturl "github.com/kubescape/go-git-url"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tagoro9/fotingo/internal/github/testutil"
)

// mockGitWithRemote implements git.Git and returns a real IGitURL for GetRemote.
type mockGitWithRemote struct {
	mockGit
	remoteURL giturl.IGitURL
	remoteErr error
}

func (m *mockGitWithRemote) GetRemote() (giturl.IGitURL, error) {
	return m.remoteURL, m.remoteErr
}

// newMockGitWithRemote creates a mockGitWithRemote that returns a remote pointing
// to the given owner and repo.
func newMockGitWithRemote(owner, repo, currentBranch string) *mockGitWithRemote {
	u, _ := giturl.NewGitURL(fmt.Sprintf("https://github.com/%s/%s.git", owner, repo))
	return &mockGitWithRemote{
		mockGit: mockGit{
			currentBranch: currentBranch,
		},
		remoteURL: u,
	}
}

// MockServerTestSuite tests the GitHub client using testutil.MockGitHubServer
// and NewWithHTTPClient.
type MockServerTestSuite struct {
	suite.Suite
	server *testutil.MockGitHubServer
	client Github
	git    *mockGitWithRemote
}

func (suite *MockServerTestSuite) SetupTest() {
	suite.server = testutil.NewMockGitHubServer()
	suite.git = newMockGitWithRemote("testowner", "testrepo", "feature-branch")

	cfg := viper.New()
	cfg.Set("cache.path", filepath.Join(suite.T().TempDir(), "cache.db"))
	cfg.Set("github.cache.labelsTTL", "1h")
	cfg.Set("github.cache.collaboratorsTTL", "1h")
	cfg.Set("github.cache.orgMembersTTL", "1h")
	cfg.Set("github.cache.teamsTTL", "1h")
	client, err := NewWithHTTPClient(suite.git, cfg, &http.Client{}, suite.server.URL()+"/api/v3/")
	require.NoError(suite.T(), err)
	suite.client = client
}

func (suite *MockServerTestSuite) TearDownTest() {
	if suite.server != nil {
		suite.server.Close()
	}
}

// -----------------------------------------------------------------------
// NewWithHTTPClient
// -----------------------------------------------------------------------

func (suite *MockServerTestSuite) TestNewWithHTTPClient_Success() {
	cfg := viper.New()
	g := newMockGitWithRemote("owner", "repo", "main")
	client, err := NewWithHTTPClient(g, cfg, &http.Client{}, suite.server.URL()+"/api/v3/")
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), client)
}

func (suite *MockServerTestSuite) TestNewWithHTTPClient_RemoteError() {
	cfg := viper.New()
	g := &mockGitWithRemote{
		remoteErr: fmt.Errorf("no remote configured"),
	}
	client, err := NewWithHTTPClient(g, cfg, &http.Client{}, suite.server.URL()+"/api/v3/")
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), client)
	assert.Contains(suite.T(), err.Error(), "no remote configured")
}

func (suite *MockServerTestSuite) TestNewWithHTTPClient_InvalidBaseURL() {
	cfg := viper.New()
	g := newMockGitWithRemote("owner", "repo", "main")
	client, err := NewWithHTTPClient(g, cfg, &http.Client{}, "://invalid-url")
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), client)
}

// -----------------------------------------------------------------------
// GetCurrentUser
// -----------------------------------------------------------------------

func (suite *MockServerTestSuite) TestGetCurrentUser_Success() {
	suite.server.SetCurrentUser(testutil.DefaultUser())

	user, err := suite.client.GetCurrentUser()

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), user)
	assert.Equal(suite.T(), "testuser", user.GetLogin())
	assert.Equal(suite.T(), "Test User", user.GetName())
}

func (suite *MockServerTestSuite) TestGetCurrentUser_Unauthorized() {
	// No user configured means 401
	user, err := suite.client.GetCurrentUser()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), user)
}

func (suite *MockServerTestSuite) TestGetCurrentUser_ServerError() {
	suite.server.SetErrorResponse("GET /user", &testutil.ErrorResponse{
		StatusCode: http.StatusInternalServerError,
		Message:    "Internal Server Error",
	})

	user, err := suite.client.GetCurrentUser()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), user)
}

// -----------------------------------------------------------------------
// GetPullRequestUrl
// -----------------------------------------------------------------------

func (suite *MockServerTestSuite) TestGetPullRequestUrl_Success() {
	suite.server.AddPullRequest("testowner", "testrepo",
		testutil.NewPullRequest(1, "Feature PR", "feature-branch", "main", "open"),
	)

	url, err := suite.client.GetPullRequestUrl()

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "https://github.com/testowner/testrepo/pull/1", url)
}

func (suite *MockServerTestSuite) TestGetPullRequestUrl_NoPRForBranch() {
	// No PRs configured
	url, err := suite.client.GetPullRequestUrl()

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), url)
	assert.Contains(suite.T(), err.Error(), "no pull request found for branch")
}

func (suite *MockServerTestSuite) TestGetPullRequestUrl_GitError() {
	suite.git.branchErr = fmt.Errorf("not a git repository")

	url, err := suite.client.GetPullRequestUrl()

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), url)
	assert.Contains(suite.T(), err.Error(), "not a git repository")
}

func (suite *MockServerTestSuite) TestGetPullRequestUrl_APIError() {
	suite.server.SetErrorResponse("GET /repos/testowner/testrepo/pulls", &testutil.ErrorResponse{
		StatusCode: http.StatusInternalServerError,
		Message:    "Internal Server Error",
	})

	url, err := suite.client.GetPullRequestUrl()

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), url)
}

// -----------------------------------------------------------------------
// CreatePullRequest
// -----------------------------------------------------------------------

func (suite *MockServerTestSuite) TestCreatePullRequest_Success() {
	suite.server.SetCurrentUser(testutil.DefaultUser())

	pr, err := suite.client.CreatePullRequest(CreatePROptions{
		Title: "New Feature",
		Body:  "Feature description",
		Head:  "feature-branch",
		Base:  "main",
		Draft: false,
	})

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), pr)
	assert.Equal(suite.T(), 1, pr.Number)
	assert.Contains(suite.T(), pr.HTMLURL, "pull/1")
}

func (suite *MockServerTestSuite) TestCreatePullRequest_Draft() {
	suite.server.SetCurrentUser(testutil.DefaultUser())

	pr, err := suite.client.CreatePullRequest(CreatePROptions{
		Title: "Draft Feature",
		Body:  "Work in progress",
		Head:  "draft-branch",
		Base:  "main",
		Draft: true,
	})

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), pr)
	assert.Equal(suite.T(), 1, pr.Number)
}

func (suite *MockServerTestSuite) TestCreatePullRequest_Duplicate() {
	suite.server.SetCurrentUser(testutil.DefaultUser())
	suite.server.AddPullRequest("testowner", "testrepo",
		testutil.NewPullRequest(1, "Existing PR", "feature-branch", "main", "open"),
	)

	pr, err := suite.client.CreatePullRequest(CreatePROptions{
		Title: "Duplicate PR",
		Body:  "Should fail",
		Head:  "feature-branch",
		Base:  "main",
	})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), pr)
}

func (suite *MockServerTestSuite) TestCreatePullRequest_APIError() {
	suite.server.SetErrorResponse("POST /repos/testowner/testrepo/pulls", &testutil.ErrorResponse{
		StatusCode: http.StatusUnauthorized,
		Message:    "Bad credentials",
	})

	pr, err := suite.client.CreatePullRequest(CreatePROptions{
		Title: "Test PR",
		Head:  "feature-branch",
		Base:  "main",
	})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), pr)
}

// -----------------------------------------------------------------------
// GetLabels
// -----------------------------------------------------------------------

func (suite *MockServerTestSuite) TestGetLabels_Success() {
	suite.server.AddLabels("testowner", "testrepo",
		testutil.NewLabel(1, "bug", "Something is broken", "d73a4a"),
		testutil.NewLabel(2, "enhancement", "New feature", "a2eeef"),
		testutil.NewLabel(3, "documentation", "Docs update", "0075ca"),
	)

	labels, err := suite.client.GetLabels()

	assert.NoError(suite.T(), err)
	require.Len(suite.T(), labels, 3)
	assert.Equal(suite.T(), "bug", labels[0].Name)
	assert.Equal(suite.T(), "Something is broken", labels[0].Description)
	assert.Equal(suite.T(), "d73a4a", labels[0].Color)
	assert.Equal(suite.T(), "enhancement", labels[1].Name)
	assert.Equal(suite.T(), "documentation", labels[2].Name)
}

func (suite *MockServerTestSuite) TestGetLabels_Empty() {
	// No labels configured
	labels, err := suite.client.GetLabels()

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), labels)
}

func (suite *MockServerTestSuite) TestGetLabels_APIError() {
	suite.server.SetErrorResponse("GET /repos/testowner/testrepo/labels", &testutil.ErrorResponse{
		StatusCode: http.StatusInternalServerError,
		Message:    "Internal Server Error",
	})

	labels, err := suite.client.GetLabels()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), labels)
}

func (suite *MockServerTestSuite) TestGetLabels_CacheHitAvoidsAPICall() {
	suite.server.AddLabels("testowner", "testrepo",
		testutil.NewLabel(1, "bug", "Something is broken", "d73a4a"),
	)

	first, err := suite.client.GetLabels()
	require.NoError(suite.T(), err)
	require.Len(suite.T(), first, 1)
	assert.Equal(suite.T(), "bug", first[0].Name)

	initialCalls := countRequests(suite.server.GetRequestLog(), http.MethodGet, "/repos/testowner/testrepo/labels")
	assert.Equal(suite.T(), 1, initialCalls)

	// If cache is used, this modified server state should not affect the second call.
	suite.server.SetLabels("testowner", "testrepo", nil)

	second, err := suite.client.GetLabels()
	require.NoError(suite.T(), err)
	require.Len(suite.T(), second, 1)
	assert.Equal(suite.T(), "bug", second[0].Name)

	finalCalls := countRequests(suite.server.GetRequestLog(), http.MethodGet, "/repos/testowner/testrepo/labels")
	assert.Equal(suite.T(), 1, finalCalls)
}

// -----------------------------------------------------------------------
// AddLabelsToPR
// -----------------------------------------------------------------------

func (suite *MockServerTestSuite) TestAddLabelsToPR_APIErrorForbidden() {
	suite.server.SetErrorResponse("POST /repos/testowner/testrepo/issues/1/labels", &testutil.ErrorResponse{
		StatusCode: http.StatusForbidden,
		Message:    "Forbidden",
	})

	err := suite.client.AddLabelsToPR(1, []string{"bug"})

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to add labels")
}

func (suite *MockServerTestSuite) TestAddLabelsToPR_APIErrorNotFound() {
	suite.server.SetErrorResponse("POST /repos/testowner/testrepo/issues/999/labels", &testutil.ErrorResponse{
		StatusCode: http.StatusNotFound,
		Message:    "Not Found",
	})

	err := suite.client.AddLabelsToPR(999, []string{"bug"})

	assert.Error(suite.T(), err)
}

// -----------------------------------------------------------------------
// GetCollaborators
// -----------------------------------------------------------------------

func (suite *MockServerTestSuite) TestGetCollaborators_Success() {
	suite.server.AddCollaborators("testowner", "testrepo",
		testutil.NewUser(1, "alice", "Alice Developer"),
		testutil.NewUser(2, "bob", "Bob Reviewer"),
	)

	collaborators, err := suite.client.GetCollaborators()

	assert.NoError(suite.T(), err)
	require.Len(suite.T(), collaborators, 2)
	assert.Equal(suite.T(), "alice", collaborators[0].Login)
	assert.Equal(suite.T(), "Alice Developer", collaborators[0].Name)
	assert.Equal(suite.T(), "bob", collaborators[1].Login)
	assert.Equal(suite.T(), "Bob Reviewer", collaborators[1].Name)
}

func (suite *MockServerTestSuite) TestGetCollaborators_Empty() {
	// No collaborators configured
	collaborators, err := suite.client.GetCollaborators()

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), collaborators)
}

func (suite *MockServerTestSuite) TestGetCollaborators_APIError() {
	suite.server.SetErrorResponse("GET /repos/testowner/testrepo/collaborators", &testutil.ErrorResponse{
		StatusCode: http.StatusForbidden,
		Message:    "Forbidden",
	})

	collaborators, err := suite.client.GetCollaborators()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), collaborators)
}

func (suite *MockServerTestSuite) TestGetCollaborators_CacheMissRepopulates() {
	suite.server.AddCollaborators("testowner", "testrepo",
		testutil.NewUser(1, "alice", "Alice Developer"),
	)

	first, err := suite.client.GetCollaborators()
	require.NoError(suite.T(), err)
	require.Len(suite.T(), first, 1)
	assert.Equal(suite.T(), "alice", first[0].Login)

	initialCalls := countRequests(suite.server.GetRequestLog(), http.MethodGet, "/repos/testowner/testrepo/collaborators")
	assert.Equal(suite.T(), 1, initialCalls)

	internalClient := suite.client.(*github)
	require.NotNil(suite.T(), internalClient.metadataCache)
	require.NoError(suite.T(), internalClient.metadataCache.Delete(internalClient.metadataCacheKey("collaborators")))

	suite.server.SetCollaborators("testowner", "testrepo", []*testutil.MockUser{
		testutil.NewUser(1, "alice", "Alice Developer"),
		testutil.NewUser(2, "bob", "Bob Reviewer"),
	})

	second, err := suite.client.GetCollaborators()
	require.NoError(suite.T(), err)
	require.Len(suite.T(), second, 2)

	finalCalls := countRequests(suite.server.GetRequestLog(), http.MethodGet, "/repos/testowner/testrepo/collaborators")
	assert.Equal(suite.T(), 2, finalCalls)
}

func (suite *MockServerTestSuite) TestGetOrgMembers_Success() {
	suite.server.AddOrgMembers("testowner",
		testutil.NewUser(1, "member1", "Member One"),
		testutil.NewUser(2, "member2", "Member Two"),
	)

	members, err := suite.client.GetOrgMembers()

	assert.NoError(suite.T(), err)
	require.Len(suite.T(), members, 2)
	assert.Equal(suite.T(), "member1", members[0].Login)
	assert.Equal(suite.T(), "Member One", members[0].Name)
	assert.Equal(suite.T(), "member2", members[1].Login)
}

func (suite *MockServerTestSuite) TestGetTeams_Success() {
	suite.server.AddTeams("testowner",
		testutil.NewTeam(1, "testowner", "platform", "Platform", "Platform team"),
		testutil.NewTeam(2, "testowner", "frontend", "Frontend", "Frontend team"),
	)

	teams, err := suite.client.GetTeams()

	assert.NoError(suite.T(), err)
	require.Len(suite.T(), teams, 2)
	assert.Equal(suite.T(), "platform", teams[0].Slug)
	assert.Equal(suite.T(), "Platform", teams[0].Name)
	assert.Equal(suite.T(), "testowner/platform", teams[0].Canonical())
	assert.Equal(suite.T(), "frontend", teams[1].Slug)
}

func (suite *MockServerTestSuite) TestSupportsOrganizationMetadata_UserOwner() {
	repository := testutil.DefaultRepository()
	suite.server.AddRepository(repository)

	checker, ok := suite.client.(interface {
		SupportsOrganizationMetadata() (bool, error)
	})
	require.True(suite.T(), ok)

	supported, err := checker.SupportsOrganizationMetadata()

	require.NoError(suite.T(), err)
	assert.False(suite.T(), supported)
}

func (suite *MockServerTestSuite) TestSupportsOrganizationMetadata_OrganizationOwnerUsesCache() {
	repository := testutil.DefaultRepository()
	repository.Owner.Type = "Organization"
	suite.server.AddRepository(repository)

	checker, ok := suite.client.(interface {
		SupportsOrganizationMetadata() (bool, error)
	})
	require.True(suite.T(), ok)

	first, err := checker.SupportsOrganizationMetadata()
	require.NoError(suite.T(), err)
	assert.True(suite.T(), first)

	second, err := checker.SupportsOrganizationMetadata()
	require.NoError(suite.T(), err)
	assert.True(suite.T(), second)

	requests := countRequests(suite.server.GetRequestLog(), http.MethodGet, "/repos/testowner/testrepo")
	assert.Equal(suite.T(), 1, requests)
}

// -----------------------------------------------------------------------
// RequestReviewers
// -----------------------------------------------------------------------

func (suite *MockServerTestSuite) TestRequestReviewers_Success() {
	suite.server.AddPullRequest("testowner", "testrepo",
		testutil.NewPullRequest(1, "Test PR", "feature-branch", "main", "open"),
	)
	suite.server.AddCollaborators("testowner", "testrepo",
		testutil.NewUser(1, "reviewer1", "Reviewer One"),
		testutil.NewUser(2, "reviewer2", "Reviewer Two"),
	)

	err := suite.client.RequestReviewers(1, []string{"reviewer1", "reviewer2"}, nil)

	assert.NoError(suite.T(), err)
}

func (suite *MockServerTestSuite) TestRequestReviewers_PRNotFound() {
	err := suite.client.RequestReviewers(999, []string{"reviewer1"}, nil)

	assert.Error(suite.T(), err)
}

func (suite *MockServerTestSuite) TestRequestReviewers_InvalidReviewer() {
	suite.server.AddPullRequest("testowner", "testrepo",
		testutil.NewPullRequest(1, "Test PR", "feature-branch", "main", "open"),
	)
	// No collaborators configured, so the reviewer is not a collaborator

	err := suite.client.RequestReviewers(1, []string{"nonexistent-user"}, nil)

	assert.Error(suite.T(), err)
}

func (suite *MockServerTestSuite) TestRequestReviewers_APIError() {
	suite.server.SetErrorResponse("POST /repos/testowner/testrepo/pulls/1/requested_reviewers", &testutil.ErrorResponse{
		StatusCode: http.StatusInternalServerError,
		Message:    "Internal Server Error",
	})
	suite.server.AddPullRequest("testowner", "testrepo",
		testutil.NewPullRequest(1, "Test PR", "feature-branch", "main", "open"),
	)

	err := suite.client.RequestReviewers(1, []string{"reviewer1"}, nil)

	assert.Error(suite.T(), err)
}

func (suite *MockServerTestSuite) TestRequestReviewers_WithTeamReviewers() {
	suite.server.AddPullRequest("testowner", "testrepo",
		testutil.NewPullRequest(1, "Test PR", "feature-branch", "main", "open"),
	)

	err := suite.client.RequestReviewers(1, nil, []string{"platform"})

	assert.NoError(suite.T(), err)
}

func (suite *MockServerTestSuite) TestAssignUsersToPR_Success() {
	suite.server.AddPullRequest("testowner", "testrepo",
		testutil.NewPullRequest(1, "Test PR", "feature-branch", "main", "open"),
	)
	suite.server.AddCollaborators("testowner", "testrepo",
		testutil.NewUser(1, "alice", "Alice Developer"),
	)

	err := suite.client.AssignUsersToPR(1, []string{"alice"})

	assert.NoError(suite.T(), err)
}

// -----------------------------------------------------------------------
// DoesPRExistForBranch
// -----------------------------------------------------------------------

func (suite *MockServerTestSuite) TestDoesPRExistForBranch_Exists() {
	suite.server.AddPullRequest("testowner", "testrepo",
		testutil.NewPullRequest(1, "Feature PR", "feature-branch", "main", "open"),
	)

	exists, pr, err := suite.client.DoesPRExistForBranch("feature-branch")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
	require.NotNil(suite.T(), pr)
	assert.Equal(suite.T(), 1, pr.Number)
	assert.Contains(suite.T(), pr.HTMLURL, "pull/1")
}

func (suite *MockServerTestSuite) TestDoesPRExistForBranch_NotExists() {
	// No PRs configured
	exists, pr, err := suite.client.DoesPRExistForBranch("nonexistent-branch")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
	assert.Nil(suite.T(), pr)
}

func (suite *MockServerTestSuite) TestDoesPRExistForBranch_ClosedPR() {
	// Add a closed PR - should not match since DoesPRExistForBranch filters by state "open"
	suite.server.AddPullRequest("testowner", "testrepo", &testutil.MockPullRequest{
		ID:      1000,
		Number:  1,
		Title:   "Closed PR",
		State:   "closed",
		HTMLURL: "https://github.com/testowner/testrepo/pull/1",
		URL:     "https://api.github.com/repos/testowner/testrepo/pulls/1",
		Head:    testutil.MockPRRef{Ref: "closed-branch", SHA: "abc123"},
		Base:    testutil.MockPRRef{Ref: "main"},
	})

	exists, pr, err := suite.client.DoesPRExistForBranch("closed-branch")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
	assert.Nil(suite.T(), pr)
}

func (suite *MockServerTestSuite) TestDoesPRExistForBranch_APIError() {
	suite.server.SetErrorResponse("GET /repos/testowner/testrepo/pulls", &testutil.ErrorResponse{
		StatusCode: http.StatusInternalServerError,
		Message:    "Internal Server Error",
	})

	exists, pr, err := suite.client.DoesPRExistForBranch("feature-branch")

	assert.Error(suite.T(), err)
	assert.False(suite.T(), exists)
	assert.Nil(suite.T(), pr)
}

// -----------------------------------------------------------------------
// CreateRelease
// -----------------------------------------------------------------------

func (suite *MockServerTestSuite) TestCreateRelease_Success() {
	release, err := suite.client.CreateRelease(CreateReleaseOptions{
		TagName:         "v1.0.0",
		TargetCommitish: "main",
		Name:            "Release v1.0.0",
		Body:            "First release",
		Draft:           false,
		Prerelease:      false,
	})

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), release)
	assert.Equal(suite.T(), "v1.0.0", release.TagName)
	assert.Equal(suite.T(), "Release v1.0.0", release.Name)
	assert.Contains(suite.T(), release.HTMLURL, "v1.0.0")
}

func (suite *MockServerTestSuite) TestCreateRelease_DraftPrerelease() {
	release, err := suite.client.CreateRelease(CreateReleaseOptions{
		TagName:         "v2.0.0-beta",
		TargetCommitish: "develop",
		Name:            "Beta Release",
		Body:            "Beta notes",
		Draft:           true,
		Prerelease:      true,
	})

	assert.NoError(suite.T(), err)
	require.NotNil(suite.T(), release)
	assert.Equal(suite.T(), "v2.0.0-beta", release.TagName)
}

func (suite *MockServerTestSuite) TestCreateRelease_DuplicateTag() {
	suite.server.AddRelease("testowner", "testrepo",
		testutil.NewRelease(1, "v1.0.0", "Release v1.0.0", "main"),
	)

	release, err := suite.client.CreateRelease(CreateReleaseOptions{
		TagName:         "v1.0.0",
		TargetCommitish: "main",
		Name:            "Duplicate",
	})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), release)
}

func (suite *MockServerTestSuite) TestCreateRelease_APIError() {
	suite.server.SetErrorResponse("POST /repos/testowner/testrepo/releases", &testutil.ErrorResponse{
		StatusCode: http.StatusUnauthorized,
		Message:    "Bad credentials",
	})

	release, err := suite.client.CreateRelease(CreateReleaseOptions{
		TagName: "v1.0.0",
		Name:    "Release v1.0.0",
	})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), release)
}

// -----------------------------------------------------------------------
// Request logging verification
// -----------------------------------------------------------------------

func (suite *MockServerTestSuite) TestRequestLogRecordsRequests() {
	suite.server.SetCurrentUser(testutil.DefaultUser())

	_, _ = suite.client.GetCurrentUser()

	log := suite.server.GetRequestLog()
	require.NotEmpty(suite.T(), log)
	assert.Equal(suite.T(), http.MethodGet, log[0].Method)
	assert.Contains(suite.T(), log[0].Path, "user")
}

func TestMockServerSuite(t *testing.T) {
	suite.Run(t, new(MockServerTestSuite))
}

func countRequests(log []testutil.RecordedRequest, method string, pathPrefix string) int {
	count := 0
	for _, request := range log {
		normalizedPath := strings.TrimPrefix(request.Path, "/api/v3")
		if request.Method == method && strings.HasPrefix(normalizedPath, pathPrefix) {
			count++
		}
	}
	return count
}
