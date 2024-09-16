package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ValidationFunc is a function that validates input and returns an error message if invalid.
type ValidationFunc func(value string) error

// InputModel represents a styled text input with validation support.
type InputModel struct {
	input       textinput.Model
	textarea    textarea.Model
	styles      Styles
	prompt      string
	placeholder string
	validate    ValidationFunc
	validating  bool
	err         error
	submitted   bool
	cancelled   bool
	masked      bool
	multiline   bool
}

// InputOption configures an InputModel.
type InputOption func(*InputModel)

// WithPrompt sets the input prompt text.
func WithPrompt(prompt string) InputOption {
	return func(m *InputModel) {
		m.prompt = prompt
		if m.multiline {
			m.setMultilinePrompt()
			return
		}
		m.input.Prompt = m.styles.InputPrompt.Render(prompt + " ")
	}
}

// WithPlaceholder sets the placeholder text.
func WithPlaceholder(placeholder string) InputOption {
	return func(m *InputModel) {
		m.placeholder = placeholder
		if m.multiline {
			m.textarea.Placeholder = placeholder
			return
		}
		m.input.Placeholder = placeholder
	}
}

// WithValidation sets the validation function.
func WithValidation(fn ValidationFunc) InputOption {
	return func(m *InputModel) {
		m.validate = fn
		m.validating = true
	}
}

// WithCharLimit sets the character limit.
func WithCharLimit(limit int) InputOption {
	return func(m *InputModel) {
		if m.multiline {
			m.textarea.CharLimit = limit
			return
		}
		m.input.CharLimit = limit
	}
}

// WithWidth sets the input width.
func WithWidth(width int) InputOption {
	return func(m *InputModel) {
		if m.multiline {
			m.textarea.SetWidth(width)
			return
		}
		m.input.Width = width
	}
}

// WithMasked enables password masking mode.
func WithMasked() InputOption {
	return func(m *InputModel) {
		m.masked = true
		m.input.EchoMode = textinput.EchoPassword
		m.input.EchoCharacter = '*'
	}
}

// WithMultiline enables wrapped multiline input.
func WithMultiline(height int) InputOption {
	return func(m *InputModel) {
		m.multiline = true
		m.textarea = textarea.New()
		m.textarea.Focus()
		m.textarea.ShowLineNumbers = false
		m.textarea.SetHeight(height)
		m.textarea.Placeholder = m.placeholder
		m.textarea.CharLimit = m.input.CharLimit
		m.applyStyles()
		m.setMultilinePrompt()
	}
}

// WithInitialValue sets the initial input value.
func WithInitialValue(value string) InputOption {
	return func(m *InputModel) {
		if m.multiline {
			m.textarea.SetValue(value)
			return
		}
		m.input.SetValue(value)
	}
}

// WithInputStyles sets custom styles for the input.
func WithInputStyles(styles Styles) InputOption {
	return func(m *InputModel) {
		m.styles = styles
		m.applyStyles()
	}
}

// NewInput creates a new InputModel.
func NewInput(opts ...InputOption) InputModel {
	ti := textinput.New()
	ti.Focus()

	styles := DefaultStyles()

	m := InputModel{
		input:  ti,
		styles: styles,
	}

	m.applyStyles()

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

// applyStyles applies the current styles to the input.
func (m *InputModel) applyStyles() {
	if m.multiline {
		m.textarea.FocusedStyle.Prompt = m.styles.InputPrompt
		m.textarea.FocusedStyle.Text = m.styles.InputText
		m.textarea.FocusedStyle.Placeholder = m.styles.InputPlaceholder
		m.textarea.BlurredStyle.Prompt = m.styles.InputPrompt
		m.textarea.BlurredStyle.Text = m.styles.InputText
		m.textarea.BlurredStyle.Placeholder = m.styles.InputPlaceholder
		m.textarea.Cursor.Style = m.styles.Highlight
		m.setMultilinePrompt()
		return
	}
	m.input.PromptStyle = m.styles.InputPrompt
	m.input.TextStyle = m.styles.InputText
	m.input.PlaceholderStyle = m.styles.InputPlaceholder
	m.input.Cursor.Style = m.styles.Highlight
}

func (m *InputModel) setMultilinePrompt() {
	if !m.multiline {
		return
	}

	promptText := strings.TrimSpace(m.prompt)
	if promptText != "" {
		promptText += " "
	}

	promptWidth := len([]rune(promptText))
	continuation := strings.Repeat(" ", promptWidth)
	m.textarea.SetPromptFunc(promptWidth, func(lineIdx int) string {
		if lineIdx == 0 {
			return promptText
		}
		return continuation
	})
}

// Init initializes the input model.
func (m InputModel) Init() tea.Cmd {
	if m.multiline {
		return textarea.Blink
	}
	return textinput.Blink
}

// InputSubmitMsg is sent when the input is submitted.
type InputSubmitMsg struct {
	Value string
}

// InputCancelMsg is sent when the input is cancelled.
type InputCancelMsg struct{}

// Update handles messages for the input model.
func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if m.multiline {
			width := msg.Width - 4
			if width < 20 {
				width = 20
			}
			m.textarea.SetWidth(width)
		}

	case tea.KeyMsg:
		if m.multiline {
			switch msg.Type {
			case tea.KeyCtrlD:
				if m.validating && m.validate != nil {
					if err := m.validate(m.textarea.Value()); err != nil {
						m.err = err
						return m, nil
					}
				}
				m.submitted = true
				m.err = nil
				return m, func() tea.Msg { return InputSubmitMsg{Value: m.textarea.Value()} }
			case tea.KeyEscape, tea.KeyCtrlC:
				m.cancelled = true
				return m, func() tea.Msg { return InputCancelMsg{} }
			default:
				m.err = nil
			}
		} else {
			switch msg.Type {
			case tea.KeyEnter:
				// Validate before submitting
				if m.validating && m.validate != nil {
					if err := m.validate(m.input.Value()); err != nil {
						m.err = err
						return m, nil
					}
				}
				m.submitted = true
				m.err = nil
				return m, func() tea.Msg { return InputSubmitMsg{Value: m.input.Value()} }

			case tea.KeyEscape, tea.KeyCtrlC:
				m.cancelled = true
				return m, func() tea.Msg { return InputCancelMsg{} }

			default:
				// Clear error on any input
				m.err = nil
			}
		}
	}

	if m.multiline {
		m.textarea, cmd = m.textarea.Update(msg)
	} else {
		m.input, cmd = m.input.Update(msg)
	}

	// Live validation if enabled
	value := m.Value()
	if m.validating && m.validate != nil && value != "" {
		m.err = m.validate(value)
	}

	return m, cmd
}

