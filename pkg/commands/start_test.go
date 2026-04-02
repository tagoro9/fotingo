package commands

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

func TestParseIssueKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		kind    string
		want    tracker.IssueType
		wantErr bool
	}{
		{
			name:    "story lowercase",
			kind:    "story",
			want:    tracker.IssueTypeStory,
			wantErr: false,
		},
		{
			name:    "Story mixed case",
			kind:    "Story",
			want:    tracker.IssueTypeStory,
			wantErr: false,
		},
		{
			name:    "STORY uppercase",
			kind:    "STORY",
			want:    tracker.IssueTypeStory,
			wantErr: false,
		},
		{
			name:    "bug lowercase",
			kind:    "bug",
			want:    tracker.IssueTypeBug,
			wantErr: false,
		},
		{
			name:    "Bug mixed case",
			kind:    "Bug",
			want:    tracker.IssueTypeBug,
			wantErr: false,
		},
		{
			name:    "task lowercase",
			kind:    "task",
			want:    tracker.IssueTypeTask,
			wantErr: false,
		},
		{
			name:    "Task mixed case",
			kind:    "Task",
			want:    tracker.IssueTypeTask,
			wantErr: false,
		},
		{
			name:    "subtask lowercase",
			kind:    "subtask",
			want:    tracker.IssueTypeSubTask,
			wantErr: false,
		},
		{
			name:    "sub-task with hyphen",
			kind:    "sub-task",
			want:    tracker.IssueTypeSubTask,
			wantErr: false,
		},
		{
			name:    "epic lowercase",
			kind:    "epic",
			want:    tracker.IssueTypeEpic,
			wantErr: false,
		},
		{
			name:    "invalid type",
			kind:    "invalid",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty string",
			kind:    "",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseIssueKind(tt.kind)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid issue type")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGetStatusIndicator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status tracker.IssueStatus
		want   string
	}{
		{
			name:   "backlog status",
			status: tracker.IssueStatusBacklog,
			want:   "(Backlog)",
		},
		{
			name:   "todo status",
			status: tracker.IssueStatusToDo,
			want:   "(To Do)",
		},
		{
			name:   "in progress status",
			status: tracker.IssueStatusInProgress,
			want:   "(In Progress)",
		},
		{
			name:   "in review status",
			status: tracker.IssueStatusInReview,
			want:   "(In Review)",
		},
		{
			name:   "done status",
			status: tracker.IssueStatusDone,
			want:   "(Done)",
		},
		{
			name:   "unknown status",
			status: tracker.IssueStatus("unknown"),
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getStatusIndicator(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStartFlags(t *testing.T) {
	t.Parallel()

	// Verify the start command has expected flags
	flags := startCmd.Flags()

	// Check flag existence and defaults
	titleFlag := flags.Lookup("title")
	assert.NotNil(t, titleFlag, "title flag should exist")
	assert.Equal(t, "t", titleFlag.Shorthand)

	descriptionFlag := flags.Lookup("description")
	assert.NotNil(t, descriptionFlag, "description flag should exist")
	assert.Equal(t, "d", descriptionFlag.Shorthand)

	projectFlag := flags.Lookup("project")
	assert.NotNil(t, projectFlag, "project flag should exist")
	assert.Equal(t, "p", projectFlag.Shorthand)

	kindFlag := flags.Lookup("kind")
	assert.NotNil(t, kindFlag, "kind flag should exist")
	assert.Equal(t, "k", kindFlag.Shorthand)
	assert.Equal(t, "Task", kindFlag.DefValue)

	parentFlag := flags.Lookup("parent")
	assert.NotNil(t, parentFlag, "parent flag should exist")
	assert.Equal(t, "a", parentFlag.Shorthand)

	epicFlag := flags.Lookup("epic")
	assert.NotNil(t, epicFlag, "epic flag should exist")
	assert.Equal(t, "e", epicFlag.Shorthand)

	labelsFlag := flags.Lookup("labels")
	assert.NotNil(t, labelsFlag, "labels flag should exist")
	assert.Equal(t, "l", labelsFlag.Shorthand)

	noBranchFlag := flags.Lookup("no-branch")
	assert.NotNil(t, noBranchFlag, "no-branch flag should exist")
	assert.Equal(t, "n", noBranchFlag.Shorthand)

	worktreeFlag := flags.Lookup("worktree")
	assert.NotNil(t, worktreeFlag, "worktree flag should exist")
	assert.Equal(t, "", worktreeFlag.Shorthand)
	assert.Equal(t, "false", worktreeFlag.DefValue)

	interactiveFlag := flags.Lookup("interactive")
	assert.NotNil(t, interactiveFlag, "interactive flag should exist")
	assert.Equal(t, "i", interactiveFlag.Shorthand)
}

func TestStartWorktreeEnabled(t *testing.T) {
	origFlags := startCmdFlags
	origConfig := fotingoConfig
	defer func() {
		startCmdFlags = origFlags
		fotingoConfig = origConfig
	}()

	cfg := viper.New()
	cfg.Set("git.worktree.enabled", true)

	startCmdFlags = startFlags{}
	assert.True(t, startWorktreeEnabled(cfg))

	cfg.Set("git.worktree.enabled", false)
	assert.False(t, startWorktreeEnabled(cfg))

	startCmdFlags.worktree = true
	assert.True(t, startWorktreeEnabled(cfg))
}

func TestStartCmdUse(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "start [issueId]", startCmd.Use)
	assert.NotEmpty(t, startCmd.Short)
	assert.NotEmpty(t, startCmd.Long)
}

func TestNormalizeStartCreateFlags_DoesNotAliasDescription(t *testing.T) {
	origFlags := startCmdFlags
	defer func() { startCmdFlags = origFlags }()

	startCmdFlags = startFlags{
		title:       "Build login page",
		description: "Build login page",
	}

	cmd := newStartFlagProbeCommand(t)
	err := normalizeStartCreateFlags(cmd, "")
	require.NoError(t, err)
	assert.Equal(t, "Build login page", startCmdFlags.title)
	assert.Equal(t, "Build login page", startCmdFlags.description)
}

func TestCollectInteractiveCreateFlags_SkipsPrefilledTitle(t *testing.T) {
	origFlags := startCmdFlags
	origTerminal := startIsInteractiveTerminalFn
	origPrompt := startPromptInputFn
	origPromptMultiline := startPromptMultilineInputFn
	origPickKind := startSelectKindFn
	origConfirm := startConfirmFn
	origProjectIssueTypes := startProjectIssueTypeNamesFn
	origNewJiraClient := newJiraClient
	defer func() {
		startCmdFlags = origFlags
		startIsInteractiveTerminalFn = origTerminal
		startPromptInputFn = origPrompt
		startPromptMultilineInputFn = origPromptMultiline
		startSelectKindFn = origPickKind
		startConfirmFn = origConfirm
		startProjectIssueTypeNamesFn = origProjectIssueTypes
		newJiraClient = origNewJiraClient
	}()

	startCmdFlags = startFlags{
		title:       "This is the title",
		description: "This is the description",
		interactive: true,
		kind:        "Task",
	}
	startIsInteractiveTerminalFn = func() bool { return true }

	promptCalls := []string{}
	startPromptInputFn = func(prompt string, _ string, _ bool, _ string) (string, bool, error) {
		promptCalls = append(promptCalls, prompt)
		switch {
		case strings.Contains(strings.ToLower(prompt), "description"):
			t.Fatalf("description prompt should be skipped when description is prefilled")
			return "", false, nil
		case strings.Contains(strings.ToLower(prompt), "project"):
			return "PROJ", false, nil
		case strings.Contains(strings.ToLower(prompt), "epic"):
			return "", false, nil
		case strings.Contains(strings.ToLower(prompt), "parent"):
			return "", false, nil
		case strings.Contains(strings.ToLower(prompt), "labels"):
			return "", false, nil
		default:
			return "unexpected", false, nil
		}
	}
	startSelectKindFn = func(_ []string, _ string) (string, bool, error) {
		return "Task", false, nil
	}
	startProjectIssueTypeNamesFn = func(_ string) ([]string, error) {
		return []string{"Task", "Story", "Bug"}, nil
	}
	startConfirmFn = func(_ string, _ bool) (bool, error) {
		return false, nil
	}
	newJiraClient = func(_ *viper.Viper) (jira.Jira, error) {
		return &mockJira{}, nil
	}

	cmd := newStartFlagProbeCommand(t)
	require.NoError(t, cmd.Flags().Set("title", "This is the title"))
	require.NoError(t, cmd.Flags().Set("interactive", "true"))

	err := collectInteractiveCreateFlags(cmd)
	require.NoError(t, err)
	assert.Equal(t, "This is the title", startCmdFlags.title)
	assert.Equal(t, "This is the description", startCmdFlags.description)
	assert.Equal(t, "PROJ", startCmdFlags.project)
	assert.NotEmpty(t, promptCalls)
}

func TestCollectInteractiveCreateFlags_PromptsForTitleAndDescription(t *testing.T) {
	origFlags := startCmdFlags
	origTerminal := startIsInteractiveTerminalFn
	origPrompt := startPromptInputFn
	origPromptMultiline := startPromptMultilineInputFn
	origPickKind := startSelectKindFn
	origConfirm := startConfirmFn
	origProjectIssueTypes := startProjectIssueTypeNamesFn
	origNewJiraClient := newJiraClient
	defer func() {
		startCmdFlags = origFlags
		startIsInteractiveTerminalFn = origTerminal
		startPromptInputFn = origPrompt
		startPromptMultilineInputFn = origPromptMultiline
		startSelectKindFn = origPickKind
		startConfirmFn = origConfirm
		startProjectIssueTypeNamesFn = origProjectIssueTypes
		newJiraClient = origNewJiraClient
	}()

	startCmdFlags = startFlags{
		interactive: true,
		kind:        "Task",
	}
	startIsInteractiveTerminalFn = func() bool { return true }

	promptCalls := []string{}
	startPromptInputFn = func(prompt string, _ string, _ bool, _ string) (string, bool, error) {
		promptCalls = append(promptCalls, prompt)
		switch {
		case strings.Contains(strings.ToLower(prompt), "title"):
			assert.NotContains(t, strings.ToLower(prompt), "summary")
			return "Implement interactive flow", false, nil
		case strings.Contains(strings.ToLower(prompt), "project"):
			return "PROJ", false, nil
		case strings.Contains(strings.ToLower(prompt), "epic"):
			return "", false, nil
		case strings.Contains(strings.ToLower(prompt), "parent"):
			return "", false, nil
		case strings.Contains(strings.ToLower(prompt), "labels"):
			return "", false, nil
		default:
			return "", false, nil
		}
	}
	startPromptMultilineInputFn = func(prompt string, _ string, _ bool, _ string) (string, bool, error) {
		promptCalls = append(promptCalls, prompt)
		assert.Contains(t, strings.ToLower(prompt), "description")
		return "Detailed creation description", false, nil
	}
	startSelectKindFn = func(_ []string, _ string) (string, bool, error) {
		return "Task", false, nil
	}
	startProjectIssueTypeNamesFn = func(_ string) ([]string, error) {
		return []string{"Task", "Story", "Bug"}, nil
	}
	startConfirmFn = func(_ string, _ bool) (bool, error) {
		return false, nil
	}
	newJiraClient = func(_ *viper.Viper) (jira.Jira, error) {
		return &mockJira{}, nil
	}

	cmd := newStartFlagProbeCommand(t)
	require.NoError(t, cmd.Flags().Set("interactive", "true"))

	err := collectInteractiveCreateFlags(cmd)
	require.NoError(t, err)
	assert.Equal(t, "Implement interactive flow", startCmdFlags.title)
	assert.Equal(t, "Detailed creation description", startCmdFlags.description)
	assert.Equal(t, "PROJ", startCmdFlags.project)
}

func TestCollectInteractiveCreateFlags_NonInteractiveTerminal(t *testing.T) {
	origFlags := startCmdFlags
	origTerminal := startIsInteractiveTerminalFn
	defer func() {
		startCmdFlags = origFlags
		startIsInteractiveTerminalFn = origTerminal
	}()

	startCmdFlags = startFlags{
		interactive: true,
	}
	startIsInteractiveTerminalFn = func() bool { return false }

	cmd := newStartFlagProbeCommand(t)
	require.NoError(t, cmd.Flags().Set("interactive", "true"))

	err := collectInteractiveCreateFlags(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interactive mode requires an interactive terminal")
}

func newStartFlagProbeCommand(t *testing.T) *cobra.Command {
	t.Helper()

	cmd := &cobra.Command{Use: "start"}
	cmd.Flags().String("title", "", "")
	cmd.Flags().String("description", "", "")
	cmd.Flags().String("project", "", "")
	cmd.Flags().String("kind", "Task", "")
	cmd.Flags().String("parent", "", "")
	cmd.Flags().String("epic", "", "")
	cmd.Flags().StringSlice("labels", []string{}, "")
	cmd.Flags().Bool("no-branch", false, "")
	cmd.Flags().Bool("interactive", false, "")
	return cmd
}

func TestResolveStartIssueAssignee_AssignsUnassignedIssue(t *testing.T) {
	statusCh := make(chan string, 32)
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)

	j := &mockJira{
		trackerIssue: &tracker.Issue{
			Key:      "TEST-123",
			Assignee: nil,
		},
		currentUser: &tracker.User{
			ID:   "user-1",
			Name: "Jane Doe",
		},
		assignIssue: &tracker.Issue{
			Key: "TEST-123",
			Assignee: &tracker.User{
				ID: "user-1",
			},
		},
	}

	resolveStartIssueAssignee(out, j, "TEST-123")

	assert.Equal(t, 1, j.assignIssueCalls)
	assert.Equal(t, "TEST-123", j.assignIssueIssueID)
	assert.Equal(t, "user-1", j.assignIssueUserID)
}

func TestResolveStartIssueAssignee_SkipsWhenAssignedToCurrentUser(t *testing.T) {
	statusCh := make(chan string, 32)
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)

	j := &mockJira{
		trackerIssue: &tracker.Issue{
			Key: "TEST-123",
			Assignee: &tracker.User{
				ID:   "user-1",
				Name: "Jane Doe",
			},
		},
		currentUser: &tracker.User{
			ID:   "user-1",
			Name: "Jane Doe",
		},
	}

	resolveStartIssueAssignee(out, j, "TEST-123")
	assert.Equal(t, 0, j.assignIssueCalls)
}

func TestResolveStartIssueAssignee_WarnsWhenAssignedToAnotherUser(t *testing.T) {
	statusCh := make(chan string, 32)
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)

	j := &mockJira{
		trackerIssue: &tracker.Issue{
			Key: "TEST-123",
			Assignee: &tracker.User{
				ID:   "user-2",
				Name: "John Other",
			},
		},
		currentUser: &tracker.User{
			ID:   "user-1",
			Name: "Jane Doe",
		},
	}

	resolveStartIssueAssignee(out, j, "TEST-123")
	assert.Equal(t, 0, j.assignIssueCalls)
}

func TestCreateNewIssue_PassesDescription(t *testing.T) {
	origFlags := startCmdFlags
	defer func() { startCmdFlags = origFlags }()

	startCmdFlags = startFlags{
		title:       "Implement flow",
		description: "Detailed issue description",
		project:     "TEST",
		kind:        "Task",
	}

	j := &mockJira{
		createIssue: &tracker.Issue{
			Key:     "TEST-123",
			Summary: "Implement flow",
		},
		jiraIssue: &jira.Issue{
			Key:     "TEST-123",
			Summary: "Implement flow",
		},
	}

	statusCh := make(chan string, 32)
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)

	issue, err := createNewIssue(out, j)
	require.NoError(t, err)
	require.NotNil(t, issue)
	assert.Equal(t, "Detailed issue description", j.createIssueInput.Description)
	assert.Equal(t, "Implement flow", j.createIssueInput.Title)
}

