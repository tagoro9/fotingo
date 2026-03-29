package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalreview "github.com/tagoro9/fotingo/internal/commands/review"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/template"
	"github.com/tagoro9/fotingo/internal/ui"
)

// reviewResult holds the result of the review command for JSON output
type reviewResult struct {
	pr            *github.PullRequest
	issue         *jira.Issue
	jiraURL       string
	labels        []string
	reviewers     []string
	teamReviewers []string
	assignees     []string
	existed       bool
	err           error
}

// reviewFlags holds the flags for the review command
type reviewFlags struct {
	draft               bool
	labels              []string
	reviewers           []string
	assignees           []string
	simple              bool
	title               string
	description         string
	templateSummary     string
	templateDescription string
}

var reviewCmdFlags = reviewFlags{}

// defaultPRTemplate is the default template used for creating pull request bodies
var defaultPRTemplate = i18n.T(i18n.ReviewDefaultTemplate)
var resolveReviewTemplateFn = func() string {
	return internalreview.ResolveTemplate(defaultPRTemplate)
}

var pickReviewMatchWithPickerFn = pickReviewMatchWithPicker
var canPromptForReviewDisambiguationFn = canPromptForReviewDisambiguation
var reviewRunInteractiveProcessWithTerminalHandoffFn = runInteractiveProcessWithTerminalHandoff
var newReviewPickerProgramFn = func(title string, items []ui.PickerItem) reviewPicker {
	return ui.NewPickerProgram(
		ui.WithPickerTitle(title),
		ui.WithPickerItems(items),
		ui.WithPickerSearch(true),
	)
}

type reviewPicker interface {
	Run() (*ui.PickerItem, error)
}

func init() {
	Fotingo.AddCommand(reviewCmd)

	reviewCmd.Flags().BoolVarP(&reviewCmdFlags.draft, "draft", "d", false, localizer.T(i18n.ReviewFlagDraft))
	reviewCmd.Flags().StringSliceVarP(&reviewCmdFlags.labels, "labels", "l", []string{}, localizer.T(i18n.ReviewFlagLabels))
	reviewCmd.Flags().StringSliceVarP(&reviewCmdFlags.reviewers, "reviewers", "r", []string{}, localizer.T(i18n.ReviewFlagReviewers))
	reviewCmd.Flags().StringSliceVarP(&reviewCmdFlags.assignees, "assignee", "a", []string{}, localizer.T(i18n.ReviewFlagAssignees))
	reviewCmd.Flags().BoolVarP(&reviewCmdFlags.simple, "simple", "s", false, localizer.T(i18n.ReviewFlagSimple))
	reviewCmd.Flags().StringVar(&reviewCmdFlags.title, "title", "", localizer.T(i18n.ReviewFlagTitle))
	reviewCmd.Flags().StringVar(&reviewCmdFlags.description, "description", "", localizer.T(i18n.ReviewFlagDescription))
	reviewCmd.Flags().StringVar(&reviewCmdFlags.templateSummary, "template-summary", "", localizer.T(i18n.ReviewFlagTemplateSummary))
	reviewCmd.Flags().StringVar(&reviewCmdFlags.templateDescription, "template-description", "", localizer.T(i18n.ReviewFlagTemplateDescription))
	_ = reviewCmd.RegisterFlagCompletionFunc("labels", completeReviewLabelsFlag)
	_ = reviewCmd.RegisterFlagCompletionFunc("reviewers", completeReviewReviewersFlag)
	_ = reviewCmd.RegisterFlagCompletionFunc("assignee", completeReviewAssigneesFlag)
}

var reviewCmd = &cobra.Command{
	Use:   i18n.T(i18n.ReviewUse),
	Short: i18n.T(i18n.ReviewShort),
	Long:  i18n.T(i18n.ReviewLong),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !reviewCmdFlags.simple {
			if err := ensureJiraRootConfigured(); err != nil {
				return err
			}
		}

		// JSON mode: run without TUI and output JSON result
		if ShouldOutputJSON() {
			// Create a no-op status channel for JSON mode
			statusCh := make(chan string, 100)
			go func() {
				// Drain the channel to prevent blocking
				for range statusCh {
				}
			}()

			result := runReviewWithResultWithOptions(&statusCh, false)
			close(statusCh)
			return outputReviewJSON(result)
		}

		return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
			out.Info(commandruntime.LogEmojiReview, i18n.ReviewInitialCreating)
			statusCh, done := out.BridgeChannel()
			defer done()
			result := runReviewWithResultWithOptions(statusCh, true)
			return result.err
		})
	},
}

// runReviewWithResult executes the main review logic and returns a result for JSON output
func runReviewWithResult(statusCh *chan string) reviewResult {
	return newReviewExecutor().runWithOptions(statusCh, false)
}

