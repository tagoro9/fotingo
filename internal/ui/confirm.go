package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/tagoro9/fotingo/internal/i18n"
)

// ConfirmModel represents a y/n confirmation prompt.
type ConfirmModel struct {
	styles      Styles
	prompt      string
	selected    bool // true = Yes, false = No
	confirmed   bool
	cancelled   bool
	defaultYes  bool
	showButtons bool
}

// ConfirmOption configures a ConfirmModel.
type ConfirmOption func(*ConfirmModel)

// WithConfirmPrompt sets the confirmation prompt text.
func WithConfirmPrompt(prompt string) ConfirmOption {
	return func(m *ConfirmModel) {
		m.prompt = prompt
	}
}

// WithDefaultYes sets the default selection to Yes.
func WithDefaultYes() ConfirmOption {
	return func(m *ConfirmModel) {
		m.defaultYes = true
		m.selected = true
	}
}

// WithDefaultNo sets the default selection to No.
func WithDefaultNo() ConfirmOption {
	return func(m *ConfirmModel) {
		m.defaultYes = false
		m.selected = false
	}
}

// WithShowButtons enables button-style display.
func WithShowButtons() ConfirmOption {
	return func(m *ConfirmModel) {
		m.showButtons = true
	}
}

// WithConfirmStyles sets custom styles for the confirmation.
func WithConfirmStyles(styles Styles) ConfirmOption {
	return func(m *ConfirmModel) {
		m.styles = styles
	}
}

// NewConfirm creates a new ConfirmModel.
func NewConfirm(opts ...ConfirmOption) ConfirmModel {
	m := ConfirmModel{
		styles:   DefaultStyles(),
		prompt:   i18n.T(i18n.UIConfirmPrompt),
		selected: false, // Default to No for safety
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

// Init initializes the confirmation model.
func (m ConfirmModel) Init() tea.Cmd {
	return nil
}

// ConfirmResultMsg is sent when the confirmation is resolved.
type ConfirmResultMsg struct {
	Confirmed bool
	Cancelled bool
}

// Update handles messages for the confirmation model.
func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "y", "Y":
			m.confirmed = true
			m.selected = true
			return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: true} }

		case "n", "N":
			m.confirmed = true
			m.selected = false
			return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: false} }

		case "left", "h":
			m.selected = true
			return m, nil

		case "right", "l":
			m.selected = false
			return m, nil

		case "tab":
			m.selected = !m.selected
			return m, nil

		case "enter":
			m.confirmed = true
			return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: m.selected} }

		case "esc", "ctrl+c":
			m.cancelled = true
			return m, func() tea.Msg { return ConfirmResultMsg{Cancelled: true} }
		}
	}

	return m, nil
}

// View renders the confirmation prompt.
func (m ConfirmModel) View() tea.View {
	var sb strings.Builder

	// Render prompt
	sb.WriteString(m.styles.InputPrompt.Render(m.prompt))
	sb.WriteString(" ")

	if m.showButtons {
		// Button style
		if m.selected {
			sb.WriteString(m.styles.ButtonActive.Render(" Yes "))
			sb.WriteString(" ")
			sb.WriteString(m.styles.ButtonInactive.Render(" No "))
		} else {
			sb.WriteString(m.styles.ButtonInactive.Render(" Yes "))
			sb.WriteString(" ")
			sb.WriteString(m.styles.ButtonActive.Render(" No "))
		}
	} else {
		// Inline style
		if m.defaultYes {
			if m.selected {
				sb.WriteString(m.styles.Highlight.Render("[Y]"))
				sb.WriteString("/")
				sb.WriteString(m.styles.Muted.Render("n"))
			} else {
				sb.WriteString(m.styles.Muted.Render("Y"))
				sb.WriteString("/")
				sb.WriteString(m.styles.Highlight.Render("[n]"))
			}
		} else {
			if m.selected {
				sb.WriteString(m.styles.Highlight.Render("[y]"))
				sb.WriteString("/")
				sb.WriteString(m.styles.Muted.Render("N"))
			} else {
				sb.WriteString(m.styles.Muted.Render("y"))
				sb.WriteString("/")
				sb.WriteString(m.styles.Highlight.Render("[N]"))
			}
		}
	}

	sb.WriteString("\n")

	return tea.NewView(sb.String())
}

// Confirmed returns whether Yes was selected.
func (m ConfirmModel) Confirmed() bool {
	return m.confirmed && m.selected
}

// Cancelled returns whether the prompt was cancelled.
func (m ConfirmModel) Cancelled() bool {
	return m.cancelled
}

// Selected returns the current selection (true = Yes, false = No).
func (m ConfirmModel) Selected() bool {
	return m.selected
}

// ConfirmProgram wraps a ConfirmModel in a tea.Program for standalone use.
type ConfirmProgram struct {
	program *tea.Program
	model   *confirmWrapper
}

// confirmWrapper wraps ConfirmModel to implement tea.Model.
type confirmWrapper struct {
	model ConfirmModel
}

func (w *confirmWrapper) Init() tea.Cmd {
	return w.model.Init()
}

func (w *confirmWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case ConfirmResultMsg:
		return w, tea.Quit
	}

	var cmd tea.Cmd
	w.model, cmd = w.model.Update(msg)
	return w, cmd
}

func (w *confirmWrapper) View() tea.View {
	return w.model.View()
}

// NewConfirmProgram creates a new confirmation program for standalone operation.
func NewConfirmProgram(opts ...ConfirmOption) *ConfirmProgram {
	m := NewConfirm(opts...)
	w := &confirmWrapper{model: m}
	p := tea.NewProgram(w)
	return &ConfirmProgram{
		program: p,
		model:   w,
	}
}

// Run runs the confirmation program and returns the result.
func (cp *ConfirmProgram) Run() (confirmed bool, cancelled bool, err error) {
	err = runTeaProgram(cp.program)
	if err != nil {
		return false, false, err
	}

	return cp.model.model.Confirmed(), cp.model.model.Cancelled(), nil
}

// Confirm is a helper function to show a confirmation prompt and return the result.
func Confirm(prompt string, defaultYes bool) (bool, error) {
	opts := []ConfirmOption{WithConfirmPrompt(prompt)}
	if defaultYes {
		opts = append(opts, WithDefaultYes())
	} else {
		opts = append(opts, WithDefaultNo())
	}

	cp := NewConfirmProgram(opts...)
	confirmed, cancelled, err := cp.Run()
	if err != nil {
		return false, err
	}

	if cancelled {
		return false, nil
	}

	return confirmed, nil
}
