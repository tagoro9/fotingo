package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	internalinspect "github.com/tagoro9/fotingo/internal/commands/inspect"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
)

func TestInspectFlags(t *testing.T) {
	t.Parallel()

	// Verify the inspect command has expected flags
	flags := inspectCmd.Flags()

	branchFlag := flags.Lookup("branch")
	assert.NotNil(t, branchFlag, "branch flag should exist")
	assert.Equal(t, "b", branchFlag.Shorthand)

	issueFlag := flags.Lookup("issue")
	assert.NotNil(t, issueFlag, "issue flag should exist")
	assert.Equal(t, "i", issueFlag.Shorthand)
}

func TestInspectCmdUse(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "inspect", inspectCmd.Use)
	assert.NotEmpty(t, inspectCmd.Short)
	assert.NotEmpty(t, inspectCmd.Long)
	assert.Contains(t, inspectCmd.Long, "JSON")
	assert.Contains(t, inspectCmd.Long, "AI agents")
}

func TestInspectPrCmd(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "pr", inspectPrCmd.Use)
	assert.NotEmpty(t, inspectPrCmd.Short)
	assert.NotEmpty(t, inspectPrCmd.Long)

	branchFlag := inspectPrCmd.Flags().Lookup("branch")
	assert.NotNil(t, branchFlag, "branch flag should exist")
	assert.Equal(t, "b", branchFlag.Shorthand)
}

func TestInspectOutput_BranchInfo(t *testing.T) {
	t.Parallel()

	output := InspectOutput{
		Branch: &BranchInfo{
			Name:          "f/TEST-123_test_branch",
			IssueID:       "TEST-123",
			DefaultBranch: "main",
		},
	}

	assert.NotNil(t, output.Branch)
	assert.Equal(t, "f/TEST-123_test_branch", output.Branch.Name)
	assert.Equal(t, "TEST-123", output.Branch.IssueID)
	assert.Equal(t, "main", output.Branch.DefaultBranch)
}

func TestInspectOutput_IssueInfo(t *testing.T) {
	t.Parallel()

	output := InspectOutput{
		Issue: &IssueInfo{
			Key:         "TEST-123",
			Summary:     "Test issue summary",
			Description: "Detailed issue description",
			Status:      "In Progress",
			Type:        "Story",
			ParentKey:   "TEST-100",
			EpicKey:     "TEST-1",
			URL:         "https://jira.example.com/browse/TEST-123",
		},
	}

	assert.NotNil(t, output.Issue)
	assert.Equal(t, "TEST-123", output.Issue.Key)
	assert.Equal(t, "Test issue summary", output.Issue.Summary)
	assert.Equal(t, "Detailed issue description", output.Issue.Description)
	assert.Equal(t, "In Progress", output.Issue.Status)
	assert.Equal(t, "Story", output.Issue.Type)
	assert.Equal(t, "TEST-100", output.Issue.ParentKey)
	assert.Equal(t, "TEST-1", output.Issue.EpicKey)
	assert.Equal(t, "https://jira.example.com/browse/TEST-123", output.Issue.URL)
}

func TestInspectOutput_Commits(t *testing.T) {
	t.Parallel()

	output := InspectOutput{
		Commits: []CommitInfo{
			{
				Hash:    "abc123def456",
				Message: "feat: add new feature",
				Author:  "Test User",
			},
			{
				Hash:    "def456ghi789",
				Message: "fix: resolve bug",
				Author:  "Test User",
			},
		},
	}

	assert.Len(t, output.Commits, 2)
	assert.Equal(t, "abc123def456", output.Commits[0].Hash)
	assert.Equal(t, "feat: add new feature", output.Commits[0].Message)
	assert.Equal(t, "fix: resolve bug", output.Commits[1].Message)
}

func TestInspectOutput_IssueIDs(t *testing.T) {
	t.Parallel()

	output := InspectOutput{
		IssueIDs: []string{"TEST-123", "TEST-456", "TEST-789"},
	}

	assert.Len(t, output.IssueIDs, 3)
	assert.Contains(t, output.IssueIDs, "TEST-123")
	assert.Contains(t, output.IssueIDs, "TEST-456")
	assert.Contains(t, output.IssueIDs, "TEST-789")
}

func TestInspectOutput_PullRequestInfo(t *testing.T) {
	t.Parallel()

	output := InspectOutput{
		PullRequest: &InspectPRInfo{
			Number:      42,
			Title:       "Inspect PR metadata",
			Description: "PR body",
			URL:         "https://github.com/testowner/testrepo/pull/42",
		},
	}

	assert.NotNil(t, output.PullRequest)
	assert.Equal(t, 42, output.PullRequest.Number)
	assert.Equal(t, "Inspect PR metadata", output.PullRequest.Title)
	assert.Equal(t, "PR body", output.PullRequest.Description)
	assert.Equal(t, "https://github.com/testowner/testrepo/pull/42", output.PullRequest.URL)
}

