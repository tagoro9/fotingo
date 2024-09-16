package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNewInput(t *testing.T) {
	t.Parallel()

	t.Run("creates input with defaults", func(t *testing.T) {
		t.Parallel()
		i := NewInput()
		assert.False(t, i.Submitted())
		assert.False(t, i.Cancelled())
		assert.Empty(t, i.Value())
	})

	t.Run("with prompt option", func(t *testing.T) {
		t.Parallel()
		i := NewInput(WithPrompt("Name:"))
		assert.NotEmpty(t, i.input.Prompt)
	})

	t.Run("with placeholder option", func(t *testing.T) {
		t.Parallel()
		i := NewInput(WithPlaceholder("Enter value"))
		assert.Equal(t, "Enter value", i.input.Placeholder)
	})

	t.Run("with char limit option", func(t *testing.T) {
		t.Parallel()
		i := NewInput(WithCharLimit(50))
		assert.Equal(t, 50, i.input.CharLimit)
	})

	t.Run("with width option", func(t *testing.T) {
		t.Parallel()
		i := NewInput(WithWidth(40))
		assert.Equal(t, 40, i.input.Width)
	})

	t.Run("with masked option", func(t *testing.T) {
		t.Parallel()
		i := NewInput(WithMasked())
		assert.True(t, i.masked)
	})

	t.Run("with initial value", func(t *testing.T) {
		t.Parallel()
		i := NewInput(WithInitialValue("preset"))
		assert.Equal(t, "preset", i.Value())
	})

	t.Run("with validation", func(t *testing.T) {
		t.Parallel()
		validate := func(v string) error {
			if len(v) < 3 {
				return errors.New("too short")
			}
			return nil
		}
		i := NewInput(WithValidation(validate))
		assert.True(t, i.validating)
	})

	t.Run("with custom styles", func(t *testing.T) {
		t.Parallel()
		styles := NewStyles(LightScheme())
		i := NewInput(WithInputStyles(styles))
		assert.NotNil(t, i.styles.InputPrompt)
	})

	t.Run("with multiline option", func(t *testing.T) {
		t.Parallel()
		i := NewInput(WithPrompt("Description"), WithMultiline(5))
		assert.True(t, i.multiline)
		assert.Contains(t, i.View(), "Ctrl+D")
	})

	t.Run("multiline prompt renders once", func(t *testing.T) {
		t.Parallel()
		i := NewInput(WithPrompt("Issue description (optional)"), WithMultiline(5))
		view := i.View()
		assert.Equal(t, 1, strings.Count(view, "Issue description (optional)"))
	})
}

func TestInputUpdate(t *testing.T) {
	t.Parallel()

	t.Run("handles enter key submission", func(t *testing.T) {
		t.Parallel()
		i := NewInput()
		i.SetValue("test value")

		updated, cmd := i.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.True(t, updated.Submitted())
		assert.NotNil(t, cmd)

		// Execute the command to get the message
		msg := cmd()
		submitMsg, ok := msg.(InputSubmitMsg)
		assert.True(t, ok)
		assert.Equal(t, "test value", submitMsg.Value)
	})

	t.Run("handles escape key cancellation", func(t *testing.T) {
		t.Parallel()
		i := NewInput()

		updated, cmd := i.Update(tea.KeyMsg{Type: tea.KeyEscape})
		assert.True(t, updated.Cancelled())
		assert.NotNil(t, cmd)

		msg := cmd()
		_, ok := msg.(InputCancelMsg)
		assert.True(t, ok)
	})

	t.Run("handles ctrl+c cancellation", func(t *testing.T) {
		t.Parallel()
		i := NewInput()

		updated, cmd := i.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		assert.True(t, updated.Cancelled())
		assert.NotNil(t, cmd)
	})

	t.Run("validates on submit", func(t *testing.T) {
		t.Parallel()
		validate := func(v string) error {
			if len(v) < 3 {
				return errors.New("too short")
			}
			return nil
		}
		i := NewInput(WithValidation(validate))
		i.SetValue("ab")

		updated, cmd := i.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.False(t, updated.Submitted())
		assert.Nil(t, cmd)
		assert.NotNil(t, updated.Error())
	})

	t.Run("clears error on valid submit", func(t *testing.T) {
		t.Parallel()
		validate := func(v string) error {
			if len(v) < 3 {
				return errors.New("too short")
			}
			return nil
		}
		i := NewInput(WithValidation(validate))
		i.SetValue("valid")

		updated, _ := i.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.True(t, updated.Submitted())
		assert.Nil(t, updated.Error())
	})

	t.Run("multiline submit uses ctrl+d", func(t *testing.T) {
		t.Parallel()
		i := NewInput(WithMultiline(4))
		i.SetValue("line one\nline two")

		updated, cmd := i.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
		assert.True(t, updated.Submitted())
		assert.NotNil(t, cmd)

		msg := cmd()
		submitMsg, ok := msg.(InputSubmitMsg)
		assert.True(t, ok)
		assert.Equal(t, "line one\nline two", submitMsg.Value)
	})

	t.Run("multiline enter inserts newline without submitting", func(t *testing.T) {
		t.Parallel()
		i := NewInput(WithMultiline(4))

		updated, _ := i.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.False(t, updated.Submitted())
	})
}

