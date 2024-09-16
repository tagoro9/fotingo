package commands

import (
	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalreview "github.com/tagoro9/fotingo/internal/commands/review"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
)

type reviewExecutorDeps struct {
	newGitClient     func(*viper.Viper, *chan string) (git.Git, error)
	newGitHubClient  func(git.Git, *viper.Viper) (github.Github, error)
	newJiraClient    func(*viper.Viper) (jira.Jira, error)
	fetchBranchIssue func(jira.Jira, string, func(string, ...any)) (*jira.Issue, error)
	resolvePRBody    func(*chan string, string, *jira.Issue, jira.Jira, []git.Commit, bool) (string, error)
	resolveLabels    func(github.Github, []string) ([]string, []string, error)
	resolveReviewers func(github.Github, []string) ([]string, []string, []string, error)
	resolveAssignees func(github.Github, []string) ([]string, []string, error)
}

func defaultReviewExecutorDeps() reviewExecutorDeps {
	return reviewExecutorDeps{
		newGitClient:     newGitClient,
		newGitHubClient:  newGitHubClient,
		newJiraClient:    newJiraClient,
		fetchBranchIssue: fetchReviewBranchIssue,
		resolvePRBody:    resolveReviewPRBody,
		resolveLabels:    resolveReviewLabels,
		resolveReviewers: resolveReviewReviewers,
		resolveAssignees: resolveReviewAssignees,
	}
}

type reviewExecutor struct {
	deps reviewExecutorDeps
}

func newReviewExecutor() reviewExecutor {
	return reviewExecutor{deps: defaultReviewExecutorDeps()}
}

func (e reviewExecutor) runWithOptions(statusCh *chan string, allowEditor bool) reviewResult {
	out := commandruntime.NewLocalizedEmitter(*statusCh, shouldEmitCommandLevel, localizer.T)
	runner := internalreview.WorkflowRunner{
		Config:   fotingoConfig,
		Localize: localizer.T,
		Options: internalreview.WorkflowOptions{
			Draft:                       reviewCmdFlags.draft,
			Labels:                      append([]string(nil), reviewCmdFlags.labels...),
			Reviewers:                   append([]string(nil), reviewCmdFlags.reviewers...),
			Assignees:                   append([]string(nil), reviewCmdFlags.assignees...),
			Simple:                      reviewCmdFlags.simple,
			Description:                 reviewCmdFlags.description,
			TemplateSummaryOverride:     reviewCmdFlags.templateSummary != "",
			TemplateDescriptionOverride: reviewCmdFlags.templateDescription != "",
		},
		Deps: internalreview.WorkflowDeps{
			NewGitClient:           e.deps.newGitClient,
			NewGitHubClient:        e.deps.newGitHubClient,
			NewJiraClient:          e.deps.newJiraClient,
			FetchBranchIssue:       e.deps.fetchBranchIssue,
			ResolvePRBody:          e.deps.resolvePRBody,
			ResolveLabels:          e.deps.resolveLabels,
			ResolveReviewers:       e.deps.resolveReviewers,
			ResolveAssignees:       e.deps.resolveAssignees,
			SplitEditorContent:     internalreview.SplitEditorContent,
			DerivePRTitle:          deriveReviewPRTitle,
			ToTeamSlugs:            internalreview.ToTeamSlugs,
			FormatReviewersWarning: formatReviewReviewersWarning,
			ShouldOpenReviewEditor: shouldOpenReviewEditor,
		},
	}

	result := runner.Run(statusCh, reviewWorkflowEmitter{out: out}, allowEditor)
	return reviewResult{
		pr:            result.PR,
		issue:         result.Issue,
		jiraURL:       result.JiraURL,
		labels:        result.Labels,
		reviewers:     result.Reviewers,
		teamReviewers: result.TeamReviewers,
		assignees:     result.Assignees,
		existed:       result.Existed,
		err:           result.Err,
	}
}

type reviewWorkflowEmitter struct {
	out commandruntime.LocalizedEmitter
}

func (e reviewWorkflowEmitter) Info(emoji string, key i18n.Key, args ...any) {
	e.out.Info(reviewWorkflowEmoji(emoji), key, args...)
}

func (e reviewWorkflowEmitter) InfoRaw(emoji string, message string) {
	e.out.InfoRaw(reviewWorkflowEmoji(emoji), message)
}

func (e reviewWorkflowEmitter) Verbose(key i18n.Key, args ...any) {
	e.out.Verbose(key, args...)
}

func (e reviewWorkflowEmitter) Debugf(format string, args ...any) {
	e.out.Debugf(format, args...)
}

func reviewWorkflowEmoji(emoji string) commandruntime.LogEmoji {
	switch emoji {
	case "link":
		return commandruntime.LogEmojiLink
	case "git":
		return commandruntime.LogEmojiGit
	case "check":
		return commandruntime.LogEmojiCheck
	case "review":
		return commandruntime.LogEmojiReview
	case "rocket":
		return commandruntime.LogEmojiRocket
	case "warning":
		return commandruntime.LogEmojiWarning
	case "success":
		return commandruntime.LogEmojiSuccess
	case "progress":
		return commandruntime.LogEmojiProgress
	default:
		return commandruntime.LogEmojiInfo
	}
}
