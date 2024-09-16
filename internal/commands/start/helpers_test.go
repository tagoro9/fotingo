package start

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tagoro9/fotingo/internal/tracker"
)

func TestParseIssueKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected tracker.IssueType
		ok       bool
	}{
		{name: "story", input: "story", expected: tracker.IssueTypeStory, ok: true},
		{name: "bug", input: "BUG", expected: tracker.IssueTypeBug, ok: true},
		{name: "task", input: "Task", expected: tracker.IssueTypeTask, ok: true},
		{name: "subtask", input: "sub-task", expected: tracker.IssueTypeSubTask, ok: true},
		{name: "epic", input: "Epic", expected: tracker.IssueTypeEpic, ok: true},
		{name: "invalid", input: "unknown", expected: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParseIssueKind(tt.input)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseInteractiveLabels(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{}, ParseInteractiveLabels(""))
	assert.Equal(t, []string{}, ParseInteractiveLabels(" ,  , "))
	assert.Equal(t, []string{"frontend", "priority", "qa"}, ParseInteractiveLabels("frontend, priority, qa"))
}

func TestIssueMatchesAllowedTypes(t *testing.T) {
	t.Parallel()

	assert.True(t, IssueMatchesAllowedTypes(tracker.IssueTypeStory, nil))
	assert.True(t, IssueMatchesAllowedTypes(tracker.IssueTypeStory, []tracker.IssueType{tracker.IssueTypeStory}))
	assert.False(t, IssueMatchesAllowedTypes(tracker.IssueTypeStory, []tracker.IssueType{tracker.IssueTypeBug}))
}

func TestProjectIssueTypeNames(t *testing.T) {
	t.Parallel()

	names := ProjectIssueTypeNames([]tracker.ProjectIssueType{
		{Name: "Story"},
		{Name: "  Bug  "},
		{Name: ""},
		{Name: "   "},
	})
	assert.Equal(t, []string{"Story", "Bug"}, names)
}
