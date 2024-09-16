package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	internalinspect "github.com/tagoro9/fotingo/internal/commands/inspect"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
)

// inspectFlags holds the flags for the inspect command
type inspectFlags struct {
	branch string
	issue  string
}

var inspectCmdFlags = inspectFlags{}

func init() {
	Fotingo.AddCommand(inspectCmd)

	inspectCmd.Flags().StringVarP(&inspectCmdFlags.branch, "branch", "b", "", localizer.T(i18n.InspectFlagBranch))
	inspectCmd.Flags().StringVarP(&inspectCmdFlags.issue, "issue", "i", "", localizer.T(i18n.InspectFlagIssue))
}

// InspectOutput represents the JSON output of the inspect command
type InspectOutput struct {
	Branch   *BranchInfo  `json:"branch,omitempty"`
	Issue    *IssueInfo   `json:"issue,omitempty"`
	IssueIDs []string     `json:"issueIds,omitempty"`
	Commits  []CommitInfo `json:"commits,omitempty"`
}

// BranchInfo contains information about a git branch
type BranchInfo struct {
	Name          string `json:"name"`
	IssueID       string `json:"issueId,omitempty"`
	DefaultBranch string `json:"defaultBranch,omitempty"`
}

// IssueInfo contains information about a Jira issue
type IssueInfo struct {
	Key         string `json:"key"`
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	Type        string `json:"type"`
	ParentKey   string `json:"parentKey,omitempty"`
	EpicKey     string `json:"epicKey,omitempty"`
	URL         string `json:"url"`
}

// CommitInfo contains information about a git commit
type CommitInfo struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Author  string `json:"author"`
}

var inspectCmd = &cobra.Command{
	Use:   i18n.T(i18n.InspectUse),
	Short: i18n.T(i18n.InspectShort),
	Long:  i18n.T(i18n.InspectLong),
	Run: func(cmd *cobra.Command, args []string) {
		runner := internalinspect.WorkflowRunner{
			Config: fotingoConfig,
			Options: internalinspect.WorkflowOptions{
				Branch: inspectCmdFlags.branch,
				Issue:  inspectCmdFlags.issue,
			},
			Deps: internalinspect.WorkflowDeps{
				NewGitClient:  git.New,
				NewJiraClient: newJiraClient,
				FetchBranchIssue: func(jiraClient jira.Jira, issueID string) (*jira.Issue, error) {
					return fetchInspectBranchIssue(jiraClient, issueID)
				},
			},
		}

		result, err := runner.Run()
		if err != nil {
			outputError(err)
			return
		}

		output := InspectOutput{
			IssueIDs: result.IssueIDs,
			Commits:  make([]CommitInfo, 0, len(result.Commits)),
		}
		if result.Branch != nil {
			output.Branch = &BranchInfo{
				Name:          result.Branch.Name,
				IssueID:       result.Branch.IssueID,
				DefaultBranch: result.Branch.DefaultBranch,
			}
		}
		if result.Issue != nil {
			output.Issue = &IssueInfo{
				Key:         result.Issue.Key,
				Summary:     result.Issue.Summary,
				Description: result.Issue.Description,
				Status:      result.Issue.Status,
				Type:        result.Issue.Type,
				ParentKey:   result.Issue.ParentKey,
				EpicKey:     result.Issue.EpicKey,
				URL:         result.Issue.URL,
			}
		}
		for _, commit := range result.Commits {
			output.Commits = append(output.Commits, CommitInfo{
				Hash:    commit.Hash,
				Message: commit.Message,
				Author:  commit.Author,
			})
		}

		outputJSON(output)
	},
}

func fetchInspectBranchIssue(jiraClient jira.Jira, issueID string) (*jira.Issue, error) {
	return getBranchDerivedJiraIssueWithRetry(jiraClient, issueID, nil)
}

// issueIDPattern matches Jira-style issue IDs like "PROJ-123"
var issueIDPattern = internalinspect.IssueIDPattern()

// extractIssueIDsFromCommits extracts unique issue IDs from commit messages
func extractIssueIDsFromCommits(commits []git.Commit) []string {
	return internalinspect.ExtractIssueIDsFromCommits(commits)
}

// outputJSON outputs the data as formatted JSON
func outputJSON(data interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, localizer.T(i18n.InspectErrEncodingJSON), err)
	}
}

// outputError outputs an error as JSON
func outputError(err error) {
	errorOutput := struct {
		Error string `json:"error"`
	}{
		Error: err.Error(),
	}
	outputJSON(errorOutput)
}
