package review

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	hub "github.com/google/go-github/v84/github"
	giturl "github.com/kubescape/go-git-url"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/auth"
	"github.com/tagoro9/fotingo/internal/config"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

type reviewTimingEmitter struct {
	debug []string
}

func (e *reviewTimingEmitter) Info(string, i18n.Key, ...any) {}
func (e *reviewTimingEmitter) InfoRaw(string, string)        {}
func (e *reviewTimingEmitter) Verbose(i18n.Key, ...any)      {}
func (e *reviewTimingEmitter) Debugf(format string, args ...any) {
	e.debug = append(e.debug, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func TestWorkflowRunnerRun_ValidatesDeps(t *testing.T) {
	runner := WorkflowRunner{}
	result := runner.Run(nil, nil, false)
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "NewGitClient")
}

func TestWorkflowRunnerRun_WrapsGitInitError(t *testing.T) {
	runner := WorkflowRunner{
		Config: viper.New(),
		Localize: func(key i18n.Key, args ...any) string {
			if key == i18n.ReviewWrapInitGit {
				return "failed to init git"
			}
			return i18n.T(key, args...)
		},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return nil, errors.New("git unavailable")
			},
			NewGitHubClient: func(git.Git, *viper.Viper) (github.Github, error) { return nil, nil },
			NewJiraClient:   func(*viper.Viper) (jira.Jira, error) { return nil, nil },
			FetchBranchIssue: func(jira.Jira, string, func(string, ...any)) (*jira.Issue, error) {
				return nil, nil
			},
			ResolvePRBody: func(*chan string, string, *jira.Issue, jira.Jira, []git.Commit, []string, bool) (string, error) {
				return "", nil
			},
			ResolveLabels: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			ResolveReviewers: func(github.Github, []string) ([]string, []string, []string, error) {
				return nil, nil, nil, nil
			},
			ResolveAssignees: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			SplitEditorContent:     SplitEditorContent,
			DerivePRTitle:          func(string, *jira.Issue, string, bool) string { return "title" },
			ToTeamSlugs:            ToTeamSlugs,
			FormatReviewersWarning: func(err error) string { return err.Error() },
			ShouldOpenReviewEditor: func(bool) bool { return false },
		},
	}

	result := runner.Run(nil, nil, false)
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "failed to init git")
	assert.Contains(t, result.Err.Error(), "git unavailable")
}

func TestShouldCollectReviewCommits(t *testing.T) {
	assert.True(t, shouldCollectReviewCommits(""))
	assert.True(t, shouldCollectReviewCommits("   "))
	assert.False(t, shouldCollectReviewCommits("manual body"))
	assert.False(t, shouldCollectReviewCommits("-"))
}

func TestLogReviewPhaseTiming_EmitsDebugTiming(t *testing.T) {
	emitter := &reviewTimingEmitter{}
	logReviewPhaseTiming(emitter, "phase_x", time.Now().Add(-150*time.Millisecond))

	require.Len(t, emitter.debug, 1)
	assert.Contains(t, emitter.debug[0], "review timing phase=phase_x duration=")
}

type reviewEvent struct {
	key  i18n.Key
	args []any
}

type reviewRawEvent struct {
	emoji   string
	message string
}

type reviewCollectingEmitter struct {
	events    []reviewEvent
	rawEvents []reviewRawEvent
}

func (e *reviewCollectingEmitter) Info(_ string, key i18n.Key, args ...any) {
	e.events = append(e.events, reviewEvent{key: key, args: append([]any{}, args...)})
}

func (e *reviewCollectingEmitter) InfoRaw(emoji string, message string) {
	e.rawEvents = append(e.rawEvents, reviewRawEvent{emoji: emoji, message: message})
}
func (e *reviewCollectingEmitter) Verbose(i18n.Key, ...any) {
}
func (e *reviewCollectingEmitter) Debugf(string, ...any) {}

type workflowSuccessMockGit struct{}

