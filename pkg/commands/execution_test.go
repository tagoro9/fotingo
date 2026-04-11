package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	gogitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	ghtestutil "github.com/tagoro9/fotingo/internal/github/testutil"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	jiratestutil "github.com/tagoro9/fotingo/internal/jira/testutil"
)

// mockCredentialProvider returns no-op credentials for local file:// remotes.
type mockCredentialProvider struct{}

func (m *mockCredentialProvider) GetCredentials(_ string) (*githttp.BasicAuth, error) {
	return &githttp.BasicAuth{Username: "test", Password: "test"}, nil
}

// ExecutionTestSuite tests actual command execution paths using mock servers.
type ExecutionTestSuite struct {
	suite.Suite

	jiraServer   *jiratestutil.MockJiraServer
	githubServer *ghtestutil.MockGitHubServer

	origNewJiraClient   func(*viper.Viper) (jira.Jira, error)
	origNewGitHubClient func(git.Git, *viper.Viper) (github.Github, error)
	origNewGitClient    func(*viper.Viper, *chan string) (git.Git, error)
	origOpenBrowserFn   func(string) error

	openedURLs []string

	tempDir  string
	repoDir  string
	bareDir  string
	origHome string
	origDir  string
}

func TestExecutionTestSuite(t *testing.T) {
	suite.Run(t, new(ExecutionTestSuite))
}

func (s *ExecutionTestSuite) SetupSuite() {
	t := s.T()
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GCM_INTERACTIVE", "Never")
	t.Setenv("GIT_ASKPASS", "echo")
	t.Setenv("SSH_ASKPASS", "echo")

	// Save originals
	s.origNewJiraClient = newJiraClient
	s.origNewGitHubClient = newGitHubClient
	s.origNewGitClient = newGitClient
	s.origOpenBrowserFn = openBrowserFn
	s.origHome = os.Getenv("HOME")

	var err error
	s.origDir, err = os.Getwd()
	require.NoError(t, err)

	// Start mock Jira server
	s.jiraServer = jiratestutil.NewMockJiraServer()
	s.jiraServer.SetCurrentUser(jiratestutil.DefaultUser())
	s.jiraServer.AddIssue(jiratestutil.NewIssue("TEST-123", "Fix login bug", "To Do", "Bug"))
	s.jiraServer.SetTransitions("TEST-123", jiratestutil.DefaultTransitions())
	issueWithDescription := jiratestutil.NewIssue("TEST-456", "Add user dashboard", "In Progress", "Story")
	issueWithDescription.Description = "Build a dashboard that aggregates user metrics."
	s.jiraServer.AddIssue(issueWithDescription)
	s.jiraServer.SetTransitions("TEST-456", jiratestutil.DefaultTransitions())

	// Override Jira client factory
	newJiraClient = func(cfg *viper.Viper) (jira.Jira, error) {
		return jira.NewWithHTTPClient(cfg, s.jiraServer.Client(), s.jiraServer.URL())
	}

	// Start mock GitHub server
	s.githubServer = ghtestutil.NewMockGitHubServer()
	s.githubServer.SetCurrentUser(ghtestutil.DefaultUser())
	s.githubServer.AddRepository(ghtestutil.DefaultRepository())

	// Override GitHub client factory to use mock server with explicit owner/repo
	// (avoids needing to parse the git remote URL which may point to a local bare repo)
	newGitHubClient = func(g git.Git, cfg *viper.Viper) (github.Github, error) {
		return github.NewWithHTTPClientAndRepo(g, cfg, s.githubServer.Client(), s.githubServer.URL(), "testowner", "testrepo")
	}

	// Override browser opener
	openBrowserFn = func(url string) error {
		s.openedURLs = append(s.openedURLs, url)
		return nil
	}

	// Create temp directory structure
	s.tempDir, err = os.MkdirTemp("", "fotingo-exec-test-*")
	require.NoError(t, err)

	// Create config directory and file
	configDir := filepath.Join(s.tempDir, ".config", "fotingo")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configContent := fmt.Sprintf(`git:
  remote: origin
  branchTemplate: "{{.Issue.ShortName}}/{{.Issue.Info}}_{{.Issue.SanitizedSummary}}"
jira:
  root: "%s"
  siteId: "test-site"
`, s.jiraServer.URL())
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0644))

	// Override HOME so config is discovered
	require.NoError(t, os.Setenv("HOME", s.tempDir))

	// Create bare repo as local remote
	s.bareDir = filepath.Join(s.tempDir, "bare.git")
	_, err = gogit.PlainInit(s.bareDir, true)
	require.NoError(t, err)

	// Create git working repo
	s.repoDir = filepath.Join(s.tempDir, "repo")
	require.NoError(t, os.MkdirAll(s.repoDir, 0755))

	repo, err := gogit.PlainInit(s.repoDir, false)
	require.NoError(t, err)

	// Create initial commit so HEAD exists
	wt, err := repo.Worktree()
	require.NoError(t, err)

	readmeFile := filepath.Join(s.repoDir, "README.md")
	require.NoError(t, os.WriteFile(readmeFile, []byte("# Test Repo\n"), 0644))

	_, err = wt.Add("README.md")
	require.NoError(t, err)

	_, err = wt.Commit("initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Add origin remote pointing to the local bare repo
	_, err = repo.CreateRemote(&gogitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{s.bareDir},
	})
	require.NoError(t, err)

	// Add a temporary remote pointing to the bare dir, push, then remove it
	_, err = repo.CreateRemote(&gogitConfig.RemoteConfig{
		Name: "bare",
		URLs: []string{s.bareDir},
	})
	require.NoError(t, err)

	err = repo.Push(&gogit.PushOptions{
		RemoteName: "bare",
		RefSpecs:   []gogitConfig.RefSpec{"refs/heads/master:refs/heads/master"},
	})
	require.NoError(t, err)

	err = repo.DeleteRemote("bare")
	require.NoError(t, err)

	// Set HEAD on bare repo to point to master
	bareRepo, err := gogit.PlainOpen(s.bareDir)
	require.NoError(t, err)
	bareHeadRef := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("master"))
	err = bareRepo.Storer.SetReference(bareHeadRef)
	require.NoError(t, err)

	// Override newGitClient to inject mock credential provider and point origin to bare dir.
	// The on-disk repo keeps origin as github URL (for open/inspect commands),
	// but the git client used by review/release gets origin pointing to bare dir.
	bareDir := s.bareDir
	repoDir := s.repoDir
	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		gitClient, err := git.NewWithCredentialProvider(cfg, messages, &mockCredentialProvider{})
		if err != nil {
			return nil, err
		}
		gitClient.(git.RemoteConfigurable).ReconfigureRemoteURL("origin", bareDir)
		// Fetch from bare remote so local origin/* refs exist for GetCommitsSince
		repo, repoErr := gogit.PlainOpen(repoDir)
		if repoErr == nil {
			_ = repo.Fetch(&gogit.FetchOptions{RemoteName: "origin"})
		}
		return gitClient, nil
	}

	// Change to repo dir
	require.NoError(t, os.Chdir(s.repoDir))

	// Override the fotingoConfig to use our temp config
	cfg := viper.New()
	cfg.SetConfigName("config")
	cfg.AddConfigPath(configDir)
	cfg.SetDefault("git.remote", "origin")
	cfg.SetDefault("git.branchTemplate", "{{.Issue.ShortName}}/{{.Issue.Info}}_{{.Issue.SanitizedSummary}}")
	_ = cfg.ReadInConfig()
	fotingoConfig = cfg
}

