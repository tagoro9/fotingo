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
		"",
		"<!-- fotingo:start fixed-issues -->",
		"",
		"<!-- fotingo:end fixed-issues -->",
		"",
		"**Changes**",
		"",
		"<!-- fotingo:start changes -->",
		"",
		"<!-- fotingo:end changes -->",
		"",
		"Outside note",
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
		"",
		"<!-- fotingo:end description -->",
		"",
		"<!-- fotingo:start fixed-issues -->",
		"",
		"<!-- fotingo:end fixed-issues -->",
		"",
		"**Changes**",
		"",
		"<!-- fotingo:start changes -->",
		"",
		"<!-- fotingo:end changes -->",
		"",
		"Outside note",
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
			Title:   "Edited title",
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
		assert.True(t, strings.HasPrefix(initialContent, "Existing title\n\n"))
		assert.Contains(t, initialContent, "<!-- fotingo:start summary -->\nfeature/editor-sync\n<!-- fotingo:end summary -->")
		assert.Contains(t, initialContent, "\nOutside note")
		edited := strings.Replace(initialContent, "Existing title", "Edited title", 1)
		edited = strings.Replace(edited, "feature/editor-sync", "edited summary", 1)
		edited = strings.Replace(edited, "Outside note", "Edited outside note", 1)
		return edited, nil
	}

	statusCh := make(chan string, 16)
	result := runReviewSync(&statusCh, true)

	require.NoError(t, result.err)
	require.NotNil(t, githubClient.lastUpdatePROptions.Body)
	expectedBody = strings.Replace(expectedBody, "feature/editor-sync", "edited summary", 1)
	expectedBody = strings.Replace(expectedBody, "Outside note", "Edited outside note", 1)
	require.NotNil(t, githubClient.lastUpdatePROptions.Title)
	assert.Equal(t, "Edited title", *githubClient.lastUpdatePROptions.Title)
	assert.Equal(t, expectedBody, *githubClient.lastUpdatePROptions.Body)
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
		assert.True(t, strings.HasPrefix(initialContent, "Old title\n\n"))
		return strings.Replace(initialContent, "Old title", "Edited title", 1), nil
	}

	statusCh := make(chan string, 16)
	result := runReviewSync(&statusCh, true)

	require.NoError(t, result.err)
	require.NotNil(t, githubClient.lastUpdatePROptions.Title)
	assert.Equal(t, "Edited title", *githubClient.lastUpdatePROptions.Title)
}

func TestRunReviewSync_OpensEditorByDefaultForChangesOnly(t *testing.T) {
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

	gitClient := &mockGit{
		currentBranch: "feature/changes-only",
		commitsSince: []git.Commit{
			{Message: "feat: refresh changes", Additions: 2, Deletions: 0},
		},
	}
	existingBody := renderReviewTemplateBodyWithOverrides("feature/changes-only", nil, nil, nil, nil, "", "")
	expectedBody := renderReviewTemplateBodyWithOverrides(
		"feature/changes-only",
		nil,
		nil,
		[]git.Commit{{Message: "feat: refresh changes", Additions: 2, Deletions: 0}},
		nil,
		"",
		"",
	)
	expectedBody = strings.Replace(expectedBody, "<!-- fotingo:end changes -->", "\nExtra note\n<!-- fotingo:end changes -->", 1)
	githubClient := &mockGitHub{
		doesPRExist: true,
		existingPR: &github.PullRequest{
			Number:  21,
			Title:   "Current title",
			Body:    existingBody,
			HTMLURL: "https://github.com/test/repo/pull/21",
		},
		updatePR: &github.PullRequest{
			Number:  21,
			Title:   "Current title",
			Body:    expectedBody,
			HTMLURL: "https://github.com/test/repo/pull/21",
		},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}
	openEditorFn = func(initialContent string) (string, error) {
		assert.True(t, strings.HasPrefix(initialContent, "Current title\n\n"))
		return strings.Replace(initialContent, "<!-- fotingo:end changes -->", "\nExtra note\n<!-- fotingo:end changes -->", 1), nil
	}

	statusCh := make(chan string, 16)
	result := runReviewSync(&statusCh, true)

	require.NoError(t, result.err)
	require.NotNil(t, githubClient.lastUpdatePROptions.Body)
	assert.Contains(t, *githubClient.lastUpdatePROptions.Body, "Extra note")
	assert.Nil(t, githubClient.lastUpdatePROptions.Title)
	assert.Equal(t, *githubClient.lastUpdatePROptions.Body, result.pr.Body)
}

