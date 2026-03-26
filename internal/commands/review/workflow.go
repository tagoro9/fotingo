package review

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
)

// WorkflowOptions carries review flags required by the orchestration workflow.
type WorkflowOptions struct {
	Draft                       bool
	Labels                      []string
	Reviewers                   []string
	Assignees                   []string
	Simple                      bool
	Description                 string
	TemplateSummaryOverride     bool
	TemplateDescriptionOverride bool
}

// WorkflowResult contains the review execution output.
type WorkflowResult struct {
	PR            *github.PullRequest
	Issue         *jira.Issue
	JiraURL       string
	Labels        []string
	Reviewers     []string
	TeamReviewers []string
	Assignees     []string
	Existed       bool
	Err           error
}

// WorkflowEmitter is the logging contract used by the review workflow.
type WorkflowEmitter interface {
	Info(emoji string, key i18n.Key, args ...any)
	InfoRaw(emoji string, message string)
	Verbose(key i18n.Key, args ...any)
	Debugf(format string, args ...any)
}

type metadataFetchInfoLoggerSetter interface {
	SetMetadataFetchInfoLogger(func(string))
}

// WorkflowDeps defines external dependencies used by the review workflow.
type WorkflowDeps struct {
	NewGitClient           func(*viper.Viper, *chan string) (git.Git, error)
	NewGitHubClient        func(git.Git, *viper.Viper) (github.Github, error)
	NewJiraClient          func(*viper.Viper) (jira.Jira, error)
	FetchBranchIssue       func(jira.Jira, string, func(string, ...any)) (*jira.Issue, error)
	ResolvePRBody          func(*chan string, string, *jira.Issue, jira.Jira, []git.Commit, []string, bool) (string, error)
	ResolveLabels          func(github.Github, []string) ([]string, []string, error)
	ResolveReviewers       func(github.Github, []string) ([]string, []string, []string, error)
	ResolveAssignees       func(github.Github, []string) ([]string, []string, error)
	SplitEditorContent     func(string) (string, string)
	DerivePRTitle          func(string, *jira.Issue, string, bool) string
	ToTeamSlugs            func([]string) []string
	FormatReviewersWarning func(error) string
	ShouldOpenReviewEditor func(bool) bool
}

// WorkflowRunner executes the review command workflow.
type WorkflowRunner struct {
	Config   *viper.Viper
	Options  WorkflowOptions
	Localize func(i18n.Key, ...any) string
	Deps     WorkflowDeps
}