func (workflowSuccessMockGit) GetRemote() (giturl.IGitURL, error) { return nil, nil }
func (workflowSuccessMockGit) GetCurrentBranch() (string, error)  { return "feature/test", nil }
func (workflowSuccessMockGit) GetIssueId() (string, error)        { return "TEST-123", nil }
func (workflowSuccessMockGit) CreateIssueBranch(*jira.Issue) (string, error) {
	return "", nil
}
func (workflowSuccessMockGit) CreateIssueWorktreeBranch(*jira.Issue, git.WorktreeOptions) (string, string, error) {
	return "", "", nil
}
func (workflowSuccessMockGit) Push() error { return nil }
func (workflowSuccessMockGit) StashChanges(string) error {
	return nil
}
func (workflowSuccessMockGit) PopStash() error { return nil }
func (workflowSuccessMockGit) HasUncommittedChanges() (bool, error) {
	return false, nil
}
func (workflowSuccessMockGit) GetCommitsSince(string) ([]git.Commit, error) {
	return nil, nil
}
func (workflowSuccessMockGit) DoesBranchExistInRemote(string) (bool, error) {
	return true, nil
}
func (workflowSuccessMockGit) GetDefaultBranch() (string, error) { return "main", nil }
func (workflowSuccessMockGit) FetchDefaultBranch() error         { return nil }
func (workflowSuccessMockGit) GetCommitsSinceDefaultBranch() ([]git.Commit, error) {
	return nil, nil
}
func (workflowSuccessMockGit) GetIssuesFromCommits([]git.Commit) []string { return nil }
func (workflowSuccessMockGit) GetConfig() *viper.Viper                    { return nil }
func (workflowSuccessMockGit) SaveConfig(string, any) error               { return nil }

type workflowSuccessMockGitHub struct {
	pr              *github.PullRequest
	parentPR        *github.PullRequest
	stackMembers    []github.PullRequest
	bodyUpdates     []github.PullRequestBodyUpdate
	createPROptions github.CreatePROptions
}

func (m workflowSuccessMockGitHub) Authenticate() (*auth.AccessToken, error) {
	return &auth.AccessToken{}, nil
}
func (m workflowSuccessMockGitHub) GetConfig() *viper.Viper      { return nil }
func (m workflowSuccessMockGitHub) SaveConfig(string, any) error { return nil }
func (m workflowSuccessMockGitHub) GetPullRequestUrl() (string, error) {
	return "", nil
}
func (m workflowSuccessMockGitHub) GetCurrentUser() (*hub.User, error) { return nil, nil }
func (m *workflowSuccessMockGitHub) CreatePullRequest(opts github.CreatePROptions) (*github.PullRequest, error) {
	m.createPROptions = opts
	return m.pr, nil
}
func (m workflowSuccessMockGitHub) UpdatePullRequest(int, github.UpdatePROptions) (*github.PullRequest, error) {
	return m.pr, nil
}
func (m workflowSuccessMockGitHub) GetPullRequestDiscussion(int) (*github.PullRequestDiscussion, error) {
	return nil, nil
}
func (m workflowSuccessMockGitHub) GetLabels() ([]github.Label, error) { return nil, nil }
func (m workflowSuccessMockGitHub) AddLabelsToPR(int, []string) error  { return nil }
func (m workflowSuccessMockGitHub) GetCollaborators() ([]github.User, error) {
	return nil, nil
}
func (m workflowSuccessMockGitHub) GetOrgMembers() ([]github.User, error) { return nil, nil }
func (m workflowSuccessMockGitHub) GetTeams() ([]github.Team, error)      { return nil, nil }
func (m workflowSuccessMockGitHub) RequestReviewers(int, []string, []string) error {
	return nil
}
func (m workflowSuccessMockGitHub) RemoveReviewers(int, []string, []string) error {
	return nil
}
func (m workflowSuccessMockGitHub) AssignUsersToPR(int, []string) error { return nil }
func (m workflowSuccessMockGitHub) RemoveAssigneesFromPR(int, []string) error {
	return nil
}
func (m workflowSuccessMockGitHub) MarkPullRequestReadyForReview(string) error { return nil }
func (m workflowSuccessMockGitHub) DoesPRExistForBranch(string) (bool, *github.PullRequest, error) {
	return false, nil, nil
}
func (m workflowSuccessMockGitHub) FindOpenPullRequestByHeadBranch(string) (*github.PullRequest, bool, error) {
	if m.parentPR == nil {
		return nil, false, nil
	}
	return m.parentPR, true, nil
}
func (m workflowSuccessMockGitHub) ListOpenPullRequestsByStackID(string) ([]github.PullRequest, error) {
	return append([]github.PullRequest(nil), m.stackMembers...), nil
}
func (m *workflowSuccessMockGitHub) UpdatePullRequestBodies(updates []github.PullRequestBodyUpdate) ([]*github.PullRequest, error) {
	m.bodyUpdates = append([]github.PullRequestBodyUpdate(nil), updates...)
	updated := make([]*github.PullRequest, 0, len(updates))
	for _, update := range updates {
		pr := &github.PullRequest{Number: update.Number, Body: update.Body, HTMLURL: fmt.Sprintf("https://github.com/test/repo/pull/%d", update.Number), State: "open"}
		if m.pr != nil && update.Number == m.pr.Number {
			pr.Title = m.pr.Title
			pr.HeadRef = m.pr.HeadRef
			pr.BaseRef = m.pr.BaseRef
		}
		updated = append(updated, pr)
	}
	return updated, nil
}
func (m workflowSuccessMockGitHub) CreateRelease(github.CreateReleaseOptions) (*github.Release, error) {
	return nil, nil
}

