package start

import (
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	giturl "github.com/kubescape/go-git-url"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/auth"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

type startTimingEmitter struct {
	debugRaw []string
}

func (e *startTimingEmitter) Info(string, i18n.Key, ...any) {}
func (e *startTimingEmitter) Verbose(i18n.Key, ...any)      {}
func (e *startTimingEmitter) Debug(i18n.Key, ...any)        {}
func (e *startTimingEmitter) DebugRaw(message string) {
	e.debugRaw = append(e.debugRaw, strings.TrimSpace(message))
}

type startEvent struct {
	key  i18n.Key
	args []any
}

type startCollectingEmitter struct {
	events []startEvent
}

func (e *startCollectingEmitter) Info(_ string, key i18n.Key, args ...any) {
	e.events = append(e.events, startEvent{key: key, args: append([]any{}, args...)})
}

func (e *startCollectingEmitter) Verbose(i18n.Key, ...any) {}
func (e *startCollectingEmitter) Debug(i18n.Key, ...any)   {}
func (e *startCollectingEmitter) DebugRaw(string)          {}

func TestWorkflowRunnerRunWithResult_ValidatesDeps(t *testing.T) {
	runner := WorkflowRunner{}
	result := runner.RunWithResult(&cobra.Command{}, nil, "TEST-1", nil)
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "NormalizeFlags")
}

func TestWorkflowRunnerRunWithResult_PropagatesNormalizeError(t *testing.T) {
	expectedErr := errors.New("normalize failed")
	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{},
		Deps: WorkflowDeps{
			NormalizeFlags: func(*cobra.Command, string) error { return expectedErr },
			NewJiraClient:  func(*viper.Viper) (jira.Jira, error) { return nil, nil },
			CreateNewIssue: func(WorkflowEmitter, jira.Jira) (*jira.Issue, error) { return nil, nil },
			SelectIssueWithPicker: func([]tracker.Issue) (*tracker.Issue, error) {
				return nil, nil
			},
			RunWithSpinner:       func(func(WorkflowEmitter) error) error { return nil },
			ResolveIssueAssignee: func(WorkflowEmitter, jira.Jira, string) {},
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return nil, nil
			},
			StashChanges: func(WorkflowEmitter, git.Git) error { return nil },
		},
	}

	result := runner.RunWithResult(&cobra.Command{}, nil, "TEST-1", nil)
	require.Error(t, result.Err)
	assert.ErrorIs(t, result.Err, expectedErr)
}

func TestWorkflowRunnerLocalizeFallback(t *testing.T) {
	runner := WorkflowRunner{}
	assert.Equal(t, i18n.T(i18n.StartStatusClean), runner.localize(i18n.StartStatusClean))
}

func TestLogStartPhaseTiming_EmitsDebugTiming(t *testing.T) {
	emitter := &startTimingEmitter{}
	logStartPhaseTiming(emitter, "phase_x", time.Now().Add(-120*time.Millisecond))

	require.Len(t, emitter.debugRaw, 1)
	assert.Contains(t, emitter.debugRaw[0], "start timing phase=phase_x duration=")
}

type blockingWorkflowEmitter struct {
	debugStarted chan struct{}
	releaseDebug chan struct{}
	debugOnce    sync.Once
}

func (e *blockingWorkflowEmitter) Info(string, i18n.Key, ...any) {}
func (e *blockingWorkflowEmitter) Verbose(i18n.Key, ...any)      {}
func (e *blockingWorkflowEmitter) Debug(i18n.Key, ...any)        {}
func (e *blockingWorkflowEmitter) DebugRaw(string) {
	e.debugOnce.Do(func() {
		close(e.debugStarted)
	})
	<-e.releaseDebug
}

type workflowMockGit struct {
	messageCh             *chan string
	createBranchName      string
	createWorktreePath    string
	createWorktreeOptions git.WorktreeOptions
	fetchDefaultCalls     int
	createBranchCalls     int
	createWorktreeCalls   int
	hasUncommitedCalls    int
}