// Run executes the review workflow and returns the result.
func (r WorkflowRunner) Run(statusCh *chan string, out WorkflowEmitter, allowEditor bool) WorkflowResult {
	result := WorkflowResult{}
	if err := r.validateDeps(); err != nil {
		result.Err = err
		return result
	}
	if out == nil {
		out = noopWorkflowEmitter{}
	}
	t := r.localize
	totalStart := time.Now()

	out.Verbose(i18n.ReviewStatusInitGit)
	initGitStart := time.Now()
	gitClient, err := r.Deps.NewGitClient(r.Config, statusCh)
	logReviewPhaseTiming(out, "init_git", initGitStart)
	if err != nil {
		result.Err = fterrors.WrapGitError(t(i18n.ReviewWrapInitGit), err)
		return result
	}
	out.Verbose(i18n.ReviewStatusGitInit)

	currentBranchStart := time.Now()
	branch, err := gitClient.GetCurrentBranch()
	logReviewPhaseTiming(out, "get_current_branch", currentBranchStart)
	if err != nil {
		result.Err = fterrors.WrapGitError(t(i18n.ReviewWrapCurrentBranch), err)
		return result
	}
	out.Verbose(i18n.ReviewStatusCurrentBranch, branch)
	out.Debugf("review branch=%s", branch)
	out.Debugf(
		"review template overrides summary=%t description=%t",
		r.Options.TemplateSummaryOverride,
		r.Options.TemplateDescriptionOverride,
	)

	out.Verbose(i18n.ReviewStatusInitGitHub)
	initGitHubStart := time.Now()
	ghClient, err := r.Deps.NewGitHubClient(gitClient, r.Config)
	logReviewPhaseTiming(out, "init_github", initGitHubStart)
	if err != nil {
		result.Err = fterrors.WrapGitHubError(t(i18n.ReviewWrapInitGitHub), err)
		return result
	}
	if setter, ok := ghClient.(metadataFetchInfoLoggerSetter); ok {
		setter.SetMetadataFetchInfoLogger(func(message string) {
			out.InfoRaw("progress", message)
		})
	}
	out.Verbose(i18n.ReviewStatusGitHubInit)

	out.Verbose(i18n.ReviewStatusCheckExistingPR)
	existingPRStart := time.Now()
	exists, existingPR, err := ghClient.DoesPRExistForBranch(branch)
	logReviewPhaseTiming(out, "check_existing_pr", existingPRStart)
	if err != nil {
		result.Err = fterrors.WrapGitHubError(t(i18n.ReviewWrapCheckExistingPR), err)
		return result
	}
	if exists {
		out.Info("link", i18n.ReviewStatusExistingPR, existingPR.HTMLURL)
		result.PR = existingPR
		result.Existed = true
		return result
	}
	out.Verbose(i18n.ReviewStatusNoExistingPR)

	out.Verbose(i18n.ReviewStatusCheckRemoteBranch)
	remoteBranchStart := time.Now()
	branchExistsInRemote, err := gitClient.DoesBranchExistInRemote(branch)
	logReviewPhaseTiming(out, "check_remote_branch", remoteBranchStart)
	if err != nil {
		result.Err = fterrors.WrapGitError(t(i18n.ReviewWrapCheckRemote), err)
		return result
	}
	if !branchExistsInRemote {
		out.Info("git", i18n.ReviewStatusPushBranch)
		if err := gitClient.Push(); err != nil {
			result.Err = fterrors.WrapGitError(t(i18n.ReviewWrapPushBranch), err)
			return result
		}
		out.Info("check", i18n.ReviewStatusBranchPushed)
	} else {
		out.Verbose(i18n.ReviewStatusBranchExists)
	}

	var prTitle, prBody string
	var issue *jira.Issue
	var jiraClient jira.Jira
	var commits []git.Commit
	var linkedIssueIDs []string
	var resolvedLabels []string
	var resolvedReviewers []string
	var resolvedTeamReviewers []string
	var resolvedAssignees []string

	if !r.Options.Simple {
		out.Verbose(i18n.ReviewStatusInitJira)
		initJiraStart := time.Now()
		jiraClient, err = r.Deps.NewJiraClient(r.Config)
		logReviewPhaseTiming(out, "init_jira", initJiraStart)
		if err != nil {
			result.Err = fterrors.WrapJiraError(t(i18n.ReviewWrapInitJira), err)
			return result
		}
		out.Verbose(i18n.ReviewStatusJiraInit)

		out.Verbose(i18n.ReviewStatusExtractIssue)
		issueID, err := gitClient.GetIssueId()
		if err != nil {
			result.Err = fterrors.WrapGitError(t(i18n.ReviewWrapIssueFromBranch), err)
			return result
		}
		out.Verbose(i18n.ReviewStatusFoundIssue, issueID)

		out.Verbose(i18n.ReviewStatusFetchingIssue, issueID)
		fetchIssueStart := time.Now()
		issue, err = r.Deps.FetchBranchIssue(jiraClient, issueID, out.Debugf)
		logReviewPhaseTiming(out, "fetch_issue", fetchIssueStart)
		if err != nil {
			result.Err = fterrors.WrapJiraError(t(i18n.ReviewWrapGetIssue), err)
			return result
		}
		result.Issue = issue
		result.JiraURL = jiraClient.GetIssueURL(issue.Key)
		out.Verbose(i18n.ReviewStatusIssueSummary, issue.Key, issue.Summary)
	}

	if len(r.Options.Labels) > 0 {
		resolveLabelsStart := time.Now()
		var missingLabels []string
		resolvedLabels, missingLabels, err = r.Deps.ResolveLabels(ghClient, r.Options.Labels)
		logReviewPhaseTiming(out, "resolve_labels", resolveLabelsStart)
		if err != nil {
			result.Err = fterrors.WrapGitHubError(t(i18n.ReviewWrapResolveLabels), err)
			return result
		}
		if len(missingLabels) > 0 {
			out.Info("warning", i18n.ReviewStatusMissingLabelsWarn, strings.Join(missingLabels, ", "))
		}
	}

	if len(r.Options.Reviewers) > 0 {
		resolveReviewersStart := time.Now()
		var resolveWarnings []string
		resolvedReviewers, resolvedTeamReviewers, resolveWarnings, err = r.Deps.ResolveReviewers(ghClient, r.Options.Reviewers)
		logReviewPhaseTiming(out, "resolve_reviewers", resolveReviewersStart)
		if err != nil {
			result.Err = fterrors.WrapGitHubError(t(i18n.ReviewWrapResolveReviewers), err)
			return result
		}
		for _, warning := range resolveWarnings {
			out.InfoRaw("warning", warning)
		}
	}

	if len(r.Options.Assignees) > 0 {
		resolveAssigneesStart := time.Now()
		var resolveWarnings []string
		resolvedAssignees, resolveWarnings, err = r.Deps.ResolveAssignees(ghClient, r.Options.Assignees)
		logReviewPhaseTiming(out, "resolve_assignees", resolveAssigneesStart)
		if err != nil {
			result.Err = fterrors.WrapGitHubError(t(i18n.ReviewWrapResolveAssignees), err)
			return result
		}
		for _, warning := range resolveWarnings {
			out.InfoRaw("warning", warning)
		}
	}

	if shouldCollectReviewCommits(r.Options.Description) {
		collectCommitsStart := time.Now()
		loadedCommits, commitErr := gitClient.GetCommitsSinceDefaultBranch()
		logReviewPhaseTiming(out, "collect_commits", collectCommitsStart)
		if commitErr != nil {
			out.Debugf("review commits unavailable: %v", commitErr)
		} else {
			commits = loadedCommits
			linkedIssueIDs = CollectLinkedIssueIDs(issue, gitClient.GetIssuesFromCommits(commits))
			out.Debugf("review commits loaded=%d", len(commits))
			out.Debugf("review linked issues=%d", len(linkedIssueIDs))
		}
	} else {
		out.Debugf("review commits skipped: description override provided")
	}
	if len(linkedIssueIDs) == 0 {
		linkedIssueIDs = CollectLinkedIssueIDs(issue, nil)
	}

	editorMode := r.Options.Description == "" && r.Deps.ShouldOpenReviewEditor(allowEditor)
	resolvePRBodyStart := time.Now()
	prBody, err = r.Deps.ResolvePRBody(statusCh, branch, issue, jiraClient, commits, linkedIssueIDs, allowEditor)
	logReviewPhaseTiming(out, "resolve_pr_body", resolvePRBodyStart)
	if err != nil {
		result.Err = err
		return result
	}
	out.Debugf("review pr body prepared length=%d", len(prBody))

	editorTitle := ""
	if editorMode {
		editorTitle, prBody = r.Deps.SplitEditorContent(prBody)
	}

	prTitle = r.Deps.DerivePRTitle(branch, issue, editorTitle, editorMode)

	defaultBranchStart := time.Now()
	defaultBranch, err := gitClient.GetDefaultBranch()
	logReviewPhaseTiming(out, "get_default_branch", defaultBranchStart)
	if err != nil {
		result.Err = fterrors.WrapGitError(t(i18n.ReviewWrapDefaultBranch), err)
		return result
	}
	out.Debugf("review default base branch=%s", defaultBranch)

	out.Info("review", i18n.ReviewStatusCreatePR)
	createPRStart := time.Now()
	pr, err := ghClient.CreatePullRequest(github.CreatePROptions{
		Title: prTitle,
		Body:  prBody,
		Head:  branch,
		Base:  defaultBranch,
		Draft: r.Options.Draft,
	})
	logReviewPhaseTiming(out, "create_pr", createPRStart)
	if err != nil {
		result.Err = fterrors.WrapGitHubError(t(i18n.ReviewWrapCreatePR), err)
		return result
	}
	result.PR = pr
	out.Info("rocket", i18n.ReviewStatusPRCreated, pr.HTMLURL)

	if len(resolvedReviewers) > 0 || len(resolvedTeamReviewers) > 0 {
		out.Verbose(i18n.ReviewStatusRequestReviewers, strings.Join(append(resolvedReviewers, resolvedTeamReviewers...), ", "))
		requestReviewersStart := time.Now()
		if err := ghClient.RequestReviewers(pr.Number, resolvedReviewers, r.Deps.ToTeamSlugs(resolvedTeamReviewers)); err != nil {
			logReviewPhaseTiming(out, "request_reviewers", requestReviewersStart)
			out.Debugf("failed to request reviewers: %v", err)
			out.InfoRaw("warning", r.Deps.FormatReviewersWarning(err))
		} else {
			logReviewPhaseTiming(out, "request_reviewers", requestReviewersStart)
			result.Reviewers = resolvedReviewers
			result.TeamReviewers = resolvedTeamReviewers
			out.Verbose(i18n.ReviewStatusReviewersDone)
		}
	}

	if len(resolvedAssignees) > 0 {
		out.Verbose(i18n.ReviewStatusAssignAssignees, strings.Join(resolvedAssignees, ", "))
		assignAssigneesStart := time.Now()
		if err := ghClient.AssignUsersToPR(pr.Number, resolvedAssignees); err != nil {
			logReviewPhaseTiming(out, "assign_assignees", assignAssigneesStart)
			out.Info("warning", i18n.ReviewStatusAssignAssigneesWarn, err)
		} else {
			logReviewPhaseTiming(out, "assign_assignees", assignAssigneesStart)
			result.Assignees = resolvedAssignees
			out.Verbose(i18n.ReviewStatusAssigneesDone)
		}
	}

	if len(resolvedLabels) > 0 {
		out.Verbose(i18n.ReviewStatusAddingLabels, strings.Join(resolvedLabels, ", "))
		addLabelsStart := time.Now()
		if err := ghClient.AddLabelsToPR(pr.Number, resolvedLabels); err != nil {
			logReviewPhaseTiming(out, "add_labels", addLabelsStart)
			out.Info("warning", i18n.ReviewStatusAddLabelsWarn, err)
		} else {
			logReviewPhaseTiming(out, "add_labels", addLabelsStart)
			result.Labels = resolvedLabels
			out.Verbose(i18n.ReviewStatusLabelsAdded)
		}
	}

	if !r.Options.Simple && jiraClient != nil {
		comment := t(i18n.ReviewCommentCreated, pr.HTMLURL)
		for _, issueID := range linkedIssueIDs {
			out.Verbose(i18n.ReviewStatusSetInReview, issueID)
			updatedIssue, err := jiraClient.SetJiraIssueStatus(issueID, jira.StatusInReview)
			if err != nil {
				out.Info("warning", i18n.ReviewStatusSetInReviewWarn, err)
			} else {
				if issue != nil && strings.EqualFold(issue.Key, issueID) {
					result.Issue = updatedIssue
				}
				out.Verbose(i18n.ReviewStatusSetInReviewDone, issueID)
			}

			out.Verbose(i18n.ReviewStatusAddComment, issueID)
			if err := jiraClient.AddComment(issueID, comment); err != nil {
				out.Info("warning", i18n.ReviewStatusAddCommentWarn, err)
			} else {
				out.Verbose(i18n.ReviewStatusAddCommentDone, issueID)
			}
		}
	}

	logReviewPhaseTiming(out, "total", totalStart)
	return result
}