type stackWorkflowMockGit struct {
	workflowSuccessMockGit
	commitsSinceRef string
	defaultCommits  bool
}

func (g *stackWorkflowMockGit) GetCommitsSince(ref string) ([]git.Commit, error) {
	g.commitsSinceRef = ref
	return []git.Commit{{Message: "feat: child delta", Additions: 1}}, nil
}

func (g *stackWorkflowMockGit) GetCommitsSinceDefaultBranch() ([]git.Commit, error) {
	g.defaultCommits = true
	return []git.Commit{{Message: "feat: default delta", Additions: 2}}, nil
}

type workflowSuccessMockJira struct{}

func (workflowSuccessMockJira) Authenticate() (*auth.AccessToken, error) {
	return &auth.AccessToken{}, nil
}
func (workflowSuccessMockJira) GetConfig() *viper.Viper                { return nil }
func (workflowSuccessMockJira) SaveConfig(string, any) error           { return nil }
func (workflowSuccessMockJira) Name() string                           { return "Jira" }
func (workflowSuccessMockJira) GetCurrentUser() (*tracker.User, error) { return nil, nil }
func (workflowSuccessMockJira) GetUserOpenIssues() ([]tracker.Issue, error) {
	return nil, nil
}
func (workflowSuccessMockJira) GetIssue(string) (*tracker.Issue, error) { return nil, nil }
func (workflowSuccessMockJira) AssignIssue(string, string) (*tracker.Issue, error) {
	return nil, nil
}
func (workflowSuccessMockJira) CreateIssue(tracker.CreateIssueInput) (*tracker.Issue, error) {
	return nil, nil
}
func (workflowSuccessMockJira) GetProjectIssueTypes(string) ([]tracker.ProjectIssueType, error) {
	return nil, nil
}
func (workflowSuccessMockJira) SetIssueStatus(string, tracker.IssueStatus) (*tracker.Issue, error) {
	return nil, nil
}
func (workflowSuccessMockJira) AddComment(string, string) error { return nil }
func (workflowSuccessMockJira) CreateRelease(tracker.CreateReleaseInput) (*tracker.Release, error) {
	return nil, nil
}
func (workflowSuccessMockJira) SetFixVersion([]string, *tracker.Release) error { return nil }
func (workflowSuccessMockJira) IsValidIssueID(string) bool                     { return true }
func (workflowSuccessMockJira) GetIssueURL(string) string                      { return "" }
func (workflowSuccessMockJira) GetIssueUrl(string) (string, error)             { return "", nil }
func (workflowSuccessMockJira) GetJiraIssue(string) (*jira.Issue, error)       { return nil, nil }
func (workflowSuccessMockJira) SetJiraIssueStatus(string, jira.IssueStatus) (*jira.Issue, error) {
	return nil, nil
}
func (workflowSuccessMockJira) SearchIssues(string, string, []tracker.IssueType, int) ([]tracker.Issue, error) {
	return nil, nil
}

var (
	_ git.Git                    = workflowSuccessMockGit{}
	_ github.Github              = &workflowSuccessMockGitHub{}
	_ jira.Jira                  = workflowSuccessMockJira{}
	_ config.ConfigurableService = workflowSuccessMockGit{}
)

