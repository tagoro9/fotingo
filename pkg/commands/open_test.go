package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	hub "github.com/google/go-github/v84/github"
	giturl "github.com/kubescape/go-git-url"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/auth"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
	"github.com/tagoro9/fotingo/internal/ui"
)

// ---------------------------------------------------------------------------
// Mock Git implementation
// ---------------------------------------------------------------------------

type mockGit struct {
	currentBranch     string
	currentBranchErr  error
	issueId           string
	issueIdErr        error
	remoteURL         giturl.IGitURL
	remoteErr         error
	hasChanges        bool
	hasChangesErr     error
	stashErr          error
	stashCalls        int
	stashMessage      string
	popStashErr       error
	pushErr           error
	createBranchName  string
	createBranchErr   error
	createBranchCalls int
	defaultBranch     string
	defaultBranchErr  error
	fetchDefaultErr   error
	fetchDefaultCalls int
	commitsSince      []git.Commit
	commitsSinceErr   error
	branchExistRemote bool
	branchExistErr    error
	issuesFromCommits []string
}

func (m *mockGit) GetRemote() (giturl.IGitURL, error) {
	return m.remoteURL, m.remoteErr
}

func (m *mockGit) GetCurrentBranch() (string, error) {
	return m.currentBranch, m.currentBranchErr
}

func (m *mockGit) GetIssueId() (string, error) {
	return m.issueId, m.issueIdErr
}

func (m *mockGit) CreateIssueBranch(_ *jira.Issue) (string, error) {
	m.createBranchCalls++
	return m.createBranchName, m.createBranchErr
}

func (m *mockGit) CreateIssueWorktreeBranch(_ *jira.Issue) (string, string, error) {
	m.createBranchCalls++
	return m.createBranchName, "", m.createBranchErr
}

func (m *mockGit) Push() error {
	return m.pushErr
}

func (m *mockGit) StashChanges(message string) error {
	m.stashCalls++
	m.stashMessage = message
	return m.stashErr
}

func (m *mockGit) PopStash() error {
	return m.popStashErr
}

func (m *mockGit) HasUncommittedChanges() (bool, error) {
	return m.hasChanges, m.hasChangesErr
}

func (m *mockGit) GetCommitsSince(_ string) ([]git.Commit, error) {
	return m.commitsSince, m.commitsSinceErr
}

func (m *mockGit) DoesBranchExistInRemote(_ string) (bool, error) {
	return m.branchExistRemote, m.branchExistErr
}

func (m *mockGit) GetDefaultBranch() (string, error) {
	return m.defaultBranch, m.defaultBranchErr
}

func (m *mockGit) FetchDefaultBranch() error {
	m.fetchDefaultCalls++
	return m.fetchDefaultErr
}

func (m *mockGit) GetCommitsSinceDefaultBranch() ([]git.Commit, error) {
	return m.commitsSince, m.commitsSinceErr
}