// runReviewWithResultWithOptions executes review logic with optional editor support.
func runReviewWithResultWithOptions(statusCh *chan string, allowEditor bool) reviewResult {
	return newReviewExecutor().runWithOptions(statusCh, allowEditor)
}

func fetchReviewBranchIssue(jiraClient jira.Jira, issueID string, debugf func(string, ...any)) (*jira.Issue, error) {
	cfg := commandruntime.JiraIssueLookupConfig{
		MaxAttempts: jiraIssueLookupMaxAttempts,
		Delay:       jiraIssueLookupDelay,
		Sleep:       jiraIssueLookupSleep,
	}
	return commandruntime.GetJiraIssueWithRetry(jiraClient, issueID, cfg, debugf)
}

// outputReviewJSON outputs the review command result as JSON
func outputReviewJSON(result reviewResult) error {
	output := ReviewOutput{
		Success: result.err == nil,
	}

	if result.err != nil {
		output.Error = result.err.Error()
		OutputJSON(output)
		return result.err
	}

	if result.pr != nil {
		state := localizer.T(i18n.ReviewStateOpen)
		if result.existed {
			state = localizer.T(i18n.ReviewStateExisting)
		}
		output.PullRequest = &PullRequestInfo{
			Number: result.pr.Number,
			URL:    result.pr.HTMLURL,
			Title:  result.pr.Title,
			Draft:  reviewCmdFlags.draft,
			State:  state,
		}
	}

	if result.issue != nil {
		output.Issue = &IssueInfo{
			Key:     result.issue.Key,
			Summary: result.issue.Summary,
			Status:  result.issue.Status,
			Type:    result.issue.Type,
			URL:     result.jiraURL,
		}
	}

	if len(result.labels) > 0 {
		output.Labels = result.labels
	}

	if len(result.reviewers) > 0 {
		output.Reviewers = result.reviewers
	}
	if len(result.teamReviewers) > 0 {
		output.TeamReviewers = result.teamReviewers
	}
	if len(result.assignees) > 0 {
		output.Assignees = result.assignees
	}

	output.Existed = result.existed

	OutputJSON(output)
	return nil
}

// buildPRBody constructs the PR body using the template
func buildPRBody(branch string, issue *jira.Issue, jiraClient jira.Jira, commits ...git.Commit) string {
	return renderReviewTemplateBodyWithOverrides(
		branch,
		issue,
		jiraClient,
		commits,
		nil,
		reviewCmdFlags.templateSummary,
		reviewCmdFlags.templateDescription,
	)
}

func resolveReviewTemplate() string {
	return resolveReviewTemplateFn()
}

func buildReviewTemplateData(
	branch string,
	issue *jira.Issue,
	jiraClient jira.Jira,
	commits []git.Commit,
	linkedIssueIDs []string,
) map[string]string {
	return buildReviewTemplateDataWithOverrides(
		branch,
		issue,
		jiraClient,
		commits,
		linkedIssueIDs,
		reviewCmdFlags.templateSummary,
		reviewCmdFlags.templateDescription,
	)
}

func buildReviewTemplateDataWithOverrides(
	branch string,
	issue *jira.Issue,
	jiraClient jira.Jira,
	commits []git.Commit,
	linkedIssueIDs []string,
	templateSummary string,
	templateDescription string,
) map[string]string {
	return internalreview.BuildTemplateData(branch, issue, jiraClient, commits, internalreview.TemplateOptions{
		TemplateSummary:       templateSummary,
		TemplateDescription:   templateDescription,
		EmptyPlaceholderValue: localizer.T(i18n.ReviewPlaceholderEmpty),
		LinkedIssueIDs:        linkedIssueIDs,
	})
}

func renderReviewTemplateBodyWithOverrides(
	branch string,
	issue *jira.Issue,
	jiraClient jira.Jira,
	commits []git.Commit,
	linkedIssueIDs []string,
	templateSummary string,
	templateDescription string,
) string {
	return renderResolvedReviewTemplate(
		resolveReviewTemplate(),
		buildReviewTemplateDataWithOverrides(
			branch,
			issue,
			jiraClient,
			commits,
			linkedIssueIDs,
			templateSummary,
			templateDescription,
		),
	)
}

func renderResolvedReviewTemplate(templateContent string, data map[string]string) string {
	body, _, err := internalreview.RenderTemplate(templateContent, data)
	if err == nil {
		return body
	}

	return template.New(templateContent).Render(data)
}

func deriveReviewPRTitle(branch string, issue *jira.Issue, editorTitle string, editorMode bool) string {
	return internalreview.DerivePRTitle(reviewCmdFlags.title, branch, issue, editorTitle, editorMode)
}