func TestWorkflowRunnerRun_LogsPRURLOnlyOnce(t *testing.T) {
	emitter := &reviewCollectingEmitter{}
	pr := &github.PullRequest{
		Number:  42,
		HTMLURL: "https://github.com/tagoro9/fotingo-playground/pull/2",
	}
	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{Simple: true},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return workflowSuccessMockGit{}, nil
			},
			NewGitHubClient: func(git.Git, *viper.Viper) (github.Github, error) {
				return &workflowSuccessMockGitHub{pr: pr}, nil
			},
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return workflowSuccessMockJira{}, nil
			},
			FetchBranchIssue: func(jira.Jira, string, func(string, ...any)) (*jira.Issue, error) {
				return nil, nil
			},
			ResolvePRBody: func(*chan string, string, *jira.Issue, jira.Jira, []git.Commit, []string, bool) (string, error) {
				return "body", nil
			},
			ResolveLabels: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			ResolveReviewers: func(github.Github, []string) ([]string, []string, []string, error) {
				return nil, nil, nil, nil
			},
			ResolveAssignees: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			SplitEditorContent:     SplitEditorContent,
			DerivePRTitle:          func(string, *jira.Issue, string, bool) string { return "title" },
			ToTeamSlugs:            ToTeamSlugs,
			FormatReviewersWarning: func(err error) string { return err.Error() },
			ShouldOpenReviewEditor: func(bool) bool { return false },
		},
	}

	statusCh := make(chan string, 1)
	result := runner.Run(&statusCh, emitter, false)
	require.NoError(t, result.Err)

	var createdCount int
	var successCount int
	for _, event := range emitter.events {
		if event.key == i18n.ReviewStatusPRCreated {
			createdCount++
			require.Len(t, event.args, 1)
			assert.Equal(t, pr.HTMLURL, event.args[0])
		}
		if event.key == i18n.ReviewStatusSuccess {
			successCount++
		}
	}

	assert.Equal(t, 1, createdCount)
	assert.Zero(t, successCount)
}

func TestWorkflowRunnerRun_UsesExplicitBaseBranchOverride(t *testing.T) {
	pr := &github.PullRequest{Number: 42, HTMLURL: "https://example.com/pr/42"}
	ghClient := &workflowSuccessMockGitHub{pr: pr}
	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{Simple: true, BaseBranch: "release/2026.04"},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return workflowSuccessMockGit{}, nil
			},
			NewGitHubClient: func(git.Git, *viper.Viper) (github.Github, error) {
				return ghClient, nil
			},
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return workflowSuccessMockJira{}, nil
			},
			FetchBranchIssue: func(jira.Jira, string, func(string, ...any)) (*jira.Issue, error) {
				return nil, nil
			},
			ResolvePRBody: func(*chan string, string, *jira.Issue, jira.Jira, []git.Commit, []string, bool) (string, error) {
				return "body", nil
			},
			ResolveLabels: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			ResolveReviewers: func(github.Github, []string) ([]string, []string, []string, error) {
				return nil, nil, nil, nil
			},
			ResolveAssignees: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			SplitEditorContent:     SplitEditorContent,
			DerivePRTitle:          func(string, *jira.Issue, string, bool) string { return "title" },
			ToTeamSlugs:            ToTeamSlugs,
			FormatReviewersWarning: func(err error) string { return err.Error() },
			ShouldOpenReviewEditor: func(bool) bool { return false },
		},
	}

	result := runner.Run(nil, nil, false)
	require.NoError(t, result.Err)
	assert.Equal(t, "release/2026.04", ghClient.createPROptions.Base)
}