func (m *workflowMockGit) GetRemote() (giturl.IGitURL, error) { return nil, nil }
func (m *workflowMockGit) GetCurrentBranch() (string, error) {
	return "", nil
}
func (m *workflowMockGit) GetIssueId() (string, error) {
	return "", nil
}
func (m *workflowMockGit) CreateIssueBranch(*jira.Issue) (string, error) {
	m.createBranchCalls++
	return m.createBranchName, nil
}
func (m *workflowMockGit) CreateIssueWorktreeBranch(_ *jira.Issue, options git.WorktreeOptions) (string, string, error) {
	m.createWorktreeCalls++
	m.createWorktreeOptions = options
	return m.createBranchName, m.createWorktreePath, nil
}
func (m *workflowMockGit) Push() error { return nil }
func (m *workflowMockGit) StashChanges(string) error {
	return nil
}
func (m *workflowMockGit) PopStash() error {
	return nil
}
func (m *workflowMockGit) HasUncommittedChanges() (bool, error) {
	m.hasUncommitedCalls++
	return false, nil
}
func (m *workflowMockGit) GetCommitsSince(string) ([]git.Commit, error) {
	return nil, nil
}
func (m *workflowMockGit) DoesBranchExistInRemote(string) (bool, error) {
	return false, nil
}
func (m *workflowMockGit) GetDefaultBranch() (string, error) {
	return "main", nil
}
func (m *workflowMockGit) FetchDefaultBranch() error {
	m.fetchDefaultCalls++
	if m.messageCh != nil {
		*m.messageCh <- "debug: fetch default branch"
	}
	return nil
}
func (m *workflowMockGit) GetCommitsSinceDefaultBranch() ([]git.Commit, error) {
	return nil, nil
}
func (m *workflowMockGit) GetIssuesFromCommits([]git.Commit) []string {
	return nil
}
func (m *workflowMockGit) GetConfig() *viper.Viper { return nil }
func (m *workflowMockGit) SaveConfig(string, any) error {
	return nil
}

type workflowMockJira struct {
	setIssue          *jira.Issue
	setIssueStatusIDs []string
}

func (m *workflowMockJira) Name() string { return "Jira" }
func (m *workflowMockJira) GetCurrentUser() (*tracker.User, error) {
	return nil, nil
}
func (m *workflowMockJira) GetUserOpenIssues() ([]tracker.Issue, error) {
	return nil, nil
}
func (m *workflowMockJira) GetIssue(string) (*tracker.Issue, error) {
	return nil, nil
}
func (m *workflowMockJira) AssignIssue(string, string) (*tracker.Issue, error) {
	return nil, nil
}
func (m *workflowMockJira) CreateIssue(tracker.CreateIssueInput) (*tracker.Issue, error) {
	return nil, nil
}
func (m *workflowMockJira) GetProjectIssueTypes(string) ([]tracker.ProjectIssueType, error) {
	return nil, nil
}
func (m *workflowMockJira) SetIssueStatus(string, tracker.IssueStatus) (*tracker.Issue, error) {
	return nil, nil
}
func (m *workflowMockJira) AddComment(string, string) error { return nil }
func (m *workflowMockJira) CreateRelease(tracker.CreateReleaseInput) (*tracker.Release, error) {
	return nil, nil
}
func (m *workflowMockJira) SetFixVersion([]string, *tracker.Release) error { return nil }
func (m *workflowMockJira) IsValidIssueID(id string) bool {
	return regexp.MustCompile(`^[A-Z][A-Z0-9_]+-\d+$`).MatchString(strings.ToUpper(strings.TrimSpace(id)))
}
func (m *workflowMockJira) GetIssueURL(string) string { return "" }
func (m *workflowMockJira) Authenticate() (*auth.AccessToken, error) {
	return &auth.AccessToken{}, nil
}
func (m *workflowMockJira) GetConfig() *viper.Viper { return nil }
func (m *workflowMockJira) SaveConfig(string, any) error {
	return nil
}
func (m *workflowMockJira) GetIssueUrl(string) (string, error) { return "", nil }
func (m *workflowMockJira) GetJiraIssue(string) (*jira.Issue, error) {
	return m.setIssue, nil
}
func (m *workflowMockJira) SetJiraIssueStatus(issueID string, _ jira.IssueStatus) (*jira.Issue, error) {
	m.setIssueStatusIDs = append(m.setIssueStatusIDs, issueID)
	return m.setIssue, nil
}
func (m *workflowMockJira) SearchIssues(string, string, []tracker.IssueType, int) ([]tracker.Issue, error) {
	return nil, nil
}