func TestCreateNewIssue_ResolvesAndPassesParentForSubtask(t *testing.T) {
	origFlags := startCmdFlags
	defer func() { startCmdFlags = origFlags }()

	startCmdFlags = startFlags{
		title:   "Implement flow",
		project: "TEST",
		kind:    "Subtask",
		parent:  "TEST-100",
	}

	j := &mockJira{
		trackerIssue: &tracker.Issue{
			Key:  "TEST-100",
			Type: tracker.IssueTypeStory,
		},
		createIssue: &tracker.Issue{
			Key:     "TEST-123",
			Summary: "Implement flow",
		},
		jiraIssue: &jira.Issue{
			Key:     "TEST-123",
			Summary: "Implement flow",
		},
	}

	statusCh := make(chan string, 32)
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)

	_, err := createNewIssue(out, j)
	require.NoError(t, err)
	assert.Equal(t, tracker.IssueTypeSubTask, j.createIssueInput.Type)
	assert.Equal(t, "Subtask", j.createIssueInput.TypeName)
	assert.Equal(t, "TEST-100", j.createIssueInput.ParentID)
}

func TestCreateNewIssue_ResolvesAndPassesEpic(t *testing.T) {
	origFlags := startCmdFlags
	defer func() { startCmdFlags = origFlags }()

	startCmdFlags = startFlags{
		title:   "Implement flow",
		project: "TEST",
		kind:    "Task",
		epic:    "TEST-200",
	}

	j := &mockJira{
		trackerIssue: &tracker.Issue{
			Key:  "TEST-200",
			Type: tracker.IssueTypeEpic,
		},
		createIssue: &tracker.Issue{
			Key:     "TEST-123",
			Summary: "Implement flow",
		},
		jiraIssue: &jira.Issue{
			Key:     "TEST-123",
			Summary: "Implement flow",
		},
	}

	statusCh := make(chan string, 32)
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)

	_, err := createNewIssue(out, j)
	require.NoError(t, err)
	assert.Equal(t, "TEST-200", j.createIssueInput.EpicID)
}

