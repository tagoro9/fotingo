package commandruntime

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockController struct {
	releaseErr   error
	restoreErr   error
	releaseCalls int
	restoreCalls int
}

func (m *mockController) ReleaseTerminal() error {
	m.releaseCalls++
	return m.releaseErr
}

func (m *mockController) RestoreTerminal() error {
	m.restoreCalls++
	return m.restoreErr
}

func TestRunWithControllerHandoffSuccess(t *testing.T) {
	mgr := NewManager()
	controller := &mockController{}
	called := false

	err := mgr.RunWithControllerHandoff(controller, func() error {
		called = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 1, controller.releaseCalls)
	assert.Equal(t, 1, controller.restoreCalls)
}

func TestRunWithControllerHandoffReleaseFailure(t *testing.T) {
	mgr := NewManager()
	controller := &mockController{releaseErr: errors.New("release failed")}

	err := mgr.RunWithControllerHandoff(controller, func() error {
		return nil
	})

	require.Error(t, err)
	assert.Equal(t, 1, controller.releaseCalls)
	assert.Equal(t, 0, controller.restoreCalls)

	var handoffErr *HandoffError
	require.ErrorAs(t, err, &handoffErr)
	assert.Equal(t, HandoffStageRelease, handoffErr.Stage)
}

func TestRunWithControllerHandoffRestoreFailure(t *testing.T) {
	mgr := NewManager()
	controller := &mockController{restoreErr: errors.New("restore failed")}

	err := mgr.RunWithControllerHandoff(controller, func() error {
		return nil
	})

	require.Error(t, err)
	assert.Equal(t, 1, controller.releaseCalls)
	assert.Equal(t, 1, controller.restoreCalls)

	var handoffErr *HandoffError
	require.ErrorAs(t, err, &handoffErr)
	assert.Equal(t, HandoffStageRestore, handoffErr.Stage)
}

func TestRunWithControllerHandoffProcessAndRestoreFailure(t *testing.T) {
	mgr := NewManager()
	controller := &mockController{restoreErr: errors.New("restore failed")}
	processErr := errors.New("process failed")

	err := mgr.RunWithControllerHandoff(controller, func() error {
		return processErr
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, processErr)

	var handoffErr *HandoffError
	require.ErrorAs(t, err, &handoffErr)
	assert.Equal(t, HandoffStageRestore, handoffErr.Stage)
}

func TestRunWithControllerHandoffReentrant(t *testing.T) {
	mgr := NewManager()
	controller := &mockController{}
	done := make(chan error, 1)

	go func() {
		done <- mgr.RunWithControllerHandoff(controller, func() error {
			return mgr.RunWithControllerHandoff(controller, func() error {
				return nil
			})
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reentrant handoff")
	}

	assert.Equal(t, 1, controller.releaseCalls)
	assert.Equal(t, 1, controller.restoreCalls)
}

func TestManagerWithActiveTerminalRestoresPreviousController(t *testing.T) {
	mgr := NewManager()
	first := &mockController{}
	second := &mockController{}

	require.NoError(t, mgr.WithActiveTerminal(first, func() error {
		assert.Same(t, first, mgr.CurrentTerminalController())
		return mgr.WithActiveTerminal(second, func() error {
			assert.Same(t, second, mgr.CurrentTerminalController())
			return nil
		})
	}))

	assert.Nil(t, mgr.CurrentTerminalController())
}

func TestRunWithTerminalHandoffUsesActiveController(t *testing.T) {
	mgr := NewManager()
	controller := &mockController{}
	called := false

	err := mgr.WithActiveTerminal(controller, func() error {
		return mgr.RunWithTerminalHandoff(func() error {
			called = true
			return nil
		})
	})
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 1, controller.releaseCalls)
	assert.Equal(t, 1, controller.restoreCalls)
}

func TestHandoffErrorFormattingAndUnwrap(t *testing.T) {
	base := errors.New("boom")
	err := &HandoffError{Stage: HandoffStageRelease, Err: base}
	assert.Contains(t, err.Error(), "failed to release terminal")
	assert.ErrorIs(t, err, base)

	restoreErr := &HandoffError{Stage: HandoffStageRestore, Err: base}
	assert.Contains(t, restoreErr.Error(), "failed to restore terminal")

	unknownErr := &HandoffError{Stage: HandoffStage("unknown"), Err: base}
	assert.Contains(t, unknownErr.Error(), "terminal handoff failed")
}