func TestWorkflowRunnerProgressStartWorkflow_WaitsForGitDebugForwarding(t *testing.T) {
	emitter := &blockingWorkflowEmitter{
		debugStarted: make(chan struct{}),
		releaseDebug: make(chan struct{}),
	}
	gitClient := &workflowMockGit{createBranchName: "f/test-123_fix_worktree"}
	jiraClient := &workflowMockJira{
		setIssue: &jira.Issue{
			Key:     "TEST-123",
			Summary: "Fix worktree start flow",
			Type:    "Task",
			Status:  string(jira.StatusInProgress),
		},
	}

	runner := WorkflowRunner{
		Config: viper.New(),
		Deps: WorkflowDeps{
			RunWithSpinner: func(work func(WorkflowEmitter) error) error {
				return work(emitter)
			},
			ResolveIssueAssignee: func(WorkflowEmitter, jira.Jira, string) {},
			NewGitClient: func(_ *viper.Viper, statusCh *chan string) (git.Git, error) {
				gitClient.messageCh = statusCh
				return gitClient, nil
			},
			StashChanges: func(WorkflowEmitter, git.Git) error { return nil },
		},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.progressStartWorkflow(
			jiraClient,
			&jira.Issue{Key: "TEST-123", Summary: "Fix worktree start flow", Type: "Task"},
			"TEST-123",
		)
	}()

	select {
	case <-emitter.debugStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for git debug forwarding to start")
	}

	select {
	case err := <-errCh:
		t.Fatalf("workflow returned before git debug forwarding completed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(emitter.releaseDebug)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for workflow to finish")
	}

	assert.Equal(t, 1, gitClient.fetchDefaultCalls)
	assert.Equal(t, 1, gitClient.createBranchCalls)
	assert.Equal(t, 1, gitClient.hasUncommitedCalls)
}

func TestWorkflowRunnerRunWithResult_NormalizesJiraBrowseURLInput(t *testing.T) {
	jiraClient := &workflowMockJira{
		setIssue: &jira.Issue{
			Key:     "DEVOPS-13148",
			Summary: "Test URL start flow",
			Type:    "Task",
			Status:  string(jira.StatusInProgress),
		},
	}
	runner := WorkflowRunner{
		Config: func() *viper.Viper {
			cfg := viper.New()
			cfg.Set("jira.root", "https://team-turo.atlassian.net")
			return cfg
		}(),
		Options: WorkflowOptions{NoBranch: true},
		Deps: WorkflowDeps{
			NormalizeFlags: func(*cobra.Command, string) error { return nil },
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return jiraClient, nil
			},
			CreateNewIssue: func(WorkflowEmitter, jira.Jira) (*jira.Issue, error) { return nil, nil },
			SelectIssueWithPicker: func([]tracker.Issue) (*tracker.Issue, error) {
				return nil, nil
			},
			RunWithSpinner:       func(func(WorkflowEmitter) error) error { return nil },
			ResolveIssueAssignee: func(WorkflowEmitter, jira.Jira, string) {},
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return nil, nil
			},
			StashChanges: func(WorkflowEmitter, git.Git) error { return nil },
		},
	}

	result := runner.RunWithResult(
		&cobra.Command{},
		nil,
		"https://team-turo.atlassian.net/browse/DEVOPS-13148",
		nil,
	)
	require.NoError(t, result.Err)
	require.Len(t, jiraClient.setIssueStatusIDs, 1)
	assert.Equal(t, "DEVOPS-13148", jiraClient.setIssueStatusIDs[0])
}

