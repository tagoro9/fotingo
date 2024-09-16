package commands

import (
	"fmt"
	"regexp"
	"time"

	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalopen "github.com/tagoro9/fotingo/internal/commands/open"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
)

func init() {
	Fotingo.AddCommand(openCmd)
}

const openBrowserOperationID = "open-browser"

var openPRNotFoundPattern = regexp.MustCompile(`(?i)no pull request found for branch\s+([^\s:]+)`)
var jiraIssueLookupMaxAttempts = 3
var jiraIssueLookupDelay = 150 * time.Millisecond
var jiraIssueLookupSleep = time.Sleep

func getBranchDerivedJiraIssueWithRetry(
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
	_, err = git.GetCurrentBranch()
	if err != nil {
		return "", "", "", err
	}
	issueId, err := git.GetIssueId()
	if err != nil {
		return "", "", "", err
	}
	issue, err := getBranchDerivedJiraIssueWithRetry(jiraClient, issueId, out)
	if err != nil {
		return "", "", "", err
	}
	url, err = jiraClient.GetIssueUrl(issueId)
	if err != nil {
		return "", "", "", err
	}
	return url, issueId, issue.Status, nil
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