func formatReviewReviewersWarning(err error) string {
	if isReviewCannotBeRequestedError(err) {
		return localizer.T(i18n.ReviewStatusReviewersCannotRequest)
	}

	return localizer.T(i18n.ReviewStatusReviewersWarn, err)
}

func isReviewCannotBeRequestedError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "422 review cannot be requested")
}

func resolveReviewPRBody(
	statusCh *chan string,
	branch string,
	issue *jira.Issue,
	jiraClient jira.Jira,
	commits []git.Commit,
	linkedIssueIDs []string,
	allowEditor bool,
) (string, error) {
	if reviewCmdFlags.description != "" {
		if reviewCmdFlags.description != "-" {
			return reviewCmdFlags.description, nil
		}

		// Read from stdin
		commandruntime.NewLocalizedEmitter(*statusCh, shouldEmitCommandLevel, localizer.T).Verbose(i18n.ReviewStatusReadStdin)
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf(localizer.T(i18n.ReviewErrReadStdin), err)
		}
		return strings.Join(lines, "\n"), nil
	}

	body := renderResolvedReviewTemplate(
		resolveReviewTemplate(),
		buildReviewTemplateData(branch, issue, jiraClient, commits, linkedIssueIDs),
	)
	if shouldOpenReviewEditor(allowEditor) {
		out := commandruntime.NewLocalizedEmitter(*statusCh, shouldEmitCommandLevel, localizer.T)
		out.Info(commandruntime.LogEmojiPrompt, i18n.ReviewStatusOpenEditor)
		editorSeed := internalreview.BuildEditorSeedContent(
			deriveReviewPRTitle(branch, issue, "", false),
			body,
		)
		editedBody, err := openEditorFn(editorSeed)
		if err != nil {
			return "", fmt.Errorf(localizer.T(i18n.ReviewErrOpenEditor), err)
		}
		out.Info(commandruntime.LogEmojiCheck, i18n.ReviewStatusEditorDone)
		return editedBody, nil
	}

	return body, nil
}

func shouldOpenReviewEditor(allowEditor bool) bool {
	if !allowEditor {
		return false
	}

	if ShouldOutputJSON() || Global.Yes {
		return false
	}

	return isInputTerminalFn()
}

type reviewMatchOption = internalreview.MatchOption
type reviewMatchKind = internalreview.MatchKind

const (
	reviewMatchKindUser  reviewMatchKind = internalreview.MatchKindUser
	reviewMatchKindTeam  reviewMatchKind = internalreview.MatchKindTeam
	reviewMatchKindLabel reviewMatchKind = internalreview.MatchKindLabel
)

func resolveReviewLabels(ghClient github.Github, requested []string) ([]string, []string, error) {
	return internalreview.ResolveLabels(
		ghClient,
		requested,
		canPromptForReviewDisambiguationFn(),
		func(kind string, token string, matches []reviewMatchOption) (string, error) {
			return pickReviewMatchWithPickerFn(kind, token, matches)
		},
	)
}

func resolveReviewReviewers(ghClient github.Github, requested []string) ([]string, []string, []string, error) {
	return internalreview.ResolveReviewers(
		ghClient,
		requested,
		canPromptForReviewDisambiguationFn(),
		func(kind string, token string, matches []reviewMatchOption) (string, error) {
			return pickReviewMatchWithPickerFn(kind, token, matches)
		},
	)
}

func resolveReviewAssignees(ghClient github.Github, requested []string) ([]string, []string, error) {
	return internalreview.ResolveAssignees(
		ghClient,
		requested,
		canPromptForReviewDisambiguationFn(),
		func(kind string, token string, matches []reviewMatchOption) (string, error) {
			return pickReviewMatchWithPickerFn(kind, token, matches)
		},
	)
}

func canPromptForReviewDisambiguation() bool {
	if Global.Yes || ShouldOutputJSON() {
		return false
	}
	return isInputTerminalFn()
}

func pickReviewMatchWithPicker(kind string, token string, matches []reviewMatchOption) (string, error) {
	selected, err := internalreview.PickMatchWithPicker(
		kind,
		token,
		matches,
		func(title string, items []ui.PickerItem) (*ui.PickerItem, error) {
			picker := newReviewPickerProgramFn(title, items)

			var item *ui.PickerItem
			runErr := reviewRunInteractiveProcessWithTerminalHandoffFn(func() error {
				var execErr error
				item, execErr = picker.Run()
				return execErr
			})
			if runErr != nil {
				return nil, fmt.Errorf("failed to run %s picker: %w", kind, runErr)
			}
			return item, nil
		},
	)
	if err != nil {
		return "", err
	}

	return selected, nil
}
