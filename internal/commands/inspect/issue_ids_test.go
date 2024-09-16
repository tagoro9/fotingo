package inspect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tagoro9/fotingo/internal/git"
)

func TestIssueIDPattern(t *testing.T) {
	matches := IssueIDPattern().FindAllString("foo ABC-123 and BAR_1-44", -1)
	assert.Equal(t, []string{"ABC-123", "BAR_1-44"}, matches)
}

func TestExtractIssueIDsFromCommits(t *testing.T) {
	commits := []git.Commit{{Message: "ABC-1 first"}, {Message: "fix DEF-2 and ABC-1"}, {Message: "none"}}
	ids := ExtractIssueIDsFromCommits(commits)
	assert.Equal(t, []string{"ABC-1", "DEF-2"}, ids)
}
