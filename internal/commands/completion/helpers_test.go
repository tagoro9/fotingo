package completion

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tagoro9/fotingo/internal/tracker"
)

func TestSplitCSVCompletionToken(t *testing.T) {
	prefix, token := SplitCSVCompletionToken("alice,b")
	assert.Equal(t, "alice,", prefix)
	assert.Equal(t, "b", token)
}

func TestApplyCSVCompletionPrefix(t *testing.T) {
	values := ApplyCSVCompletionPrefix("alice,", []string{"bob\tBob"})
	assert.Equal(t, []string{"alice,bob\tBob"}, values)
}

func TestCollectProjectsFromIssues(t *testing.T) {
	projects := CollectProjectsFromIssues([]tracker.Issue{
		{Key: "DEVEX-1"},
		{Key: "FOTINGO-2"},
		{Key: "DEVEX-3"},
	})
	assert.Equal(t, []string{"DEVEX", "FOTINGO"}, projects)
}

func TestFilterByContainsFold(t *testing.T) {
	filtered := FilterByContainsFold([]string{"Bug", "Story"}, "st")
	assert.Equal(t, []string{"Story"}, filtered)
}

func TestFormatReviewCompletionCandidate(t *testing.T) {
	user := FormatReviewCompletionCandidate(ReviewMatchOption{
		Resolved: "alice",
		Detail:   "Alice Dev",
		Kind:     ReviewMatchKindUser,
	})
	assert.Equal(t, "alice\tAlice Dev", user)

	label := FormatReviewCompletionCandidate(ReviewMatchOption{
		Resolved: "bug",
		Detail:   "Bug fixes",
		Kind:     "label",
	})
	assert.Equal(t, "bug", label)
}
