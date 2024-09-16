package commands

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/tracker"
	"github.com/tagoro9/fotingo/internal/ui"
)

func TestSelectIssueWithPicker(t *testing.T) {
	origSelectOne := startSelectOneFn
	t.Cleanup(func() { startSelectOneFn = origSelectOne })

	issues := []tracker.Issue{
		{Key: "ABC-1", Summary: "First", Status: tracker.IssueStatusToDo, Type: tracker.IssueTypeTask},
		{Key: "ABC-2", Summary: "Second", Status: tracker.IssueStatusInProgress, Type: tracker.IssueTypeBug},
	}

	startSelectOneFn = func(title string, items []ui.PickerItem) (*ui.PickerItem, error) {
		require.NotEmpty(t, title)
		require.Len(t, items, 2)
		return &ui.PickerItem{ID: items[1].ID, Value: 1}, nil
	}

	selected, err := selectIssueWithPicker(issues)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "ABC-2", selected.Key)
}

func TestSelectIssueWithPicker_Cancelled(t *testing.T) {
	origSelectOne := startSelectOneFn
	t.Cleanup(func() { startSelectOneFn = origSelectOne })

	startSelectOneFn = func(_ string, _ []ui.PickerItem) (*ui.PickerItem, error) {
		return nil, nil
	}

	selected, err := selectIssueWithPicker([]tracker.Issue{{Key: "ABC-1", Summary: "First"}})
	require.Error(t, err)
	assert.Nil(t, selected)
	assert.Contains(t, err.Error(), "cancel")
}

func TestSelectIssueWithPicker_RunError(t *testing.T) {
	origSelectOne := startSelectOneFn
	t.Cleanup(func() { startSelectOneFn = origSelectOne })

	startSelectOneFn = func(_ string, _ []ui.PickerItem) (*ui.PickerItem, error) {
		return nil, errors.New("picker failed")
	}

	selected, err := selectIssueWithPicker([]tracker.Issue{{Key: "ABC-1", Summary: "First"}})
	require.Error(t, err)
	assert.Nil(t, selected)
	assert.Contains(t, err.Error(), "picker failed")
}

func TestSelectIssueLinkWithPicker(t *testing.T) {
	origSelectOne := startSelectOneFn
	t.Cleanup(func() { startSelectOneFn = origSelectOne })

	issues := []tracker.Issue{
		{Key: "ABC-1", Summary: "First"},
		{Key: "ABC-2", Summary: "Second"},
	}

	startSelectOneFn = func(title string, items []ui.PickerItem) (*ui.PickerItem, error) {
		require.Equal(t, "Pick issue", title)
		require.Len(t, items, 2)
		return &ui.PickerItem{ID: items[0].ID, Value: 0}, nil
	}

	selected, err := selectIssueLinkWithPicker(issues, "Pick issue")
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "ABC-1", selected.Key)
}

func TestSelectIssueLinkWithPicker_Cancelled(t *testing.T) {
	origSelectOne := startSelectOneFn
	t.Cleanup(func() { startSelectOneFn = origSelectOne })

	startSelectOneFn = func(_ string, _ []ui.PickerItem) (*ui.PickerItem, error) {
		return nil, nil
	}

	selected, err := selectIssueLinkWithPicker([]tracker.Issue{{Key: "ABC-1", Summary: "First"}}, "Pick issue")
	require.NoError(t, err)
	assert.Nil(t, selected)
}

func TestSelectIssueLinkWithPicker_RunError(t *testing.T) {
	origSelectOne := startSelectOneFn
	t.Cleanup(func() { startSelectOneFn = origSelectOne })

	startSelectOneFn = func(_ string, _ []ui.PickerItem) (*ui.PickerItem, error) {
		return nil, errors.New("picker failed")
	}

	selected, err := selectIssueLinkWithPicker([]tracker.Issue{{Key: "ABC-1", Summary: "First"}}, "Pick issue")
	require.Error(t, err)
	assert.Nil(t, selected)
	assert.Contains(t, err.Error(), "picker failed")
}