func (m *mockGit) GetIssuesFromCommits(_ []git.Commit) []string {
	return m.issuesFromCommits
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

// ---------------------------------------------------------------------------
// Mock GitHub implementation
// ---------------------------------------------------------------------------

type mockGitHub struct {
	prURL                           string
	prURLErr                        error
	currentUser                     *hub.User
	currentUserErr                  error
	lastCreatePROptions             github.CreatePROptions
	lastUpdatePROptions             github.UpdatePROptions
	createPR                        *github.PullRequest
	createPRErr                     error
	updatePR                        *github.PullRequest
	updatePRErr                     error
	lastAddedLabels                 []string
	labels                          []github.Label
	labelsErr                       error
	addLabelsErr                    error
	lastRequestedReviewers          []string
	lastRequestedTeamReviewers      []string
	lastRemovedReviewers            []string
	lastRemovedTeamReviewers        []string
	lastAssignedUsers               []string
	lastRemovedAssignees            []string
	collaborators                   []github.User
	orgMembers                      []github.User
	teams                           []github.Team
	collaboratorsErr                error
	orgMembersErr                   error
	teamsErr                        error
	supportsOrganizationMetadata    *bool
	supportsOrganizationMetadataErr error
	requestReviewersErr             error
	assignUsersErr                  error
	removeReviewersErr              error
	removeAssigneesErr              error
	markReadyErr                    error
	markReadyNodeID                 string
	doesPRExist                     bool
	existingPR                      *github.PullRequest
	doesPRExistErr                  error
	createRelease                   *github.Release
	createReleaseErr                error
	calls                           []string
	metadataFetchInfoLogger         func(string)
}

func (m *mockGitHub) GetPullRequestUrl() (string, error) {
	return m.prURL, m.prURLErr
}

func (m *mockGitHub) GetCurrentUser() (*hub.User, error) {
	return m.currentUser, m.currentUserErr
}

func (m *mockGitHub) CreatePullRequest(options github.CreatePROptions) (*github.PullRequest, error) {
	m.calls = append(m.calls, "create_pr")
	m.lastCreatePROptions = options
	return m.createPR, m.createPRErr
}

func (m *mockGitHub) UpdatePullRequest(_ int, options github.UpdatePROptions) (*github.PullRequest, error) {
	m.calls = append(m.calls, "update_pr")
	m.lastUpdatePROptions = options
	return m.updatePR, m.updatePRErr
}

func (m *mockGitHub) GetLabels() ([]github.Label, error) {
	m.calls = append(m.calls, "get_labels")
	return m.labels, m.labelsErr
}

func (m *mockGitHub) AddLabelsToPR(_ int, labels []string) error {
	m.calls = append(m.calls, "add_labels")
	m.lastAddedLabels = append([]string{}, labels...)
	return m.addLabelsErr
}

func (m *mockGitHub) GetCollaborators() ([]github.User, error) {
	m.calls = append(m.calls, "get_collaborators")
	if m.metadataFetchInfoLogger != nil {
		m.metadataFetchInfoLogger("Loaded 1 GitHub repository collaborators for testowner/testrepo from cache in 1ms")
	}
	return m.collaborators, m.collaboratorsErr
}

func (m *mockGitHub) GetOrgMembers() ([]github.User, error) {
	m.calls = append(m.calls, "get_org_members")
	if m.metadataFetchInfoLogger != nil {
		m.metadataFetchInfoLogger("Loaded 1 GitHub organization members for testowner from cache in 1ms")
	}
	return m.orgMembers, m.orgMembersErr
}

func (m *mockGitHub) GetTeams() ([]github.Team, error) {
	m.calls = append(m.calls, "get_teams")
	if m.metadataFetchInfoLogger != nil {
		m.metadataFetchInfoLogger("Loaded 1 GitHub organization teams for testowner from cache in 1ms")
	}
	return m.teams, m.teamsErr
}

// SupportsOrganizationMetadata reports whether the mock repository owner should
// behave like a GitHub organization for participant lookups.
func (m *mockGitHub) SupportsOrganizationMetadata() (bool, error) {
	if m.supportsOrganizationMetadataErr != nil {
		return false, m.supportsOrganizationMetadataErr
	}
	if m.supportsOrganizationMetadata != nil {
		return *m.supportsOrganizationMetadata, nil
	}
	return true, nil
}

func (m *mockGitHub) SetMetadataFetchInfoLogger(logf func(string)) {
	m.metadataFetchInfoLogger = logf
}

func (m *mockGitHub) RequestReviewers(_ int, reviewers []string, teamReviewers []string) error {
	m.calls = append(m.calls, "request_reviewers")
	m.lastRequestedReviewers = append([]string{}, reviewers...)
	m.lastRequestedTeamReviewers = append([]string{}, teamReviewers...)
	return m.requestReviewersErr
}

func (m *mockGitHub) RemoveReviewers(_ int, reviewers []string, teamReviewers []string) error {
	m.calls = append(m.calls, "remove_reviewers")
	m.lastRemovedReviewers = append([]string{}, reviewers...)
	m.lastRemovedTeamReviewers = append([]string{}, teamReviewers...)
	return m.removeReviewersErr
}

func (m *mockGitHub) AssignUsersToPR(_ int, assignees []string) error {
	m.calls = append(m.calls, "assign_users")
	m.lastAssignedUsers = append([]string{}, assignees...)
	return m.assignUsersErr
}

func (m *mockGitHub) RemoveAssigneesFromPR(_ int, assignees []string) error {
	m.calls = append(m.calls, "remove_assignees")
	m.lastRemovedAssignees = append([]string{}, assignees...)
	return m.removeAssigneesErr
}

func (m *mockGitHub) MarkPullRequestReadyForReview(nodeID string) error {
	m.calls = append(m.calls, "mark_ready_for_review")
	m.markReadyNodeID = nodeID
	if m.existingPR != nil {
		m.existingPR.Draft = false
	}
	if m.updatePR != nil {
		m.updatePR.Draft = false
	}
	return m.markReadyErr
}

func (m *mockGitHub) DoesPRExistForBranch(_ string) (bool, *github.PullRequest, error) {
	return m.doesPRExist, m.existingPR, m.doesPRExistErr
}

func (m *mockGitHub) CreateRelease(_ github.CreateReleaseOptions) (*github.Release, error) {
	return m.createRelease, m.createReleaseErr
}

func (m *mockGitHub) Authenticate() (*auth.AccessToken, error) {
	return &auth.AccessToken{}, nil
}

func (m *mockGitHub) IsAuthenticated() bool {
	return true
}

func (m *mockGitHub) SetTokenStore(_ func() string, _ func(string) error) {}

func (m *mockGitHub) GetConfig() *viper.Viper {
	return nil
}

func (m *mockGitHub) GetConfigString(_ string) string {
	return ""
}

func (m *mockGitHub) SaveConfig(_ string, _ interface{}) error {
	return nil
}

// ---------------------------------------------------------------------------
// Mock Jira implementation
// ---------------------------------------------------------------------------

type mockJira struct {
	jiraIssue             *jira.Issue
	jiraIssueErr          error
	issueURL              string
	trackerIssue          *tracker.Issue
	trackerIssueErr       error
	getIssueFn            func(string) (*tracker.Issue, error)
	currentUser           *tracker.User
	currentUserErr        error
	userOpenIssues        []tracker.Issue
	userOpenIssuesErr     error
	searchIssues          []tracker.Issue
	searchIssuesErr       error
	searchIssuesProject   string
	searchIssuesQuery     string
	searchIssuesTypes     []tracker.IssueType
	searchIssuesLimit     int
	searchIssuesFn        func(projectKey string, query string, issueTypes []tracker.IssueType, limit int) ([]tracker.Issue, error)
	assignIssue           *tracker.Issue
	assignIssueErr        error
	assignIssueCalls      int
	assignIssueIssueID    string
	assignIssueUserID     string
	projectIssueTypes     []tracker.ProjectIssueType
	projectIssueTypesErr  error
	createIssue           *tracker.Issue
	createIssueErr        error
	createIssueInput      tracker.CreateIssueInput
	setIssueStatus        *tracker.Issue
	setIssueStatusErr     error
	setJiraIssueStatus    *jira.Issue
	setJiraIssueStatusErr error
	addCommentErr         error
	setJiraIssueStatusIDs []string
	addCommentIssueIDs    []string
	addCommentBodies      []string
	createRelease         *tracker.Release
	createReleaseErr      error
	setFixVersionErr      error
}

func (m *mockJira) Name() string { return "Jira" }

func (m *mockJira) GetCurrentUser() (*tracker.User, error) {
	return m.currentUser, m.currentUserErr
}

func (m *mockJira) GetUserOpenIssues() ([]tracker.Issue, error) {
	return m.userOpenIssues, m.userOpenIssuesErr
}

func (m *mockJira) SearchIssues(projectKey string, query string, issueTypes []tracker.IssueType, limit int) ([]tracker.Issue, error) {
	m.searchIssuesProject = projectKey
	m.searchIssuesQuery = query
	m.searchIssuesTypes = append([]tracker.IssueType{}, issueTypes...)
	m.searchIssuesLimit = limit
	if m.searchIssuesFn != nil {
		return m.searchIssuesFn(projectKey, query, issueTypes, limit)
	}
	return m.searchIssues, m.searchIssuesErr
}

func (m *mockJira) GetIssue(id string) (*tracker.Issue, error) {
	if m.getIssueFn != nil {
		return m.getIssueFn(id)
	}
	return m.trackerIssue, m.trackerIssueErr
}

func (m *mockJira) AssignIssue(issueID string, userID string) (*tracker.Issue, error) {
	m.assignIssueCalls++
	m.assignIssueIssueID = issueID
	m.assignIssueUserID = userID
	return m.assignIssue, m.assignIssueErr
}

func (m *mockJira) CreateIssue(input tracker.CreateIssueInput) (*tracker.Issue, error) {
	m.createIssueInput = input
	return m.createIssue, m.createIssueErr
}

func (m *mockJira) GetProjectIssueTypes(_ string) ([]tracker.ProjectIssueType, error) {
	return m.projectIssueTypes, m.projectIssueTypesErr
}

func (m *mockJira) SetIssueStatus(_ string, _ tracker.IssueStatus) (*tracker.Issue, error) {
	return m.setIssueStatus, m.setIssueStatusErr
}

func (m *mockJira) AddComment(issueID string, comment string) error {
	m.addCommentIssueIDs = append(m.addCommentIssueIDs, issueID)
	m.addCommentBodies = append(m.addCommentBodies, comment)
	return m.addCommentErr
}

func (m *mockJira) CreateRelease(_ tracker.CreateReleaseInput) (*tracker.Release, error) {
	return m.createRelease, m.createReleaseErr
}

func (m *mockJira) SetFixVersion(_ []string, _ *tracker.Release) error {
	return m.setFixVersionErr
}

func (m *mockJira) IsValidIssueID(id string) bool {
	return regexp.MustCompile(`^[A-Z][A-Z0-9_]+-\d+$`).MatchString(strings.ToUpper(strings.TrimSpace(id)))
}

func (m *mockJira) GetIssueURL(id string) string {
	if strings.Contains(m.issueURL, "%s") {
		return fmt.Sprintf(m.issueURL, id)
	}
	return m.issueURL
}

func (m *mockJira) Authenticate() (*auth.AccessToken, error) {
	return &auth.AccessToken{}, nil
}

func (m *mockJira) IsAuthenticated() bool {
	return true
}

func (m *mockJira) SetTokenStore(_ func() string, _ func(string) error) {}

func (m *mockJira) GetConfig() *viper.Viper {
	return nil
}

func (m *mockJira) GetConfigString(_ string) string {
	return ""
}

func (m *mockJira) SaveConfig(_ string, _ interface{}) error {
	return nil
}

func (m *mockJira) GetIssueUrl(issueId string) (string, error) {
	return m.issueURL, nil
}

func (m *mockJira) GetJiraIssue(issueId string) (*jira.Issue, error) {
	return m.jiraIssue, m.jiraIssueErr
}

func (m *mockJira) SetJiraIssueStatus(issueId string, status jira.IssueStatus) (*jira.Issue, error) {
	m.setJiraIssueStatusIDs = append(m.setJiraIssueStatusIDs, issueId)
	return m.setJiraIssueStatus, m.setJiraIssueStatusErr
}

// Ensure mock implements the auth.OauthService interface properly
var _ jira.Jira = (*mockJira)(nil)
var _ github.Github = (*mockGitHub)(nil)
var _ git.Git = (*mockGit)(nil)

// ---------------------------------------------------------------------------
// handleOpenPr tests
// ---------------------------------------------------------------------------

func TestHandleOpenPr_Success(t *testing.T) {
	t.Parallel()

	gh := &mockGitHub{
		prURL: "https://github.com/owner/repo/pull/42",
	}

	url, err := handleOpenPr(gh)
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", url)
}

func TestHandleOpenPr_NoPRFound(t *testing.T) {
	t.Parallel()

	gh := &mockGitHub{
		prURLErr: fmt.Errorf("no pull request found for branch feature-branch"),
	}

	url, err := handleOpenPr(gh)
	assert.Error(t, err)
	assert.Empty(t, url)
	assert.Contains(t, err.Error(), "no pull request found")
}

func TestHandleOpenPr_APIError(t *testing.T) {
	t.Parallel()

	gh := &mockGitHub{
		prURLErr: fmt.Errorf("API rate limit exceeded"),
	}

	url, err := handleOpenPr(gh)
	assert.Error(t, err)
	assert.Empty(t, url)
	assert.Contains(t, err.Error(), "rate limit")
}

func TestMapOpenPRError_KnownBranchMessageAddsGuidance(t *testing.T) {
	t.Parallel()

	original := errors.New("no pull request found for branch main")
	err := mapOpenPRError(original)

	require.Error(t, err)
	assert.ErrorIs(t, err, original)
	assert.Contains(t, err.Error(), "no pull request found for branch main")
	assert.Contains(t, err.Error(), "fotingo review -y")
	assert.Equal(t, fterrors.ExitGitHub, fterrors.GetExitCode(err))
}

func TestMapOpenPRError_UnknownPRFailureUsesDefaultWrapper(t *testing.T) {
	t.Parallel()

	original := errors.New("API rate limit exceeded")
	err := mapOpenPRError(original)

	require.Error(t, err)
	assert.ErrorIs(t, err, original)
	assert.Contains(t, err.Error(), "failed to get PR URL")
	assert.Equal(t, fterrors.ExitGitHub, fterrors.GetExitCode(err))
}

// ---------------------------------------------------------------------------
// handleOpenIssue tests
// ---------------------------------------------------------------------------

func TestHandleOpenIssue_Success(t *testing.T) {
	t.Parallel()

	g := &mockGit{
		currentBranch: "b/TEST-123_fix_login_bug",
		issueId:       "TEST-123",
	}

	j := &mockJira{
		jiraIssue: &jira.Issue{
			Key:     "TEST-123",
			Summary: "Fix login bug",
			Status:  "In Progress",
			Type:    "Bug",
		},
		issueURL: "https://jira.example.com/browse/TEST-123",
	}

	url, err := handleOpenIssue(g, j)
	require.NoError(t, err)
	assert.Equal(t, "https://jira.example.com/browse/TEST-123", url)
}

func TestHandleOpenIssue_GetBranchError(t *testing.T) {
	t.Parallel()

	g := &mockGit{
		currentBranchErr: fmt.Errorf("HEAD is not pointing to a branch"),
	}

	j := &mockJira{}

	_, err := handleOpenIssue(g, j)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HEAD is not pointing to a branch")
}

func TestHandleOpenIssue_NoLinkedIssueFound(t *testing.T) {
	t.Parallel()

	g := &mockGit{
		currentBranch: "main",
		issueIdErr:    fmt.Errorf("no issue id found in branch name: main"),
	}

	j := &mockJira{}

	_, err := handleOpenIssue(g, j)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no linked Jira issue found")
}

func TestHandleOpenIssue_GetJiraIssueError(t *testing.T) {
	t.Parallel()

	g := &mockGit{
		currentBranch: "b/NOPE-999_missing_issue",
		issueId:       "NOPE-999",
	}

	j := &mockJira{
		jiraIssueErr: fmt.Errorf("failed to get issue NOPE-999: issue not found"),
	}

	_, err := handleOpenIssue(g, j)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NOPE-999")
}

func TestHandleOpenIssue_FallsBackToCommitLinkedIssue(t *testing.T) {
	t.Parallel()

	g := &mockGit{
		currentBranch: "feature/worktree-cleanup",
		issueIdErr:    fmt.Errorf("no issue id found in branch name: feature/worktree-cleanup"),
		commitsSince: []git.Commit{
			{Message: "wire open issue fallback"},
		},
		issuesFromCommits: []string{"TEST-456"},
	}

	j := &mockJira{
		jiraIssue: &jira.Issue{
			Key:     "TEST-456",
			Summary: "Add user dashboard",
			Status:  "In Progress",
			Type:    "Story",
		},
		issueURL: "https://jira.example.com/browse/%s",
	}

	url, err := handleOpenIssue(g, j)
	require.NoError(t, err)
	assert.Equal(t, "https://jira.example.com/browse/TEST-456", url)
}

func TestHandleOpenIssue_AmbiguousNonInteractive(t *testing.T) {
	savedJSON := Global.JSON
	Global.JSON = true
	t.Cleanup(func() {
		Global.JSON = savedJSON
	})

	g := &mockGit{
		currentBranch: "feature/open-issue",
		issueIdErr:    fmt.Errorf("no issue id found in branch name: feature/open-issue"),
		commitsSince: []git.Commit{
			{Message: "FOTINGO-26"},
			{Message: "FOTINGO-31"},
		},
		issuesFromCommits: []string{"FOTINGO-26", "FOTINGO-31"},
	}

	j := &mockJira{}

	_, err := handleOpenIssue(g, j)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple linked Jira issues found")
	assert.Contains(t, err.Error(), "FOTINGO-26, FOTINGO-31")
}

func TestSelectOpenIssueID_UsesPickerSelection(t *testing.T) {
	savedSelectOne := openSelectOneFn
	openSelectOneFn = func(title string, items []ui.PickerItem) (*ui.PickerItem, error) {
		assert.Equal(t, localizer.T(i18n.OpenPickerIssueTitle), title)
		require.Len(t, items, 2)
		assert.Equal(t, "TEST-123", items[0].Label)
		assert.Equal(t, "Fix login bug | InProgress", items[0].Detail)
		return &items[1], nil
	}
	t.Cleanup(func() {
		openSelectOneFn = savedSelectOne
	})

	j := &mockJira{
		getIssueFn: func(id string) (*tracker.Issue, error) {
			switch id {
			case "TEST-123":
				return &tracker.Issue{
					Key:     "TEST-123",
					Summary: "Fix login bug",
					Status:  tracker.IssueStatusInProgress,
				}, nil
			case "TEST-456":
				return &tracker.Issue{
					Key:     "TEST-456",
					Summary: "Add user dashboard",
					Status:  tracker.IssueStatusInReview,
				}, nil
			default:
				return nil, fmt.Errorf("issue %s not found", id)
			}
		},
	}

	issueID, err := selectOpenIssueID([]string{"TEST-123", "TEST-456"}, j)
	require.NoError(t, err)
	assert.Equal(t, "TEST-456", issueID)
}

// ---------------------------------------------------------------------------
// handleOpenRepo tests
// ---------------------------------------------------------------------------

func TestHandleOpenRepo_Success(t *testing.T) {
	t.Parallel()

	remoteURL, err := giturl.NewGitURL("https://github.com/testowner/testrepo.git")
	require.NoError(t, err)

	g := &mockGit{
		remoteURL: remoteURL,
	}

	url, err := handleOpenRepo(g)
	require.NoError(t, err)
	assert.Contains(t, url, "github.com")
	assert.Contains(t, url, "testowner")
	assert.Contains(t, url, "testrepo")
}

func TestHandleOpenRepo_Error(t *testing.T) {
	t.Parallel()

	g := &mockGit{
		remoteErr: fmt.Errorf("remote origin not found"),
	}

	url, err := handleOpenRepo(g)
	assert.Error(t, err)
	assert.Empty(t, url)
	assert.Contains(t, err.Error(), "remote origin not found")
}

// ---------------------------------------------------------------------------
// handleOpenBranch tests
// ---------------------------------------------------------------------------

func TestHandleOpenBranch_Success(t *testing.T) {
	t.Parallel()

	remoteURL, err := giturl.NewGitURL("https://github.com/testowner/testrepo.git")
	require.NoError(t, err)

	g := &mockGit{
		remoteURL:     remoteURL,
		currentBranch: "feature/my-branch",
	}

	url, err := handleOpenBranch(g)
	require.NoError(t, err)
	assert.Contains(t, url, "github.com/testowner/testrepo/tree/feature/my-branch")
}

func TestHandleOpenBranch_RemoteError(t *testing.T) {
	t.Parallel()

	g := &mockGit{
		remoteErr: fmt.Errorf("remote not found"),
	}

	url, err := handleOpenBranch(g)
	assert.Error(t, err)
	assert.Empty(t, url)
}

func TestHandleOpenBranch_BranchError(t *testing.T) {
	t.Parallel()

	remoteURL, err := giturl.NewGitURL("https://github.com/testowner/testrepo.git")
	require.NoError(t, err)

	g := &mockGit{
		remoteURL:        remoteURL,
		currentBranchErr: fmt.Errorf("HEAD is detached"),
	}

	url, err := handleOpenBranch(g)
	assert.Error(t, err)
	assert.Empty(t, url)
}

func TestHandleOpenBranch_UnsupportedHost(t *testing.T) {
	t.Parallel()

	remoteURL, err := giturl.NewGitURL("https://gitlab.com/testowner/testrepo.git")
	require.NoError(t, err)

	g := &mockGit{
		remoteURL:     remoteURL,
		currentBranch: "feature/my-branch",
	}

	url, err := handleOpenBranch(g)
	assert.Error(t, err)
	assert.Empty(t, url)
	assert.Contains(t, err.Error(), "unsupported host")
}

// ---------------------------------------------------------------------------
// fetchIssueDetails tests (unit)
// ---------------------------------------------------------------------------

func TestFetchIssueDetails_AllFound(t *testing.T) {
	t.Parallel()

	j := &mockJira{
		trackerIssue: &tracker.Issue{
			Key:     "TEST-1",
			Summary: "Test issue",
			Type:    tracker.IssueTypeBug,
			Status:  tracker.IssueStatusInProgress,
		},
	}

	issues, err := fetchIssueDetails(j, []string{"TEST-1"})
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "TEST-1", issues[0].Key)
}

