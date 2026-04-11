package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	internalinspect "github.com/tagoro9/fotingo/internal/commands/inspect"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
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
	inspectCmd.AddCommand(inspectPrCmd)

	inspectCmd.Flags().StringVarP(&inspectCmdFlags.branch, "branch", "b", "", localizer.T(i18n.InspectFlagBranch))
	inspectCmd.Flags().StringVarP(&inspectCmdFlags.issue, "issue", "i", "", localizer.T(i18n.InspectFlagIssue))
	inspectPrCmd.Flags().StringVarP(&inspectCmdFlags.branch, "branch", "b", "", localizer.T(i18n.InspectFlagBranch))
}

// InspectOutput represents the JSON output of the inspect command
type InspectOutput struct {
	Branch      *BranchInfo    `json:"branch,omitempty"`
	Issue       *IssueInfo     `json:"issue,omitempty"`
	PullRequest *InspectPRInfo `json:"pullRequest,omitempty"`
	IssueIDs    []string       `json:"issueIds,omitempty"`
	Commits     []CommitInfo   `json:"commits,omitempty"`
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

// InspectPRInfo contains information about a pull request related to branch inspect output.
type InspectPRInfo struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
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
				NewGitClient: git.New,
				NewGitHubClient: func(gitClient git.Git, cfg *viper.Viper) (internalinspect.PullRequestInspector, error) {
					return newGitHubClient(gitClient, cfg)
				},
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
		if result.PullRequest != nil {
			output.PullRequest = &InspectPRInfo{
				Number:      result.PullRequest.Number,
				Title:       result.PullRequest.Title,
				Description: result.PullRequest.Description,
				URL:         result.PullRequest.URL,
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

var inspectPrCmd = &cobra.Command{
	Use:   "pr",
	Short: "Inspect pull request discussion context as JSON",
	Long:  "Inspect the open pull request for the current branch or an explicit branch and output comments, reviews, and conversations as JSON.",
	Run: func(cmd *cobra.Command, args []string) {
		runner := internalinspect.WorkflowRunner{
			Config: fotingoConfig,
			Options: internalinspect.WorkflowOptions{
				Branch: inspectCmdFlags.branch,
			},
			Deps: internalinspect.WorkflowDeps{
				NewGitClient: git.New,
				NewGitHubClient: func(gitClient git.Git, cfg *viper.Viper) (internalinspect.PullRequestInspector, error) {
					return newGitHubClient(gitClient, cfg)
				},
			},
		}

		result, err := runner.RunPullRequest()
		if err != nil {
			outputError(err)
			return
		}

		outputJSON(buildInspectPROutput(result))
	},
}

// InspectPROutput represents the JSON output of the inspect pr command.
type InspectPROutput struct {
	Branch      *BranchInfo              `json:"branch,omitempty"`
	PullRequest *PullRequestInfo         `json:"pullRequest,omitempty"`
	Comments    []PullRequestCommentInfo `json:"comments"`
	Reviews     []PullRequestReviewInfo  `json:"reviews"`
}

// PullRequestCommentInfo represents a top-level pull request comment in inspect output.
type PullRequestCommentInfo struct {
	ID                int64  `json:"id"`
	Author            string `json:"author,omitempty"`
	Body              string `json:"body,omitempty"`
	URL               string `json:"url,omitempty"`
	HTMLURL           string `json:"htmlUrl,omitempty"`
	AuthorAssociation string `json:"authorAssociation,omitempty"`
	CreatedAt         string `json:"createdAt,omitempty"`
	UpdatedAt         string `json:"updatedAt,omitempty"`
}

// PullRequestReviewInfo represents a submitted pull request review in inspect output.
type PullRequestReviewInfo struct {
	ID                int64                         `json:"id"`
	Author            string                        `json:"author,omitempty"`
	State             string                        `json:"state,omitempty"`
	Body              string                        `json:"body,omitempty"`
	CommitID          string                        `json:"commitId,omitempty"`
	URL               string                        `json:"url,omitempty"`
	HTMLURL           string                        `json:"htmlUrl,omitempty"`
	AuthorAssociation string                        `json:"authorAssociation,omitempty"`
	SubmittedAt       string                        `json:"submittedAt,omitempty"`
	Conversations     []PullRequestConversationInfo `json:"conversations"`
}

// PullRequestReviewCommentInfo represents an inline pull request review comment in inspect output.
type PullRequestReviewCommentInfo struct {
	ID                   int64  `json:"id"`
	NodeID               string `json:"nodeId,omitempty"`
	ReviewID             int64  `json:"reviewId,omitempty"`
	InReplyToID          int64  `json:"inReplyToId,omitempty"`
	Author               string `json:"author,omitempty"`
	Body                 string `json:"body,omitempty"`
	Path                 string `json:"path,omitempty"`
	DiffHunk             string `json:"diffHunk,omitempty"`
	Side                 string `json:"side,omitempty"`
	StartSide            string `json:"startSide,omitempty"`
	Line                 int    `json:"line,omitempty"`
	StartLine            int    `json:"startLine,omitempty"`
	OriginalLine         int    `json:"originalLine,omitempty"`
	OriginalStartLine    int    `json:"originalStartLine,omitempty"`
	Position             int    `json:"position,omitempty"`
	OriginalPosition     int    `json:"originalPosition,omitempty"`
	CommitID             string `json:"commitId,omitempty"`
	OriginalCommitID     string `json:"originalCommitId,omitempty"`
	SubjectType          string `json:"subjectType,omitempty"`
	URL                  string `json:"url,omitempty"`
	HTMLURL              string `json:"htmlUrl,omitempty"`
	PullRequestURL       string `json:"pullRequestUrl,omitempty"`
	AuthorAssociation    string `json:"authorAssociation,omitempty"`
	CreatedAt            string `json:"createdAt,omitempty"`
	UpdatedAt            string `json:"updatedAt,omitempty"`
	ConversationID       string `json:"conversationId,omitempty"`
	ConversationResolved *bool  `json:"conversationResolved,omitempty"`
}

// PullRequestConversationInfo represents grouped inline review comments.
type PullRequestConversationInfo struct {
	ID       string                         `json:"id"`
	Resolved *bool                          `json:"resolved,omitempty"`
	Comments []PullRequestReviewCommentInfo `json:"comments"`
}

func buildInspectPROutput(result internalinspect.WorkflowResult) InspectPROutput {
	output := InspectPROutput{
		Comments: []PullRequestCommentInfo{},
		Reviews:  []PullRequestReviewInfo{},
	}
	if result.Branch != nil {
		output.Branch = &BranchInfo{Name: result.Branch.Name}
	}
	if result.PullRequest != nil {
		output.PullRequest = &PullRequestInfo{
			Number:      result.PullRequest.Number,
			URL:         result.PullRequest.URL,
			APIURL:      result.PullRequest.APIURL,
			Title:       result.PullRequest.Title,
			Description: result.PullRequest.Description,
			Draft:       result.PullRequest.Draft,
			State:       result.PullRequest.State,
		}
	}
	if result.Discussion == nil {
		return output
	}

	output.Comments = mapPullRequestCommentInfos(result.Discussion.Comments)
	output.Reviews = mapPullRequestReviewInfos(result.Discussion.Reviews, result.Discussion.Conversations)
	return output
}

func mapPullRequestCommentInfos(comments []github.PullRequestIssueComment) []PullRequestCommentInfo {
	mapped := make([]PullRequestCommentInfo, 0, len(comments))
	for _, comment := range comments {
		mapped = append(mapped, PullRequestCommentInfo{
			ID:                comment.ID,
			Author:            comment.Author,
			Body:              comment.Body,
			URL:               comment.URL,
			HTMLURL:           comment.HTMLURL,
			AuthorAssociation: comment.AuthorAssociation,
			CreatedAt:         comment.CreatedAt,
			UpdatedAt:         comment.UpdatedAt,
		})
	}
	return mapped
}

func mapPullRequestReviewInfos(
	reviews []github.PullRequestReview,
	conversations []github.PullRequestConversation,
) []PullRequestReviewInfo {
	mapped := make([]PullRequestReviewInfo, 0, len(reviews))
	reviewIndexByID := make(map[int64]int, len(reviews))
	for _, review := range reviews {
		reviewIndexByID[review.ID] = len(mapped)
		mapped = append(mapped, PullRequestReviewInfo{
			ID:                review.ID,
			Author:            review.Author,
			State:             review.State,
			Body:              review.Body,
			CommitID:          review.CommitID,
			URL:               review.URL,
			HTMLURL:           review.HTMLURL,
			AuthorAssociation: review.AuthorAssociation,
			SubmittedAt:       review.SubmittedAt,
			Conversations:     []PullRequestConversationInfo{},
		})
	}

	for _, conversation := range conversations {
		if len(conversation.Comments) == 0 {
			continue
		}

		reviewID := conversation.Comments[0].ReviewID
		idx, ok := reviewIndexByID[reviewID]
		if !ok {
			continue
		}
		mapped[idx].Conversations = append(mapped[idx].Conversations, mapPullRequestConversationInfo(conversation))
	}

	return mapped
}

func mapPullRequestReviewCommentInfos(comments []github.PullRequestReviewComment) []PullRequestReviewCommentInfo {
	mapped := make([]PullRequestReviewCommentInfo, 0, len(comments))
	for _, comment := range comments {
		mapped = append(mapped, mapPullRequestReviewCommentInfo(comment))
	}
	return mapped
}

func mapPullRequestReviewCommentInfo(comment github.PullRequestReviewComment) PullRequestReviewCommentInfo {
	return PullRequestReviewCommentInfo{
		ID:                   comment.ID,
		NodeID:               comment.NodeID,
		ReviewID:             comment.ReviewID,
		InReplyToID:          comment.InReplyToID,
		Author:               comment.Author,
		Body:                 comment.Body,
		Path:                 comment.Path,
		DiffHunk:             comment.DiffHunk,
		Side:                 comment.Side,
		StartSide:            comment.StartSide,
		Line:                 comment.Line,
		StartLine:            comment.StartLine,
		OriginalLine:         comment.OriginalLine,
		OriginalStartLine:    comment.OriginalStartLine,
		Position:             comment.Position,
		OriginalPosition:     comment.OriginalPosition,
		CommitID:             comment.CommitID,
		OriginalCommitID:     comment.OriginalCommitID,
		SubjectType:          comment.SubjectType,
		URL:                  comment.URL,
		HTMLURL:              comment.HTMLURL,
		PullRequestURL:       comment.PullRequestURL,
		AuthorAssociation:    comment.AuthorAssociation,
		CreatedAt:            comment.CreatedAt,
		UpdatedAt:            comment.UpdatedAt,
		ConversationID:       comment.ConversationID,
		ConversationResolved: comment.ConversationResolved,
	}
}

func mapPullRequestConversationInfo(conversation github.PullRequestConversation) PullRequestConversationInfo {
	return PullRequestConversationInfo{
		ID:       conversation.ID,
		Resolved: conversation.Resolved,
		Comments: mapPullRequestReviewCommentInfos(conversation.Comments),
	}
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