func TestCreateNewIssue_FailsWhenParentResolutionIsAmbiguousNonInteractive(t *testing.T) {
	origFlags := startCmdFlags
	defer func() { startCmdFlags = origFlags }()

	startCmdFlags = startFlags{
		title:   "Implement flow",
		project: "TEST",
		kind:    "Subtask",
		parent:  "auth",
	}

	j := &mockJira{
		searchIssues: []tracker.Issue{
			{Key: "TEST-100", Summary: "Auth parent"},
			{Key: "TEST-101", Summary: "Auth cleanup"},
		},
	}

	statusCh := make(chan string, 32)
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)

	_, err := createNewIssue(out, j)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple issues matched")
}

func TestResolveIssueLink_InteractiveAmbiguousUsesPicker(t *testing.T) {
	origRoot := fotingoConfig.GetString("jira.root")
	fotingoConfig.Set("jira.root", "https://tagoro9.atlassian.net")
	defer fotingoConfig.Set("jira.root", origRoot)

	origSelect := startSelectIssueLinkFn
	defer func() { startSelectIssueLinkFn = origSelect }()

	selected := tracker.Issue{Key: "TEST-101", Summary: "Auth cleanup"}
	startSelectIssueLinkFn = func(issues []tracker.Issue, _ string) (*tracker.Issue, error) {
		require.Len(t, issues, 2)
		return &selected, nil
	}

	j := &mockJira{
		searchIssues: []tracker.Issue{
			{Key: "TEST-100", Summary: "Auth parent"},
			{Key: "TEST-101", Summary: "Auth cleanup"},
		},
	}

	resolved, err := resolveIssueLink(j, "TEST", "auth", nil, true, "Pick issue")
	require.NoError(t, err)
	assert.Equal(t, "TEST-101", resolved)
}