func TestFetchIssueDetails_SomeFail(t *testing.T) {
	t.Parallel()

	callCount := 0
	j := &mockJiraWithCallCount{
		issues: map[string]*tracker.Issue{
			"TEST-1": {Key: "TEST-1", Summary: "First", Type: tracker.IssueTypeBug},
		},
	}

	_ = callCount
	issues, err := fetchIssueDetails(j, []string{"TEST-1", "NOPE-999"})
	assert.Error(t, err) // last error from NOPE-999
	assert.Len(t, issues, 1)
	assert.Equal(t, "TEST-1", issues[0].Key)
}

func TestFetchIssueDetails_Empty(t *testing.T) {
	t.Parallel()

	j := &mockJira{}

	issues, err := fetchIssueDetails(j, []string{})
	require.NoError(t, err)
	assert.Nil(t, issues)
}

// mockJiraWithCallCount is a more sophisticated mock that returns different results per issue
type mockJiraWithCallCount struct {
	mockJira
	issues map[string]*tracker.Issue
}

func (m *mockJiraWithCallCount) GetIssue(id string) (*tracker.Issue, error) {
	if issue, ok := m.issues[id]; ok {
		return issue, nil
	}
	return nil, fmt.Errorf("issue %s not found", id)
}

// ---------------------------------------------------------------------------
// outputReviewJSON edge cases
// ---------------------------------------------------------------------------

