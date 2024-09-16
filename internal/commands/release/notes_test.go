package release

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tagoro9/fotingo/internal/template"
	"github.com/tagoro9/fotingo/internal/tracker"
)

func testNotesText() NotesText {
	return NotesText{
		NoIssues:              "No issues",
		HeadingCategoryFormat: "## %s\n",
		IssueBulletFormat:     "- [%s](%s) %s\n",
		CategoryBugFixes:      "Bug fixes",
		CategoryFeatures:      "Features",
		CategoryTasks:         "Tasks",
		CategorySubtasks:      "Subtasks",
		CategoryEpics:         "Epics",
	}
}

func TestFormatIssuesByCategory(t *testing.T) {
	issues := []*tracker.Issue{
		{Key: "ABC-2", Summary: "task", Type: tracker.IssueTypeTask, URL: "https://jira/ABC-2"},
		{Key: "ABC-1", Summary: "bug", Type: tracker.IssueTypeBug, URL: "https://jira/ABC-1"},
	}
	out := FormatIssuesByCategory(issues, nil, testNotesText())
	assert.Contains(t, out, "## Bug fixes")
	assert.Contains(t, out, "## Tasks")
	assert.True(t, strings.Index(out, "ABC-1") < strings.Index(out, "ABC-2"))
}

func TestFormatIssuesByCategory_MultipleCategories(t *testing.T) {
	issues := []*tracker.Issue{
		{Key: "ABC-10", Summary: "bug", Type: tracker.IssueTypeBug, URL: "https://jira/ABC-10"},
		{Key: "ABC-11", Summary: "feature", Type: tracker.IssueTypeStory, URL: "https://jira/ABC-11"},
		{Key: "ABC-12", Summary: "task", Type: tracker.IssueTypeTask, URL: "https://jira/ABC-12"},
	}

	out := FormatIssuesByCategory(issues, nil, testNotesText())
	assert.Contains(t, out, "## Bug fixes")
	assert.Contains(t, out, "## Features")
	assert.Contains(t, out, "## Tasks")
}

func TestFormatIssuesByCategory_SortsByKey(t *testing.T) {
	issues := []*tracker.Issue{
		{Key: "ABC-3", Summary: "Third", Type: tracker.IssueTypeBug, URL: "https://jira/ABC-3"},
		{Key: "ABC-1", Summary: "First", Type: tracker.IssueTypeBug, URL: "https://jira/ABC-1"},
		{Key: "ABC-2", Summary: "Second", Type: tracker.IssueTypeBug, URL: "https://jira/ABC-2"},
	}

	out := FormatIssuesByCategory(issues, nil, testNotesText())
	assert.True(t, strings.Index(out, "ABC-1") < strings.Index(out, "ABC-2"))
	assert.True(t, strings.Index(out, "ABC-2") < strings.Index(out, "ABC-3"))
}

func TestFormatIssuesByCategoryNoIssues(t *testing.T) {
	assert.Equal(t, "No issues", FormatIssuesByCategory(nil, nil, testNotesText()))
}

func TestBuildReleaseNotes(t *testing.T) {
	issues := []*tracker.Issue{{Key: "ABC-1", Summary: "bug", Type: tracker.IssueTypeBug, URL: "https://jira/ABC-1"}}
	notes := BuildReleaseNotes(
		"v1.2.3",
		issues,
		nil,
		nil,
		"{version}\n{fixedIssuesByCategory}\n{fotingo.banner}",
		testNotesText(),
	)
	assert.Contains(t, notes, "v1.2.3")
	assert.Contains(t, notes, "ABC-1")
	assert.Contains(t, notes, template.DefaultFotingoBanner)
}

func TestBuildReleaseNotes_WithJiraRelease(t *testing.T) {
	notes := BuildReleaseNotes(
		"v1.2.3",
		nil,
		&tracker.Release{
			ID:   "1",
			Name: "v1.2.3",
			URL:  "https://jira/releases/1",
		},
		nil,
		"{version}\n{jira.release}",
		testNotesText(),
	)
	assert.Contains(t, notes, "v1.2.3")
	assert.Contains(t, notes, "https://jira/releases/1")
}
