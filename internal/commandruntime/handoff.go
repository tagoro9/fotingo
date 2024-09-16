package commandruntime

import (
	"errors"
	"fmt"
	"sync"
)

// TerminalController controls Bubble Tea terminal ownership transitions.
type TerminalController interface {
	ReleaseTerminal() error
	RestoreTerminal() error
}

// HandoffStage identifies which handoff phase failed.
type HandoffStage string

const (
	HandoffStageRelease HandoffStage = "release"
	HandoffStageRestore HandoffStage = "restore"
)

// HandoffError captures which part of terminal ownership transition failed.
type HandoffError struct {
	Stage HandoffStage
	Err   error
}

func (e *HandoffError) Error() string {
	switch e.Stage {
	case HandoffStageRelease:
		return fmt.Sprintf("failed to release terminal for external process: %v", e.Err)
	case HandoffStageRestore:
		return fmt.Sprintf("failed to restore terminal after external process: %v", e.Err)
	default:
		return fmt.Sprintf("terminal handoff failed: %v", e.Err)
	}
}

func (e *HandoffError) Unwrap() error {
	return e.Err
}

// Manager coordinates terminal handoff for external interactive processes.
type Manager struct {
	activeTerminalMu sync.RWMutex
	activeTerminal   TerminalController

	externalProcessMu sync.Mutex
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) WithActiveTerminal(controller TerminalController, fn func() error) error {
	m.activeTerminalMu.Lock()
	previous := m.activeTerminal
	m.activeTerminal = controller
	m.activeTerminalMu.Unlock()

	defer func() {
		m.activeTerminalMu.Lock()
		m.activeTerminal = previous
		m.activeTerminalMu.Unlock()
	}()

	return fn()
}

func (m *Manager) CurrentTerminalController() TerminalController {
	m.activeTerminalMu.RLock()
	defer m.activeTerminalMu.RUnlock()
	return m.activeTerminal
}

// RunWithTerminalHandoff executes interactive subprocess work while
// temporarily releasing Bubble Tea terminal ownership, then restoring it afterwards.
func (m *Manager) RunWithTerminalHandoff(run func() error) error {
	return m.RunWithControllerHandoff(m.CurrentTerminalController(), run)
}

func (m *Manager) RunWithControllerHandoff(controller TerminalController, run func() error) error {
	if controller == nil {
		return run()
	}

	locked := m.externalProcessMu.TryLock()
	if !locked {
		// Nested handoff call while terminal is already released.
		return run()
	}
	defer m.externalProcessMu.Unlock()

	if err := controller.ReleaseTerminal(); err != nil {
		return &HandoffError{Stage: HandoffStageRelease, Err: err}
	}

	runErr := run()
	restoreErr := controller.RestoreTerminal()
	if restoreErr != nil {
		restoreFailure := &HandoffError{Stage: HandoffStageRestore, Err: restoreErr}
		if runErr != nil {
			return errors.Join(runErr, restoreFailure)
		}
		return restoreFailure
	}

	return runErr
}
