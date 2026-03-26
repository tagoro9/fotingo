package commands

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalreview "github.com/tagoro9/fotingo/internal/commands/review"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/ui"
)

type fakeReviewPicker struct {
	selected *ui.PickerItem
	err      error
}

func (f fakeReviewPicker) Run() (*ui.PickerItem, error) {
	return f.selected, f.err
}

func TestReviewFlags(t *testing.T) {
	t.Parallel()

	// Verify the review command has expected flags
	flags := reviewCmd.Flags()

	// Check flag existence and defaults
	draftFlag := flags.Lookup("draft")
	assert.NotNil(t, draftFlag, "draft flag should exist")
	assert.Equal(t, "d", draftFlag.Shorthand)
	assert.Equal(t, "false", draftFlag.DefValue)

	labelsFlag := flags.Lookup("labels")
	assert.NotNil(t, labelsFlag, "labels flag should exist")
	assert.Equal(t, "l", labelsFlag.Shorthand)

	reviewersFlag := flags.Lookup("reviewers")
	assert.NotNil(t, reviewersFlag, "reviewers flag should exist")
	assert.Equal(t, "r", reviewersFlag.Shorthand)

	assigneeFlag := flags.Lookup("assignee")
	assert.NotNil(t, assigneeFlag, "assignee flag should exist")
	assert.Equal(t, "a", assigneeFlag.Shorthand)

	simpleFlag := flags.Lookup("simple")
	assert.NotNil(t, simpleFlag, "simple flag should exist")
	assert.Equal(t, "s", simpleFlag.Shorthand)
	assert.Equal(t, "false", simpleFlag.DefValue)

	titleFlag := flags.Lookup("title")
	assert.NotNil(t, titleFlag, "title flag should exist")
	assert.Equal(t, "", titleFlag.DefValue)

	descriptionFlag := flags.Lookup("description")
	assert.NotNil(t, descriptionFlag, "description flag should exist")

	templateSummaryFlag := flags.Lookup("template-summary")
	assert.NotNil(t, templateSummaryFlag, "template-summary flag should exist")

	templateDescriptionFlag := flags.Lookup("template-description")
	assert.NotNil(t, templateDescriptionFlag, "template-description flag should exist")
}

func TestReviewCmdUse(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "review", reviewCmd.Use)
	assert.NotEmpty(t, reviewCmd.Short)
	assert.NotEmpty(t, reviewCmd.Long)
}

func TestDefaultPRTemplate(t *testing.T) {
	t.Parallel()

	// Verify the default PR template contains expected placeholders
	assert.Contains(t, defaultPRTemplate, "{summary}")
	assert.Contains(t, defaultPRTemplate, "{description}")
	assert.Contains(t, defaultPRTemplate, "{fixedIssues}")
	assert.Contains(t, defaultPRTemplate, "{changes}")
	assert.Contains(t, defaultPRTemplate, "{fotingo.banner}")
	assert.Contains(t, defaultPRTemplate, "**Description**")
	assert.Contains(t, defaultPRTemplate, "**Changes**")
}

func TestReviewResultStruct(t *testing.T) {
	t.Parallel()

	// Test that reviewResult struct works as expected
	result := reviewResult{
		labels:    []string{"bug", "priority"},
		reviewers: []string{"alice", "bob"},
		assignees: []string{"alice"},
		existed:   false,
		err:       nil,
	}

	assert.Equal(t, []string{"bug", "priority"}, result.labels)
	assert.Equal(t, []string{"alice", "bob"}, result.reviewers)
	assert.Equal(t, []string{"alice"}, result.assignees)
	assert.False(t, result.existed)
	assert.Nil(t, result.err)
}

func TestShouldOpenReviewEditor(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()

	origTTY := isInputTerminalFn
	defer func() { isInputTerminalFn = origTTY }()

	isInputTerminalFn = func() bool { return true }
	Global.JSON = false
	Global.Yes = false
	assert.True(t, shouldOpenReviewEditor(true))

	Global.Yes = true
	assert.False(t, shouldOpenReviewEditor(true))

	Global.Yes = false
	Global.JSON = true
	assert.False(t, shouldOpenReviewEditor(true))

	Global.JSON = false
	isInputTerminalFn = func() bool { return false }
	assert.False(t, shouldOpenReviewEditor(true))
	assert.False(t, shouldOpenReviewEditor(false))
}

func TestResolveReviewPRBody_OpensEditorWhenInteractive(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origTTY := isInputTerminalFn
	origEditor := openEditorFn
	defer func() {
		isInputTerminalFn = origTTY
		openEditorFn = origEditor
	}()

	Global.JSON = false
	Global.Yes = false
	isInputTerminalFn = func() bool { return true }

	opened := false
	openEditorFn = func(initialContent string) (string, error) {
		opened = true
		assert.Contains(t, initialContent, "**Description**")
		return "edited body", nil
	}

	statusCh := make(chan string, 4)
	body, err := resolveReviewPRBody(&statusCh, "feature/test", nil, nil, nil, nil, true)
	require.NoError(t, err)
	assert.Equal(t, "edited body", body)
	assert.True(t, opened)
}

