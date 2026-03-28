package commands

import (
	"fmt"
	"slices"
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
	if issue == nil && strings.TrimSpace(branchIssueID) != "" {
		linkedIssueIDs = internalreview.DedupeStringsPreserveOrder(
			append([]string{strings.TrimSpace(branchIssueID)}, linkedIssueIDs...),
		)
	}
	existingLinkedIssueIDs := extractLinkedIssueIDsFromPRBody(pr.Body)
	newLinkedIssueIDs := diffReviewSyncIssueIDs(linkedIssueIDs, existingLinkedIssueIDs)
	if jiraClient == nil && len(linkedIssueIDs) > 0 {
		jiraClient, err = newJiraClient(fotingoConfig)
		if err != nil {
			return reviewResult{err: err}
		}
		if issue != nil {
			jiraURL = jiraClient.GetIssueURL(issue.Key)
		}
	}

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
	editorOpened := false
	if shouldOpenReviewSyncEditor(allowEditor, sections) {
		seedBody, seedErr := buildReviewSyncEditorBody(pr.Body, freshBody, sections)
		if seedErr != nil {
			return reviewResult{err: seedErr}
		}

		editorSeed := internalreview.BuildEditorSeedContent(pr.Title, seedBody)

		out.Info(commandruntime.LogEmojiPrompt, i18n.ReviewStatusOpenEditor)
		editedContent, editErr := openEditorFn(editorSeed)
		if editErr != nil {
			return reviewResult{err: fmt.Errorf(localizer.T(i18n.ReviewErrOpenEditor), editErr)}
		}
		out.Info(commandruntime.LogEmojiCheck, i18n.ReviewStatusEditorDone)

		editorOpened = true
		editorTitle, bodySource = internalreview.SplitEditorContent(editedContent)
	}

	updatedBody := pr.Body
	if editorOpened {
		if err := validateManagedSectionsPresent(bodySource, sections); err != nil {
			return reviewResult{err: err}
		}
		updatedBody = bodySource
	} else {
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
	}

	updateOpts := github.UpdatePROptions{}
	if updatedBody != pr.Body {
		updateOpts.Body = &updatedBody
	}

	if title := reviewSyncTitleOverride(pr.Title, branch, issue, editorTitle, editorOpened); title != nil {
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

	if len(newLinkedIssueIDs) > 0 {
		if jiraClient == nil {
			jiraClient, err = newJiraClient(fotingoConfig)
			if err != nil {
				return reviewResult{err: err}
			}
			if jiraURL == "" && issue != nil {
				jiraURL = jiraClient.GetIssueURL(issue.Key)
			}
		}

		comment := localizer.T(i18n.ReviewCommentCreated, updatedPR.HTMLURL)
		for _, issueID := range newLinkedIssueIDs {
			out.Verbose(i18n.ReviewStatusSetInReview, issueID)
			updatedIssue, setErr := jiraClient.SetJiraIssueStatus(issueID, jira.StatusInReview)
			if setErr != nil {
				out.Info("warning", i18n.ReviewStatusSetInReviewWarn, setErr)
			} else {
				if issue != nil && strings.EqualFold(issue.Key, issueID) {
					issue = updatedIssue
					jiraURL = jiraClient.GetIssueURL(issueID)
				}
				out.Verbose(i18n.ReviewStatusSetInReviewDone, issueID)
			}

			out.Verbose(i18n.ReviewStatusAddComment, issueID)
			if commentErr := jiraClient.AddComment(issueID, comment); commentErr != nil {
				out.Info("warning", i18n.ReviewStatusAddCommentWarn, commentErr)
			} else {
				out.Verbose(i18n.ReviewStatusAddCommentDone, issueID)
			}
		}
	}

	return reviewResult{
		pr:      updatedPR,
		issue:   issue,
		jiraURL: jiraURL,
	}
}

func buildReviewSyncEditorBody(currentBody string, freshBody string, _ []string) (string, error) {
	updatedBody := currentBody
	for _, section := range internalreview.ManagedSections() {
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

func validateManagedSectionsPresent(body string, sections []string) error {
	for _, section := range sections {
		if _, err := internalreview.ExtractManagedSectionContent(body, section); err != nil {
			return err
		}
	}

	return nil
}

func extractLinkedIssueIDsFromPRBody(body string) []string {
	fixedIssues, err := internalreview.ExtractManagedSectionContent(body, internalreview.ManagedSectionFixedIssues)
	if err != nil {
		return nil
	}

	return internalreview.DedupeStringsPreserveOrder(issueIDPattern.FindAllString(strings.ToUpper(fixedIssues), -1))
}

func diffReviewSyncIssueIDs(next []string, existing []string) []string {
	if len(next) == 0 {
		return nil
	}

	newIssues := make([]string, 0, len(next))
	for _, issueID := range internalreview.DedupeStringsPreserveOrder(next) {
		if !slices.Contains(existing, issueID) {
			newIssues = append(newIssues, issueID)
		}
	}

	return newIssues
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

	return !reviewSyncHasSufficientCLIInputs(sections)
}

func reviewSyncHasSufficientCLIInputs(sections []string) bool {
	hasOverride := strings.TrimSpace(reviewSyncCmdFlags.title) != "" ||
		strings.TrimSpace(reviewSyncCmdFlags.templateSummary) != "" ||
		strings.TrimSpace(reviewSyncCmdFlags.templateDescription) != ""
	if !hasOverride {
		return false
	}

	for _, section := range sections {
		switch section {
		case internalreview.ManagedSectionSummary:
			if strings.TrimSpace(reviewSyncCmdFlags.templateSummary) == "" {
				return false
			}
		case internalreview.ManagedSectionDescription:
			if strings.TrimSpace(reviewSyncCmdFlags.templateDescription) == "" {
				return false
			}
		}
	}

	if reviewSyncCmdFlags.syncTitle && strings.TrimSpace(reviewSyncCmdFlags.title) == "" {
		return false
	}

	return true
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

func reviewSyncTitleOverride(currentTitle string, branch string, issue *jira.Issue, editorTitle string, editorOpened bool) *string {
	if strings.TrimSpace(reviewSyncCmdFlags.title) != "" {
		title := strings.TrimSpace(reviewSyncCmdFlags.title)
		return &title
	}

	if editorOpened {
		title := strings.TrimSpace(editorTitle)
		if title != "" && title != strings.TrimSpace(currentTitle) {
			return &title
		}
		return nil
	}

	if !reviewSyncCmdFlags.syncTitle {
		return nil
	}

	title := internalreview.DerivePRTitle("", branch, issue, "", false)
	if title == strings.TrimSpace(currentTitle) {
		return nil
	}
	return &title
}