func shouldCollectReviewCommits(description string) bool {
	return strings.TrimSpace(description) == ""
}

func logReviewPhaseTiming(out WorkflowEmitter, phase string, start time.Time) {
	if out == nil {
		return
	}
	out.Debugf("review timing phase=%s duration=%s", phase, commandruntime.HumanizeDuration(time.Since(start)))
}

func (r WorkflowRunner) validateDeps() error {
	if r.Deps.NewGitClient == nil {
		return fmt.Errorf("review workflow dependency NewGitClient is required")
	}
	if r.Deps.NewGitHubClient == nil {
		return fmt.Errorf("review workflow dependency NewGitHubClient is required")
	}
	if r.Deps.NewJiraClient == nil {
		return fmt.Errorf("review workflow dependency NewJiraClient is required")
	}
	if r.Deps.FetchBranchIssue == nil {
		return fmt.Errorf("review workflow dependency FetchBranchIssue is required")
	}
	if r.Deps.ResolvePRBody == nil {
		return fmt.Errorf("review workflow dependency ResolvePRBody is required")
	}
	if r.Deps.ResolveLabels == nil {
		return fmt.Errorf("review workflow dependency ResolveLabels is required")
	}
	if r.Deps.ResolveReviewers == nil {
		return fmt.Errorf("review workflow dependency ResolveReviewers is required")
	}
	if r.Deps.ResolveAssignees == nil {
		return fmt.Errorf("review workflow dependency ResolveAssignees is required")
	}
	if r.Deps.SplitEditorContent == nil {
		return fmt.Errorf("review workflow dependency SplitEditorContent is required")
	}
	if r.Deps.DerivePRTitle == nil {
		return fmt.Errorf("review workflow dependency DerivePRTitle is required")
	}
	if r.Deps.ToTeamSlugs == nil {
		return fmt.Errorf("review workflow dependency ToTeamSlugs is required")
	}
	if r.Deps.FormatReviewersWarning == nil {
		return fmt.Errorf("review workflow dependency FormatReviewersWarning is required")
	}
	if r.Deps.ShouldOpenReviewEditor == nil {
		return fmt.Errorf("review workflow dependency ShouldOpenReviewEditor is required")
	}
	return nil
}

func (r WorkflowRunner) localize(key i18n.Key, args ...any) string {
	if r.Localize != nil {
		return r.Localize(key, args...)
	}
	return i18n.T(key, args...)
}

type noopWorkflowEmitter struct{}

func (noopWorkflowEmitter) Info(string, i18n.Key, ...any) {}
func (noopWorkflowEmitter) InfoRaw(string, string)        {}
func (noopWorkflowEmitter) Verbose(i18n.Key, ...any)      {}
func (noopWorkflowEmitter) Debugf(string, ...any)         {}
