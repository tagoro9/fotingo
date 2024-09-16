package commandruntime

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenEditorWithTerminalHandoff_WithoutRunner(t *testing.T) {
	content, err := OpenEditorWithTerminalHandoff("seed", nil, func(initial string) (string, error) {
		return initial + "\nupdated", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "seed\nupdated", content)
}

func TestOpenEditorWithTerminalHandoff_UsesRunner(t *testing.T) {
	runnerCalled := false
	openCalled := false
	content, err := OpenEditorWithTerminalHandoff(
		"seed",
		func(run func() error) error {
			runnerCalled = true
			return run()
		},
		func(initial string) (string, error) {
			openCalled = true
			assert.Equal(t, "seed", initial)
			return "edited", nil
		},
	)
	require.NoError(t, err)
	assert.True(t, runnerCalled)
	assert.True(t, openCalled)
	assert.Equal(t, "edited", content)
}

func TestOpenEditorWithTerminalHandoff_PropagatesRunnerError(t *testing.T) {
	expectedErr := errors.New("handoff failed")
	_, err := OpenEditorWithTerminalHandoff(
		"seed",
		func(_ func() error) error {
			return expectedErr
		},
		func(_ string) (string, error) {
			return "edited", nil
		},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}
