package inspect

import (
	"fmt"
	"testing"

	hub "github.com/google/go-github/v84/github"
	giturl "github.com/kubescape/go-git-url"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/auth"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/jira"
)

type inspectGitStub struct {
	currentBranch string
	defaultBranch string
	issueID       string
	issueErr      error

	commitsSinceDefault      []git.Commit
	commitsSinceDefaultErr   error
	commitsSinceDefaultCalls int

	commitsSinceCalls int
}

func (s *inspectGitStub) GetConfig() *viper.Viper { return viper.New() }

func (s *inspectGitStub) SaveConfig(string, any) error { return nil }

func (s *inspectGitStub) GetRemote() (giturl.IGitURL, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *inspectGitStub) GetCurrentBranch() (string, error) {
	return s.currentBranch, nil
}

func (s *inspectGitStub) GetIssueId() (string, error) {
	if s.issueErr != nil {
		return "", s.issueErr
	}
	return s.issueID, nil
}

func (s *inspectGitStub) CreateIssueBranch(*jira.Issue) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (s *inspectGitStub) CreateIssueWorktreeBranch(*jira.Issue) (string, string, error) {
	return "", "", fmt.Errorf("not implemented")
}

func (s *inspectGitStub) Push() error { return fmt.Errorf("not implemented") }

func (s *inspectGitStub) StashChanges(string) error { return fmt.Errorf("not implemented") }

func (s *inspectGitStub) PopStash() error { return fmt.Errorf("not implemented") }

func (s *inspectGitStub) HasUncommittedChanges() (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (s *inspectGitStub) GetCommitsSince(string) ([]git.Commit, error) {
	s.commitsSinceCalls++
	return nil, fmt.Errorf("unexpected call")
}

func (s *inspectGitStub) DoesBranchExistInRemote(string) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (s *inspectGitStub) GetDefaultBranch() (string, error) {
	return s.defaultBranch, nil
}

func (s *inspectGitStub) FetchDefaultBranch() error { return fmt.Errorf("not implemented") }

func (s *inspectGitStub) GetCommitsSinceDefaultBranch() ([]git.Commit, error) {
	s.commitsSinceDefaultCalls++
	if s.commitsSinceDefaultErr != nil {
		return nil, s.commitsSinceDefaultErr
	}
	return s.commitsSinceDefault, nil
}

func (s *inspectGitStub) GetIssuesFromCommits([]git.Commit) []string { return nil }

type inspectGitHubStub struct {
	exists bool
	pr     *github.PullRequest
	err    error
	calls  []string
}

func (s *inspectGitHubStub) Authenticate() (*auth.AccessToken, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) IsAuthenticated() bool { return true }

func (s *inspectGitHubStub) SetTokenStore(func() string, func(string) error) {}

func (s *inspectGitHubStub) GetConfig() *viper.Viper { return viper.New() }

func (s *inspectGitHubStub) GetConfigString(string) string { return "" }

func (s *inspectGitHubStub) SaveConfig(string, interface{}) error { return nil }

func (s *inspectGitHubStub) GetPullRequestUrl() (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) GetCurrentUser() (*hub.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) CreatePullRequest(github.CreatePROptions) (*github.PullRequest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) UpdatePullRequest(int, github.UpdatePROptions) (*github.PullRequest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) GetLabels() ([]github.Label, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) AddLabelsToPR(int, []string) error { return fmt.Errorf("not implemented") }

func (s *inspectGitHubStub) GetCollaborators() ([]github.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) GetOrgMembers() ([]github.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) GetTeams() ([]github.Team, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) RequestReviewers(int, []string, []string) error {
	return fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) RemoveReviewers(int, []string, []string) error {
	return fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) AssignUsersToPR(int, []string) error {
	return fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) RemoveAssigneesFromPR(int, []string) error {
	return fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) MarkPullRequestReadyForReview(string) error {
	return fmt.Errorf("not implemented")
}

func (s *inspectGitHubStub) DoesPRExistForBranch(branch string) (bool, *github.PullRequest, error) {
	s.calls = append(s.calls, branch)
	return s.exists, s.pr, s.err
}

func (s *inspectGitHubStub) CreateRelease(github.CreateReleaseOptions) (*github.Release, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestWorkflowRunnerRun_UsesDefaultBranchDivergenceCommits(t *testing.T) {
	gitStub := &inspectGitStub{
		currentBranch: "feature/TEST-123",
		defaultBranch: "main",
		issueErr:      fmt.Errorf("no issue in branch"),
		commitsSinceDefault: []git.Commit{
			{Hash: "abc123", Message: "feat: TEST-123 implement", Author: "dev"},
		},
	}

	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) { return gitStub, nil },
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return nil, fmt.Errorf("jira should not be initialized")
			},
			FetchBranchIssue: func(jira.Jira, string) (*jira.Issue, error) {
				return nil, fmt.Errorf("branch issue should not be fetched")
			},
		},
	}

	result, err := runner.Run()
	require.NoError(t, err)
	require.NotNil(t, result.Branch)
	assert.Equal(t, "feature/TEST-123", result.Branch.Name)
	assert.Len(t, result.Commits, 1)
	assert.Equal(t, 1, gitStub.commitsSinceDefaultCalls)
	assert.Equal(t, 0, gitStub.commitsSinceCalls)
}

