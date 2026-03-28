package commands

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	internalreview "github.com/tagoro9/fotingo/internal/commands/review"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/jira"
)

func TestRunReviewSync_UpdatesSelectedSectionsOnly(t *testing.T) {
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

	reviewSyncCmdFlags.sections = []string{internalreview.ManagedSectionChanges}

	gitClient := &mockGit{
		currentBranch: "feature/sync-body",
		commitsSince: []git.Commit{
			{Message: "feat: refresh changes\n\nbody", Additions: 2, Deletions: 1},
		},
	}

	existingBody := strings.Join([]string{
		"Intro text",
		"**Summary**",
		"",
		"<!-- fotingo:start summary -->",
		"old summary",
		"<!-- fotingo:end summary -->",
		"",
		"Keep this note",
		"",
		"**Description**",
		"",
		"<!-- fotingo:start description -->",
		"old description",
		"<!-- fotingo:end description -->",
		"",
		"<!-- fotingo:start fixed-issues -->",
		"Fixes OLD-1",
		"<!-- fotingo:end fixed-issues -->",
		"",
		"**Changes**",
		"",
		"<!-- fotingo:start changes -->",
		"* old change (+1/-0)",
		"<!-- fotingo:end changes -->",
		"",
		"Footer text",
	}, "\n")
	expectedBody := strings.Join([]string{
		"Intro text",
		"**Summary**",
		"",
		"<!-- fotingo:start summary -->",
		"old summary",
		"<!-- fotingo:end summary -->",
		"",
		"Keep this note",
		"",
		"**Description**",
		"",
		"<!-- fotingo:start description -->",
		"old description",
		"<!-- fotingo:end description -->",
		"",
		"<!-- fotingo:start fixed-issues -->",
		"Fixes OLD-1",
		"<!-- fotingo:end fixed-issues -->",
		"",
		"**Changes**",
		"",
		"<!-- fotingo:start changes -->",
		"* feat: refresh changes (+2/-1)",
		"<!-- fotingo:end changes -->",
		"",
		"Footer text",
	}, "\n")
	githubClient := &mockGitHub{
		doesPRExist: true,
		existingPR: &github.PullRequest{
			Number:  42,
			Title:   "Manual title",
			Body:    existingBody,
			HTMLURL: "https://github.com/test/repo/pull/42",
		},
		updatePR: &github.PullRequest{
			Number:  42,
			Title:   "Manual title",
			Body:    expectedBody,
			HTMLURL: "https://github.com/test/repo/pull/42",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}

	statusCh := make(chan string, 16)
	result := runReviewSync(&statusCh, false)

	require.NoError(t, result.err)
	require.NotNil(t, githubClient.lastUpdatePROptions.Body)
	assert.Equal(t, expectedBody, *githubClient.lastUpdatePROptions.Body)
	assert.Nil(t, githubClient.lastUpdatePROptions.Title)
	assert.Equal(t, expectedBody, result.pr.Body)
}

func TestRunReviewSync_SyncsTitleOnlyWhenRequested(t *testing.T) {
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

	reviewSyncCmdFlags.syncTitle = true

	gitClient := &mockGit{
		currentBranch: "feature/new-title",
	}
	body := renderReviewTemplateBodyWithOverrides("feature/new-title", nil, nil, nil, nil, "", "")
	githubClient := &mockGitHub{
		doesPRExist: true,
		existingPR: &github.PullRequest{
			Number:  7,
			Title:   "Manual title",
			Body:    body,
			HTMLURL: "https://github.com/test/repo/pull/7",
		},
		updatePR: &github.PullRequest{
			Number:  7,
			Title:   "feature/new-title",
			Body:    body,
			HTMLURL: "https://github.com/test/repo/pull/7",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}

	statusCh := make(chan string, 16)
	result := runReviewSync(&statusCh, false)

	require.NoError(t, result.err)
	require.NotNil(t, githubClient.lastUpdatePROptions.Title)
	assert.Equal(t, "feature/new-title", *githubClient.lastUpdatePROptions.Title)
	assert.Nil(t, githubClient.lastUpdatePROptions.Body)
	assert.Equal(t, "feature/new-title", result.pr.Title)
}

func TestRunReviewSync_RejectsSummaryOverrideWithoutSummarySection(t *testing.T) {
	defer resetReviewFlags()

	reviewSyncCmdFlags.sections = []string{internalreview.ManagedSectionChanges}
	reviewSyncCmdFlags.templateSummary = "Custom summary"

	statusCh := make(chan string, 1)
	result := runReviewSync(&statusCh, false)

	require.Error(t, result.err)
	assert.Contains(t, result.err.Error(), `--template-summary requires syncing the "summary" section`)
}

func TestRunReviewSync_FailsWhenRequestedMarkersMissing(t *testing.T) {
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

	reviewSyncCmdFlags.sections = []string{internalreview.ManagedSectionChanges}

	gitClient := &mockGit{
		currentBranch: "feature/missing-markers",
		commitsSince: []git.Commit{
			{Message: "feat: missing markers", Additions: 1, Deletions: 0},
		},
	}
	githubClient := &mockGitHub{
		doesPRExist: true,
		existingPR: &github.PullRequest{
			Number:  9,
			Title:   "PR title",
			Body:    "**Summary**\n\n<!-- fotingo:start summary -->\nold summary\n<!-- fotingo:end summary -->",
			HTMLURL: "https://github.com/test/repo/pull/9",
		},
		updatePR: &github.PullRequest{
			Number:  9,
			Title:   "PR title",
			HTMLURL: "https://github.com/test/repo/pull/9",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}

	statusCh := make(chan string, 16)
	result := runReviewSync(&statusCh, false)

	require.Error(t, result.err)
	assert.Contains(t, result.err.Error(), `missing fotingo markers for section "changes"`)
	assert.NotContains(t, githubClient.calls, "update_pr")
}

func TestRunReviewSync_SummaryOverrideDoesNotRequireJira(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origNewGitClient := newGitClient
	origNewGitHubClient := newGitHubClient
	origNewJiraClient := newJiraClient
	defer func() {
		newGitClient = origNewGitClient
		newGitHubClient = origNewGitHubClient
		newJiraClient = origNewJiraClient
	}()

	reviewSyncCmdFlags.sections = []string{internalreview.ManagedSectionSummary}
	reviewSyncCmdFlags.templateSummary = "Custom summary"

	gitClient := &mockGit{
		currentBranch: "feature/with-issue",
		issueId:       "FOTINGO-123",
	}
	existingBody := renderReviewTemplateBodyWithOverrides("feature/with-issue", nil, nil, nil, nil, "", "")
	expectedBody := renderReviewTemplateBodyWithOverrides("feature/with-issue", nil, nil, nil, nil, "Custom summary", "")
	githubClient := &mockGitHub{
		doesPRExist: true,
		existingPR: &github.PullRequest{
			Number:  11,
			Title:   "Existing title",
			Body:    existingBody,
			HTMLURL: "https://github.com/test/repo/pull/11",
		},
		updatePR: &github.PullRequest{
			Number:  11,
			Title:   "Existing title",
			Body:    expectedBody,
			HTMLURL: "https://github.com/test/repo/pull/11",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}
	newJiraClient = func(cfg *viper.Viper) (jira.Jira, error) {
		return nil, errors.New("jira should not be initialized")
	}

	statusCh := make(chan string, 16)
	result := runReviewSync(&statusCh, false)

	require.NoError(t, result.err)
	require.NotNil(t, githubClient.lastUpdatePROptions.Body)
	assert.Equal(t, expectedBody, *githubClient.lastUpdatePROptions.Body)
}

func TestRunReviewSync_OpensEditorForManagedContentWhenInteractive(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origNewGitClient := newGitClient
	origNewGitHubClient := newGitHubClient
	origTTY := isInputTerminalFn
	origEditor := openEditorFn
	defer func() {
		newGitClient = origNewGitClient
		newGitHubClient = origNewGitHubClient
		isInputTerminalFn = origTTY
		openEditorFn = origEditor
	}()

	Global.JSON = false
	Global.Yes = false
	isInputTerminalFn = func() bool { return true }

	reviewSyncCmdFlags.sections = []string{internalreview.ManagedSectionSummary}

	gitClient := &mockGit{
		currentBranch: "feature/editor-sync",
	}
	existingBody := strings.Join([]string{
		"**Summary**",
		"",
		"<!-- fotingo:start summary -->",
		"old summary",
		"<!-- fotingo:end summary -->",
		"",
		"**Description**",
		"",
		"<!-- fotingo:start description -->",
		"manual description",
		"<!-- fotingo:end description -->",
	}, "\n")
	expectedBody := strings.Join([]string{
		"**Summary**",
		"",
		"<!-- fotingo:start summary -->",
		"edited summary",
		"<!-- fotingo:end summary -->",
		"",
		"**Description**",
		"",
		"<!-- fotingo:start description -->",
		"manual description",
		"<!-- fotingo:end description -->",
	}, "\n")
	githubClient := &mockGitHub{
		doesPRExist: true,
		existingPR: &github.PullRequest{
			Number:  12,
			Title:   "Existing title",
			Body:    existingBody,
			HTMLURL: "https://github.com/test/repo/pull/12",
		},
		updatePR: &github.PullRequest{
			Number:  12,
			Title:   "Existing title",
			Body:    expectedBody,
			HTMLURL: "https://github.com/test/repo/pull/12",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}
	openEditorFn = func(initialContent string) (string, error) {
		assert.Contains(t, initialContent, "<!-- fotingo:start summary -->\nfeature/editor-sync\n<!-- fotingo:end summary -->")
		assert.NotContains(t, initialContent, "Existing title\n\n")
		return strings.Replace(initialContent, "feature/editor-sync", "edited summary", 1), nil
	}

	statusCh := make(chan string, 16)
	result := runReviewSync(&statusCh, true)

	require.NoError(t, result.err)
	require.NotNil(t, githubClient.lastUpdatePROptions.Body)
	assert.Equal(t, expectedBody, *githubClient.lastUpdatePROptions.Body)
	assert.Nil(t, githubClient.lastUpdatePROptions.Title)
}

func TestRunReviewSync_SkipsEditorWhenOverridesProvideManagedContent(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origNewGitClient := newGitClient
	origNewGitHubClient := newGitHubClient
	origTTY := isInputTerminalFn
	origEditor := openEditorFn
	defer func() {
		newGitClient = origNewGitClient
		newGitHubClient = origNewGitHubClient
		isInputTerminalFn = origTTY
		openEditorFn = origEditor
	}()

	Global.JSON = false
	Global.Yes = false
	isInputTerminalFn = func() bool { return true }

	reviewSyncCmdFlags.sections = []string{
		internalreview.ManagedSectionSummary,
		internalreview.ManagedSectionDescription,
	}
	reviewSyncCmdFlags.templateSummary = "Custom summary"
	reviewSyncCmdFlags.templateDescription = "Custom description"

	gitClient := &mockGit{
		currentBranch: "feature/no-editor",
	}
	existingBody := renderReviewTemplateBodyWithOverrides("feature/no-editor", nil, nil, nil, nil, "", "")
	expectedBody := renderReviewTemplateBodyWithOverrides("feature/no-editor", nil, nil, nil, nil, "Custom summary", "Custom description")
	githubClient := &mockGitHub{
		doesPRExist: true,
		existingPR: &github.PullRequest{
			Number:  13,
			Title:   "Existing title",
			Body:    existingBody,
			HTMLURL: "https://github.com/test/repo/pull/13",
		},
		updatePR: &github.PullRequest{
			Number:  13,
			Title:   "Existing title",
			Body:    expectedBody,
			HTMLURL: "https://github.com/test/repo/pull/13",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}
	openEditorFn = func(initialContent string) (string, error) {
		return "", errors.New("editor should not open")
	}

	statusCh := make(chan string, 16)
	result := runReviewSync(&statusCh, true)

	require.NoError(t, result.err)
	require.NotNil(t, githubClient.lastUpdatePROptions.Body)
	assert.Equal(t, expectedBody, *githubClient.lastUpdatePROptions.Body)
}

func TestRunReviewSync_OpensEditorForDerivedTitleWhenRequested(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()
	defer resetReviewFlags()
	withDefaultReviewTemplateResolver(t)

	origNewGitClient := newGitClient
	origNewGitHubClient := newGitHubClient
	origTTY := isInputTerminalFn
	origEditor := openEditorFn
	defer func() {
		newGitClient = origNewGitClient
		newGitHubClient = origNewGitHubClient
		isInputTerminalFn = origTTY
		openEditorFn = origEditor
	}()

	Global.JSON = false
	Global.Yes = false
	isInputTerminalFn = func() bool { return true }

	reviewSyncCmdFlags.sections = []string{internalreview.ManagedSectionChanges}
	reviewSyncCmdFlags.syncTitle = true

	gitClient := &mockGit{
		currentBranch: "feature/title-editor",
		commitsSince: []git.Commit{
			{Message: "feat: refresh changes", Additions: 1, Deletions: 0},
		},
	}
	existingBody := renderReviewTemplateBodyWithOverrides("feature/title-editor", nil, nil, nil, nil, "", "")
	expectedBody := renderReviewTemplateBodyWithOverrides(
		"feature/title-editor",
		nil,
		nil,
		[]git.Commit{{Message: "feat: refresh changes", Additions: 1, Deletions: 0}},
		nil,
		"",
		"",
	)
	githubClient := &mockGitHub{
		doesPRExist: true,
		existingPR: &github.PullRequest{
			Number:  14,
			Title:   "Old title",
			Body:    existingBody,
			HTMLURL: "https://github.com/test/repo/pull/14",
		},
		updatePR: &github.PullRequest{
			Number:  14,
			Title:   "Edited title",
			Body:    expectedBody,
			HTMLURL: "https://github.com/test/repo/pull/14",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}
	openEditorFn = func(initialContent string) (string, error) {
		assert.True(t, strings.HasPrefix(initialContent, "feature/title-editor\n\n"))
		return strings.Replace(initialContent, "feature/title-editor", "Edited title", 1), nil
	}

	statusCh := make(chan string, 16)
	result := runReviewSync(&statusCh, true)

	require.NoError(t, result.err)
	require.NotNil(t, githubClient.lastUpdatePROptions.Title)
	assert.Equal(t, "Edited title", *githubClient.lastUpdatePROptions.Title)
}