func TestWorkflowRunnerRunWithResult_UsesWorktreeBranchCreationWhenEnabled(t *testing.T) {
	gitClient := &workflowMockGit{
		createBranchName:   "f/test-123_fix_worktree",
		createWorktreePath: "/tmp/fotingo-f-test-123_fix_worktree",
	}
	jiraClient := &workflowMockJira{
		setIssue: &jira.Issue{
			Key:     "TEST-123",
			Summary: "Fix worktree start flow",
			Type:    "Task",
			Status:  string(jira.StatusInProgress),
		},
	}

	runner := WorkflowRunner{
		Config: viper.New(),
		Options: WorkflowOptions{
			Worktree:     true,
			WorktreePath: ".claude/worktrees",
		},
		Deps: WorkflowDeps{
			NormalizeFlags: func(*cobra.Command, string) error { return nil },
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return jiraClient, nil
			},
			CreateNewIssue: func(WorkflowEmitter, jira.Jira) (*jira.Issue, error) { return nil, nil },
			SelectIssueWithPicker: func([]tracker.Issue) (*tracker.Issue, error) {
				return nil, nil
			},
			RunWithSpinner:       func(func(WorkflowEmitter) error) error { return nil },
			ResolveIssueAssignee: func(WorkflowEmitter, jira.Jira, string) {},
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return gitClient, nil
			},
			StashChanges: func(WorkflowEmitter, git.Git) error { return nil },
		},
	}

	statusCh := make(chan string, 8)
	result := runner.RunWithResult(&cobra.Command{}, &statusCh, "TEST-123", nil)
	require.NoError(t, result.Err)
	assert.Equal(t, "f/test-123_fix_worktree", result.BranchName)
	assert.Equal(t, "/tmp/fotingo-f-test-123_fix_worktree", result.WorktreePath)
	assert.Equal(t, 1, gitClient.fetchDefaultCalls)
	assert.Zero(t, gitClient.createBranchCalls)
	assert.Equal(t, 1, gitClient.createWorktreeCalls)
	assert.Equal(t, git.WorktreeOptions{
		ParentPath: ".claude/worktrees",
	}, gitClient.createWorktreeOptions)
}

func TestWorkflowRunnerCreateIssueBranch_EmitsBranchAndWorktreeLocation(t *testing.T) {
	gitClient := &workflowMockGit{
		createBranchName:   "f/test-123_fix_worktree",
		createWorktreePath: "/tmp/fotingo-f-test-123_fix_worktree",
	}
	emitter := &startCollectingEmitter{}
	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{Worktree: true},
		Localize: func(key i18n.Key, args ...any) string {
			return i18n.T(key, args...)
		},
	}

	branchName, worktreePath, err := runner.createIssueBranch(emitter, gitClient, &jira.Issue{
		Key:     "TEST-123",
		Summary: "Fix worktree start flow",
		Type:    "Task",
	})
	require.NoError(t, err)
	assert.Equal(t, "f/test-123_fix_worktree", branchName)
	assert.Equal(t, "/tmp/fotingo-f-test-123_fix_worktree", worktreePath)

	var sawBranchDone bool
	var sawWorktreeReady bool
	for _, event := range emitter.events {
		switch event.key {
		case i18n.StartStatusBranchDone:
			sawBranchDone = len(event.args) == 1 && event.args[0] == "f/test-123_fix_worktree"
		case i18n.StartStatusWorktreeReady:
			sawWorktreeReady = len(event.args) == 3 &&
				event.args[0] == "f/test-123_fix_worktree" &&
				event.args[1] == "/tmp/fotingo-f-test-123_fix_worktree" &&
				event.args[2] == "/tmp/fotingo-f-test-123_fix_worktree"
		}
	}

	assert.True(t, sawBranchDone)
	assert.True(t, sawWorktreeReady)
}

func TestWorktreeReadyMessage_IncludesExplicitCdInstruction(t *testing.T) {
	message := i18n.T(
		i18n.StartStatusWorktreeReady,
		"f/test-123_fix_worktree",
		"/tmp/fotingo-f-test-123_fix_worktree",
		"/tmp/fotingo-f-test-123_fix_worktree",
	)

	assert.Contains(t, message, "Ready to work in the new worktree.")
	assert.Contains(t, message, "Branch: f/test-123_fix_worktree")
	assert.Contains(t, message, "Directory: /tmp/fotingo-f-test-123_fix_worktree")
	assert.Contains(t, message, "Next: cd /tmp/fotingo-f-test-123_fix_worktree")
	assert.Contains(t, message, "\n")
}