func TestOutputReviewJSON_WithLabelsAndReviewers(t *testing.T) {
	result := reviewResult{
		pr: &github.PullRequest{
			Number:  10,
			HTMLURL: "https://github.com/o/r/pull/10",
		},
		issue: &jira.Issue{
			Key:     "TEST-1",
			Summary: "Test",
			Status:  "In Review",
			Type:    "Bug",
		},
		jiraURL:   "https://jira.example.com/browse/TEST-1",
		labels:    []string{"bug", "priority"},
		reviewers: []string{"alice", "bob"},
		existed:   false,
		err:       nil,
	}

	output := captureStdout(t, func() {
		err := outputReviewJSON(result)
		assert.NoError(t, err)
	})

	var decoded ReviewOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.Success)
	assert.Equal(t, 10, decoded.PullRequest.Number)
	assert.Equal(t, "TEST-1", decoded.Issue.Key)
	assert.Equal(t, "In Review", decoded.Issue.Status)
	assert.Equal(t, "https://jira.example.com/browse/TEST-1", decoded.Issue.URL)
	assert.Equal(t, []string{"bug", "priority"}, decoded.Labels)
	assert.Equal(t, []string{"alice", "bob"}, decoded.Reviewers)
	assert.False(t, decoded.Existed)
}

func TestOutputReviewJSON_ExistedWithPR(t *testing.T) {
	result := reviewResult{
		pr: &github.PullRequest{
			Number:  42,
			HTMLURL: "https://github.com/o/r/pull/42",
		},
		existed: true,
		err:     nil,
	}

	output := captureStdout(t, func() {
		err := outputReviewJSON(result)
		assert.NoError(t, err)
	})

	var decoded ReviewOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.Success)
	assert.True(t, decoded.Existed)
	assert.Equal(t, 42, decoded.PullRequest.Number)
	assert.Equal(t, "existing", decoded.PullRequest.State)
	assert.Nil(t, decoded.Issue)
	assert.Nil(t, decoded.Labels)
	assert.Nil(t, decoded.Reviewers)
}