func TestInputView(t *testing.T) {
	t.Parallel()

	t.Run("renders input", func(t *testing.T) {
		t.Parallel()
		i := NewInput(WithPrompt("Name:"))
		view := i.View()
		assert.NotEmpty(t, view)
	})

	t.Run("renders validation error", func(t *testing.T) {
		t.Parallel()
		i := NewInput()
		i.err = errors.New("validation failed")
		view := i.View()
		assert.Contains(t, view, "validation failed")
	})
}

func TestInputMethods(t *testing.T) {
	t.Parallel()

	t.Run("Value and SetValue", func(t *testing.T) {
		t.Parallel()
		i := NewInput()
		i.SetValue("new value")
		assert.Equal(t, "new value", i.Value())
	})

	t.Run("Focus and Blur", func(t *testing.T) {
		t.Parallel()
		i := NewInput()
		i.Blur()
		// Should not panic
		cmd := i.Focus()
		assert.NotNil(t, cmd)
	})

	t.Run("Reset", func(t *testing.T) {
		t.Parallel()
		i := NewInput()
		i.SetValue("test")
		i.submitted = true
		i.err = errors.New("error")

		i.Reset()
		assert.Empty(t, i.Value())
		assert.False(t, i.Submitted())
		assert.Nil(t, i.Error())
	})
}

func TestInputInit(t *testing.T) {
	t.Parallel()

	i := NewInput()
	cmd := i.Init()
	assert.NotNil(t, cmd) // Should return textinput.Blink
}

func TestInputLiveValidation(t *testing.T) {
	t.Parallel()

	t.Run("live validation on typing", func(t *testing.T) {
		t.Parallel()
		validate := func(v string) error {
			if len(v) < 3 {
				return errors.New("too short")
			}
			return nil
		}
		i := NewInput(WithValidation(validate))
		i.SetValue("a")

		// Simulate typing 'b' — value becomes "ab" which is still < 3
		updated, _ := i.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
		// Should have validation error for "ab" (2 chars < 3)
		assert.NotNil(t, updated.Error())
	})

	t.Run("clears error on typing", func(t *testing.T) {
		t.Parallel()
		i := NewInput()
		i.err = errors.New("old error")
		updated, _ := i.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		// Error should be cleared on input
		assert.Nil(t, updated.err)
	})
}

func TestInputWrapper(t *testing.T) {
	t.Parallel()

	t.Run("wrapper Init calls model Init", func(t *testing.T) {
		t.Parallel()
		m := NewInput()
		w := &inputWrapper{model: m}
		cmd := w.Init()
		assert.NotNil(t, cmd)
	})

	t.Run("wrapper Update quits on InputSubmitMsg", func(t *testing.T) {
		t.Parallel()
		m := NewInput()
		w := &inputWrapper{model: m}
		_, cmd := w.Update(InputSubmitMsg{Value: "test"})
		assert.NotNil(t, cmd)
	})

	t.Run("wrapper Update quits on InputCancelMsg", func(t *testing.T) {
		t.Parallel()
		m := NewInput()
		w := &inputWrapper{model: m}
		_, cmd := w.Update(InputCancelMsg{})
		assert.NotNil(t, cmd)
	})

	t.Run("wrapper Update propagates to model", func(t *testing.T) {
		t.Parallel()
		m := NewInput()
		w := &inputWrapper{model: m}
		_, _ = w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		// Should not panic
	})

	t.Run("wrapper View renders model", func(t *testing.T) {
		t.Parallel()
		m := NewInput(WithPrompt("Test:"))
		w := &inputWrapper{model: m}
		view := w.View()
		assert.NotEmpty(t, view)
	})
}