// View renders the input.
func (m InputModel) View() string {
	var sb strings.Builder

	// Render the input
	if m.multiline {
		sb.WriteString(m.textarea.View())
		sb.WriteString("\n")
		sb.WriteString(m.styles.Muted.Render("  Press Ctrl+D to submit"))
	} else {
		sb.WriteString(m.input.View())
	}
	sb.WriteString("\n")

	// Render validation error if present
	if m.err != nil {
		sb.WriteString(m.styles.Error.Render("  " + m.err.Error()))
		sb.WriteString("\n")
	}

	return sb.String()
}

// Value returns the current input value.
func (m InputModel) Value() string {
	if m.multiline {
		return m.textarea.Value()
	}
	return m.input.Value()
}

// SetValue sets the input value.
func (m *InputModel) SetValue(value string) {
	if m.multiline {
		m.textarea.SetValue(value)
		return
	}
	m.input.SetValue(value)
}

// Focus focuses the input.
func (m *InputModel) Focus() tea.Cmd {
	if m.multiline {
		return m.textarea.Focus()
	}
	return m.input.Focus()
}

// Blur removes focus from the input.
func (m *InputModel) Blur() {
	if m.multiline {
		m.textarea.Blur()
		return
	}
	m.input.Blur()
}

// Submitted returns whether the input was submitted.
func (m InputModel) Submitted() bool {
	return m.submitted
}

// Cancelled returns whether the input was cancelled.
func (m InputModel) Cancelled() bool {
	return m.cancelled
}

// Error returns any validation error.
func (m InputModel) Error() error {
	return m.err
}

// Reset resets the input state.
func (m *InputModel) Reset() {
	if m.multiline {
		m.textarea.Reset()
	} else {
		m.input.Reset()
	}
	m.submitted = false
	m.cancelled = false
	m.err = nil
}

// InputProgram wraps an InputModel in a tea.Program for standalone use.
type InputProgram struct {
	program *tea.Program
	model   *inputWrapper
}

// inputWrapper wraps InputModel to implement tea.Model.
type inputWrapper struct {
	model InputModel
}

func (w *inputWrapper) Init() tea.Cmd {
	return w.model.Init()
}

func (w *inputWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case InputSubmitMsg, InputCancelMsg:
		return w, tea.Quit
	}

	var cmd tea.Cmd
	w.model, cmd = w.model.Update(msg)
	return w, cmd
}

func (w *inputWrapper) View() string {
	return w.model.View()
}

// NewInputProgram creates a new input program for standalone operation.
func NewInputProgram(opts ...InputOption) *InputProgram {
	m := NewInput(opts...)
	w := &inputWrapper{model: m}
	p := tea.NewProgram(w)
	return &InputProgram{
		program: p,
		model:   w,
	}
}

// Run runs the input program and returns the entered value.
func (ip *InputProgram) Run() (string, error) {
	err := runTeaProgram(ip.program)
	if err != nil {
		return "", err
	}

	if ip.model.model.Cancelled() {
		return "", nil
	}

	return ip.model.model.Value(), nil
}

// RunWithCancel runs the input program and returns the entered value and cancellation status.
func (ip *InputProgram) RunWithCancel() (value string, cancelled bool, err error) {
	err = runTeaProgram(ip.program)
	if err != nil {
		return "", false, err
	}

	return ip.model.model.Value(), ip.model.model.Cancelled(), nil
}