func TestWorkflowRunnerRun_UpdatesStackSectionsWhenBaseBranchHasPR(t *testing.T) {
	parent := &github.PullRequest{
		Number:  12,
		Title:   "ABC-1 Parent",
		Body:    "<!-- fotingo:start stacked-prs -->\n<!-- fotingo:end stacked-prs -->",
		HTMLURL: "https://github.com/testowner/testrepo/pull/12",
		HeadRef: "feature/ABC-1-parent",
		BaseRef: "main",
		State:   "open",
	}
	child := &github.PullRequest{
		Number:  13,
		Title:   "ABC-2 Child",
		Body:    "<!-- fotingo:start stacked-prs -->\n<!-- fotingo:end stacked-prs -->",
		HTMLURL: "https://github.com/testowner/testrepo/pull/13",
		HeadRef: "feature/ABC-2-child",
		BaseRef: "feature/ABC-1-parent",
		State:   "open",
	}
	ghClient := &workflowSuccessMockGitHub{pr: child, parentPR: parent}
	gitClient := &stackWorkflowMockGit{}
	runner := stackWorkflowRunner(gitClient, ghClient, WorkflowOptions{Simple: true, BaseBranch: "feature/ABC-1-parent"})
	emitter := &reviewCollectingEmitter{}

	result := runner.Run(nil, emitter, false)

	require.NoError(t, result.Err)
	assert.Equal(t, "feature/ABC-1-parent", ghClient.createPROptions.Base)
	require.Len(t, ghClient.bodyUpdates, 2)
	assert.Equal(t, 12, ghClient.bodyUpdates[0].Number)
	assert.Equal(t, 13, ghClient.bodyUpdates[1].Number)
	assert.Contains(t, ghClient.bodyUpdates[0].Body, `<!-- fotingo:stack id="testowner/testrepo#12" version="1" -->`)
	assert.Contains(t, ghClient.bodyUpdates[0].Body, "| 👉 1 | ABC-1 | [#12 ABC-1 Parent](https://github.com/testowner/testrepo/pull/12) |")
	assert.Contains(t, ghClient.bodyUpdates[0].Body, "| 2 | ABC-2 | [#13 ABC-2 Child](https://github.com/testowner/testrepo/pull/13) |")
	assert.Contains(t, ghClient.bodyUpdates[1].Body, "| 1 | ABC-1 | [#12 ABC-1 Parent](https://github.com/testowner/testrepo/pull/12) |")
	assert.Contains(t, ghClient.bodyUpdates[1].Body, "| 👉 2 | ABC-2 | [#13 ABC-2 Child](https://github.com/testowner/testrepo/pull/13) |")
	assertStackWorkflowRawEvent(t, emitter.rawEvents, "Stack mode enabled: base branch feature/ABC-1-parent is pull request #12")
	assertStackWorkflowRawEvent(t, emitter.rawEvents, "Updating stacked PR sections for parent #12 and new PR #13")
	assertStackWorkflowRawEvent(t, emitter.rawEvents, "Stacked PR sections updated")
	assert.Equal(t, "feature/ABC-1-parent", gitClient.commitsSinceRef)
	assert.False(t, gitClient.defaultCommits)
}

func TestWorkflowRunnerRun_ExtendsExistingStackSections(t *testing.T) {
	parent := &github.PullRequest{
		Number:  13,
		Title:   "ABC-2 Middle",
		Body:    "<!-- fotingo:start stacked-prs -->\n<!-- fotingo:stack id=\"testowner/testrepo#12\" version=\"1\" -->\n<!-- fotingo:end stacked-prs -->",
		HTMLURL: "https://github.com/testowner/testrepo/pull/13",
		HeadRef: "feature/ABC-2-middle",
		BaseRef: "feature/ABC-1-parent",
		State:   "open",
	}
	root := github.PullRequest{
		Number:  12,
		Title:   "ABC-1 Parent",
		Body:    "<!-- fotingo:start stacked-prs -->\n<!-- fotingo:stack id=\"testowner/testrepo#12\" version=\"1\" -->\n<!-- fotingo:end stacked-prs -->",
		HTMLURL: "https://github.com/testowner/testrepo/pull/12",
		HeadRef: "feature/ABC-1-parent",
		BaseRef: "main",
		State:   "open",
	}
	middle := *parent
	child := &github.PullRequest{
		Number:  14,
		Title:   "ABC-3 Child",
		Body:    "<!-- fotingo:start stacked-prs -->\n<!-- fotingo:end stacked-prs -->",
		HTMLURL: "https://github.com/testowner/testrepo/pull/14",
		HeadRef: "feature/ABC-3-child",
		BaseRef: "feature/ABC-2-middle",
		State:   "open",
	}
	ghClient := &workflowSuccessMockGitHub{
		pr:           child,
		parentPR:     parent,
		stackMembers: []github.PullRequest{root, middle},
	}
	runner := stackWorkflowRunner(&stackWorkflowMockGit{}, ghClient, WorkflowOptions{Simple: true, BaseBranch: "feature/ABC-2-middle"})

	result := runner.Run(nil, nil, false)

	require.NoError(t, result.Err)
	require.Len(t, ghClient.bodyUpdates, 3)
	assert.Equal(t, []int{12, 13, 14}, []int{ghClient.bodyUpdates[0].Number, ghClient.bodyUpdates[1].Number, ghClient.bodyUpdates[2].Number})
	assert.Contains(t, ghClient.bodyUpdates[2].Body, `<!-- fotingo:stack id="testowner/testrepo#12" version="1" -->`)
	assert.Contains(t, ghClient.bodyUpdates[2].Body, "| 👉 3 | ABC-3 | [#14 ABC-3 Child](https://github.com/testowner/testrepo/pull/14) |")
}

