package ui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectOne(t *testing.T) {
	origRunner := runSelectOneProgram
	t.Cleanup(func() { runSelectOneProgram = origRunner })

	runSelectOneProgram = func(opts ...PickerOption) (*PickerItem, error) {
		model := NewPicker(opts...)
		assert.Equal(t, "Pick", model.title)
		assert.True(t, model.showSearch)
		require.Len(t, model.items, 1)
		return &PickerItem{ID: "a"}, nil
	}

	selected, err := SelectOne("Pick", []PickerItem{{ID: "a", Label: "A"}})
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "a", selected.ID)
}

func TestSelectOne_PropagatesError(t *testing.T) {
	origRunner := runSelectOneProgram
	t.Cleanup(func() { runSelectOneProgram = origRunner })

	runSelectOneProgram = func(opts ...PickerOption) (*PickerItem, error) {
		return nil, errors.New("picker failed")
	}

	selected, err := SelectOne("Pick", []PickerItem{{ID: "a", Label: "A"}})
	require.Error(t, err)
	assert.Nil(t, selected)
	assert.Contains(t, err.Error(), "picker failed")
}
