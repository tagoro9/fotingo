package review

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/ui"
)

func TestPickMatchWithPicker(t *testing.T) {
	selected, err := PickMatchWithPicker(
		"reviewer",
		"alice",
		[]MatchOption{{Resolved: "alice", Label: "alice", Detail: "Alice"}},
		func(title string, items []ui.PickerItem) (*ui.PickerItem, error) {
			require.Contains(t, title, "reviewer")
			require.Len(t, items, 1)
			assert.Equal(t, "alice", items[0].ID)
			return &ui.PickerItem{ID: "alice"}, nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, "alice", selected)
}

func TestPickMatchWithPicker_Cancelled(t *testing.T) {
	_, err := PickMatchWithPicker(
		"reviewer",
		"alice",
		[]MatchOption{{Resolved: "alice", Label: "alice"}},
		func(_ string, _ []ui.PickerItem) (*ui.PickerItem, error) {
			return nil, nil
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection cancelled")
}

func TestPickMatchWithPicker_RunError(t *testing.T) {
	_, err := PickMatchWithPicker(
		"reviewer",
		"alice",
		[]MatchOption{{Resolved: "alice", Label: "alice"}},
		func(_ string, _ []ui.PickerItem) (*ui.PickerItem, error) {
			return nil, errors.New("picker failed")
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "picker failed")
}