func assertStackWorkflowRawEvent(t *testing.T, events []reviewRawEvent, message string) {
	t.Helper()

	for _, event := range events {
		if event.message == message {
			return
		}
	}
	assert.Failf(t, "missing raw event", "message %q not found in %#v", message, events)
}

func TestWorkflowRunnerRun_UsesDefaultCommitScopeWhenBaseBranchHasNoPR(t *testing.T) {
	child := &github.PullRequest{Number: 13, HTMLURL: "https://github.com/testowner/testrepo/pull/13", State: "open"}
	ghClient := &workflowSuccessMockGitHub{pr: child}
	gitClient := &stackWorkflowMockGit{}
	runner := stackWorkflowRunner(gitClient, ghClient, WorkflowOptions{Simple: true, BaseBranch: "release/2026.04"})

	result := runner.Run(nil, nil, false)

	require.NoError(t, result.Err)
	assert.Empty(t, gitClient.commitsSinceRef)
	assert.True(t, gitClient.defaultCommits)
	assert.Empty(t, ghClient.bodyUpdates)
}

func stackWorkflowRunner(gitClient git.Git, ghClient *workflowSuccessMockGitHub, options WorkflowOptions) WorkflowRunner {
	return WorkflowRunner{
		Config:  viper.New(),
		Options: options,
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return gitClient, nil
			},
			NewGitHubClient: func(git.Git, *viper.Viper) (github.Github, error) {
				return ghClient, nil
			},
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return workflowSuccessMockJira{}, nil
			},
			FetchBranchIssue: func(jira.Jira, string, func(string, ...any)) (*jira.Issue, error) {
				return nil, nil
			},
			ResolvePRBody: func(*chan string, string, *jira.Issue, jira.Jira, []git.Commit, []string, bool) (string, error) {
				return "body", nil
			},
			ResolveLabels: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			ResolveReviewers: func(github.Github, []string) ([]string, []string, []string, error) {
				return nil, nil, nil, nil
			},
			ResolveAssignees: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			SplitEditorContent:     SplitEditorContent,
			DerivePRTitle:          func(string, *jira.Issue, string, bool) string { return "title" },
			ToTeamSlugs:            ToTeamSlugs,
			FormatReviewersWarning: func(err error) string { return err.Error() },
			ShouldOpenReviewEditor: func(bool) bool { return false },
		},
	}
}

func TestWorkflowRunnerRun_UsesDefaultBaseBranchWithoutOverride(t *testing.T) {
	pr := &github.PullRequest{Number: 42, HTMLURL: "https://example.com/pr/42"}
	ghClient := &workflowSuccessMockGitHub{pr: pr}
	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{Simple: true},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return workflowSuccessMockGit{}, nil
			},
			NewGitHubClient: func(git.Git, *viper.Viper) (github.Github, error) {
				return ghClient, nil
			},
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return workflowSuccessMockJira{}, nil
			},
			FetchBranchIssue: func(jira.Jira, string, func(string, ...any)) (*jira.Issue, error) {
				return nil, nil
			},
			ResolvePRBody: func(*chan string, string, *jira.Issue, jira.Jira, []git.Commit, []string, bool) (string, error) {
				return "body", nil
			},
			ResolveLabels: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			ResolveReviewers: func(github.Github, []string) ([]string, []string, []string, error) {
				return nil, nil, nil, nil
			},
			ResolveAssignees: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			SplitEditorContent:     SplitEditorContent,
			DerivePRTitle:          func(string, *jira.Issue, string, bool) string { return "title" },
			ToTeamSlugs:            ToTeamSlugs,
			FormatReviewersWarning: func(err error) string { return err.Error() },
			ShouldOpenReviewEditor: func(bool) bool { return false },
		},
	}

	result := runner.Run(nil, nil, false)
	require.NoError(t, result.Err)
	assert.Equal(t, "main", ghClient.createPROptions.Base)
}

type workflowLinkedIssuesGit struct {
	workflowSuccessMockGit
}