func TestOutputReviewJSON_WithPRNoIssueNoLabels(t *testing.T) {
	result := reviewResult{
		pr: &github.PullRequest{
			Number:  5,
			HTMLURL: "https://github.com/o/r/pull/5",
		},
		err: nil,
	}

	output := captureStdout(t, func() {
		err := outputReviewJSON(result)
		assert.NoError(t, err)
	})

	var decoded ReviewOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.Success)
	assert.Equal(t, 5, decoded.PullRequest.Number)
	assert.Equal(t, "open", decoded.PullRequest.State)
	assert.Nil(t, decoded.Issue)
	assert.Nil(t, decoded.Labels)
	assert.Nil(t, decoded.Reviewers)
	assert.False(t, decoded.Existed)
}

// ---------------------------------------------------------------------------
// outputStartJSON edge cases
// ---------------------------------------------------------------------------

func TestOutputStartJSON_SuccessWithBranch(t *testing.T) {
	// Override newJiraClient to avoid real Jira connection
	origNewJiraClient := newJiraClient
	defer func() { newJiraClient = origNewJiraClient }()
	newJiraClient = func(cfg *viper.Viper) (jira.Jira, error) {
		return &mockJira{issueURL: "https://jira.example.com/browse/TEST-1"}, nil
	}

	result := startResult{
		issue: &jira.Issue{
			Key:     "TEST-1",
			Summary: "Do thing",
			Status:  "In Progress",
			Type:    "Story",
		},
		branchName: "f/TEST-1_do_thing",
		created:    true,
		err:        nil,
	}

	output := captureStdout(t, func() {
		err := outputStartJSON(result)
		assert.NoError(t, err)
	})

	var decoded StartOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.Success)
	assert.Equal(t, "TEST-1", decoded.Issue.Key)
	assert.Equal(t, "In Progress", decoded.Issue.Status)
	assert.Equal(t, "f/TEST-1_do_thing", decoded.Branch.Name)
	assert.True(t, decoded.Branch.Created)
}

