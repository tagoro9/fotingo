package commands

import (
	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/jira"
)

// runStartWithResult executes the main start logic and returns a result for JSON output.
// This is used only in JSON mode where no interactive TUI is needed.
func runStartWithResult(cmd *cobra.Command, statusCh *chan string, issueId string) startResult {
	return newStartExecutor().runWithResult(cmd, statusCh, issueId)
}

// outputStartJSON outputs the start command result as JSON.
func outputStartJSON(result startResult) error {
	output := StartOutput{
		Success: result.err == nil,
	}

	if result.err != nil {
		output.Error = result.err.Error()
		OutputJSON(output)
		return result.err
	}

	if result.issue != nil {
		jiraClient, _ := newJiraClient(fotingoConfig)
		issueURL := ""
		if jiraClient != nil {
			issueURL = jiraClient.GetIssueURL(result.issue.Key)
		}
		output.Issue = &IssueInfo{
			Key:     result.issue.Key,
			Summary: result.issue.Summary,
			Status:  result.issue.Status,
			Type:    result.issue.Type,
			URL:     issueURL,
		}
	}

	if result.branchName != "" {
		output.Branch = &StartBranchInfo{
			Name:         result.branchName,
			Created:      result.created,
			WorktreePath: result.worktreePath,
		}
	}

	OutputJSON(output)
	return nil
}

// startResult holds the result of the start command for JSON output.
type startResult struct {
	issue        *jira.Issue
	branchName   string
	worktreePath string
	created      bool
	err          error
}