func (workflowLinkedIssuesGit) GetCommitsSinceDefaultBranch() ([]git.Commit, error) {
	return []git.Commit{
		{Message: "feat: update workflow\n\nFixes FOTINGO-1, FOTINGO-2"},
	}, nil
}

func (workflowLinkedIssuesGit) GetIssuesFromCommits([]git.Commit) []string {
	return []string{"FOTINGO-1", "FOTINGO-2", "FOTINGO-1"}
}

type workflowCommitOnlyIssuesGit struct {
	workflowLinkedIssuesGit
}

func (workflowCommitOnlyIssuesGit) GetIssueId() (string, error) {
	return "", errors.New("no issue id found in branch name: feature/no-jira")
}

type workflowNoIssuesGit struct {
	workflowSuccessMockGit
}

func (workflowNoIssuesGit) GetIssueId() (string, error) {
	return "", errors.New("no issue id found in branch name: feature/no-jira")
}

type workflowRecordingJira struct {
	workflowSuccessMockJira
	statusCalls  []string
	commentCalls []string
}

func (j *workflowRecordingJira) SetJiraIssueStatus(issueID string, _ jira.IssueStatus) (*jira.Issue, error) {
	j.statusCalls = append(j.statusCalls, issueID)
	return &jira.Issue{Key: issueID, Status: "In Review"}, nil
}

func (j *workflowRecordingJira) AddComment(issueID string, _ string) error {
	j.commentCalls = append(j.commentCalls, issueID)
	return nil
}

func TestWorkflowRunnerRun_UpdatesAllLinkedIssues(t *testing.T) {
	emitter := &reviewCollectingEmitter{}
	recordingJira := &workflowRecordingJira{}
	pr := &github.PullRequest{
		Number:  42,
		HTMLURL: "https://github.com/tagoro9/fotingo/pull/42",
	}

	var receivedLinkedIssues []string
	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{Simple: false},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return workflowLinkedIssuesGit{}, nil
			},
			NewGitHubClient: func(git.Git, *viper.Viper) (github.Github, error) {
				return &workflowSuccessMockGitHub{pr: pr}, nil
			},
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return recordingJira, nil
			},
			FetchBranchIssue: func(jira.Jira, string, func(string, ...any)) (*jira.Issue, error) {
				return &jira.Issue{Key: "FOTINGO-10", Summary: "Linked issue branch"}, nil
			},
			ResolvePRBody: func(
				_ *chan string,
				_ string,
				_ *jira.Issue,
				_ jira.Jira,
				_ []git.Commit,
				linkedIssueIDs []string,
				_ bool,
			) (string, error) {
				receivedLinkedIssues = append([]string{}, linkedIssueIDs...)
				return "body", nil
			},
			ResolveLabels: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			ResolveReviewers: func(github.Github, []string) ([]string, []string, []string, error) {
				return nil, nil, nil, nil
			},
			ResolveAssignees: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			SplitEditorContent:     SplitEditorContent,
			DerivePRTitle:          func(string, *jira.Issue, string, bool) string { return "title" },
			ToTeamSlugs:            ToTeamSlugs,
			FormatReviewersWarning: func(err error) string { return err.Error() },
			ShouldOpenReviewEditor: func(bool) bool { return false },
		},
	}

	statusCh := make(chan string, 1)
	result := runner.Run(&statusCh, emitter, false)
	require.NoError(t, result.Err)
	assert.Equal(t, []string{"FOTINGO-10", "FOTINGO-1", "FOTINGO-2"}, receivedLinkedIssues)
	assert.Equal(t, []string{"FOTINGO-10", "FOTINGO-1", "FOTINGO-2"}, recordingJira.statusCalls)
	assert.Equal(t, []string{"FOTINGO-10", "FOTINGO-1", "FOTINGO-2"}, recordingJira.commentCalls)
	require.NotNil(t, result.Issue)
	assert.Equal(t, "FOTINGO-10", result.Issue.Key)
}