func TestBuildInspectPROutput(t *testing.T) {
	t.Parallel()

	resolved := true
	output := buildInspectPROutput(internalinspect.WorkflowResult{
		Branch: &internalinspect.BranchInfo{Name: "feature/TEST-123"},
		PullRequest: &internalinspect.PullRequestInfo{
			Title:       "Feature PR",
			Description: "PR body",
			Number:      42,
			URL:         "https://github.com/owner/repo/pull/42",
			APIURL:      "https://api.github.com/repos/owner/repo/pulls/42",
			State:       "open",
			Draft:       false,
		},
		Discussion: &github.PullRequestDiscussion{
			Comments: []github.PullRequestIssueComment{
				{ID: 101, Author: "alice", Body: "Top-level comment"},
			},
			Reviews: []github.PullRequestReview{
				{ID: 201, Author: "bob", State: "COMMENTED", Body: "Review body"},
			},
			ReviewComments: []github.PullRequestReviewComment{
				{ID: 301, ReviewID: 201, Author: "bob", Body: "Inline comment", ConversationID: "review-comment-301"},
			},
			Conversations: []github.PullRequestConversation{
				{
					ID:       "review-comment-301",
					Resolved: &resolved,
					Comments: []github.PullRequestReviewComment{
						{ID: 301, ReviewID: 201, Author: "bob", Body: "Inline comment", ConversationID: "review-comment-301"},
					},
				},
			},
		},
	})

	require := assert.New(t)
	require.NotNil(output.Branch)
	require.Equal("feature/TEST-123", output.Branch.Name)
	require.NotNil(output.PullRequest)
	require.Equal(42, output.PullRequest.Number)
	require.Equal("PR body", output.PullRequest.Description)
	require.Len(output.Comments, 1)
	require.Equal("alice", output.Comments[0].Author)
	require.Len(output.Reviews, 1)
	require.Equal("COMMENTED", output.Reviews[0].State)
	require.Len(output.Reviews[0].Conversations, 1)
	require.NotNil(output.Reviews[0].Conversations[0].Resolved)
	require.True(*output.Reviews[0].Conversations[0].Resolved)
	require.Len(output.Reviews[0].Conversations[0].Comments, 1)
	require.Equal("review-comment-301", output.Reviews[0].Conversations[0].Comments[0].ConversationID)
}

func TestExtractIssueIDsFromCommits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		commits []git.Commit
		want    []string
	}{
		{
			name: "single issue ID",
			commits: []git.Commit{
				{Message: "feat: implement feature for TEST-123"},
			},
			want: []string{"TEST-123"},
		},
		{
			name: "multiple issue IDs in same message",
			commits: []git.Commit{
				{Message: "fix: resolve TEST-123 and TEST-456"},
			},
			want: []string{"TEST-123", "TEST-456"},
		},
		{
			name: "duplicate issue IDs are deduplicated",
			commits: []git.Commit{
				{Message: "feat: TEST-123 first commit"},
				{Message: "feat: TEST-123 second commit"},
			},
			want: []string{"TEST-123"},
		},
		{
			name: "no issue IDs",
			commits: []git.Commit{
				{Message: "chore: update dependencies"},
			},
			want: nil,
		},
		{
			name: "multiple commits with different IDs",
			commits: []git.Commit{
				{Message: "feat: implement PROJ-123"},
				{Message: "fix: resolve PROJ-456"},
				{Message: "chore: cleanup for PROJ-789"},
			},
			want: []string{"PROJ-123", "PROJ-456", "PROJ-789"},
		},
		{
			name: "issue ID at start of message",
			commits: []git.Commit{
				{Message: "TEST-123: fix bug"},
			},
			want: []string{"TEST-123"},
		},
		{
			name: "issue ID in brackets",
			commits: []git.Commit{
				{Message: "[TEST-123] Fix bug"},
			},
			want: []string{"TEST-123"},
		},
		{
			name: "complex project key",
			commits: []git.Commit{
				{Message: "feat: TEAM_DEV-123 new feature"},
			},
			want: []string{"TEAM_DEV-123"},
		},
		{
			name: "project key with numbers",
			commits: []git.Commit{
				{Message: "feat: PROJECT2-123 new feature"},
			},
			want: []string{"PROJECT2-123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractIssueIDsFromCommits(tt.commits)

			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestIssueIDPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "standard issue ID",
			input: "TEST-123",
			want:  []string{"TEST-123"},
		},
		{
			name:  "issue ID with underscore",
			input: "TEST_PROJECT-123",
			want:  []string{"TEST_PROJECT-123"},
		},
		{
			name:  "issue ID with numbers in key",
			input: "TEST2-123",
			want:  []string{"TEST2-123"},
		},
		{
			name:  "multiple issue IDs",
			input: "TEST-123 and PROJ-456",
			want:  []string{"TEST-123", "PROJ-456"},
		},
		{
			name:  "lowercase should not match",
			input: "test-123",
			want:  nil,
		},
		{
			name:  "no hyphen should not match",
			input: "TEST123",
			want:  nil,
		},
		{
			name:  "starts with number extracts only valid part",
			input: "1TEST-123",
			want:  []string{"TEST-123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := issueIDPattern.FindAllString(tt.input, -1)

			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