func TestOutputStartJSON_SuccessNoBranch(t *testing.T) {
	// Override newJiraClient to avoid real Jira connection
	origNewJiraClient := newJiraClient
	defer func() { newJiraClient = origNewJiraClient }()
	newJiraClient = func(cfg *viper.Viper) (jira.Jira, error) {
		return &mockJira{issueURL: "https://jira.example.com/browse/TEST-2"}, nil
	}

	result := startResult{
		issue: &jira.Issue{
			Key:     "TEST-2",
			Summary: "No branch",
			Status:  "In Progress",
			Type:    "Task",
		},
		err: nil,
	}

	output := captureStdout(t, func() {
		err := outputStartJSON(result)
		assert.NoError(t, err)
	})

	var decoded StartOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.Success)
	assert.NotNil(t, decoded.Issue)
	assert.Nil(t, decoded.Branch)
}

// ---------------------------------------------------------------------------
// stashChanges tests
// ---------------------------------------------------------------------------

func TestStashChanges_Success(t *testing.T) {
	t.Parallel()
	setDefaultOutputFlags(t)

	g := &mockGit{}

	statusCh := make(chan string, 10)
	err := stashChanges(commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T), g)
	require.NoError(t, err)

	// Verify stash message was sent to channel
	close(statusCh)
	messages := []string{}
	for raw := range statusCh {
		event, ok := decodeStatusEvent(raw)
		require.True(t, ok)
		messages = append(messages, event.Message)
	}
	assert.Contains(t, messages, "Stashing uncommitted changes...")
}

func TestStashChanges_Error(t *testing.T) {
	t.Parallel()

	g := &mockGit{
		stashErr: fmt.Errorf("stash failed"),
	}

	statusCh := make(chan string, 10)
	err := stashChanges(commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T), g)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stash changes")
}

// ---------------------------------------------------------------------------
// createNewIssue tests (unit)
// ---------------------------------------------------------------------------

func TestCreateNewIssue_MissingProject(t *testing.T) {
	// Save and restore startCmdFlags
	origFlags := startCmdFlags
	defer func() { startCmdFlags = origFlags }()

	startCmdFlags = startFlags{
		title:   "Test title",
		project: "",
		kind:    "Task",
	}

	statusCh := make(chan string, 10)
	j := &mockJira{}

	issue, err := createNewIssue(commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T), j)
	assert.Error(t, err)
	assert.Nil(t, issue)
	assert.Contains(t, err.Error(), "--project")
}

func TestCreateNewIssue_InvalidKind(t *testing.T) {
	origFlags := startCmdFlags
	defer func() { startCmdFlags = origFlags }()

	startCmdFlags = startFlags{
		title:   "Test title",
		project: "TEST",
		kind:    "InvalidType",
	}

	statusCh := make(chan string, 10)
	j := &mockJira{}

	issue, err := createNewIssue(commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T), j)
	assert.Error(t, err)
	assert.Nil(t, issue)
	assert.Contains(t, err.Error(), "invalid issue type")
}

func TestCreateNewIssue_CreateFails(t *testing.T) {
	origFlags := startCmdFlags
	defer func() { startCmdFlags = origFlags }()

	startCmdFlags = startFlags{
		title:   "Test title",
		project: "TEST",
		kind:    "Task",
	}

	statusCh := make(chan string, 10)
	j := &mockJira{
		createIssueErr: fmt.Errorf("permission denied"),
	}

	issue, err := createNewIssue(commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T), j)
	assert.Error(t, err)
	assert.Nil(t, issue)
	assert.Contains(t, err.Error(), "failed to create issue")
}

func TestCreateNewIssue_Success(t *testing.T) {
	origFlags := startCmdFlags
	defer func() { startCmdFlags = origFlags }()

	startCmdFlags = startFlags{
		title:   "New feature",
		project: "TEST",
		kind:    "Story",
	}

	statusCh := make(chan string, 10)
	j := &mockJira{
		createIssue: &tracker.Issue{
			Key:     "TEST-999",
			Summary: "New feature",
			Type:    tracker.IssueTypeStory,
		},
		jiraIssue: &jira.Issue{
			Key:     "TEST-999",
			Summary: "New feature",
			Status:  "Backlog",
			Type:    "Story",
		},
	}

	issue, err := createNewIssue(commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T), j)
	require.NoError(t, err)
	assert.Equal(t, "TEST-999", issue.Key)
	assert.Equal(t, "New feature", issue.Summary)
}

func TestCreateNewIssue_WithParent(t *testing.T) {
	origFlags := startCmdFlags
	defer func() { startCmdFlags = origFlags }()

	startCmdFlags = startFlags{
		title:   "Sub task",
		project: "TEST",
		kind:    "Task",
		parent:  "TEST-100",
	}

	statusCh := make(chan string, 10)
	j := &mockJira{
		trackerIssue: &tracker.Issue{
			Key:  "TEST-100",
			Type: tracker.IssueTypeStory,
		},
		createIssue: &tracker.Issue{
			Key:     "TEST-101",
			Summary: "Sub task",
			Type:    tracker.IssueTypeSubTask,
		},
		jiraIssue: &jira.Issue{
			Key:     "TEST-101",
			Summary: "Sub task",
			Status:  "Backlog",
			Type:    "Sub-task",
		},
	}

	issue, err := createNewIssue(commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T), j)
	require.NoError(t, err)
	assert.Equal(t, "TEST-101", issue.Key)
}

func TestCreateNewIssue_GetJiraIssueFails(t *testing.T) {
	origFlags := startCmdFlags
	defer func() { startCmdFlags = origFlags }()

	startCmdFlags = startFlags{
		title:   "New feature",
		project: "TEST",
		kind:    "Task",
	}

	statusCh := make(chan string, 10)
	j := &mockJira{
		createIssue: &tracker.Issue{
			Key:     "TEST-999",
			Summary: "New feature",
		},
		jiraIssueErr: fmt.Errorf("failed to get created issue"),
	}

	issue, err := createNewIssue(commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T), j)
	assert.Error(t, err)
	assert.Nil(t, issue)
	assert.Contains(t, err.Error(), "failed to get created issue")
}

// ---------------------------------------------------------------------------
// buildPRBody additional tests
// ---------------------------------------------------------------------------

