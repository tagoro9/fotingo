package commands

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tagoro9/fotingo/internal/auth"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/ui"
)

type terminalController = commandruntime.TerminalController

var commandRuntimeManager = commandruntime.NewManager()

func init() {
	ui.SetProgramRunner(runInteractiveProcessWithTerminalHandoff)
	auth.SetInteractiveFlowRunner(runInteractiveProcessWithTerminalHandoff)
}

func withActiveTerminal(controller terminalController, fn func() error) error {
	return commandRuntimeManager.WithActiveTerminal(controller, fn)
}

func currentTerminalController() terminalController {
	return commandRuntimeManager.CurrentTerminalController()
}

// runInteractiveProcessWithTerminalHandoff executes interactive subprocess work while
// temporarily releasing Bubble Tea terminal ownership, then restoring it afterwards.
func runInteractiveProcessWithTerminalHandoff(run func() error) error {
	return commandRuntimeManager.RunWithTerminalHandoff(run)
}

func runInteractiveProcessWithControllerHandoff(controller terminalController, run func() error) error {
	return commandRuntimeManager.RunWithControllerHandoff(controller, run)
}

// runWithSpinner runs a work function inside a Bubble Tea spinner program.
// The work function receives a status emitter.
// If the work function returns an error, it is displayed with a boom emoji
// and returned after the spinner program exits.
func runWithSpinner(work func(out commandruntime.LocalizedEmitter) error) error {
	// Buffer startup/status events so producers don't block during program startup.
	statusCh := make(chan string, 32)
	var workErr error

	model := commandruntime.NewStatusModel(commandruntime.StatusModelOptions{
		SuppressOutput: ShouldSuppressOutput,
	})
	p := tea.NewProgram(model, tea.WithInput(os.Stdin))
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)

	go func() {
		for msg := range statusCh {
			p.Send(commandruntime.UpdateStatus(msg))
		}
		p.Send(commandruntime.FinishProcess())
	}()

	emitPendingStartupAnnouncements(out)

	go func() {
		workErr = work(out)
		if workErr != nil {
			out.InfoRaw(commandruntime.LogEmojiError, formatCommandError(workErr))
		}
		close(statusCh)
	}()

	err := withActiveTerminal(p, func() error {
		_, runErr := p.Run()
		return runErr
	})
	if err != nil {
		return fmt.Errorf(localizer.T(i18n.RootErrRunningUI), err)
	}

	return workErr
}

func formatCommandError(err error) string {
	return commandruntime.FormatCommandError(err, ShouldOutputDebug())
}

func summarizeCommandError(err error) string {
	return commandruntime.SummarizeCommandError(err)
}

func shouldEmitCommandLevel(level commandruntime.OutputLevel) bool {
	switch level {
	case commandruntime.OutputLevelInfo:
		return !ShouldSuppressOutput()
	case commandruntime.OutputLevelVerbose:
		return ShouldOutputVerbose()
	case commandruntime.OutputLevelDebug:
		return ShouldOutputDebug()
	default:
		return false
	}
}

// runWithSharedShell executes work with the shared status emitter.
// New user-facing commands should use this helper by default to keep rendering
// and progress transitions consistent across command flows.
func runWithSharedShell(work func(out commandruntime.LocalizedEmitter) error) error {
	return runWithSpinner(work)
}
