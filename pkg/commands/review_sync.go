package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalreview "github.com/tagoro9/fotingo/internal/commands/review"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
)

type reviewSyncFlags struct {
	sections            []string
	syncTitle           bool
	title               string
	templateSummary     string
	templateDescription string
}

var reviewSyncCmdFlags = reviewSyncFlags{}

func init() {
	reviewSyncCmd.Flags().StringSliceVar(&reviewSyncCmdFlags.sections, "section", nil, "Sync only the specified fotingo-managed section (repeatable)")
	reviewSyncCmd.Flags().BoolVar(&reviewSyncCmdFlags.syncTitle, "sync-title", false, "Refresh the pull request title using fotingo's derived title rules")
	reviewSyncCmd.Flags().StringVar(&reviewSyncCmdFlags.title, "title", "", "Override the pull request title during sync")
	reviewSyncCmd.Flags().StringVar(&reviewSyncCmdFlags.templateSummary, "template-summary", "", "Override the synced Summary section content")
	reviewSyncCmd.Flags().StringVar(&reviewSyncCmdFlags.templateDescription, "template-description", "", "Override the synced Description section content; expands escaped \\n, \\r\\n, and \\t")

	reviewCmd.AddCommand(reviewSyncCmd)
}

var reviewSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Refresh fotingo-managed sections in the current pull request",
	Long:  "Refresh fotingo-managed pull request sections for the current branch using marker-delimited ownership in the PR body.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if ShouldOutputJSON() {
			statusCh := make(chan string, 100)
			go func() {
				for range statusCh {
				}
			}()

			result := runReviewSync(&statusCh, false)
			close(statusCh)
			return outputReviewJSON(result)
		}

		return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
			out.InfoRaw(commandruntime.LogEmojiReview, "Starting pull request sync...")
			statusCh, done := out.BridgeChannel()
			defer done()
			result := runReviewSync(statusCh, true)
			return result.err
		})
	},
}

func runReviewSync(statusCh *chan string, allowEditor bool) reviewResult {
	out := commandruntime.NewLocalizedEmitter(*statusCh, shouldEmitCommandLevel, localizer.T)

	sections, err := internalreview.NormalizeManagedSections(reviewSyncCmdFlags.sections)
	if err != nil {
		return reviewResult{err: err}
	}
	if err := validateReviewSyncOverrides(sections); err != nil {
		return reviewResult{err: err}
	}

	gitClient, err := newGitClient(fotingoConfig, statusCh)
	if err != nil {
		return reviewResult{err: err}
	}

	branch, err := gitClient.GetCurrentBranch()
	if err != nil {
		return reviewResult{err: err}
	}
	out.Verbose(i18n.ReviewStatusCurrentBranch, branch)

	ghClient, err := newGitHubClient(gitClient, fotingoConfig)
	if err != nil {
		return reviewResult{err: err}
	}
	if setter, ok := ghClient.(interface{ SetMetadataFetchInfoLogger(func(string)) }); ok {
		setter.SetMetadataFetchInfoLogger(func(message string) {
			out.InfoRaw(commandruntime.LogEmojiProgress, message)
		})
	}

	exists, pr, err := ghClient.DoesPRExistForBranch(branch)
	if err != nil {
		return reviewResult{err: err}
	}
	if !exists || pr == nil {
		return reviewResult{err: fmt.Errorf("no pull request found for branch %s", branch)}
	}

	needsDerivedTitle := reviewSyncCmdFlags.syncTitle && strings.TrimSpace(reviewSyncCmdFlags.title) == ""
	needsIssueData := reviewSyncNeedsIssueData(sections) || needsDerivedTitle

	var issue *jira.Issue
	var jiraClient jira.Jira
	var jiraURL string

	branchIssueID, issueIDErr := gitClient.GetIssueId()
	if issueIDErr != nil && !strings.Contains(strings.ToLower(issueIDErr.Error()), "no issue id found") {
		return reviewResult{err: issueIDErr}
	}
	if strings.TrimSpace(branchIssueID) != "" && needsIssueData {
		jiraClient, err = newJiraClient(fotingoConfig)
		if err != nil {
			return reviewResult{err: err}
		}
		issue, err = fetchReviewBranchIssue(jiraClient, branchIssueID, out.Debugf)
		if err != nil {
			return reviewResult{err: err}
		}
		jiraURL = jiraClient.GetIssueURL(issue.Key)
	}

	commits, err := gitClient.GetCommitsSinceDefaultBranch()
	if err != nil {
		return reviewResult{err: err}
	}
	linkedIssueIDs := internalreview.CollectLinkedIssueIDs(issue, gitClient.GetIssuesFromCommits(commits))

	freshBody := renderReviewTemplateBodyWithOverrides(
		branch,
		issue,
		jiraClient,
		commits,
		linkedIssueIDs,
		reviewSyncCmdFlags.templateSummary,
		reviewSyncCmdFlags.templateDescription,
	)

	bodySource := freshBody
	editorTitle := ""
	editorMode := false
	if shouldOpenReviewSyncEditor(allowEditor, sections) {
		seedBody, seedErr := buildReviewSyncEditorBody(pr.Body, freshBody, sections)
		if seedErr != nil {
			return reviewResult{err: seedErr}
		}

		editorSeed := seedBody
		if reviewSyncNeedsTitleEditorInput() {
			editorSeed = internalreview.BuildEditorSeedContent(
				internalreview.DerivePRTitle("", branch, issue, "", false),
				seedBody,
			)
		}

		out.Info(commandruntime.LogEmojiPrompt, i18n.ReviewStatusOpenEditor)
		editedContent, editErr := openEditorFn(editorSeed)
		if editErr != nil {
			return reviewResult{err: fmt.Errorf(localizer.T(i18n.ReviewErrOpenEditor), editErr)}
		}
		out.Info(commandruntime.LogEmojiCheck, i18n.ReviewStatusEditorDone)

		if reviewSyncNeedsTitleEditorInput() {
			editorMode = true
			editorTitle, bodySource = internalreview.SplitEditorContent(editedContent)
		} else {
			bodySource = editedContent
		}
	}

	updatedBody := pr.Body
	for _, section := range sections {
		replacement, extractErr := internalreview.ExtractManagedSectionContent(bodySource, section)
		if extractErr != nil {
			return reviewResult{err: extractErr}
		}

		updatedBody, err = internalreview.ReplaceManagedSectionContent(updatedBody, section, replacement)
		if err != nil {
			return reviewResult{err: err}
		}
	}

	updateOpts := github.UpdatePROptions{}
	if updatedBody != pr.Body {
		updateOpts.Body = &updatedBody
	}

	if title := reviewSyncTitleOverride(branch, issue, editorTitle, editorMode); title != nil {
		updateOpts.Title = title
	}

	if updateOpts.Title == nil && updateOpts.Body == nil {
		return reviewResult{
			pr:      pr,
			issue:   issue,
			jiraURL: jiraURL,
		}
	}

	out.InfoRaw(commandruntime.LogEmojiReview, "Updating pull request...")
	updatedPR, err := ghClient.UpdatePullRequest(pr.Number, updateOpts)
	if err != nil {
		return reviewResult{err: err}
	}
	out.InfoRaw(commandruntime.LogEmojiCheck, fmt.Sprintf("Pull request synchronized: %s", updatedPR.HTMLURL))

	return reviewResult{
		pr:      updatedPR,
		issue:   issue,
		jiraURL: jiraURL,
	}
}