func TestBuildPRBody_WithJiraClient(t *testing.T) {
	withDefaultReviewTemplateResolver(t)

	issue := &jira.Issue{
		Key:         "PROJ-42",
		Summary:     "Add awesome feature",
		Description: "Implements the awesome feature.",
		Status:      "In Progress",
		Type:        "Story",
	}

	j := &mockJira{
		issueURL: "https://jira.example.com/browse/PROJ-42",
	}

	body := buildPRBody("f/PROJ-42_add_awesome_feature", issue, j)
	assert.Contains(t, body, "PROJ-42")
	assert.Contains(t, body, "Add awesome feature")
	assert.Contains(t, body, "https://jira.example.com/browse/PROJ-42")
	assert.Contains(t, body, "Fixes [PROJ-42](https://jira.example.com/browse/PROJ-42)")
	assert.Contains(t, body, "🚀 PR created with [fotingo](https://github.com/tagoro9/fotingo)")
}

// ---------------------------------------------------------------------------
// formatIssuesByCategory with jiraClient
// ---------------------------------------------------------------------------

func TestFormatIssuesByCategory_WithJiraClient(t *testing.T) {
	t.Parallel()

	issues := []*tracker.Issue{
		{Key: "TEST-1", Summary: "Bug fix", Type: tracker.IssueTypeBug},
	}

	j := &mockJira{
		issueURL: "https://jira.example.com/browse/TEST-1",
	}

	result := formatIssuesByCategory(issues, j)
	assert.Contains(t, result, "### Bug Fixes")
	assert.Contains(t, result, "TEST-1")
	assert.Contains(t, result, "Bug fix")
	assert.Contains(t, result, "https://jira.example.com/browse/TEST-1")
}

// ---------------------------------------------------------------------------
// buildPRBody edge cases
// ---------------------------------------------------------------------------

func TestBuildPRBody_EmptyBranch(t *testing.T) {
	withDefaultReviewTemplateResolver(t)

	origConfig := fotingoConfig
	defer func() { fotingoConfig = origConfig }()

	fotingoConfig = viper.New()

	body := buildPRBody("", nil, nil)
	assert.Contains(t, body, "**Description**")
	assert.Contains(t, body, "🚀 PR created with [fotingo](https://github.com/tagoro9/fotingo)")
}

func TestBuildPRBody_IssueWithoutJiraClient(t *testing.T) {
	withDefaultReviewTemplateResolver(t)

	origConfig := fotingoConfig
	defer func() { fotingoConfig = origConfig }()

	fotingoConfig = viper.New()

	issue := &jira.Issue{
		Key:         "PROJ-789",
		Summary:     "Test issue",
		Description: "Issue description",
	}

	body := buildPRBody("f/PROJ-789_test", issue, nil)
	assert.Contains(t, body, "PROJ-789")
	assert.Contains(t, body, "Test issue")
	assert.Contains(t, body, "Issue description")
	assert.Contains(t, body, "Fixes PROJ-789")
	assert.NotContains(t, body, "{fixedIssues}")
}

// ---------------------------------------------------------------------------
// contains helper edge cases
// ---------------------------------------------------------------------------

func TestContains_EmptyValue(t *testing.T) {
	t.Parallel()

	assert.True(t, contains([]string{"", "a", "b"}, ""))
	assert.False(t, contains([]string{"a", "b"}, ""))
}

// ---------------------------------------------------------------------------
// fetchIssueDetails additional coverage
// ---------------------------------------------------------------------------

func TestFetchIssueDetails_AllFail(t *testing.T) {
	t.Parallel()

	j := &mockJira{
		trackerIssueErr: fmt.Errorf("all issues failed"),
	}

	issues, err := fetchIssueDetails(j, []string{"TEST-1", "TEST-2"})
	assert.Error(t, err)
	assert.Empty(t, issues)
}

func TestFetchIssueDetails_MixedResults(t *testing.T) {
	t.Parallel()

	callCount := 0
	j := &mockJiraWithDynamicResponse{
		getIssue: func(id string) (*tracker.Issue, error) {
			callCount++
			if id == "TEST-1" {
				return &tracker.Issue{Key: "TEST-1", Summary: "First"}, nil
			}
			return nil, fmt.Errorf("issue %s not found", id)
		},
	}

	issues, err := fetchIssueDetails(j, []string{"TEST-1", "NOPE-999"})
	assert.Error(t, err, "should return last error")
	assert.Len(t, issues, 1)
	assert.Equal(t, "TEST-1", issues[0].Key)
}

// mockJiraWithDynamicResponse allows dynamic responses per call
type mockJiraWithDynamicResponse struct {
	mockJira
	getIssue func(string) (*tracker.Issue, error)
}

func (m *mockJiraWithDynamicResponse) GetIssue(id string) (*tracker.Issue, error) {
	if m.getIssue != nil {
		return m.getIssue(id)
	}
	return nil, fmt.Errorf("not implemented")
}

// ---------------------------------------------------------------------------
// parseIssueKind edge cases (for start command)
// ---------------------------------------------------------------------------