func TestWorkflowRunnerRun_UsesCommitLinkedIssuesWithoutBranchIssue(t *testing.T) {
	emitter := &reviewCollectingEmitter{}
	recordingJira := &workflowRecordingJira{}
	pr := &github.PullRequest{
		Number:  42,
		HTMLURL: "https://github.com/tagoro9/fotingo/pull/42",
	}

	var receivedLinkedIssues []string
	var fetchBranchIssueCalls int
	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{Simple: false},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return workflowCommitOnlyIssuesGit{}, nil
			},
			NewGitHubClient: func(git.Git, *viper.Viper) (github.Github, error) {
				return &workflowSuccessMockGitHub{pr: pr}, nil
			},
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return recordingJira, nil
			},
			FetchBranchIssue: func(jira.Jira, string, func(string, ...any)) (*jira.Issue, error) {
				fetchBranchIssueCalls++
				return nil, fmt.Errorf("branch issue should not be fetched")
			},
			ResolvePRBody: func(
				_ *chan string,
				_ string,
				_ *jira.Issue,
				_ jira.Jira,
				_ []git.Commit,
				linkedIssueIDs []string,
				_ bool,
			) (string, error) {
				receivedLinkedIssues = append([]string{}, linkedIssueIDs...)
				return "body", nil
			},
			ResolveLabels: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			ResolveReviewers: func(github.Github, []string) ([]string, []string, []string, error) {
				return nil, nil, nil, nil
			},
			ResolveAssignees: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			SplitEditorContent:     SplitEditorContent,
			DerivePRTitle:          func(string, *jira.Issue, string, bool) string { return "title" },
			ToTeamSlugs:            ToTeamSlugs,
			FormatReviewersWarning: func(err error) string { return err.Error() },
			ShouldOpenReviewEditor: func(bool) bool { return false },
		},
	}

	statusCh := make(chan string, 1)
	result := runner.Run(&statusCh, emitter, false)
	require.NoError(t, result.Err)
	assert.Equal(t, []string{"FOTINGO-1", "FOTINGO-2"}, receivedLinkedIssues)
	assert.Equal(t, []string{"FOTINGO-1", "FOTINGO-2"}, recordingJira.statusCalls)
	assert.Equal(t, []string{"FOTINGO-1", "FOTINGO-2"}, recordingJira.commentCalls)
	assert.Zero(t, fetchBranchIssueCalls)
	assert.Nil(t, result.Issue)
}

func TestWorkflowRunnerRun_SkipsJiraWhenNoIssuesExist(t *testing.T) {
	emitter := &reviewCollectingEmitter{}
	pr := &github.PullRequest{
		Number:  42,
		HTMLURL: "https://github.com/tagoro9/fotingo/pull/42",
	}

	var jiraInitCalls int
	var fetchBranchIssueCalls int
	var receivedLinkedIssues []string
	var receivedJiraClient jira.Jira
	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{Simple: false},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return workflowNoIssuesGit{}, nil
			},
			NewGitHubClient: func(git.Git, *viper.Viper) (github.Github, error) {
				return &workflowSuccessMockGitHub{pr: pr}, nil
			},
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				jiraInitCalls++
				return workflowSuccessMockJira{}, nil
			},
			FetchBranchIssue: func(jira.Jira, string, func(string, ...any)) (*jira.Issue, error) {
				fetchBranchIssueCalls++
				return nil, fmt.Errorf("branch issue should not be fetched")
			},
			ResolvePRBody: func(
				_ *chan string,
				_ string,
				_ *jira.Issue,
				jiraClient jira.Jira,
				_ []git.Commit,
				linkedIssueIDs []string,
				_ bool,
			) (string, error) {
				receivedJiraClient = jiraClient
				receivedLinkedIssues = append([]string{}, linkedIssueIDs...)
				return "body", nil
			},
			ResolveLabels: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			ResolveReviewers: func(github.Github, []string) ([]string, []string, []string, error) {
				return nil, nil, nil, nil
			},
			ResolveAssignees: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			SplitEditorContent: SplitEditorContent,
			DerivePRTitle: func(branch string, issue *jira.Issue, _ string, _ bool) string {
				return BuildDefaultTitle(branch, issue)
			},
			ToTeamSlugs:            ToTeamSlugs,
			FormatReviewersWarning: func(err error) string { return err.Error() },
			ShouldOpenReviewEditor: func(bool) bool { return false },
		},
	}

	statusCh := make(chan string, 1)
	result := runner.Run(&statusCh, emitter, false)
	require.NoError(t, result.Err)
	assert.Zero(t, jiraInitCalls)
	assert.Zero(t, fetchBranchIssueCalls)
	assert.Nil(t, receivedJiraClient)
	assert.Empty(t, receivedLinkedIssues)
	assert.Nil(t, result.Issue)
}