func TestRunReviewSync_TransitionsOnlyNewlyDetectedIssues(t *testing.T) {
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

	gitClient := &mockGit{
		currentBranch:     "feature/new-issues",
		commitsSince:      []git.Commit{{Message: "feat: add work", Additions: 1, Deletions: 0}},
		issuesFromCommits: []string{"FOTINGO-2", "FOTINGO-3"},
		issueId:           "FOTINGO-1",
	}
	existingBody := strings.Join([]string{
		"**Summary**",
		"",
		"<!-- fotingo:start summary -->",
		"FOTINGO-1: Existing",
		"<!-- fotingo:end summary -->",
		"",
		"**Description**",
		"",
		"<!-- fotingo:start description -->",
		"desc",
		"<!-- fotingo:end description -->",
		"",
		"<!-- fotingo:start fixed-issues -->",
		"Fixes FOTINGO-1",
		"<!-- fotingo:end fixed-issues -->",
		"",
		"**Changes**",
		"",
		"<!-- fotingo:start changes -->",
		"* old",
		"<!-- fotingo:end changes -->",
	}, "\n")
	expectedBody := strings.Join([]string{
		"**Summary**",
		"",
		"<!-- fotingo:start summary -->",
		"FOTINGO-1: Existing",
		"<!-- fotingo:end summary -->",
		"",
		"**Description**",
		"",
		"<!-- fotingo:start description -->",
		"desc",
		"<!-- fotingo:end description -->",
		"",
		"<!-- fotingo:start fixed-issues -->",
		"Fixes [FOTINGO-1](https://jira.example.com/browse/FOTINGO-1)\nFixes [FOTINGO-2](https://jira.example.com/browse/FOTINGO-2)\nFixes [FOTINGO-3](https://jira.example.com/browse/FOTINGO-3)",
		"<!-- fotingo:end fixed-issues -->",
		"",
		"**Changes**",
		"",
		"<!-- fotingo:start changes -->",
		"* feat: add work (+1/-0)",
		"<!-- fotingo:end changes -->",
	}, "\n")
	githubClient := &mockGitHub{
		doesPRExist: true,
		existingPR: &github.PullRequest{
			Number:  22,
			Title:   "Existing title",
			Body:    existingBody,
			HTMLURL: "https://github.com/test/repo/pull/22",
		},
		updatePR: &github.PullRequest{
			Number:  22,
			Title:   "Existing title",
			Body:    expectedBody,
			HTMLURL: "https://github.com/test/repo/pull/22",
		},
	}
	jiraClient := &mockJira{
		issueURL:           "https://jira.example.com/browse/%s",
		jiraIssue:          &jira.Issue{Key: "FOTINGO-1", Summary: "Existing", Description: "desc"},
		setJiraIssueStatus: &jira.Issue{Key: "FOTINGO-2", Status: "In Review"},
	}

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return gitClient, nil
	}
	newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
		return githubClient, nil
	}
	newJiraClient = func(cfg *viper.Viper) (jira.Jira, error) {
		return jiraClient, nil
	}

	statusCh := make(chan string, 16)
	result := runReviewSync(&statusCh, false)

	require.NoError(t, result.err)
	assert.Equal(t, []string{"FOTINGO-2", "FOTINGO-3"}, jiraClient.setJiraIssueStatusIDs)
	assert.Equal(t, []string{"FOTINGO-2", "FOTINGO-3"}, jiraClient.addCommentIssueIDs)
	require.Len(t, jiraClient.addCommentBodies, 2)
	assert.Contains(t, jiraClient.addCommentBodies[0], "https://github.com/test/repo/pull/22")
}
