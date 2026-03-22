package ui

import (
	"sync"

	tea "charm.land/bubbletea/v2"
)

var (
	programRunnerMu sync.RWMutex
	programRunner   = func(run func() error) error {
		return run()
	}
)

// SetProgramRunner configures an optional wrapper for interactive TUI program execution.
// Passing nil resets to direct execution.
func SetProgramRunner(runner func(run func() error) error) {
	programRunnerMu.Lock()
	defer programRunnerMu.Unlock()

	if runner == nil {
		programRunner = func(run func() error) error {
			return run()
		}
		return
	}

	programRunner = runner
}

func runTeaProgram(program *tea.Program) error {
	programRunnerMu.RLock()
	runner := programRunner
	programRunnerMu.RUnlock()

	return runner(func() error {
		_, err := program.Run()
		return err
	})
}