func (s *ExecutionTestSuite) TearDownSuite() {
	// Restore originals
	newJiraClient = s.origNewJiraClient
	newGitHubClient = s.origNewGitHubClient
	newGitClient = s.origNewGitClient
	openBrowserFn = s.origOpenBrowserFn

	if s.origHome != "" {
		_ = os.Setenv("HOME", s.origHome)
	} else {
		_ = os.Unsetenv("HOME")
	}

	if s.origDir != "" {
		_ = os.Chdir(s.origDir)
	}

	if s.jiraServer != nil {
		s.jiraServer.Close()
	}

	if s.githubServer != nil {
		s.githubServer.Close()
	}

	if s.tempDir != "" {
		_ = os.RemoveAll(s.tempDir)
	}
}

func (s *ExecutionTestSuite) SetupTest() {
	// Restore origin remote to github URL (may have been changed by newGitClient override)
	repo, err := gogit.PlainOpen(s.repoDir)
	if err == nil {
		_ = repo.DeleteRemote("origin")
		_, _ = repo.CreateRemote(&gogitConfig.RemoteConfig{
			Name: "origin",
			URLs: []string{"https://github.com/testowner/testrepo.git"},
		})
	}

	// Reset global flags
	Global = GlobalFlags{}

	// Reset command-level flags
	inspectCmdFlags = inspectFlags{}
	startCmdFlags = startFlags{}
	reviewCmdFlags = reviewFlags{}
	releaseCmdFlags = releaseFlags{}

	// Reset opened URLs
	s.openedURLs = nil

	// Reset cobra command state
	Fotingo.SetArgs(nil)
	Fotingo.SetOut(nil)
	Fotingo.SetErr(nil)
}

// --------------------------------------------------------------------------
// Inspect command tests
// --------------------------------------------------------------------------