func TestParseIssueKind_AllValidTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected tracker.IssueType
	}{
		{"Story", tracker.IssueTypeStory},
		{"story", tracker.IssueTypeStory},
		{"STORY", tracker.IssueTypeStory},
		{"Bug", tracker.IssueTypeBug},
		{"bug", tracker.IssueTypeBug},
		{"Task", tracker.IssueTypeTask},
		{"task", tracker.IssueTypeTask},
		{"SubTask", tracker.IssueTypeSubTask},
		{"subtask", tracker.IssueTypeSubTask},
		{"sub-task", tracker.IssueTypeSubTask},
		{"Epic", tracker.IssueTypeEpic},
		{"epic", tracker.IssueTypeEpic},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseIssueKind(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseIssueKind_Invalid(t *testing.T) {
	t.Parallel()

	_, err := parseIssueKind("InvalidType")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid issue type")
}

// ---------------------------------------------------------------------------
// getStatusIndicator coverage
// ---------------------------------------------------------------------------

func TestGetStatusIndicator_AllStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status   tracker.IssueStatus
		expected string
	}{
		{tracker.IssueStatusBacklog, "(Backlog)"},
		{tracker.IssueStatusToDo, "(To Do)"},
		{tracker.IssueStatusInProgress, "(In Progress)"},
		{tracker.IssueStatusInReview, "(In Review)"},
		{tracker.IssueStatusDone, "(Done)"},
		{tracker.IssueStatus("Unknown"), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := getStatusIndicator(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// buildReleaseNotes coverage
// ---------------------------------------------------------------------------

func TestBuildReleaseNotes_WithCustomTemplate(t *testing.T) {
	t.Parallel()

	origConfig := fotingoConfig
	defer func() { fotingoConfig = origConfig }()

	cfg := viper.New()
	cfg.Set("github.releaseTemplate", "## Custom {version}\n\n{fixedIssuesByCategory}")
	fotingoConfig = cfg

	issues := []*tracker.Issue{
		{Key: "TEST-1", Summary: "Fix", Type: tracker.IssueTypeBug, URL: "https://example.com"},
	}

	notes := buildReleaseNotes("v2.0.0", issues, nil, nil)
	assert.Contains(t, notes, "## Custom v2.0.0")
	assert.Contains(t, notes, "TEST-1")
}

func TestBuildReleaseNotes_WithJiraRelease(t *testing.T) {
	t.Parallel()

	origConfig := fotingoConfig
	defer func() { fotingoConfig = origConfig }()

	fotingoConfig = viper.New()

	release := &tracker.Release{
		ID:   "release-1",
		Name: "v1.0.0",
		URL:  "https://jira.example.com/versions/1",
	}

	notes := buildReleaseNotes("v1.0.0", nil, release, nil)
	// The default template doesn't actually use jiraRelease placeholder,
	// but we test it doesn't error
	assert.Contains(t, notes, "v1.0.0")
}

// ---------------------------------------------------------------------------
// extractIssueIDsFromCommits additional edge cases
// ---------------------------------------------------------------------------

func TestExtractIssueIDsFromCommits_Deduplication(t *testing.T) {
	t.Parallel()

	commits := []git.Commit{
		{Message: "TEST-1 first mention"},
		{Message: "TEST-1 second mention"},
		{Message: "TEST-2 different issue"},
		{Message: "TEST-1 third mention"},
	}

	result := extractIssueIDsFromCommits(commits)
	assert.Len(t, result, 2, "should deduplicate issue IDs")
	assert.Contains(t, result, "TEST-1")
	assert.Contains(t, result, "TEST-2")
}

func TestExtractIssueIDsFromCommits_NoValidIDs(t *testing.T) {
	t.Parallel()

	commits := []git.Commit{
		{Message: "no issue id here"},
		{Message: "just a commit"},
	}

	result := extractIssueIDsFromCommits(commits)
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// stashChanges additional coverage
// ---------------------------------------------------------------------------

func TestStashChanges_SendsStatusMessages(t *testing.T) {
	setDefaultOutputFlags(t)

	g := &mockGit{}

	statusCh := make(chan string, 10)
	err := stashChanges(commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T), g)
	require.NoError(t, err)

	// Verify stash messages were sent to channel
	close(statusCh)
	messages := []string{}
	for raw := range statusCh {
		event, ok := decodeStatusEvent(raw)
		require.True(t, ok)
		messages = append(messages, event.Message)
	}
	assert.Contains(t, messages, "Stashing uncommitted changes...")
	assert.Len(t, messages, 2, "should send both stashing and restore messages")
}

// ---------------------------------------------------------------------------
// createNewIssue additional tests
// ---------------------------------------------------------------------------

func TestCreateNewIssue_WithLabels(t *testing.T) {
	origFlags := startCmdFlags
	defer func() { startCmdFlags = origFlags }()

	startCmdFlags = startFlags{
		title:   "Feature with labels",
		project: "TEST",
		kind:    "Story",
		labels:  []string{"frontend", "priority"},
	}

	statusCh := make(chan string, 10)
	j := &mockJira{
		createIssue: &tracker.Issue{
			Key:     "TEST-888",
			Summary: "Feature with labels",
			Type:    tracker.IssueTypeStory,
		},
		jiraIssue: &jira.Issue{
			Key:     "TEST-888",
			Summary: "Feature with labels",
			Status:  "Backlog",
			Type:    "Story",
		},
	}

	issue, err := createNewIssue(commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T), j)
	require.NoError(t, err)
	assert.Equal(t, "TEST-888", issue.Key)
}

// ---------------------------------------------------------------------------
// formatIssuesByCategory edge cases
// ---------------------------------------------------------------------------

func TestFormatIssuesByCategory_UnsortedInput(t *testing.T) {
	t.Parallel()

	issues := []*tracker.Issue{
		{Key: "PROJ-100", Summary: "Last", Type: tracker.IssueTypeBug},
		{Key: "PROJ-5", Summary: "Middle", Type: tracker.IssueTypeBug},
		{Key: "PROJ-1", Summary: "First", Type: tracker.IssueTypeBug},
	}

	result := formatIssuesByCategory(issues, nil)
	// Should sort by key
	assert.Contains(t, result, "PROJ-1")
	assert.Contains(t, result, "PROJ-5")
	assert.Contains(t, result, "PROJ-100")

	// Verify PROJ-1 appears before PROJ-100
	idx1 := strings.Index(result, "PROJ-1")
	idx100 := strings.Index(result, "PROJ-100")
	assert.Less(t, idx1, idx100)
}

func TestFormatIssuesByCategory_MixedTypesOrder(t *testing.T) {
	t.Parallel()

	issues := []*tracker.Issue{
		{Key: "TEST-5", Summary: "Task", Type: tracker.IssueTypeTask},
		{Key: "TEST-3", Summary: "Bug", Type: tracker.IssueTypeBug},
		{Key: "TEST-4", Summary: "Story", Type: tracker.IssueTypeStory},
	}

	result := formatIssuesByCategory(issues, nil)

	// Bug Fixes should come before Features which should come before Tasks
	idxBugSection := strings.Index(result, "### Bug Fixes")
	idxFeatureSection := strings.Index(result, "### Features")
	idxTaskSection := strings.Index(result, "### Tasks")

	assert.Less(t, idxBugSection, idxFeatureSection)
	assert.Less(t, idxFeatureSection, idxTaskSection)
}

func TestBuildPRBody_IgnoresConfiguredTemplateOverride(t *testing.T) {
	withDefaultReviewTemplateResolver(t)

	origConfig := fotingoConfig
	defer func() { fotingoConfig = origConfig }()

	cfg := viper.New()
	cfg.Set("github.pullRequestTemplate", "Custom PR: {issue.key}")
	fotingoConfig = cfg

	issue := &jira.Issue{
		Key:     "CUSTOM-1",
		Summary: "Custom",
	}

	body := buildPRBody("branch", issue, nil)
	assert.NotContains(t, body, "Custom PR: CUSTOM-1")
	assert.Contains(t, body, "Custom")
	assert.Contains(t, body, "**Description**")
}
