package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SpinnerStep represents a single step in a multi-step progress display.
type SpinnerStep struct {
	Message   string
	Completed bool
	Error     error
}

// SpinnerModel represents a progress spinner with optional multi-step display.
type SpinnerModel struct {
	spinner spinner.Model
	styles  Styles
	message string
	steps   []SpinnerStep
	done    bool
	err     error
}

// SpinnerOption configures a SpinnerModel.
type SpinnerOption func(*SpinnerModel)

// WithSpinnerStyle sets the spinner animation style.
func WithSpinnerStyle(s spinner.Spinner) SpinnerOption {
	return func(m *SpinnerModel) {
		m.spinner.Spinner = s
	}
}

// WithMessage sets the initial message.
func WithMessage(msg string) SpinnerOption {
	return func(m *SpinnerModel) {
		m.message = msg
	}
}

// WithStyles sets custom styles for the spinner.
func WithStyles(styles Styles) SpinnerOption {
	return func(m *SpinnerModel) {
		m.styles = styles
	}
}

// NewSpinner creates a new SpinnerModel.
func NewSpinner(opts ...SpinnerOption) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot

	styles := DefaultStyles()
	s.Style = styles.Spinner

	m := SpinnerModel{
		spinner: s,
		styles:  styles,
		steps:   []SpinnerStep{},
	}

	for _, opt := range opts {
		opt(&m)
	}

	// Apply spinner style after options
	m.spinner.Style = m.styles.Spinner

	return m
}

// Init initializes the spinner model.
func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// SpinnerUpdateMsg updates the spinner message.
type SpinnerUpdateMsg string

// SpinnerStepMsg adds a new step to the spinner.
type SpinnerStepMsg struct {
	Message string
}

// SpinnerCompleteStepMsg marks the current step as completed.
type SpinnerCompleteStepMsg struct {
	Error error
}

// SpinnerDoneMsg signals that the spinner should stop.
type SpinnerDoneMsg struct {
	Error error
}