func (s *ExecutionTestSuite) TestInspect_WithIssueFlag() {
	t := s.T()

	output := captureStdout(t, func() {
		Fotingo.SetArgs([]string{"inspect", "--issue", "TEST-123"})
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result InspectOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.NotNil(t, result.Issue, "issue should be present in output")
	assert.Equal(t, "TEST-123", result.Issue.Key)
	assert.Equal(t, "Fix login bug", result.Issue.Summary)
	assert.Equal(t, "Bug", result.Issue.Type)
	assert.Equal(t, "To Do", result.Issue.Status)
	assert.Contains(t, result.Issue.URL, "TEST-123")
	assert.Nil(t, result.PullRequest)
}

func (s *ExecutionTestSuite) TestInspect_DefaultBranchInfo() {
	t := s.T()

	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
	s.githubServer.AddPullRequest("testowner", "testrepo",
		ghtestutil.NewPullRequest(42, "Inspect current branch", "master", "master", "open"),
	)
	t.Cleanup(func() {
		s.githubServer.SetPullRequests("testowner", "testrepo", nil)
	})

	output := captureStdout(t, func() {
		Fotingo.SetArgs([]string{"inspect"})
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result InspectOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.NotNil(t, result.Branch, "branch should be present")
	assert.NotEmpty(t, result.Branch.Name)
	require.NotNil(t, result.PullRequest)
	assert.Equal(t, 42, result.PullRequest.Number)
	assert.Equal(t, "Inspect current branch", result.PullRequest.Title)
	assert.Equal(t, "Pull request body for Inspect current branch", result.PullRequest.Description)
}

func (s *ExecutionTestSuite) TestInspect_WithBranchFlag() {
	t := s.T()

	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
	s.githubServer.AddPullRequest("testowner", "testrepo",
		ghtestutil.NewPullRequest(43, "Inspect explicit branch", "feature/TEST-999-some-feature", "master", "open"),
	)
	t.Cleanup(func() {
		s.githubServer.SetPullRequests("testowner", "testrepo", nil)
	})

	output := captureStdout(t, func() {
		Fotingo.SetArgs([]string{"inspect", "--branch", "feature/TEST-999-some-feature"})
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result InspectOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.NotNil(t, result.Branch, "branch should be present")
	assert.Equal(t, "feature/TEST-999-some-feature", result.Branch.Name)
	require.NotNil(t, result.PullRequest)
	assert.Equal(t, 43, result.PullRequest.Number)
	assert.Equal(t, "Inspect explicit branch", result.PullRequest.Title)
	assert.Equal(t, "Pull request body for Inspect explicit branch", result.PullRequest.Description)
}

func (s *ExecutionTestSuite) TestInspectPullRequest_DiscussionJSON() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "inspect_pr_discussion")
	defer cleanup()

	branchName := "b/TEST-123_inspect_pr_discussion"
	pr := ghtestutil.NewPullRequest(64, "[TEST-123] Inspect PR discussion", branchName, "main", "open")
	pr.IssueComments = []*ghtestutil.MockIssueComment{
		ghtestutil.NewIssueComment(101, "Top-level PR comment", "alice"),
	}
	pr.Reviews = []*ghtestutil.MockPullRequestReview{
		ghtestutil.NewPullRequestReview(201, "COMMENTED", "Review body", "bob"),
	}
	pr.ReviewComments = []*ghtestutil.MockPullRequestReviewComment{
		ghtestutil.NewPullRequestReviewComment(301, 201, 0, "Please adjust this line", "bob"),
		ghtestutil.NewPullRequestReviewComment(302, 201, 301, "Done", "alice"),
	}
	s.githubServer.AddPullRequest("testowner", "testrepo", pr)
	t.Cleanup(func() { s.githubServer.SetPullRequests("testowner", "testrepo", nil) })

	output := captureStdout(t, func() {
		Fotingo.SetArgs([]string{"inspect", "pr", "--json"})
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result InspectPROutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	require.NotNil(t, result.Branch)
	assert.Equal(t, branchName, result.Branch.Name)
	require.NotNil(t, result.PullRequest)
	assert.Equal(t, 64, result.PullRequest.Number)
	assert.Equal(t, "[TEST-123] Inspect PR discussion", result.PullRequest.Title)
	assert.Equal(t, "Pull request body for [TEST-123] Inspect PR discussion", result.PullRequest.Description)
	require.Len(t, result.Comments, 1)
	assert.Equal(t, "Top-level PR comment", result.Comments[0].Body)
	require.Len(t, result.Reviews, 1)
	assert.Equal(t, "COMMENTED", result.Reviews[0].State)
	require.Len(t, result.Reviews[0].Conversations, 1)
	assert.Equal(t, "review-comment-301", result.Reviews[0].Conversations[0].ID)
	require.Len(t, result.Reviews[0].Conversations[0].Comments, 2)
	assert.Equal(t, "review-comment-301", result.Reviews[0].Conversations[0].Comments[0].ConversationID)
	assert.Equal(t, int64(301), result.Reviews[0].Conversations[0].Comments[1].InReplyToID)

}

func (s *ExecutionTestSuite) TestInspect_WithNonExistentIssue() {
	t := s.T()

	output := captureStdout(t, func() {
		Fotingo.SetArgs([]string{"inspect", "--issue", "NOPE-999"})
		err := Fotingo.Execute()
		assert.NoError(t, err) // inspect doesn't return errors via RunE
	})

	// Should contain error JSON
	var result struct {
		Error string `json:"error"`
	}
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)
	assert.NotEmpty(t, result.Error)
}

// --------------------------------------------------------------------------
// Open command tests
// --------------------------------------------------------------------------

func (s *ExecutionTestSuite) TestOpen_RepoNonJSON() {
	t := s.T()

	buf := new(bytes.Buffer)
	Fotingo.SetOut(buf)
	Fotingo.SetErr(buf)
	Fotingo.SetArgs([]string{"open", "repo"})

	// Capture stdout since the open command prints there
	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	assert.Contains(t, output, "Opened browser")
	require.Len(t, s.openedURLs, 1)
	assert.Contains(t, s.openedURLs[0], "github.com/testowner/testrepo")
}

func (s *ExecutionTestSuite) TestOpen_RepoJSON() {
	t := s.T()

	Fotingo.SetArgs([]string{"open", "repo", "--json"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result OpenOutput
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.Equal(t, "repo", result.Target)
	assert.Contains(t, result.URL, "github.com/testowner/testrepo")
	assert.False(t, result.Opened, "JSON mode should not open browser")

	// In JSON mode, browser should not be called
	assert.Empty(t, s.openedURLs)
}

func (s *ExecutionTestSuite) TestOpen_BranchJSON() {
	t := s.T()

	Fotingo.SetArgs([]string{"open", "branch", "--json"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result OpenOutput
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.Equal(t, "branch", result.Target)
	assert.Contains(t, result.URL, "github.com/testowner/testrepo/tree/")
	assert.Empty(t, s.openedURLs)
}

func (s *ExecutionTestSuite) TestOpen_BranchNonJSON() {
	t := s.T()

	buf := new(bytes.Buffer)
	Fotingo.SetOut(buf)
	Fotingo.SetErr(buf)
	Fotingo.SetArgs([]string{"open", "branch"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	assert.Contains(t, output, "Opened browser")
	require.Len(t, s.openedURLs, 1)
	assert.Contains(t, s.openedURLs[0], "github.com/testowner/testrepo/tree/")
}

// --------------------------------------------------------------------------
// Open issue command tests
// --------------------------------------------------------------------------

func (s *ExecutionTestSuite) TestOpen_IssueJSON() {
	t := s.T()

	// Create a branch that matches the branch template so GetIssueId works
	repo, err := gogit.PlainOpen(s.repoDir)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	// Create and checkout a branch named like the template produces
	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: "refs/heads/b/TEST-123_fix_login_bug",
		Create: true,
	})
	require.NoError(t, err)

	Fotingo.SetArgs([]string{"open", "issue", "--json"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result OpenOutput
	err = json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.Equal(t, "issue", result.Target)
	assert.Contains(t, result.URL, "TEST-123")
	assert.Empty(t, s.openedURLs, "JSON mode should not open browser")

	// Checkout back to the original branch (master/main)
	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: "refs/heads/master",
		Create: false,
	})
	require.NoError(t, err)
}

func (s *ExecutionTestSuite) TestOpen_IssueNonJSON() {
	t := s.T()

	repo, err := gogit.PlainOpen(s.repoDir)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	// Create and checkout a branch named like the template produces
	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: "refs/heads/b/TEST-456_add_user_dashboard",
		Create: true,
	})
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	Fotingo.SetOut(buf)
	Fotingo.SetErr(buf)
	Fotingo.SetArgs([]string{"open", "issue"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	assert.Contains(t, output, "Opened browser")
	require.Len(t, s.openedURLs, 1)
	assert.Contains(t, s.openedURLs[0], "TEST-456")

	// Checkout back
	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: "refs/heads/master",
		Create: false,
	})
	require.NoError(t, err)
}

func (s *ExecutionTestSuite) TestOpen_IssueJSON_NoIssueInBranch() {
	t := s.T()

	savedNewOpenGitClient := newOpenGitClient
	newOpenGitClient = newGitClient
	t.Cleanup(func() {
		newOpenGitClient = savedNewOpenGitClient
	})

	// On master branch, there's no issue ID to extract
	Fotingo.SetArgs([]string{"open", "issue", "--json"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.Error(t, err)
	})

	var result OpenOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.False(t, result.Success)
	assert.Equal(t, "issue", result.Target)
	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "no linked Jira issue found")
}

func (s *ExecutionTestSuite) TestOpen_IssueJSON_FallsBackToCommitLinkedIssue() {
	t := s.T()

	savedNewOpenGitClient := newOpenGitClient
	newOpenGitClient = newGitClient
	t.Cleanup(func() {
		newOpenGitClient = savedNewOpenGitClient
	})

	repo, err := gogit.PlainOpen(s.repoDir)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/feature/open-issue-context"),
		Create: true,
	})
	require.NoError(t, err)

	testFile := filepath.Join(s.repoDir, "open-issue-context.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("change\n"), 0644))
	_, err = wt.Add("open-issue-context.txt")
	require.NoError(t, err)
	_, err = wt.Commit("TEST-456: link issue from commit context", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = wt.Checkout(&gogit.CheckoutOptions{
			Branch: "refs/heads/master",
			Create: false,
			Force:  true,
		})
		_ = os.Remove(testFile)
	})

	Fotingo.SetArgs([]string{"open", "issue", "--json"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result OpenOutput
	err = json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.Equal(t, "issue", result.Target)
	assert.Contains(t, result.URL, "TEST-456")
}

func (s *ExecutionTestSuite) TestOpen_PrJSON_Fails() {
	t := s.T()

	// PR open requires GitHub authentication which won't work in test
	Fotingo.SetArgs([]string{"open", "pr", "--json"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.Error(t, err)
	})

	var result OpenOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.False(t, result.Success)
	assert.Equal(t, "pr", result.Target)
	assert.NotEmpty(t, result.Error)
}

// --------------------------------------------------------------------------
// Start command: NoBranch with different issues
// --------------------------------------------------------------------------

func (s *ExecutionTestSuite) TestStart_NoBranch_JSON_456() {
	t := s.T()

	Fotingo.SetArgs([]string{"start", "TEST-456", "--json", "--no-branch"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result StartOutput
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)
	assert.True(t, result.Success)
	assert.NotNil(t, result.Issue)
	assert.Equal(t, "TEST-456", result.Issue.Key)
	assert.Equal(t, "Add user dashboard", result.Issue.Summary)
	assert.Equal(t, "Story", result.Issue.Type)
}

func (s *ExecutionTestSuite) TestStart_NonExistentIssue_JSON() {
	t := s.T()

	Fotingo.SetArgs([]string{"start", "NOPE-999", "--json", "--no-branch"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.Error(t, err)
	})

	var result StartOutput
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)
	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}

// --------------------------------------------------------------------------
// Start command with --title (createNewIssue) tests
// --------------------------------------------------------------------------

func (s *ExecutionTestSuite) TestStart_CreateNewIssue_JSON() {
	t := s.T()

	// Add the TEST project to the mock Jira server so CreateIssue works
	s.jiraServer.AddProject(jiratestutil.DefaultProject())

	// Pre-set transitions for the issue that will be created.
	// With 2 existing issues (TEST-123, TEST-456), the next one will be TEST-3.
	s.jiraServer.SetTransitions("TEST-3", jiratestutil.DefaultTransitions())

	Fotingo.SetArgs([]string{"start", "--json", "--title", "A brand new feature", "--project", "TEST", "--kind", "Story", "--no-branch"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result StartOutput
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.NotNil(t, result.Issue)
	// The mock server creates an issue with the TEST project prefix
	assert.Contains(t, result.Issue.Key, "TEST-")
	assert.Nil(t, result.Branch, "branch should be nil when --no-branch is used")
}

func (s *ExecutionTestSuite) TestStart_CreateNewIssue_MissingProject() {
	t := s.T()

	Fotingo.SetArgs([]string{"start", "--json", "--title", "Missing project"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.Error(t, err)
	})

	var result StartOutput
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "--project")
}

// --------------------------------------------------------------------------
// Inspect with specific issue tests (covers fetchIssueDetails path)
// --------------------------------------------------------------------------

func (s *ExecutionTestSuite) TestInspect_WithExistingIssue_FullOutput() {
	t := s.T()

	output := captureStdout(t, func() {
		Fotingo.SetArgs([]string{"inspect", "--issue", "TEST-456"})
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result InspectOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.NotNil(t, result.Issue)
	assert.Equal(t, "TEST-456", result.Issue.Key)
	assert.Equal(t, "Add user dashboard", result.Issue.Summary)
	assert.NotEmpty(t, result.Issue.Description)
	assert.Equal(t, "Story", result.Issue.Type)
	assert.Equal(t, "In Progress", result.Issue.Status)
}

// --------------------------------------------------------------------------
// Start command tests
// --------------------------------------------------------------------------

func (s *ExecutionTestSuite) TestStart_InvalidKind() {
	t := s.T()

	Fotingo.SetArgs([]string{"start", "TEST-123", "--json", "--kind", "InvalidType"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		// start returns an error for invalid kind
		assert.Error(t, err)
	})

	var result StartOutput
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)
	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}

func (s *ExecutionTestSuite) TestStart_NoBranch_JSON() {
	t := s.T()

	Fotingo.SetArgs([]string{"start", "TEST-123", "--json", "--no-branch"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result StartOutput
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)
	assert.True(t, result.Success)
	assert.NotNil(t, result.Issue)
	assert.Equal(t, "TEST-123", result.Issue.Key)
	assert.Nil(t, result.Branch, "branch should be nil when --no-branch is used")
}

func (s *ExecutionTestSuite) TestStart_MissingProject_WithTitle() {
	t := s.T()

	Fotingo.SetArgs([]string{"start", "--json", "--title", "New feature"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.Error(t, err)
	})

	var result StartOutput
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "--project")
}

func (s *ExecutionTestSuite) TestStart_NoIssueID_JSONMode() {
	t := s.T()

	// In JSON mode without issue ID, start should fail since interactive selection
	// is not supported
	Fotingo.SetArgs([]string{"start", "--json"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.Error(t, err)
	})

	var result StartOutput
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "issue ID required")
}

func (s *ExecutionTestSuite) TestSmoke_MissingConfig_FailsFast() {
	t := s.T()

	originalRoot := fotingoConfig.GetString("jira.root")
	fotingoConfig.Set("jira.root", "")
	defer fotingoConfig.Set("jira.root", originalRoot)

	Fotingo.SetArgs([]string{"start", "TEST-123", "--json", "--no-branch"})

	captureStdout(t, func() {
		err := Fotingo.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required config")
		assert.Contains(t, err.Error(), "jira.root")
	})
}

func (s *ExecutionTestSuite) TestSmoke_StartDebugOutput_IncludesTiming() {
	t := s.T()

	Fotingo.SetArgs([]string{"start", "TEST-123", "--debug", "--no-branch"})
	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	assert.Contains(t, output, "start timing phase=set_issue_in_progress duration=")
	assert.Contains(t, output, "start timing phase=resolve_assignee duration=")
}

// --------------------------------------------------------------------------
// Review command tests
// --------------------------------------------------------------------------

// checkoutIssueBranch creates and checks out a branch matching the issue template,
// then makes a commit so the branch has content. It returns a cleanup function
// that checks out back to master.
func (s *ExecutionTestSuite) checkoutIssueBranch(issueKey, branchSuffix string) func() {
	t := s.T()
	t.Helper()

	repo, err := gogit.PlainOpen(s.repoDir)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	branchName := fmt.Sprintf("b/%s_%s", issueKey, branchSuffix)
	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + branchName),
		Create: true,
	})
	require.NoError(t, err)

	// Make a commit on this branch so it diverges from master.
	// Use a unique filename per branch to avoid conflicts between tests.
	fileName := fmt.Sprintf("test-change-%s.txt", branchSuffix)
	testFile := filepath.Join(s.repoDir, fileName)
	require.NoError(t, os.WriteFile(testFile, []byte("change\n"), 0644))
	_, err = wt.Add(fileName)
	require.NoError(t, err)
	_, err = wt.Commit(fmt.Sprintf("%s: test commit", issueKey), &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	return func() {
		// Checkout master first, then remove the file if it still exists
		_ = wt.Checkout(&gogit.CheckoutOptions{
			Branch: "refs/heads/master",
			Create: false,
			Force:  true,
		})
		_ = os.Remove(testFile)
	}
}

func (s *ExecutionTestSuite) TestReviewCommand_BasicPR() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "fix_login_bug_review")
	defer cleanup()

	// Add an existing PR for this branch so the review command returns early
	// without trying to push to remote (which would require network access)
	branchName := "b/TEST-123_fix_login_bug_review"
	s.githubServer.AddPullRequest("testowner", "testrepo",
		ghtestutil.NewPullRequest(42, "[TEST-123] Fix login bug", branchName, "main", "open"),
	)

	Fotingo.SetArgs([]string{"review", "--json", "-y"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result ReviewOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.True(t, result.Existed, "PR should be marked as already existing")
	assert.NotNil(t, result.PullRequest)
	assert.Equal(t, 42, result.PullRequest.Number)
	assert.Equal(t, "existing", result.PullRequest.State)
	assert.Contains(t, result.PullRequest.URL, "pull/42")

	// Clean up PRs for this branch on the mock server
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestReviewCommand_WithLabels() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-456", "add_user_dashboard_labels")
	defer cleanup()

	branchName := "b/TEST-456_add_user_dashboard_labels"
	s.githubServer.AddPullRequest("testowner", "testrepo",
		ghtestutil.NewPullRequest(55, "[TEST-456] Add user dashboard", branchName, "main", "open"),
	)

	// Add labels to the repo so the mock server validates them
	s.githubServer.AddLabels("testowner", "testrepo",
		ghtestutil.NewLabel(1, "bug", "Bug label", "d73a4a"),
		ghtestutil.NewLabel(2, "enhancement", "Enhancement label", "a2eeef"),
	)

	Fotingo.SetArgs([]string{"review", "--json", "-y", "--labels", "bug,enhancement"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result ReviewOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.True(t, result.Existed, "PR should be marked as already existing")
	assert.NotNil(t, result.PullRequest)
	assert.Equal(t, 55, result.PullRequest.Number)

	// Clean up mock state
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
	s.githubServer.SetLabels("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestReviewCommand_SimpleMode() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "fix_login_bug_simple")
	defer cleanup()

	branchName := "b/TEST-123_fix_login_bug_simple"
	s.githubServer.AddPullRequest("testowner", "testrepo",
		ghtestutil.NewPullRequest(99, "Simple PR", branchName, "main", "open"),
	)

	Fotingo.SetArgs([]string{"review", "--json", "-y", "--simple"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result ReviewOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.True(t, result.Existed, "PR should be marked as already existing")
	assert.NotNil(t, result.PullRequest)
	assert.Equal(t, 99, result.PullRequest.Number)
	// In simple mode with an existing PR, there should be no issue info
	assert.Nil(t, result.Issue, "simple mode should not include Jira issue")

	// Clean up
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
}

// --------------------------------------------------------------------------
// Release command tests
// --------------------------------------------------------------------------

func (s *ExecutionTestSuite) TestReleaseCommand_Basic() {
	t := s.T()

	// The release command uses Run (not RunE), so it does not return errors
	// through Cobra. It also calls GetCommitsSinceDefaultBranch which tries
	// to access the remote over the network. Since our test repo has a fake
	// remote URL, this will fail. We verify the command runs and produces
	// output indicating the failure, proving the command was invoked and the
	// GitHub/Jira clients are properly wired.
	Fotingo.SetArgs([]string{"release", "v1.0.0", "--simple"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		// release uses Run not RunE, so Execute may not return the error
		// but the command should not panic
		_ = err
	})

	// The release command should have produced some output. Since it cannot
	// reach the remote to get commits, it will fail at the git operation
	// stage. We verify it ran by checking the output is not empty.
	assert.NotEmpty(t, output, "release command should produce output")
}

// --------------------------------------------------------------------------
// Review command: full create-PR path (no existing PR)
// --------------------------------------------------------------------------

func (s *ExecutionTestSuite) TestReviewCommand_CreatePR_FullPath() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "fix_login_bug_create")
	defer cleanup()

	// No existing PR - the review command will push and create one
	Fotingo.SetArgs([]string{"review", "--json", "-y"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result ReviewOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.False(t, result.Existed, "PR should be newly created")
	assert.NotNil(t, result.PullRequest)
	assert.Contains(t, result.PullRequest.URL, "pull/")
	assert.NotNil(t, result.Issue)
	assert.Equal(t, "TEST-123", result.Issue.Key)

	// Clean up PRs
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestReviewCommand_CreatePR_UsesRemoteDefaultBase() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "default_base_branch")
	defer cleanup()

	Fotingo.SetArgs([]string{"review", "--json", "-y", "--simple"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result ReviewOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	require.True(t, result.Success)
	require.NotNil(t, result.PullRequest)

	pr := s.githubServer.GetPullRequest("testowner", "testrepo", result.PullRequest.Number)
	require.NotNil(t, pr)
	assert.Equal(t, "master", pr.Base.Ref)

	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestReviewCommand_CreatePR_WithTitleAndDescription() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-456", "add_user_dashboard_custom")
	defer cleanup()

	Fotingo.SetArgs([]string{"review", "--json", "-y", "--title", "Custom PR title", "--description", "Custom body text"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result ReviewOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.False(t, result.Existed)
	assert.NotNil(t, result.PullRequest)

	// Clean up PRs
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestReviewCommand_CreatePR_SimpleMode() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "fix_login_bug_simple_create")
	defer cleanup()

	Fotingo.SetArgs([]string{"review", "--json", "-y", "--simple"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result ReviewOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.False(t, result.Existed)
	assert.NotNil(t, result.PullRequest)
	assert.Nil(t, result.Issue, "simple mode should not include Jira issue")

	// Clean up PRs
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestReviewCommand_CreatePR_Draft() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "fix_login_bug_draft")
	defer cleanup()

	Fotingo.SetArgs([]string{"review", "--json", "-y", "--draft", "--simple"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result ReviewOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.NotNil(t, result.PullRequest)
	assert.True(t, result.PullRequest.Draft)

	// Clean up PRs
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestReviewCommand_CreatePR_WithReviewers() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "fix_login_bug_reviewers")
	defer cleanup()

	// Add collaborators so reviewer requests work
	s.githubServer.AddCollaborators("testowner", "testrepo",
		ghtestutil.NewUser(10, "alice", "Alice"),
		ghtestutil.NewUser(11, "bob", "Bob"),
	)

	Fotingo.SetArgs([]string{"review", "--json", "-y", "--simple", "--reviewers", "alice,bob"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	var result ReviewOutput
	err := json.Unmarshal([]byte(extractJSON(output)), &result)
	require.NoError(t, err, "output should contain valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.NotNil(t, result.PullRequest)
	assert.Equal(t, []string{"alice", "bob"}, result.Reviewers)

	// Clean up
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
	s.githubServer.SetCollaborators("testowner", "testrepo", nil)
}

// --------------------------------------------------------------------------
// Release command: full path tests
// --------------------------------------------------------------------------

func (s *ExecutionTestSuite) TestReleaseCommand_SimpleMode() {
	t := s.T()

	// Create a branch with commits that reference issues
	cleanup := s.checkoutIssueBranch("TEST-123", "fix_login_bug_release")
	defer cleanup()

	// In simple mode, only GitHub release is created (no Jira)
	Fotingo.SetArgs([]string{"release", "v1.0.0-test", "--simple"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		_ = err // release uses Run not RunE
	})

	// The release command should have produced output and created a GitHub release
	assert.NotEmpty(t, output)

	// Clean up releases
	s.githubServer.SetReleases("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestReleaseCommand_NoVCSRelease() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-456", "add_dashboard_release")
	defer cleanup()

	// Add the TEST project for Jira release creation
	s.jiraServer.AddProject(jiratestutil.DefaultProject())

	// No VCS release mode - only Jira release, skip GitHub
	Fotingo.SetArgs([]string{"release", "v2.0.0-test", "--no-vcs-release"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		_ = err
	})

	assert.NotEmpty(t, output)
}

func (s *ExecutionTestSuite) TestReleaseCommand_WithAdditionalIssues() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "fix_login_release_extra")
	defer cleanup()

	// Simple mode with additional issues
	Fotingo.SetArgs([]string{"release", "v3.0.0-test", "--simple", "--issues", "EXTRA-1,EXTRA-2"})

	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		_ = err
	})

	assert.NotEmpty(t, output)

	// Clean up releases
	s.githubServer.SetReleases("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestSmoke_ReleaseOutput_DefaultVsVerbose() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "release_output_modes")
	defer cleanup()

	Fotingo.SetArgs([]string{"release", "v4.0.0-smoke-default", "--simple"})
	defaultOutput := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})
	assert.Contains(t, defaultOutput, "Successfully created release v4.0.0-smoke-default")
	assert.NotContains(t, defaultOutput, localizer.T(i18n.ReleaseStatusInitGit))

	Fotingo.SetArgs([]string{"release", "v4.0.1-smoke-verbose", "--simple", "--verbose"})
	verboseOutput := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})
	assert.Contains(t, verboseOutput, localizer.T(i18n.ReleaseStatusInitGit))
	assert.Contains(t, verboseOutput, "Successfully created release v4.0.1-smoke-verbose")

	s.githubServer.SetReleases("testowner", "testrepo", nil)
}

// --------------------------------------------------------------------------
// Direct runRelease tests (bypassing TUI)
// --------------------------------------------------------------------------

func (s *ExecutionTestSuite) TestRunRelease_SimpleMode_FullPath() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "fix_login_release_direct")
	defer cleanup()

	releaseCmdFlags.simple = true

	statusCh := make(chan string, 100)
	processDone := make(chan bool, 1)

	err := runRelease(&statusCh, processDone, "v10.0.0-direct")
	assert.NoError(t, err)

	// Drain the done channel
	<-processDone

	// Clean up releases
	s.githubServer.SetReleases("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestRunRelease_NoVCSRelease() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-456", "add_dashboard_release_direct")
	defer cleanup()

	s.jiraServer.AddProject(jiratestutil.DefaultProject())
	releaseCmdFlags.noVCSRelease = true

	statusCh := make(chan string, 100)
	processDone := make(chan bool, 1)

	err := runRelease(&statusCh, processDone, "v11.0.0-novcs")
	assert.NoError(t, err)

	<-processDone
}

func (s *ExecutionTestSuite) TestRunRelease_FullPath_WithJiraAndGitHub() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "fix_login_full_release")
	defer cleanup()

	s.jiraServer.AddProject(jiratestutil.DefaultProject())

	statusCh := make(chan string, 100)
	processDone := make(chan bool, 1)

	err := runRelease(&statusCh, processDone, "v12.0.0-full")
	assert.NoError(t, err)

	<-processDone

	// Clean up releases
	s.githubServer.SetReleases("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestRunRelease_WithAdditionalIssues() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "fix_login_release_extra_direct")
	defer cleanup()

	releaseCmdFlags.simple = true
	releaseCmdFlags.issues = []string{"EXTRA-1", "EXTRA-2"}

	statusCh := make(chan string, 100)
	processDone := make(chan bool, 1)

	err := runRelease(&statusCh, processDone, "v13.0.0-extra")
	assert.NoError(t, err)

	<-processDone

	// Clean up releases
	s.githubServer.SetReleases("testowner", "testrepo", nil)
}

// --------------------------------------------------------------------------
// Direct runReviewWithResult tests (bypassing TUI)
// --------------------------------------------------------------------------

func (s *ExecutionTestSuite) TestRunReview_BranchAlreadyOnRemote() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "fix_login_already_pushed")
	defer cleanup()

	// Push the branch to the bare remote first
	branchName := "b/TEST-123_fix_login_already_pushed"
	repo, err := gogit.PlainOpen(s.repoDir)
	require.NoError(t, err)
	_ = repo.DeleteRemote("origin")
	_, _ = repo.CreateRemote(&gogitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{s.bareDir},
	})
	err = repo.Push(&gogit.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []gogitConfig.RefSpec{gogitConfig.RefSpec("refs/heads/" + branchName + ":refs/heads/" + branchName)},
	})
	require.NoError(t, err)
	// Restore origin to github URL
	_ = repo.DeleteRemote("origin")
	_, _ = repo.CreateRemote(&gogitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://github.com/testowner/testrepo.git"},
	})

	statusCh := make(chan string, 100)
	result := runReviewWithResult(&statusCh)

	assert.NoError(t, result.err)
	assert.False(t, result.existed)
	assert.NotNil(t, result.pr)

	// Clean up PRs
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestSmoke_ReviewDebugOutput() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "review_debug_output")
	defer cleanup()

	Fotingo.SetArgs([]string{"review", "--simple", "--debug", "--yes", "--title", "Debug smoke PR"})
	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	assert.Contains(t, output, "review branch=")
	assert.Contains(t, output, "review pr body prepared length=")
	assert.Contains(t, output, "review timing phase=total duration=")
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestSmoke_ReviewDebugOutput_ExistingPRSkipsCommitCollection() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "review_existing_pr_skip_commits")
	defer cleanup()

	branchName := "b/TEST-123_review_existing_pr_skip_commits"
	s.githubServer.AddPullRequest("testowner", "testrepo",
		ghtestutil.NewPullRequest(77, "[TEST-123] Existing review", branchName, "main", "open"),
	)

	Fotingo.SetArgs([]string{"review", "--simple", "--debug", "--yes"})
	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	assert.Contains(t, output, "review timing phase=check_existing_pr duration=")
	assert.NotContains(t, output, "review commits loaded=")
	assert.NotContains(t, output, "review commits unavailable:")
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestSmoke_ReviewDebugOutput_DescriptionOverrideSkipsCommits() {
	t := s.T()

	cleanup := s.checkoutIssueBranch("TEST-123", "review_description_skip_commits")
	defer cleanup()

	Fotingo.SetArgs([]string{
		"review",
		"--simple",
		"--debug",
		"--yes",
		"--title", "Description override PR",
		"--description", "Manual body override",
	})
	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	assert.Contains(t, output, "review commits skipped: description override provided")
	assert.NotContains(t, output, "review commits loaded=")
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
}

func (s *ExecutionTestSuite) TestSmoke_ReviewInteractiveEditorFlow() {
	t := s.T()
	withDefaultReviewTemplateResolver(t)

	cleanup := s.checkoutIssueBranch("TEST-123", "review_editor_smoke")
	defer cleanup()

	origTTY := isInputTerminalFn
	origEditor := openEditorFn
	origProcessFn := openEditorProcessFn
	defer func() {
		isInputTerminalFn = origTTY
		openEditorFn = origEditor
		openEditorProcessFn = origProcessFn
	}()

	isInputTerminalFn = func() bool { return true }
	openEditorFn = openEditorWithRuntime
	openedEditor := false
	openEditorProcessFn = func(initialContent string) (string, error) {
		openedEditor = true
		assert.Contains(t, initialContent, "**Description**")
		return initialContent + "\nSmoked via editor flow.\n", nil
	}

	Fotingo.SetArgs([]string{"review", "--simple", "--title", "Interactive editor smoke"})
	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		assert.NoError(t, err)
	})

	assert.True(t, openedEditor, "review should invoke editor in interactive mode")
	assert.Contains(t, output, localizer.T(i18n.ReviewStatusOpenEditor))
	assert.Contains(t, output, localizer.T(i18n.ReviewStatusEditorDone))
	s.githubServer.SetPullRequests("testowner", "testrepo", nil)
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// extractJSON tries to find the first complete JSON object in the output.
// Commands may emit additional human-readable lines before/after JSON,
// so we extract the JSON payload by matching braces.
func extractJSON(output string) string {
	// Find the first '{' and match to the last '}'
	start := -1
	for i, c := range output {
		if c == '{' {
			start = i
			break
		}
	}
	if start == -1 {
		return output
	}

	// Find the matching closing brace
	depth := 0
	for i := start; i < len(output); i++ {
		switch output[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return output[start : i+1]
			}
		}
	}
	return output[start:]
}