func TestResolveReviewPRBody_SkipsEditorInNonInteractiveMode(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origEditor := openEditorFn
	defer func() { openEditorFn = origEditor }()

	Global.Yes = true
	openEditorFn = func(initialContent string) (string, error) {
		return "", errors.New("should not be called")
	}

	statusCh := make(chan string, 4)
	body, err := resolveReviewPRBody(&statusCh, "feature/test", nil, nil, nil, nil, true)
	require.NoError(t, err)
	assert.Contains(t, body, "**Changes**")
}

func TestResolveReviewPRBody_DescriptionOverrideTakesPrecedenceOverTemplateOverrides(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	reviewCmdFlags.description = "Raw body override"
	reviewCmdFlags.templateSummary = "Custom summary"
	reviewCmdFlags.templateDescription = "Custom template description"

	statusCh := make(chan string, 4)
	body, err := resolveReviewPRBody(&statusCh, "feature/test", nil, nil, nil, nil, true)
	require.NoError(t, err)
	assert.Equal(t, "Raw body override", body)
}

func TestResolveReviewPRBody_EditorFailure(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origTTY := isInputTerminalFn
	origEditor := openEditorFn
	defer func() {
		isInputTerminalFn = origTTY
		openEditorFn = origEditor
	}()

	Global.JSON = false
	Global.Yes = false
	isInputTerminalFn = func() bool { return true }
	openEditorFn = func(initialContent string) (string, error) {
		return "", errors.New("editor failed")
	}

	statusCh := make(chan string, 4)
	_, err := resolveReviewPRBody(&statusCh, "feature/test", nil, nil, nil, nil, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to edit pull request description")
}

func TestResolveReviewPRBody_InteractivePathUsesRuntimeHandoff(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origTTY := isInputTerminalFn
	origEditor := openEditorFn
	origProcessFn := openEditorProcessFn
	defer func() {
		isInputTerminalFn = origTTY
		openEditorFn = origEditor
		openEditorProcessFn = origProcessFn
	}()

	Global.JSON = false
	Global.Yes = false
	isInputTerminalFn = func() bool { return true }
	openEditorFn = openEditorWithRuntime
	openEditorProcessFn = func(initialContent string) (string, error) {
		return initialContent + "\nEdited via runtime handoff", nil
	}

	controller := &mockTerminalController{}
	statusCh := make(chan string, 4)
	var body string
	err := withActiveTerminal(controller, func() error {
		var runErr error
		body, runErr = resolveReviewPRBody(&statusCh, "feature/test", nil, nil, nil, nil, true)
		return runErr
	})

	require.NoError(t, err)
	assert.Contains(t, body, "Edited via runtime handoff")
	assert.Equal(t, 1, controller.releaseCalls)
	assert.Equal(t, 1, controller.restoreCalls)
}

func resetReviewFlags() {
	reviewCmdFlags = reviewFlags{}
}

func mockGitHubCallIndex(calls []string, target string) int {
	for idx, call := range calls {
		if call == target {
			return idx
		}
	}
	return -1
}

func TestReviewModel(t *testing.T) {
	t.Parallel()

	model := commandruntime.NewStatusModel(commandruntime.StatusModelOptions{})

	// Init should return a tick command for the spinner
	cmd := model.Init()
	assert.NotNil(t, cmd)

	// Test View returns a Bubble Tea view
	view := model.View()
	assert.IsType(t, tea.View{}, view)
}

func TestBuildReviewTemplateData_IssueSummaryIsKeyPrefixedAndTruncated(t *testing.T) {
	longSummary := strings.Repeat("x", 130)
	issue := &jira.Issue{
		Key:         "PROJ-123",
		Summary:     longSummary,
		Description: "Issue description",
	}

	data := buildReviewTemplateData("feature/branch", issue, nil, nil, nil)
	expected := internalreview.TakePrefix("PROJ-123: "+longSummary, 100)

	assert.Equal(t, expected, data["summary"])
	assert.Equal(t, "Issue description", data["description"])
}

func TestBuildReviewTemplateData_CommitFallbackSummaryDescriptionAndChanges(t *testing.T) {
	commits := []git.Commit{
		{Message: "feat: latest work\n\nlatest body", Additions: 9, Deletions: 4},
		{Message: "feat: oldest work\n\noldest body", Additions: 5, Deletions: 1},
	}

	data := buildReviewTemplateData("feature/branch", nil, nil, commits, nil)

	assert.Equal(t, "feat: oldest work", data["summary"])
	assert.Equal(t, "oldest body", data["description"])
	assert.Equal(
		t,
		"* feat: oldest work (+5/-1)\n* feat: latest work (+9/-4)",
		data["changes"],
	)
}

func TestBuildReviewTemplateData_BranchFallbackSummaryAndEmptyDescription(t *testing.T) {
	data := buildReviewTemplateData("feature/fallback", nil, nil, nil, nil)

	assert.Equal(t, "feature/fallback", data["summary"])
	assert.Equal(t, "", data["description"])
	assert.Equal(t, "", data["changes"])
}

func TestBuildReviewTemplateData_NormalizesIssueDescriptionLineEndings(t *testing.T) {
	issue := &jira.Issue{
		Key:         "PROJ-1",
		Summary:     "Summary",
		Description: "line1\r\nline2\r\nline3",
	}

	data := buildReviewTemplateData("feature/branch", issue, nil, nil, nil)
	assert.Equal(t, "line1\nline2\nline3", data["description"])
}

func TestBuildReviewTemplateData_AppliesTemplateSummaryOverride(t *testing.T) {
	defer resetReviewFlags()
	reviewCmdFlags.templateSummary = "Custom summary"

	issue := &jira.Issue{
		Key:         "PROJ-1",
		Summary:     "Issue summary",
		Description: "Issue description",
	}

	data := buildReviewTemplateData("feature/branch", issue, nil, nil, nil)
	assert.Equal(t, "Custom summary", data["summary"])
	assert.Equal(t, "Issue description", data["description"])
}

func TestBuildReviewTemplateData_AppliesTemplateDescriptionOverride(t *testing.T) {
	defer resetReviewFlags()
	reviewCmdFlags.templateDescription = "line1\r\nline2"

	issue := &jira.Issue{
		Key:         "PROJ-1",
		Summary:     "Issue summary",
		Description: "Issue description",
	}

	data := buildReviewTemplateData("feature/branch", issue, nil, nil, nil)
	assert.Equal(t, "PROJ-1: Issue summary", data["summary"])
	assert.Equal(t, "line1\nline2", data["description"])
}

func TestBuildReviewTemplateData_RendersAllLinkedIssues(t *testing.T) {
	issue := &jira.Issue{
		Key:         "FOTINGO-10",
		Summary:     "Issue summary",
		Description: "Issue description",
	}

	data := buildReviewTemplateData(
		"feature/branch",
		issue,
		nil,
		nil,
		[]string{"FOTINGO-10", "FOTINGO-1", "FOTINGO-2"},
	)

	assert.Equal(t, "Fixes FOTINGO-10\nFixes FOTINGO-1\nFixes FOTINGO-2", data["fixedIssues"])
}

func TestDeriveEditorPRTitle_FirstLineOnly(t *testing.T) {
	title := internalreview.DeriveEditorTitle("  First line title  \nSecond line")
	assert.Equal(t, "First line title", title)
}

func TestDeriveEditorPRTitle_EmptyFirstLine(t *testing.T) {
	title := internalreview.DeriveEditorTitle("\nSecond line")
	assert.Equal(t, "", title)
}

func TestDeriveReviewPRTitle_UsesEditorFirstLineWhenInteractive(t *testing.T) {
	defer resetReviewFlags()

	issue := &jira.Issue{Key: "PROJ-1", Summary: "Issue summary"}
	title := deriveReviewPRTitle("feature/branch", issue, "Editor title", true)

	assert.Equal(t, "Editor title", title)
}

func TestDeriveReviewPRTitle_FallsBackWhenEditorFirstLineEmpty(t *testing.T) {
	defer resetReviewFlags()

	issue := &jira.Issue{Key: "PROJ-1", Summary: "Issue summary"}
	title := deriveReviewPRTitle("feature/branch", issue, "", true)

	assert.Equal(t, "[PROJ-1] Issue summary", title)
}

func TestDeriveReviewPRTitle_ExplicitTitleOverridesEditorContent(t *testing.T) {
	defer resetReviewFlags()

	reviewCmdFlags.title = "Explicit title"
	issue := &jira.Issue{Key: "PROJ-1", Summary: "Issue summary"}
	title := deriveReviewPRTitle("feature/branch", issue, "Editor title", true)

	assert.Equal(t, "Explicit title", title)
}

func TestSplitReviewEditorContent_SeparatesTitleAndBody(t *testing.T) {
	title, body := internalreview.SplitEditorContent("PR title\n\nBody line 1\nBody line 2")
	assert.Equal(t, "PR title", title)
	assert.Equal(t, "Body line 1\nBody line 2", body)
}

func TestSplitReviewEditorContent_EmptyFirstLineUsesRemainingBody(t *testing.T) {
	title, body := internalreview.SplitEditorContent("\nBody line 1")
	assert.Equal(t, "", title)
	assert.Equal(t, "Body line 1", body)
}

func TestReviewWrapperHelpers(t *testing.T) {
	issue := &jira.Issue{
		Key:         "PROJ-1",
		Summary:     "Summary",
		Description: "Line 1\r\nLine 2",
	}
	commits := []git.Commit{
		{Message: "newer title\n\nnewer body", Additions: 8, Deletions: 3},
		{Message: "older title\n\nolder body", Additions: 2, Deletions: 1},
	}

	assert.Equal(t, "[PROJ-1] Summary", internalreview.BuildDefaultTitle("feature/proj-1", issue))
	assert.Equal(t, "PROJ-1: Summary", internalreview.DeriveSummary("feature/proj-1", issue, commits))
	assert.Equal(t, "Line 1\nLine 2", internalreview.DeriveDescription(issue, commits, ""))

	assert.Equal(
		t,
		"* older title (+2/-1)\n* newer title (+8/-3)",
		internalreview.FormatChanges(commits),
	)
	header, body := internalreview.OldestCommitHeaderAndBody(commits)
	assert.Equal(t, "older title", header)
	assert.Equal(t, "older body", body)

	splitHeader, splitBody := internalreview.SplitCommitMessage("header\r\n\r\nbody")
	assert.Equal(t, "header", splitHeader)
	assert.Equal(t, "body", splitBody)

	assert.Equal(t, "a\nb", internalreview.NormalizeLineEndings("a\r\nb"))
}

func TestFormatReviewReviewersWarning_Known422Message(t *testing.T) {
	message := formatReviewReviewersWarning(
		errors.New("POST /requested_reviewers: 422 Review cannot be requested"),
	)
	assert.Contains(t, strings.ToLower(message), "review cannot be requested")
	assert.NotContains(t, strings.ToLower(message), "post /requested_reviewers")
}

func TestResolveReviewTokens_SuccessAndNoMatch(t *testing.T) {
	options := []reviewMatchOption{
		{
			Resolved: "alice",
			Label:    "alice",
			Fields:   []string{"alice"},
			Kind:     reviewMatchKindUser,
		},
	}

	resolved, err := internalreview.ResolveTokens("reviewer", []string{"alice", "alice"}, options, true, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"alice"}, resolved)

	_, err = internalreview.ResolveTokens("reviewer", []string{"unknown"}, options, true, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no reviewer matches found for "unknown"`)
}

func TestResolveReviewTokens_AmbiguousWithoutPrompt(t *testing.T) {
	origCanPrompt := canPromptForReviewDisambiguationFn
	t.Cleanup(func() { canPromptForReviewDisambiguationFn = origCanPrompt })
	canPromptForReviewDisambiguationFn = func() bool { return false }

	options := []reviewMatchOption{
		{
			Resolved: "alice",
			Label:    "alice",
			Fields:   []string{"ali"},
			Kind:     reviewMatchKindUser,
		},
		{
			Resolved: "alicia",
			Label:    "alicia",
			Fields:   []string{"ali"},
			Kind:     reviewMatchKindUser,
		},
	}

	_, err := internalreview.ResolveTokens("reviewer", []string{"ali"}, options, false, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous reviewer")
}

func TestCanPromptForReviewDisambiguation(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()

	origTTY := isInputTerminalFn
	t.Cleanup(func() { isInputTerminalFn = origTTY })

	Global.Yes = false
	Global.JSON = false
	isInputTerminalFn = func() bool { return true }
	assert.True(t, canPromptForReviewDisambiguation())

	Global.Yes = true
	assert.False(t, canPromptForReviewDisambiguation())

	Global.Yes = false
	Global.JSON = true
	assert.False(t, canPromptForReviewDisambiguation())
}

func TestFindReviewOptionByResolved(t *testing.T) {
	options := []reviewMatchOption{
		{Resolved: "alice", Label: "alice", Kind: reviewMatchKindUser},
	}

	match, ok := internalreview.FindOptionByResolved(options, "alice")
	require.True(t, ok)
	assert.Equal(t, "alice", match.Resolved)

	_, ok = internalreview.FindOptionByResolved(options, "bob")
	assert.False(t, ok)
}

func TestReviewMatchingWrapperHelpers(t *testing.T) {
	options := []reviewMatchOption{
		{Resolved: "alice", Label: "alice", Detail: "Alice", Fields: []string{"alice", "alice smith"}, Kind: reviewMatchKindUser},
		{Resolved: "acme/platform", Label: "acme/platform", Detail: "Platform", Fields: []string{"acme/platform", "platform"}, Kind: reviewMatchKindTeam},
	}

	matches := internalreview.FindTokenMatches("ali", options)
	require.NotEmpty(t, matches)
	assert.Equal(t, "alice", matches[0].Resolved)

	completion := internalreview.FindTokenMatchesForCompletion("pla", options)
	require.NotEmpty(t, completion)

	alternatives := internalreview.FindTokenAlternatives("pltf", options, 5)
	require.NotEmpty(t, alternatives)

	score, ok := internalreview.ScoreTokenMatch("alice", []string{"alice smith"})
	assert.True(t, ok)
	assert.GreaterOrEqual(t, score, 0)

	distance, ok := internalreview.ScoreTokenDistance("pltf", []string{"platform"})
	assert.True(t, ok)
	assert.GreaterOrEqual(t, distance, 0)

	assert.NotEmpty(t, internalreview.FieldTokenCandidates("acme/platform team"))
	assert.Equal(t, []string{"x", "y"}, internalreview.DedupeStringsPreserveOrder([]string{"x", "x", "y"}))
	assert.Equal(
		t,
		[]reviewMatchOption{{Resolved: "alice", Kind: reviewMatchKindUser}},
		internalreview.DedupeMatchesPreserveOrder([]reviewMatchOption{
			{Resolved: "alice", Kind: reviewMatchKindUser},
			{Resolved: "alice", Kind: reviewMatchKindUser},
		}),
	)

	assert.Contains(t, internalreview.FormatMatchOption(options[0]), "alice")
	assert.Contains(t, internalreview.FormatMatchList(options), "acme/platform")

	current := github.User{Login: "alice", Name: ""}
	candidate := github.User{Login: "alice", Name: "Alice"}
	preferred := internalreview.PreferParticipantUser(current, candidate)
	assert.Equal(t, "Alice", preferred.Name)
}

func TestResolveReviewTokenMatches_Wrapper(t *testing.T) {
	origCanPrompt := canPromptForReviewDisambiguationFn
	origPicker := pickReviewMatchWithPickerFn
	t.Cleanup(func() {
		canPromptForReviewDisambiguationFn = origCanPrompt
		pickReviewMatchWithPickerFn = origPicker
	})

	options := []reviewMatchOption{
		{Resolved: "alice", Label: "alice", Fields: []string{"ali"}, Kind: reviewMatchKindUser},
		{Resolved: "alicia", Label: "alicia", Fields: []string{"ali"}, Kind: reviewMatchKindUser},
	}

	canPromptForReviewDisambiguationFn = func() bool { return true }
	pickReviewMatchWithPickerFn = func(kind string, token string, matches []reviewMatchOption) (string, error) {
		assert.Equal(t, "reviewer", kind)
		assert.Equal(t, "ali", token)
		require.Len(t, matches, 2)
		return "alice", nil
	}

	resolved, err := internalreview.ResolveTokenMatches(
		"reviewer",
		[]string{"ali"},
		options,
		true,
		func(kind string, token string, matches []reviewMatchOption) (string, error) {
			return pickReviewMatchWithPickerFn(kind, token, matches)
		},
	)
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	assert.Equal(t, "alice", resolved[0].Resolved)
}

func TestPickReviewMatchWithPicker(t *testing.T) {
	origFactory := newReviewPickerProgramFn
	origHandoff := reviewRunInteractiveProcessWithTerminalHandoffFn
	t.Cleanup(func() {
		newReviewPickerProgramFn = origFactory
		reviewRunInteractiveProcessWithTerminalHandoffFn = origHandoff
	})

	reviewRunInteractiveProcessWithTerminalHandoffFn = func(run func() error) error {
		return run()
	}

	newReviewPickerProgramFn = func(title string, items []ui.PickerItem) reviewPicker {
		require.Contains(t, title, "reviewer")
		require.Len(t, items, 1)
		selected := ui.PickerItem{ID: items[0].ID}
		return fakeReviewPicker{selected: &selected}
	}

	selected, err := pickReviewMatchWithPicker(
		"reviewer",
		"alice",
		[]reviewMatchOption{{Resolved: "alice", Label: "alice"}},
	)
	require.NoError(t, err)
	assert.Equal(t, "alice", selected)
}

func TestToTeamSlugs(t *testing.T) {
	slugs := internalreview.ToTeamSlugs([]string{"acme/platform", "ops", "acme/platform", "acme/security"})
	assert.Equal(t, []string{"platform", "ops", "security"}, slugs)
}

func TestRunReviewWithOptions_UsesGitDefaultBranchAsPRBase(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origNewGitClient := newGitClient
	origNewGitHubClient := newGitHubClient
	defer func() {
		newGitClient = origNewGitClient
		newGitHubClient = origNewGitHubClient
	}()

	reviewCmdFlags.simple = true
	reviewCmdFlags.description = "Pre-filled body"

	gitClient := &mockGit{
		currentBranch:     "feature/default-base",
		branchExistRemote: true,
		defaultBranch:     "release",
	}
	githubClient := &mockGitHub{
		createPR: &github.PullRequest{
			Number:  1,
			HTMLURL: "https://github.com/test/repo/pull/1",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}

	statusCh := make(chan string, 16)
	result := runReviewWithResultWithOptions(&statusCh, false)

	require.NoError(t, result.err)
	assert.Equal(t, "release", githubClient.lastCreatePROptions.Base)
	assert.Equal(t, "feature/default-base", githubClient.lastCreatePROptions.Head)
}

func TestRunReviewWithOptions_FailsWhenDefaultBranchResolutionFails(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origNewGitClient := newGitClient
	origNewGitHubClient := newGitHubClient
	defer func() {
		newGitClient = origNewGitClient
		newGitHubClient = origNewGitHubClient
	}()

	reviewCmdFlags.simple = true
	reviewCmdFlags.description = "Pre-filled body"

	gitClient := &mockGit{
		currentBranch:     "feature/default-base-error",
		branchExistRemote: true,
		defaultBranchErr:  errors.New("remote HEAD unavailable"),
	}
	githubClient := &mockGitHub{
		createPR: &github.PullRequest{
			Number:  1,
			HTMLURL: "https://github.com/test/repo/pull/1",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}

	statusCh := make(chan string, 16)
	result := runReviewWithResultWithOptions(&statusCh, false)

	require.Error(t, result.err)
	assert.Contains(t, result.err.Error(), "failed to resolve default branch")
	assert.Nil(t, result.pr)
	assert.Equal(t, "", githubClient.lastCreatePROptions.Base)
}

func TestResolveReviewLabels_ExactAndSubstringMatches(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	ghClient := &mockGitHub{
		labels: []github.Label{
			{Name: "bug", Description: "Something is broken", Color: "ff0000"},
			{Name: "documentation", Description: "Docs updates", Color: "00ff00"},
		},
	}

	resolved, missing, err := resolveReviewLabels(ghClient, []string{"bug", "doc"})
	require.NoError(t, err)
	assert.Equal(t, []string{"bug", "documentation"}, resolved)
	assert.Empty(t, missing)
}

func TestResolveReviewReviewers_SubstringMatchesCollaboratorName(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	ghClient := &mockGitHub{
		collaborators: []github.User{
			{Login: "alice", Name: "Alice Developer"},
			{Login: "bob", Name: "Bob Reviewer"},
		},
	}

	resolved, teamResolved, warnings, err := resolveReviewReviewers(ghClient, []string{"alice dev"})
	require.NoError(t, err)
	assert.Equal(t, []string{"alice"}, resolved)
	assert.Empty(t, teamResolved)
	assert.Empty(t, warnings)
}

func TestResolveReviewReviewers_DisplayNamePrefixMatchesDifferentLogin(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	ghClient := &mockGitHub{
		collaborators: []github.User{
			{Login: "yprk", Name: "YoungJun Park"},
		},
	}

	resolved, teamResolved, warnings, err := resolveReviewReviewers(ghClient, []string{"young"})
	require.NoError(t, err)
	assert.Equal(t, []string{"yprk"}, resolved)
	assert.Empty(t, teamResolved)
	assert.Empty(t, warnings)
}

func TestBuildReviewParticipantOptions_PrefersNamedDuplicateUser(t *testing.T) {
	ghClient := &mockGitHub{
		collaborators: []github.User{
			{Login: "yprk", Name: ""},
		},
		orgMembers: []github.User{
			{Login: "yprk", Name: "YoungJun Park"},
		},
	}

	options, warnings, err := internalreview.BuildParticipantOptions(ghClient)
	require.NoError(t, err)
	assert.Empty(t, warnings)

	var yprkOption *reviewMatchOption
	for i := range options {
		if options[i].Resolved == "yprk" {
			yprkOption = &options[i]
			break
		}
	}

	require.NotNil(t, yprkOption)
	assert.Equal(t, "YoungJun Park", yprkOption.Detail)
	assert.Contains(t, yprkOption.Fields, "YoungJun Park")
}

func TestResolveReviewLabels_AmbiguousInteractiveUsesPicker(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	origCanPrompt := canPromptForReviewDisambiguationFn
	origPicker := pickReviewMatchWithPickerFn
	defer func() {
		canPromptForReviewDisambiguationFn = origCanPrompt
		pickReviewMatchWithPickerFn = origPicker
	}()

	canPromptForReviewDisambiguationFn = func() bool { return true }
	pickReviewMatchWithPickerFn = func(kind string, token string, matches []reviewMatchOption) (string, error) {
		require.Equal(t, "label", kind)
		require.Equal(t, "bug-", token)
		require.Len(t, matches, 2)
		return "bug-a", nil
	}

	ghClient := &mockGitHub{
		labels: []github.Label{
			{Name: "bug-a", Description: "Bug label A", Color: "ff0000"},
			{Name: "bug-b", Description: "Bug label B", Color: "00ff00"},
		},
	}

	resolved, missing, err := resolveReviewLabels(ghClient, []string{"bug-"})
	require.NoError(t, err)
	assert.Equal(t, []string{"bug-a"}, resolved)
	assert.Empty(t, missing)
}

func TestResolveReviewReviewers_AmbiguousNonInteractiveFails(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	origCanPrompt := canPromptForReviewDisambiguationFn
	defer func() { canPromptForReviewDisambiguationFn = origCanPrompt }()
	canPromptForReviewDisambiguationFn = func() bool { return false }

	ghClient := &mockGitHub{
		collaborators: []github.User{
			{Login: "alice1", Name: "Alice One"},
			{Login: "alice2", Name: "Alice Two"},
		},
	}

	_, _, _, err := resolveReviewReviewers(ghClient, []string{"alice"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous reviewer")
}

func TestResolveReviewReviewers_AmbiguousNonInteractiveIncludesNames(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	origCanPrompt := canPromptForReviewDisambiguationFn
	defer func() { canPromptForReviewDisambiguationFn = origCanPrompt }()
	canPromptForReviewDisambiguationFn = func() bool { return false }

	ghClient := &mockGitHub{
		collaborators: []github.User{
			{Login: "young1", Name: "Young One"},
			{Login: "young2", Name: "Young Two"},
		},
	}

	_, _, _, err := resolveReviewReviewers(ghClient, []string{"young"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous reviewer")
	assert.Contains(t, err.Error(), "young1 (Young One)")
}

func TestResolveReviewLabels_NoMatchFails(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	ghClient := &mockGitHub{
		labels: []github.Label{
			{Name: "bug", Description: "Bug label", Color: "ff0000"},
		},
	}

	resolved, missing, err := resolveReviewLabels(ghClient, []string{"frontend"})
	require.NoError(t, err)
	assert.Empty(t, resolved)
	assert.Equal(t, []string{"frontend"}, missing)
}

func TestResolveReviewAssignees_NoMatchIncludesAlternatives(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	ghClient := &mockGitHub{
		collaborators: []github.User{
			{Login: "yprk", Name: "YoungJun Park"},
		},
	}

	_, _, err := resolveReviewAssignees(ghClient, []string{"zzzz"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no assignee matches found")
	assert.Contains(t, err.Error(), "Closest matches:")
	assert.Contains(t, err.Error(), "yprk (YoungJun Park)")
}

func TestRunReviewWithOptions_UsesResolvedLabelsAndReviewers(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origNewGitClient := newGitClient
	origNewGitHubClient := newGitHubClient
	defer func() {
		newGitClient = origNewGitClient
		newGitHubClient = origNewGitHubClient
	}()

	reviewCmdFlags.simple = true
	reviewCmdFlags.description = "Pre-filled body"
	reviewCmdFlags.labels = []string{"bu"}
	reviewCmdFlags.reviewers = []string{"alice dev"}

	gitClient := &mockGit{
		currentBranch:     "feature/resolution",
		branchExistRemote: true,
		defaultBranch:     "main",
	}
	githubClient := &mockGitHub{
		labels: []github.Label{
			{Name: "bug", Description: "Bug label", Color: "ff0000"},
			{Name: "documentation", Description: "Docs", Color: "00ff00"},
		},
		collaborators: []github.User{
			{Login: "alice", Name: "Alice Developer"},
		},
		createPR: &github.PullRequest{
			Number:  1,
			HTMLURL: "https://github.com/test/repo/pull/1",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}

	statusCh := make(chan string, 16)
	result := runReviewWithResultWithOptions(&statusCh, false)

	require.NoError(t, result.err)
	assert.Equal(t, []string{"bug"}, result.labels)
	assert.Equal(t, []string{"alice"}, result.reviewers)
	assert.Equal(t, []string{"bug"}, githubClient.lastAddedLabels)
	assert.Equal(t, []string{"alice"}, githubClient.lastRequestedReviewers)
}

func TestRunReviewWithOptions_ResolvesMetadataBeforePRCreation(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origNewGitClient := newGitClient
	origNewGitHubClient := newGitHubClient
	defer func() {
		newGitClient = origNewGitClient
		newGitHubClient = origNewGitHubClient
	}()

	reviewCmdFlags.simple = true
	reviewCmdFlags.description = "Pre-filled body"
	reviewCmdFlags.labels = []string{"bu"}
	reviewCmdFlags.reviewers = []string{"alice dev"}

	gitClient := &mockGit{
		currentBranch:     "feature/review-order",
		branchExistRemote: true,
		defaultBranch:     "main",
	}
	githubClient := &mockGitHub{
		labels: []github.Label{
			{Name: "bug", Description: "Bug label", Color: "ff0000"},
		},
		collaborators: []github.User{
			{Login: "alice", Name: "Alice Developer"},
		},
		createPR: &github.PullRequest{
			Number:  1,
			HTMLURL: "https://github.com/test/repo/pull/1",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}

	statusCh := make(chan string, 16)
	result := runReviewWithResultWithOptions(&statusCh, false)

	require.NoError(t, result.err)
	createPRIndex := mockGitHubCallIndex(githubClient.calls, "create_pr")
	getLabelsIndex := mockGitHubCallIndex(githubClient.calls, "get_labels")
	getCollaboratorsIndex := mockGitHubCallIndex(githubClient.calls, "get_collaborators")
	requestReviewersIndex := mockGitHubCallIndex(githubClient.calls, "request_reviewers")
	addLabelsIndex := mockGitHubCallIndex(githubClient.calls, "add_labels")

	require.GreaterOrEqual(t, createPRIndex, 0)
	require.GreaterOrEqual(t, getLabelsIndex, 0)
	require.GreaterOrEqual(t, getCollaboratorsIndex, 0)
	require.GreaterOrEqual(t, requestReviewersIndex, 0)
	require.GreaterOrEqual(t, addLabelsIndex, 0)
	assert.Greater(t, createPRIndex, getLabelsIndex)
	assert.Greater(t, createPRIndex, getCollaboratorsIndex)
	assert.Greater(t, requestReviewersIndex, createPRIndex)
	assert.Greater(t, addLabelsIndex, createPRIndex)
}

func TestRunReviewWithOptions_ResolveReviewersFailureSkipsPRCreation(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origNewGitClient := newGitClient
	origNewGitHubClient := newGitHubClient
	defer func() {
		newGitClient = origNewGitClient
		newGitHubClient = origNewGitHubClient
	}()

	reviewCmdFlags.simple = true
	reviewCmdFlags.description = "Pre-filled body"
	reviewCmdFlags.reviewers = []string{"alice"}

	gitClient := &mockGit{
		currentBranch:     "feature/review-resolve-fail",
		branchExistRemote: true,
		defaultBranch:     "main",
	}
	githubClient := &mockGitHub{
		createPR: &github.PullRequest{
			Number:  1,
			HTMLURL: "https://github.com/test/repo/pull/1",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}

	statusCh := make(chan string, 16)
	result := runReviewWithResultWithOptions(&statusCh, false)

	require.Error(t, result.err)
	assert.Contains(t, result.err.Error(), "failed to resolve reviewers")
	assert.Nil(t, result.pr)
	assert.Equal(t, -1, mockGitHubCallIndex(githubClient.calls, "create_pr"))
}

func TestResolveReviewReviewers_ResolvesTeamTargets(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	ghClient := &mockGitHub{
		orgMembers: []github.User{
			{Login: "alice", Name: "Alice Developer"},
		},
		teams: []github.Team{
			{Organization: "acme", Slug: "platform", Name: "Platform Team", Description: "Core platform"},
		},
	}

	users, teams, warnings, err := resolveReviewReviewers(ghClient, []string{"alice", "acme/platform"})
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Equal(t, []string{"alice"}, users)
	assert.Equal(t, []string{"acme/platform"}, teams)
}

func TestResolveReviewAssignees_RejectsTeamMatch(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	ghClient := &mockGitHub{
		teams: []github.Team{
			{Organization: "acme", Slug: "platform", Name: "Platform Team"},
		},
	}

	_, _, err := resolveReviewAssignees(ghClient, []string{"acme/platform"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be used as an assignee")
}

func TestRunReviewWithOptions_AssignsResolvedUsers(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origNewGitClient := newGitClient
	origNewGitHubClient := newGitHubClient
	defer func() {
		newGitClient = origNewGitClient
		newGitHubClient = origNewGitHubClient
	}()

	reviewCmdFlags.simple = true
	reviewCmdFlags.description = "Pre-filled body"
	reviewCmdFlags.assignees = []string{"alice dev"}

	gitClient := &mockGit{
		currentBranch:     "feature/assign",
		branchExistRemote: true,
		defaultBranch:     "main",
	}
	githubClient := &mockGitHub{
		orgMembers: []github.User{
			{Login: "alice", Name: "Alice Developer"},
		},
		createPR: &github.PullRequest{
			Number:  1,
			HTMLURL: "https://github.com/test/repo/pull/1",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}

	statusCh := make(chan string, 16)
	result := runReviewWithResultWithOptions(&statusCh, false)

	require.NoError(t, result.err)
	assert.Equal(t, []string{"alice"}, result.assignees)
	assert.Equal(t, []string{"alice"}, githubClient.lastAssignedUsers)
}

func TestRunReviewWithOptions_MissingLabelsDoNotFail(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origNewGitClient := newGitClient
	origNewGitHubClient := newGitHubClient
	defer func() {
		newGitClient = origNewGitClient
		newGitHubClient = origNewGitHubClient
	}()

	reviewCmdFlags.simple = true
	reviewCmdFlags.description = "Pre-filled body"
	reviewCmdFlags.labels = []string{"bug", "missing-label"}

	gitClient := &mockGit{
		currentBranch:     "feature/labels",
		branchExistRemote: true,
		defaultBranch:     "main",
	}
	githubClient := &mockGitHub{
		labels: []github.Label{
			{Name: "bug", Description: "Bug label", Color: "ff0000"},
		},
		createPR: &github.PullRequest{
			Number:  1,
			HTMLURL: "https://github.com/test/repo/pull/1",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}

	statusCh := make(chan string, 16)
	result := runReviewWithResultWithOptions(&statusCh, false)

	require.NoError(t, result.err)
	assert.Equal(t, []string{"bug"}, result.labels)
	assert.Equal(t, []string{"bug"}, githubClient.lastAddedLabels)
}
