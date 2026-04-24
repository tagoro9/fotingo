package inspect

import (
	"fmt"
	"testing"

	giturl "github.com/kubescape/go-git-url"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/jira"
)

type inspectGitStub struct {
	currentBranch      string
	currentBranchErr   error
	currentBranchCalls int
	defaultBranch      string
	issueID            string
	issueErr           error

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
	s.currentBranchCalls++
	if s.currentBranchErr != nil {
		return "", s.currentBranchErr
	}
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

func (s *inspectGitStub) CreateIssueWorktreeBranch(*jira.Issue, git.WorktreeOptions) (string, string, error) {
	return "", "", fmt.Errorf("not implemented")
}

func (s *inspectGitStub) Push() error { return fmt.Errorf("not implemented") }

func (s *inspectGitStub) StashChanges(string) error { return fmt.Errorf("not implemented") }

func (s *inspectGitStub) PopStash() error { return fmt.Errorf("not implemented") }

func (s *inspectGitStub) HasUncommittedChanges() (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (s *inspectGitStub) HasStashableChanges() (bool, error) {
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
	pr            *github.PullRequest
	prExists      bool
	prErr         error
	discussion    *github.PullRequestDiscussion
	discussionErr error

	branchCalls []string
	prNumbers   []int
}

func (s *inspectGitHubStub) DoesPRExistForBranch(branch string) (bool, *github.PullRequest, error) {
	s.branchCalls = append(s.branchCalls, branch)
	if s.prErr != nil {
		return false, nil, s.prErr
	}
	return s.prExists, s.pr, nil
}

func (s *inspectGitHubStub) GetPullRequestDiscussion(prNumber int) (*github.PullRequestDiscussion, error) {
	s.prNumbers = append(s.prNumbers, prNumber)
	if s.discussionErr != nil {
		return nil, s.discussionErr
	}
	return s.discussion, nil
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
		prExists: true,
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
			NewGitHubClient: func(git.Git, *viper.Viper) (PullRequestInspector, error) { return ghStub, nil },
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
	assert.Equal(t, []string{"feature/FOTINGO-30"}, ghStub.branchCalls)
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
			NewGitHubClient: func(git.Git, *viper.Viper) (PullRequestInspector, error) {
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

func TestWorkflowRunnerRunPullRequest_UsesCurrentBranchPRDiscussion(t *testing.T) {
	gitStub := &inspectGitStub{currentBranch: "feature/TEST-123"}
	discussion := &github.PullRequestDiscussion{
		Comments: []github.PullRequestIssueComment{
			{ID: 101, Author: "alice", Body: "Looks good"},
		},
	}
	ghStub := &inspectGitHubStub{
		prExists: true,
		pr: &github.PullRequest{
			Title:   "Feature PR",
			Body:    "PR body",
			Number:  42,
			URL:     "https://api.github.com/repos/acme/repo/pulls/42",
			HTMLURL: "https://github.com/acme/repo/pull/42",
			State:   "open",
			Draft:   true,
		},
		discussion: discussion,
	}

	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) { return gitStub, nil },
			NewGitHubClient: func(git.Git, *viper.Viper) (PullRequestInspector, error) {
				return ghStub, nil
			},
		},
	}

	result, err := runner.RunPullRequest()

	require.NoError(t, err)
	require.NotNil(t, result.Branch)
	assert.Equal(t, "feature/TEST-123", result.Branch.Name)
	require.NotNil(t, result.PullRequest)
	assert.Equal(t, 42, result.PullRequest.Number)
	assert.Equal(t, "Feature PR", result.PullRequest.Title)
	assert.Equal(t, "PR body", result.PullRequest.Description)
	assert.Equal(t, "https://github.com/acme/repo/pull/42", result.PullRequest.URL)
	assert.Equal(t, "https://api.github.com/repos/acme/repo/pulls/42", result.PullRequest.APIURL)
	assert.Equal(t, "open", result.PullRequest.State)
	assert.True(t, result.PullRequest.Draft)
	assert.Same(t, discussion, result.Discussion)
	assert.Equal(t, []string{"feature/TEST-123"}, ghStub.branchCalls)
	assert.Equal(t, []int{42}, ghStub.prNumbers)
	assert.Equal(t, 1, gitStub.currentBranchCalls)
}

func TestWorkflowRunnerRunPullRequest_UsesExplicitBranch(t *testing.T) {
	gitStub := &inspectGitStub{currentBranch: "current-branch"}
	ghStub := &inspectGitHubStub{
		prExists: true,
		pr:       &github.PullRequest{Number: 7, Title: "Explicit branch PR"},
	}

	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{Branch: "feature/explicit"},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) { return gitStub, nil },
			NewGitHubClient: func(git.Git, *viper.Viper) (PullRequestInspector, error) {
				return ghStub, nil
			},
		},
	}

	result, err := runner.RunPullRequest()

	require.NoError(t, err)
	assert.Equal(t, "feature/explicit", result.Branch.Name)
	assert.Equal(t, []string{"feature/explicit"}, ghStub.branchCalls)
	assert.Equal(t, 0, gitStub.currentBranchCalls)
}

func TestWorkflowRunnerRunPullRequest_NoOpenPullRequest(t *testing.T) {
	gitStub := &inspectGitStub{currentBranch: "feature/missing"}
	ghStub := &inspectGitHubStub{}

	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) { return gitStub, nil },
			NewGitHubClient: func(git.Git, *viper.Viper) (PullRequestInspector, error) {
				return ghStub, nil
			},
		},
	}

	result, err := runner.RunPullRequest()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no open pull request found for branch feature/missing")
	assert.Nil(t, result.PullRequest)
	assert.Empty(t, ghStub.prNumbers)
}
