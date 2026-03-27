package review

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/jira"
)

func TestDeriveSummaryAndDescription(t *testing.T) {
	issue := &jira.Issue{Key: "ABC-1", Summary: "Hello", Description: "Body\r\nLine"}
	assert.Equal(t, "ABC-1: Hello", DeriveSummary("feature/abc-1", issue, nil))
	assert.Equal(t, "Body\nLine", DeriveDescription(issue, nil, "-"))

	commits := []git.Commit{{Message: "newest"}, {Message: "oldest\n\nbody"}}
	assert.Equal(t, "oldest", DeriveSummary("feature/abc-1", nil, commits))
	assert.Equal(t, "body", DeriveDescription(nil, commits, "-"))
}

func TestSplitEditorContent(t *testing.T) {
	title, body := SplitEditorContent("My title\n\nBody line")
	assert.Equal(t, "My title", title)
	assert.Equal(t, "Body line", body)
}

func TestBuildEditorSeedContent(t *testing.T) {
	assert.Equal(t, "PR title\n\nBody line", BuildEditorSeedContent("PR title", "Body line"))
	assert.Equal(t, "PR title", BuildEditorSeedContent("PR title", ""))
	assert.Equal(t, "Body line", BuildEditorSeedContent("", "Body line"))
	assert.Equal(t, "PR title\n\nBody line", BuildEditorSeedContent("PR title\r\n", "Body line"))
}

func TestFormatChangesIncludesLineStats(t *testing.T) {
	commits := []git.Commit{
		{Message: "feat: newest", Additions: 7, Deletions: 2},
		{Message: "feat: oldest", Additions: 3, Deletions: 1},
	}

	assert.Equal(
		t,
		"* feat: oldest (+3/-1)\n* feat: newest (+7/-2)",
		FormatChanges(commits),
	)
}

func TestCollectLinkedIssueIDs_PreservesBranchIssueAndDeduplicates(t *testing.T) {
	issue := &jira.Issue{Key: "FOTINGO-10"}

	assert.Equal(
		t,
		[]string{"FOTINGO-10", "FOTINGO-1", "FOTINGO-2"},
		CollectLinkedIssueIDs(issue, []string{"FOTINGO-1", "FOTINGO-2", "FOTINGO-1"}),
	)
}

func TestFormatFixedIssues_RendersOneLinePerIssue(t *testing.T) {
	assert.Equal(
		t,
		"Fixes FOTINGO-1\nFixes FOTINGO-2",
		FormatFixedIssues([]string{"FOTINGO-1", "FOTINGO-2"}, nil),
	)
}