func TestResolveIssueLink_MatchingJiraBrowseURL(t *testing.T) {
	origRoot := fotingoConfig.GetString("jira.root")
	fotingoConfig.Set("jira.root", "https://tagoro9.atlassian.net")
	defer fotingoConfig.Set("jira.root", origRoot)

	j := &mockJira{
		trackerIssue: &tracker.Issue{
			Key: "FOTINGO-17",
		},
	}

	resolved, err := resolveIssueLink(
		j,
		"FOTINGO",
		"https://tagoro9.atlassian.net/browse/FOTINGO-17",
		nil,
		false,
		"Pick issue",
	)
	require.NoError(t, err)
	assert.Equal(t, "FOTINGO-17", resolved)
	assert.Empty(t, j.searchIssuesQuery)
}

func TestResolveIssueLink_InteractiveAmbiguousEscPromptsToRefine(t *testing.T) {
	origSelect := startSelectIssueLinkFn
	origPrompt := startPromptInputFn
	defer func() {
		startSelectIssueLinkFn = origSelect
		startPromptInputFn = origPrompt
	}()

	selectCalls := 0
	startSelectIssueLinkFn = func(issues []tracker.Issue, _ string) (*tracker.Issue, error) {
		selectCalls++
		require.Len(t, issues, 2)
		return nil, nil // Simulate pressing Esc in picker.
	}

	startPromptInputFn = func(prompt string, _ string, required bool, initialValue string) (string, bool, error) {
		assert.Contains(t, strings.ToLower(prompt), "refine")
		assert.True(t, required)
		assert.Equal(t, "auth", initialValue)
		return "auth-api", false, nil
	}

	j := &mockJira{
		searchIssuesFn: func(_ string, query string, _ []tracker.IssueType, _ int) ([]tracker.Issue, error) {
			switch query {
			case "auth":
				return []tracker.Issue{
					{Key: "TEST-100", Summary: "Auth parent"},
					{Key: "TEST-101", Summary: "Auth cleanup"},
				}, nil
			case "auth-api":
				return []tracker.Issue{
					{Key: "TEST-101", Summary: "Auth cleanup"},
				}, nil
			default:
				return []tracker.Issue{}, nil
			}
		},
	}

	resolved, err := resolveIssueLink(j, "TEST", "auth", nil, true, "Pick issue")
	require.NoError(t, err)
	assert.Equal(t, "TEST-101", resolved)
	assert.Equal(t, 1, selectCalls)
}

