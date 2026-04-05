package commands

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalopen "github.com/tagoro9/fotingo/internal/commands/open"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
	"github.com/tagoro9/fotingo/internal/ui"
)

func init() {
	Fotingo.AddCommand(openCmd)
}

const openBrowserOperationID = "open-browser"

var openPRNotFoundPattern = regexp.MustCompile(`(?i)no pull request found for branch\s+([^\s:]+)`)
var jiraIssueLookupMaxAttempts = 3
var jiraIssueLookupDelay = 150 * time.Millisecond
var jiraIssueLookupSleep = time.Sleep
var openSelectOneFn = ui.SelectOne

func getBranchDerivedJiraIssueWithRetry(
	jiraClient jira.Jira,
	issueID string,
	out *commandruntime.LocalizedEmitter,
) (*jira.Issue, error) {
	return getJiraIssueWithRetry(jiraClient, issueID, out)
}

func getJiraIssueWithRetry(
	jiraClient jira.Jira,
	issueID string,
	out *commandruntime.LocalizedEmitter,
) (*jira.Issue, error) {
	cfg := commandruntime.JiraIssueLookupConfig{
		MaxAttempts: jiraIssueLookupMaxAttempts,
		Delay:       jiraIssueLookupDelay,
		Sleep:       jiraIssueLookupSleep,
	}

	var debugf func(format string, args ...any)
	if out != nil {
		debugf = out.Debugf
	}

	return commandruntime.GetJiraIssueWithRetry(jiraClient, issueID, cfg, debugf)
}

type openIssueSelection struct {
	Branch   string
	IssueIDs []string
}

func resolveOpenIssueSelection(gitClient git.Git) (*openIssueSelection, error) {
	branch, err := gitClient.GetCurrentBranch()
	if err != nil {
		return nil, err
	}

	branchIssueID, branchIssueErr := gitClient.GetIssueId()
	if branchIssueErr != nil && !isOpenBranchIssueMissingErr(branchIssueErr) {
		return nil, branchIssueErr
	}

	var commitIssueIDs []string
	commits, commitErr := gitClient.GetCommitsSinceDefaultBranch()
	if commitErr == nil {
		commitIssueIDs = gitClient.GetIssuesFromCommits(commits)
	}

	issueIDs := internalopen.CollectLinkedIssueIDs(strings.TrimSpace(branchIssueID), commitIssueIDs)
	if len(issueIDs) > 0 {
		return &openIssueSelection{Branch: branch, IssueIDs: issueIDs}, nil
	}
	if commitErr != nil {
		return nil, commitErr
	}

	return &openIssueSelection{Branch: branch}, nil
}

func isOpenBranchIssueMissingErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no issue id found in branch name:")
}

func shouldPromptForOpenIssueSelection() bool {
	return !ShouldOutputJSON() &&
		!Global.Yes &&
		isInputTerminalFn() &&
		commandruntime.IsInputTerminal() &&
		commandruntime.IsOutputTerminal()
}

func selectOpenIssueID(issueIDs []string, jiraClient jira.Jira) (string, error) {
	items := make([]ui.PickerItem, 0, len(issueIDs))
	for _, issueID := range issueIDs {
		if strings.TrimSpace(issueID) == "" {
			continue
		}

		item := ui.PickerItem{
			ID:    issueID,
			Label: issueID,
			Value: issueID,
		}

		issue, err := jiraClient.GetIssue(issueID)
		if err == nil && issue != nil {
			if strings.TrimSpace(issue.Key) != "" {
				item.Label = issue.Key
			}
			item.Detail = buildOpenIssuePickerDetail(issue)
		}

		items = append(items, item)
	}

	selected, err := openSelectOneFn(localizer.T(i18n.OpenPickerIssueTitle), items)
	if err != nil {
		return "", fmt.Errorf(localizer.T(i18n.OpenErrPickerRun), err)
	}
	if selected == nil {
		return "", errors.New(localizer.T(i18n.OpenErrSelectionCancel))
	}
	if value, ok := selected.Value.(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value), nil
	}
	return strings.TrimSpace(selected.ID), nil
}

func buildOpenIssuePickerDetail(issue *tracker.Issue) string {
	if issue == nil {
		return ""
	}

	parts := make([]string, 0, 2)
	if strings.TrimSpace(issue.Summary) != "" {
		parts = append(parts, strings.TrimSpace(issue.Summary))
	}
	if strings.TrimSpace(string(issue.Status)) != "" {
		parts = append(parts, strings.TrimSpace(string(issue.Status)))
	}
	return strings.Join(parts, " | ")
}

func handleOpenRepo(git git.Git) (string, error) {
	return internalopen.ResolveRepoURL(git)
}

func handleOpenBranch(git git.Git) (string, error) {
	return internalopen.ResolveBranchURL(git, func(host string) error {
		return fmt.Errorf(localizer.T(i18n.OpenErrUnsupportedHost), host)
	})
}

func handleOpenPr(github github.Github) (string, error) {
	return internalopen.ResolvePRURL(github)
}

func handleOpenIssue(git git.Git, jiraClient jira.Jira) (string, error) {
	url, _, _, err := handleOpenIssueWithMetadata(git, jiraClient, nil)
	return url, err
}

