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
