package commands

import (
	"fmt"
	"os"
	"strings"

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
	if ShouldSuppressOutput() || !isInputTerminalFn() || !commandruntime.IsInputTerminal() {
		return runWithoutSpinner(work)
	}

	// Buffer startup/status events so producers don't block during program startup.
	statusCh := make(chan string, 32)

	model := commandruntime.NewStatusModel(commandruntime.StatusModelOptions{
		SuppressOutput: ShouldSuppressOutput,
	})
	// Spinner views don't need interactive input. Using an inert reader avoids
	// Bubble Tea cancel-reader failures in non-interactive CI environments.
	p := tea.NewProgram(model, tea.WithInput(strings.NewReader("")))
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)
	workDone := make(chan error, 1)
	forwardDone := make(chan struct{})

	go func() {
		defer close(forwardDone)
		for msg := range statusCh {
			p.Send(commandruntime.UpdateStatus(msg))
		}
		p.Send(commandruntime.FinishProcess())
	}()

	emitPendingStartupAnnouncements(out)

	go func() {
		workErr := work(out)
		if workErr != nil {
			out.InfoRaw(commandruntime.LogEmojiError, formatCommandError(workErr))
		}
		close(statusCh)
		workDone <- workErr
	}()

	err := withActiveTerminal(p, func() error {
		_, runErr := p.Run()
		return runErr
	})
	workErr := <-workDone
	<-forwardDone
	if err != nil {
		if workErr != nil {
			return workErr
		}
		return fmt.Errorf(localizer.T(i18n.RootErrRunningUI), err)
	}

	return workErr
}

func runWithoutSpinner(work func(out commandruntime.LocalizedEmitter) error) error {
	statusCh := make(chan string, 32)
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)
	drainDone := make(chan struct{})

	go func() {
		defer close(drainDone)
		for raw := range statusCh {
			printStatusMessage(raw)
		}
	}()

	emitPendingStartupAnnouncements(out)
	workErr := work(out)
	if workErr != nil {
		out.InfoRaw(commandruntime.LogEmojiError, formatCommandError(workErr))
	}
	close(statusCh)
	<-drainDone

	return workErr
}

func printStatusMessage(raw string) {
	if event, ok := commandruntime.DecodeStatusEvent(raw); ok {
		if strings.TrimSpace(event.Message) == "" {
			return
		}
		_, _ = fmt.Fprintln(os.Stdout, event.Message)
		return
	}

	if strings.TrimSpace(raw) == "" {
		return
	}
	_, _ = fmt.Fprintln(os.Stdout, raw)
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
