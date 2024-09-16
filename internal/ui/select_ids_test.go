package ui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectIDsReturnsSelectedIDs(t *testing.T) {
	origRunner := runMultiSelectIDsProgram
	t.Cleanup(func() {
		runMultiSelectIDsProgram = origRunner
	})

	runMultiSelectIDsProgram = func(opts ...MultiSelectOption) ([]PickerItem, error) {
		return []PickerItem{{ID: "key:one"}, {ID: "key:two"}}, nil
	}

	ids, err := SelectIDs("Select", []MultiSelectItem{
		{ID: "key:one", Label: "one"},
		{ID: "key:two", Label: "two"},
	}, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{"key:one", "key:two"}, ids)
}

func TestSelectIDsPropagatesRunnerError(t *testing.T) {
	origRunner := runMultiSelectIDsProgram
	t.Cleanup(func() {
		runMultiSelectIDsProgram = origRunner
	})

	expectedErr := errors.New("picker failed")
	runMultiSelectIDsProgram = func(opts ...MultiSelectOption) ([]PickerItem, error) {
		return nil, expectedErr
	}

	ids, err := SelectIDs("Select", []MultiSelectItem{{ID: "key:one", Label: "one"}}, 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, ids)
}
