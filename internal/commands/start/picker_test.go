package start

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/tracker"
	"github.com/tagoro9/fotingo/internal/ui"
)

func TestSelectIssueWithPicker(t *testing.T) {
	t.Parallel()

	issues := []tracker.Issue{
		{Key: "TEST-1", Summary: "Issue one", Type: tracker.IssueTypeStory, Status: tracker.IssueStatusToDo},
		{Key: "TEST-2", Summary: "Issue two", Type: tracker.IssueTypeBug, Status: tracker.IssueStatusInProgress},
	}

	selected, err := SelectIssueWithPicker(
		issues,
		"Select",
		func(title string, items []ui.PickerItem) (*ui.PickerItem, error) {
			require.Equal(t, "Select", title)
			require.Len(t, items, 2)
			return &ui.PickerItem{Value: 1}, nil
		},
		func(status tracker.IssueStatus) string { return string(status) },
		func(issueType tracker.IssueType) string { return string(issueType) },
	)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "TEST-2", selected.Key)
}

func TestSelectIssueWithPicker_ErrorsAndNil(t *testing.T) {
	t.Parallel()

	issues := []tracker.Issue{{Key: "TEST-1", Summary: "Issue one"}}

	_, err := SelectIssueWithPicker(
		issues,
		"Select",
		func(title string, items []ui.PickerItem) (*ui.PickerItem, error) {
			return nil, errors.New("boom")
		},
		func(status tracker.IssueStatus) string { return "" },
		func(issueType tracker.IssueType) string { return "" },
	)
	require.Error(t, err)

	selected, err := SelectIssueWithPicker(
		issues,
		"Select",
		func(title string, items []ui.PickerItem) (*ui.PickerItem, error) {
			return nil, nil
		},
		func(status tracker.IssueStatus) string { return "" },
		func(issueType tracker.IssueType) string { return "" },
	)
	require.NoError(t, err)
	assert.Nil(t, selected)
}

func TestSelectIssueLinkWithPicker(t *testing.T) {
	t.Parallel()

	issues := []tracker.Issue{
		{Key: "TEST-1", Summary: "Issue one"},
		{Key: "TEST-2", Summary: "Issue two"},
	}

	selected, err := SelectIssueLinkWithPicker(
		issues,
		"Link",
		func(title string, items []ui.PickerItem) (*ui.PickerItem, error) {
			require.Equal(t, "Link", title)
			require.Len(t, items, 2)
			return &ui.PickerItem{Value: 0}, nil
		},
	)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "TEST-1", selected.Key)
}