func handleOpenIssueWithMetadata(
	git git.Git,
	jiraClient jira.Jira,
	out *commandruntime.LocalizedEmitter,
) (url string, issueID string, issueStatus string, err error) {
	selection, err := resolveOpenIssueSelection(git)
	if err != nil {
		return "", "", "", err
	}

	switch len(selection.IssueIDs) {
	case 0:
		return "", "", "", fmt.Errorf(localizer.T(i18n.OpenErrNoLinkedIssue), selection.Branch)
	case 1:
		issueID = selection.IssueIDs[0]
	default:
		if !shouldPromptForOpenIssueSelection() {
			return "", "", "", fmt.Errorf(
				localizer.T(i18n.OpenErrAmbiguousIssues),
				strings.Join(selection.IssueIDs, ", "),
			)
		}

		issueID, err = selectOpenIssueID(selection.IssueIDs, jiraClient)
		if err != nil {
			return "", "", "", err
		}
	}

	issue, err := getJiraIssueWithRetry(jiraClient, issueID, out)
	if err != nil {
		return "", "", "", err
	}

	return jiraClient.GetIssueURL(issueID), issueID, issue.Status, nil
}

func runOpenWithStatus(statusCh *chan string, option string, openBrowser bool) (string, error) {
	out := commandruntime.NewLocalizedEmitter(*statusCh, shouldEmitCommandLevel, localizer.T)
	if openBrowser {
		out.Start(openBrowserOperationID, commandruntime.LogEmojiBrowser, i18n.OpenStatusOpeningBrowser, option)
	}

	gitClient, err := newOpenGitClient(fotingoConfig, statusCh)
	if err != nil {
		return "", fterrors.WrapGitError(localizer.T(i18n.OpenWrapInitGit), err)
	}

	var url string
	switch option {
	case "branch":
		url, err = handleOpenBranch(gitClient)
		if err != nil {
			return "", fterrors.WrapGitError(localizer.T(i18n.OpenWrapGetBranchURL), err)
		}
	case "issue":
		jiraClient, jiraErr := newJiraClient(fotingoConfig)
		if jiraErr != nil {
			return "", fterrors.WrapJiraError(localizer.T(i18n.OpenWrapInitJira), jiraErr)
		}

		var issueID, issueStatus string
		url, issueID, issueStatus, err = handleOpenIssueWithMetadata(gitClient, jiraClient, &out)
		if err != nil {
			return "", fterrors.WrapJiraError(localizer.T(i18n.OpenWrapGetIssueURL), err)
		}
		out.Verbose(i18n.OpenIssueStatus, issueID, issueStatus)
	case "pr":
		hub, hubErr := newGitHubClient(gitClient, fotingoConfig)
		if hubErr != nil {
			return "", fterrors.WrapGitHubError(localizer.T(i18n.OpenWrapInitGitHub), hubErr)
		}
		url, err = handleOpenPr(hub)
		if err != nil {
			out.Debugf("open pr resolution failed: %v", err)
			return "", mapOpenPRError(err)
		}
	case "repo":
		url, err = handleOpenRepo(gitClient)
		if err != nil {
			return "", fterrors.WrapGitError(localizer.T(i18n.OpenWrapGetRepoURL), err)
		}
	default:
		return "", fmt.Errorf(localizer.T(i18n.OpenErrNoURL), option)
	}

	if url == "" {
		return "", fmt.Errorf(localizer.T(i18n.OpenErrNoURL), option)
	}

	if !openBrowser {
		return url, nil
	}

	out.Update(openBrowserOperationID, commandruntime.LogEmojiBrowser, i18n.OpenStatusOpeningBrowser, url)
	if err := openBrowserFn(url); err != nil {
		return "", fmt.Errorf(localizer.T(i18n.OpenErrOpenBrowser), err)
	}
	out.Success(openBrowserOperationID, commandruntime.LogEmojiBrowser, i18n.OpenStatusOpenedBrowser, url)

	return url, nil
}

func mapOpenPRError(err error) error {
	return internalopen.MapPRError(
		err,
		openPRNotFoundPattern,
		func(branch string, cause error) error {
			return fterrors.WrapGitHubError(localizer.T(i18n.OpenErrNoPRForBranch, branch), cause)
		},
		func(cause error) error {
			return fterrors.WrapGitHubError(localizer.T(i18n.OpenWrapGetPRURL), cause)
		},
	)
}

var openCmd = &cobra.Command{
	Use:       i18n.T(i18n.OpenUse),
	Short:     i18n.T(i18n.OpenShort),
	Long:      i18n.T(i18n.OpenLong),
	ValidArgs: []string{"branch", "issue", "pr", "repo"},
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		option := args[0]

		if option == "issue" {
			if err := ensureJiraRootConfigured(); err != nil {
				if ShouldOutputJSON() {
					return outputOpenJSON(option, "", err)
				}
				return err
			}
		}

		if ShouldOutputJSON() {
			statusCh := make(chan string, 100)
			go func() {
				for range statusCh {
				}
			}()
			url, err := runOpenWithStatus(&statusCh, option, false)
			close(statusCh)
			return outputOpenJSON(option, url, err)
		}

		return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
			statusCh, done := out.BridgeChannel()
			defer done()
			_, err := runOpenWithStatus(statusCh, option, true)
			return err
		})
	},
}

// outputOpenJSON outputs the open command result as JSON
func outputOpenJSON(target, url string, err error) error {
	output := OpenOutput{
		Success: err == nil,
		Target:  target,
	}

	if err != nil {
		output.Error = err.Error()
		OutputJSON(output)
		return err
	}

	output.URL = url
	output.Opened = false // In JSON mode, we don't actually open the browser

	OutputJSON(output)
	return nil
}