func TestWorkflowRunnerRun_DefaultBranchSkipsCommitCollection(t *testing.T) {
	gitStub := &inspectGitStub{
		currentBranch: "main",
		defaultBranch: "main",
		issueErr:      fmt.Errorf("no issue in branch"),
	}

	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) { return gitStub, nil },
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return nil, fmt.Errorf("jira should not be initialized")
			},
			FetchBranchIssue: func(jira.Jira, string) (*jira.Issue, error) {
				return nil, fmt.Errorf("branch issue should not be fetched")
			},
		},
	}

	result, err := runner.Run()
	require.NoError(t, err)
	require.NotNil(t, result.Branch)
	assert.Equal(t, "main", result.Branch.Name)
	assert.Empty(t, result.Commits)
	assert.Equal(t, 0, gitStub.commitsSinceDefaultCalls)
	assert.Equal(t, 0, gitStub.commitsSinceCalls)
}

func TestWorkflowRunnerRun_IncludesPullRequestWhenBranchHasOne(t *testing.T) {
	gitStub := &inspectGitStub{
		currentBranch: "feature/FOTINGO-30",
		defaultBranch: "main",
		issueErr:      fmt.Errorf("no issue in branch"),
	}
	ghStub := &inspectGitHubStub{
		exists: true,
		pr: &github.PullRequest{
			Number:  42,
			Title:   "Inspect PR metadata",
			Body:    "PR body",
			HTMLURL: "https://github.com/testowner/testrepo/pull/42",
		},
	}

	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{},
		Deps: WorkflowDeps{
			NewGitClient:    func(*viper.Viper, *chan string) (git.Git, error) { return gitStub, nil },
			NewGitHubClient: func(git.Git, *viper.Viper) (github.Github, error) { return ghStub, nil },
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return nil, fmt.Errorf("jira should not be initialized")
			},
			FetchBranchIssue: func(jira.Jira, string) (*jira.Issue, error) {
				return nil, fmt.Errorf("branch issue should not be fetched")
			},
		},
	}

	result, err := runner.Run()
	require.NoError(t, err)
	require.NotNil(t, result.PullRequest)
	assert.Equal(t, 42, result.PullRequest.Number)
	assert.Equal(t, "Inspect PR metadata", result.PullRequest.Title)
	assert.Equal(t, "PR body", result.PullRequest.Description)
	assert.Equal(t, "https://github.com/testowner/testrepo/pull/42", result.PullRequest.URL)
	assert.Equal(t, []string{"feature/FOTINGO-30"}, ghStub.calls)
}

func TestWorkflowRunnerRun_GitHubLookupFailureDoesNotFailInspect(t *testing.T) {
	gitStub := &inspectGitStub{
		currentBranch: "feature/FOTINGO-30",
		defaultBranch: "main",
		issueErr:      fmt.Errorf("no issue in branch"),
	}

	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) { return gitStub, nil },
			NewGitHubClient: func(git.Git, *viper.Viper) (github.Github, error) {
				return nil, fmt.Errorf("github auth unavailable")
			},
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return nil, fmt.Errorf("jira should not be initialized")
			},
			FetchBranchIssue: func(jira.Jira, string) (*jira.Issue, error) {
				return nil, fmt.Errorf("branch issue should not be fetched")
			},
		},
	}

	result, err := runner.Run()
	require.NoError(t, err)
	require.NotNil(t, result.Branch)
	assert.Nil(t, result.PullRequest)
}