func buildReviewSyncEditorBody(currentBody string, freshBody string, sections []string) (string, error) {
	updatedBody := currentBody
	for _, section := range sections {
		replacement, err := internalreview.ExtractManagedSectionContent(freshBody, section)
		if err != nil {
			return "", err
		}

		updatedBody, err = internalreview.ReplaceManagedSectionContent(updatedBody, section, replacement)
		if err != nil {
			return "", err
		}
	}

	return updatedBody, nil
}

func reviewSyncNeedsIssueData(sections []string) bool {
	for _, section := range sections {
		switch section {
		case internalreview.ManagedSectionSummary:
			if strings.TrimSpace(reviewSyncCmdFlags.templateSummary) == "" {
				return true
			}
		case internalreview.ManagedSectionDescription:
			if strings.TrimSpace(reviewSyncCmdFlags.templateDescription) == "" {
				return true
			}
		}
	}
	return false
}

func shouldOpenReviewSyncEditor(allowEditor bool, sections []string) bool {
	if !shouldOpenReviewEditor(allowEditor) {
		return false
	}

	return reviewSyncNeedsEditorInput(sections)
}

func reviewSyncNeedsEditorInput(sections []string) bool {
	if reviewSyncNeedsTitleEditorInput() {
		return true
	}

	for _, section := range sections {
		switch section {
		case internalreview.ManagedSectionSummary:
			if strings.TrimSpace(reviewSyncCmdFlags.templateSummary) == "" {
				return true
			}
		case internalreview.ManagedSectionDescription:
			if strings.TrimSpace(reviewSyncCmdFlags.templateDescription) == "" {
				return true
			}
		}
	}

	return false
}

func reviewSyncNeedsTitleEditorInput() bool {
	return reviewSyncCmdFlags.syncTitle && strings.TrimSpace(reviewSyncCmdFlags.title) == ""
}

func validateReviewSyncOverrides(sections []string) error {
	selected := make(map[string]struct{}, len(sections))
	for _, section := range sections {
		selected[section] = struct{}{}
	}

	if strings.TrimSpace(reviewSyncCmdFlags.templateSummary) != "" {
		if _, ok := selected[internalreview.ManagedSectionSummary]; !ok {
			return fmt.Errorf("--template-summary requires syncing the %q section", internalreview.ManagedSectionSummary)
		}
	}

	if strings.TrimSpace(reviewSyncCmdFlags.templateDescription) != "" {
		if _, ok := selected[internalreview.ManagedSectionDescription]; !ok {
			return fmt.Errorf("--template-description requires syncing the %q section", internalreview.ManagedSectionDescription)
		}
	}

	return nil
}

func reviewSyncTitleOverride(branch string, issue *jira.Issue, editorTitle string, editorMode bool) *string {
	if strings.TrimSpace(reviewSyncCmdFlags.title) != "" {
		title := strings.TrimSpace(reviewSyncCmdFlags.title)
		return &title
	}

	if !reviewSyncCmdFlags.syncTitle {
		return nil
	}

	title := internalreview.DerivePRTitle("", branch, issue, editorTitle, editorMode)
	return &title
}