func TestResolveIssueLink_InteractiveAmbiguousEscAndCancelledRefineCancels(t *testing.T) {
	origSelect := startSelectIssueLinkFn
	origPrompt := startPromptInputFn
	defer func() {
		startSelectIssueLinkFn = origSelect
		startPromptInputFn = origPrompt
	}()

	startSelectIssueLinkFn = func(_ []tracker.Issue, _ string) (*tracker.Issue, error) {
		return nil, nil
	}
	startPromptInputFn = func(_ string, _ string, _ bool, _ string) (string, bool, error) {
		return "", true, nil
	}

	j := &mockJira{
		searchIssues: []tracker.Issue{
			{Key: "TEST-100", Summary: "Auth parent"},
			{Key: "TEST-101", Summary: "Auth cleanup"},
		},
	}

	_, err := resolveIssueLink(j, "TEST", "auth", nil, true, "Pick issue")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestRunStartWithResult_FetchesDefaultBranchBeforeCreatingBranch(t *testing.T) {
	origFlags := startCmdFlags
	origNewJiraClient := newJiraClient
	origNewGitClient := newGitClient
	defer func() {
		startCmdFlags = origFlags
		newJiraClient = origNewJiraClient
		newGitClient = origNewGitClient
	}()

	startCmdFlags = startFlags{}
	j := &mockJira{
		trackerIssue: &tracker.Issue{
			Key: "TEST-123",
		},
		currentUser: &tracker.User{
			ID:   "user-1",
			Name: "Jane Doe",
		},
		setJiraIssueStatus: &jira.Issue{
			Key:     "TEST-123",
			Summary: "Implement fetch",
			Status:  string(jira.StatusInProgress),
			Type:    "Task",
		},
	}
	g := &mockGit{
		createBranchName: "f/test-123_implement_fetch",
	}

	newJiraClient = func(_ *viper.Viper) (jira.Jira, error) {
		return j, nil
	}
	newGitClient = func(_ *viper.Viper, _ *chan string) (git.Git, error) {
		return g, nil
	}

	statusCh := make(chan string, 32)
	defer close(statusCh)

	cmd := newStartFlagProbeCommand(t)
	result := runStartWithResult(cmd, &statusCh, "TEST-123")
	require.NoError(t, result.err)
	assert.Equal(t, 1, g.fetchDefaultCalls)
	assert.Equal(t, 1, g.createBranchCalls)
}

func TestRunStartWithResult_StopsWhenFetchDefaultBranchFails(t *testing.T) {
	origFlags := startCmdFlags
	origNewJiraClient := newJiraClient
	origNewGitClient := newGitClient
	defer func() {
		startCmdFlags = origFlags
		newJiraClient = origNewJiraClient
		newGitClient = origNewGitClient
	}()

	startCmdFlags = startFlags{}
	j := &mockJira{
		trackerIssue: &tracker.Issue{
			Key: "TEST-123",
		},
		currentUser: &tracker.User{
			ID:   "user-1",
			Name: "Jane Doe",
		},
		setJiraIssueStatus: &jira.Issue{
			Key:     "TEST-123",
			Summary: "Implement fetch",
			Status:  string(jira.StatusInProgress),
			Type:    "Task",
		},
	}
	g := &mockGit{
		fetchDefaultErr: errors.New("fetch failed"),
	}

	newJiraClient = func(_ *viper.Viper) (jira.Jira, error) {
		return j, nil
	}
	newGitClient = func(_ *viper.Viper, _ *chan string) (git.Git, error) {
		return g, nil
	}

	statusCh := make(chan string, 32)
	defer close(statusCh)

	cmd := newStartFlagProbeCommand(t)
	result := runStartWithResult(cmd, &statusCh, "TEST-123")
	require.Error(t, result.err)
	assert.Contains(t, result.err.Error(), "failed to create issue branch")
	assert.Equal(t, 1, g.fetchDefaultCalls)
	assert.Equal(t, 0, g.createBranchCalls)
}

func TestRunStartWithResult_AutoStashesWhenChangesExist(t *testing.T) {
	origFlags := startCmdFlags
	origNewJiraClient := newJiraClient
	origNewGitClient := newGitClient
	defer func() {
		startCmdFlags = origFlags
		newJiraClient = origNewJiraClient
		newGitClient = origNewGitClient
	}()

	startCmdFlags = startFlags{}
	j := &mockJira{
		trackerIssue: &tracker.Issue{
			Key: "TEST-123",
		},
		currentUser: &tracker.User{
			ID:   "user-1",
			Name: "Jane Doe",
		},
		setJiraIssueStatus: &jira.Issue{
			Key:     "TEST-123",
			Summary: "Auto stash issue",
			Status:  string(jira.StatusInProgress),
			Type:    "Task",
		},
	}
	g := &mockGit{
		hasChanges:       true,
		createBranchName: "f/test-123_auto_stash_issue",
	}

	newJiraClient = func(_ *viper.Viper) (jira.Jira, error) {
		return j, nil
	}
	newGitClient = func(_ *viper.Viper, _ *chan string) (git.Git, error) {
		return g, nil
	}

	statusCh := make(chan string, 32)
	defer close(statusCh)

	cmd := newStartFlagProbeCommand(t)
	result := runStartWithResult(cmd, &statusCh, "TEST-123")
	require.NoError(t, result.err)
	assert.Equal(t, 1, g.stashCalls)
	assert.Contains(t, g.stashMessage, "auto-stash")
	assert.Equal(t, 1, g.createBranchCalls)
}

func TestRunStartWithResult_ReturnsErrorWhenAutoStashFails(t *testing.T) {
	origFlags := startCmdFlags
	origNewJiraClient := newJiraClient
	origNewGitClient := newGitClient
	defer func() {
		startCmdFlags = origFlags
		newJiraClient = origNewJiraClient
		newGitClient = origNewGitClient
	}()

	startCmdFlags = startFlags{}
	j := &mockJira{
		trackerIssue: &tracker.Issue{
			Key: "TEST-123",
		},
		currentUser: &tracker.User{
			ID:   "user-1",
			Name: "Jane Doe",
		},
		setJiraIssueStatus: &jira.Issue{
			Key:     "TEST-123",
			Summary: "Auto stash issue",
			Status:  string(jira.StatusInProgress),
			Type:    "Task",
		},
	}
	g := &mockGit{
		hasChanges: true,
		stashErr:   errors.New("stash failed"),
	}

	newJiraClient = func(_ *viper.Viper) (jira.Jira, error) {
		return j, nil
	}
	newGitClient = func(_ *viper.Viper, _ *chan string) (git.Git, error) {
		return g, nil
	}

	statusCh := make(chan string, 32)
	defer close(statusCh)

	cmd := newStartFlagProbeCommand(t)
	result := runStartWithResult(cmd, &statusCh, "TEST-123")
	require.Error(t, result.err)
	assert.Contains(t, result.err.Error(), "failed to stash changes")
	assert.Equal(t, 1, g.stashCalls)
	assert.Equal(t, 0, g.createBranchCalls)
}
