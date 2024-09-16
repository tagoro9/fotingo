package commands

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/commandruntime"
)

func TestOpenEditorWithRuntime_NoActiveTerminal(t *testing.T) {
	origProcessFn := openEditorProcessFn
	defer func() { openEditorProcessFn = origProcessFn }()

	openEditorProcessFn = func(initialContent string) (string, error) {
		return initialContent + "\nupdated", nil
	}

	content, err := openEditorWithRuntime("seed")
	require.NoError(t, err)
	assert.Equal(t, "seed\nupdated", content)
}

func TestOpenEditorWithRuntime_UsesTerminalHandoff(t *testing.T) {
	controller := &mockTerminalController{}
	origProcessFn := openEditorProcessFn
	defer func() { openEditorProcessFn = origProcessFn }()

	openEditorProcessFn = func(initialContent string) (string, error) {
		assert.Equal(t, "seed", initialContent)
		return "edited", nil
	}

	var content string
	err := withActiveTerminal(controller, func() error {
		var runErr error
		content, runErr = openEditorWithRuntime("seed")
		return runErr
	})

	require.NoError(t, err)
	assert.Equal(t, "edited", content)
	assert.Equal(t, 1, controller.releaseCalls)
	assert.Equal(t, 1, controller.restoreCalls)
}

func TestOpenEditorWithRuntime_PropagatesTerminalRestoreFailure(t *testing.T) {
	controller := &mockTerminalController{restoreErr: errors.New("restore failed")}
	origProcessFn := openEditorProcessFn
	defer func() { openEditorProcessFn = origProcessFn }()

	openEditorProcessFn = func(initialContent string) (string, error) {
		return "edited", nil
	}

	err := withActiveTerminal(controller, func() error {
		_, runErr := openEditorWithRuntime("seed")
		return runErr
	})

	require.Error(t, err)
	var handoffErr *commandruntime.HandoffError
	require.ErrorAs(t, err, &handoffErr)
	assert.Equal(t, commandruntime.HandoffStageRestore, handoffErr.Stage)
}