// Update handles messages for the spinner model.
func (m SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case SpinnerUpdateMsg:
		m.message = string(msg)
		return m, nil

	case SpinnerStepMsg:
		// Mark previous step as completed if any
		if len(m.steps) > 0 && !m.steps[len(m.steps)-1].Completed {
			m.steps[len(m.steps)-1].Completed = true
		}
		m.steps = append(m.steps, SpinnerStep{Message: msg.Message})
		return m, nil

	case SpinnerCompleteStepMsg:
		if len(m.steps) > 0 {
			m.steps[len(m.steps)-1].Completed = true
			m.steps[len(m.steps)-1].Error = msg.Error
		}
		return m, nil

	case SpinnerDoneMsg:
		m.done = true
		m.err = msg.Error
		// Mark last step as completed
		if len(m.steps) > 0 && !m.steps[len(m.steps)-1].Completed {
			m.steps[len(m.steps)-1].Completed = true
			m.steps[len(m.steps)-1].Error = msg.Error
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the spinner.
func (m SpinnerModel) View() string {
	var sb strings.Builder

	if len(m.steps) > 0 {
		// Multi-step progress display
		for i, step := range m.steps {
			isLast := i == len(m.steps)-1

			if step.Completed {
				if step.Error != nil {
					sb.WriteString(m.styles.Error.Render(Icons.Cross))
				} else {
					sb.WriteString(m.styles.Success.Render(Icons.Check))
				}
				sb.WriteString(" ")
				if step.Error != nil {
					sb.WriteString(m.styles.Error.Render(step.Message))
				} else {
					sb.WriteString(m.styles.Muted.Render(step.Message))
				}
			} else if isLast && !m.done {
				sb.WriteString(m.spinner.View())
				sb.WriteString(" ")
				sb.WriteString(m.styles.Normal.Render(step.Message))
			} else {
				sb.WriteString("  ")
				sb.WriteString(m.styles.Normal.Render(step.Message))
			}
			sb.WriteString("\n")
		}
	} else if m.message != "" {
		// Simple spinner with message
		if m.done {
			if m.err != nil {
				sb.WriteString(m.styles.Error.Render(Icons.Cross))
				sb.WriteString(" ")
				sb.WriteString(m.styles.Error.Render(m.message))
			} else {
				sb.WriteString(m.styles.Success.Render(Icons.Check))
				sb.WriteString(" ")
				sb.WriteString(m.styles.Muted.Render(m.message))
			}
		} else {
			sb.WriteString(m.spinner.View())
			sb.WriteString(" ")
			sb.WriteString(m.styles.Normal.Render(m.message))
		}
		sb.WriteString("\n")
	} else {
		// Just the spinner
		if !m.done {
			sb.WriteString(m.spinner.View())
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// Done returns whether the spinner has completed.
func (m SpinnerModel) Done() bool {
	return m.done
}

// Error returns any error that occurred.
func (m SpinnerModel) Error() error {
	return m.err
}

// SetMessage sets the current message.
func (m *SpinnerModel) SetMessage(msg string) {
	m.message = msg
}

// AddStep adds a new step to the multi-step progress display.
func (m *SpinnerModel) AddStep(msg string) {
	// Mark previous step as completed
	if len(m.steps) > 0 && !m.steps[len(m.steps)-1].Completed {
		m.steps[len(m.steps)-1].Completed = true
	}
	m.steps = append(m.steps, SpinnerStep{Message: msg})
}

// CompleteStep marks the current step as completed.
func (m *SpinnerModel) CompleteStep(err error) {
	if len(m.steps) > 0 {
		m.steps[len(m.steps)-1].Completed = true
		m.steps[len(m.steps)-1].Error = err
	}
}

// Finish marks the spinner as done.
func (m *SpinnerModel) Finish(err error) {
	m.done = true
	m.err = err
	if len(m.steps) > 0 && !m.steps[len(m.steps)-1].Completed {
		m.steps[len(m.steps)-1].Completed = true
		m.steps[len(m.steps)-1].Error = err
	}
}

// SpinnerProgram wraps a SpinnerModel in a tea.Program for standalone use.
type SpinnerProgram struct {
	program *tea.Program
	model   SpinnerModel
}

// NewSpinnerProgram creates a new spinner program for standalone operation.
func NewSpinnerProgram(opts ...SpinnerOption) *SpinnerProgram {
	m := NewSpinner(opts...)
	p := tea.NewProgram(spinnerWrapper{model: m})
	return &SpinnerProgram{
		program: p,
		model:   m,
	}
}

// spinnerWrapper wraps SpinnerModel to implement tea.Model.
type spinnerWrapper struct {
	model SpinnerModel
}

func (w spinnerWrapper) Init() tea.Cmd {
	return w.model.Init()
}

func (w spinnerWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return w, tea.Quit
		}
	case SpinnerDoneMsg:
		var cmd tea.Cmd
		w.model, cmd = w.model.Update(msg)
		return w, tea.Batch(cmd, tea.Quit)
	}

	var cmd tea.Cmd
	w.model, cmd = w.model.Update(msg)
	return w, cmd
}

func (w spinnerWrapper) View() string {
	return w.model.View()
}

// Start starts the spinner program in the background.
func (sp *SpinnerProgram) Start() error {
	go func() {
		_, _ = sp.program.Run()
	}()
	return nil
}

// UpdateMessage sends a message update to the spinner.
func (sp *SpinnerProgram) UpdateMessage(msg string) {
	sp.program.Send(SpinnerUpdateMsg(msg))
}

// AddStep adds a new step to the spinner.
func (sp *SpinnerProgram) AddStep(msg string) {
	sp.program.Send(SpinnerStepMsg{Message: msg})
}

// CompleteStep completes the current step.
func (sp *SpinnerProgram) CompleteStep(err error) {
	sp.program.Send(SpinnerCompleteStepMsg{Error: err})
}

// Done signals completion and stops the spinner.
func (sp *SpinnerProgram) Done(err error) {
	sp.program.Send(SpinnerDoneMsg{Error: err})
}

// Wait blocks until the program exits.
func (sp *SpinnerProgram) Wait() error {
	sp.program.Wait()
	return sp.model.Error()
}

// RenderStaticSpinner renders a non-animated spinner view for non-TTY output.
func RenderStaticSpinner(message string, completed bool, err error) string {
	styles := DefaultStyles()

	var sb strings.Builder

	if completed {
		if err != nil {
			sb.WriteString(styles.Error.Render(Icons.Cross))
			sb.WriteString(" ")
			sb.WriteString(styles.Error.Render(message))
		} else {
			sb.WriteString(styles.Success.Render(Icons.Check))
			sb.WriteString(" ")
			sb.WriteString(message)
		}
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(styles.scheme.Primary).Render(Icons.Spinner))
		sb.WriteString(" ")
		sb.WriteString(message)
	}

	return sb.String()
}
