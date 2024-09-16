package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNewConfirm(t *testing.T) {
	t.Parallel()

	t.Run("creates confirm with defaults", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		assert.Equal(t, "Are you sure?", c.prompt)
		assert.False(t, c.selected) // Default to No
		assert.False(t, c.defaultYes)
	})

	t.Run("with prompt option", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm(WithConfirmPrompt("Delete file?"))
		assert.Equal(t, "Delete file?", c.prompt)
	})

	t.Run("with default yes option", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm(WithDefaultYes())
		assert.True(t, c.selected)
		assert.True(t, c.defaultYes)
	})

	t.Run("with default no option", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm(WithDefaultNo())
		assert.False(t, c.selected)
		assert.False(t, c.defaultYes)
	})

	t.Run("with show buttons option", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm(WithShowButtons())
		assert.True(t, c.showButtons)
	})

	t.Run("with custom styles", func(t *testing.T) {
		t.Parallel()
		styles := NewStyles(LightScheme())
		c := NewConfirm(WithConfirmStyles(styles))
		assert.NotNil(t, c.styles.InputPrompt)
	})
}

func TestConfirmUpdate(t *testing.T) {
	t.Parallel()

	t.Run("handles y key", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		updated, cmd := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
		assert.True(t, updated.confirmed)
		assert.True(t, updated.selected)
		assert.NotNil(t, cmd)

		msg := cmd()
		result, ok := msg.(ConfirmResultMsg)
		assert.True(t, ok)
		assert.True(t, result.Confirmed)
	})

	t.Run("handles Y key", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		updated, cmd := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
		assert.True(t, updated.confirmed)
		assert.True(t, updated.selected)
		assert.NotNil(t, cmd)
	})

	t.Run("handles n key", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		updated, cmd := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		assert.True(t, updated.confirmed)
		assert.False(t, updated.selected)
		assert.NotNil(t, cmd)

		msg := cmd()
		result, ok := msg.(ConfirmResultMsg)
		assert.True(t, ok)
		assert.False(t, result.Confirmed)
	})

	t.Run("handles N key", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		updated, cmd := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
		assert.True(t, updated.confirmed)
		assert.False(t, updated.selected)
		assert.NotNil(t, cmd)
	})

	t.Run("handles left arrow for selection", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		c.selected = false
		// Simulate left key
		updated, cmd := c.Update(tea.KeyMsg{Type: tea.KeyLeft})
		assert.True(t, updated.selected)
		assert.Nil(t, cmd)
	})

	t.Run("handles right arrow for selection", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		c.selected = true
		updated, cmd := c.Update(tea.KeyMsg{Type: tea.KeyRight})
		assert.False(t, updated.selected)
		assert.Nil(t, cmd)
	})

	t.Run("handles tab to toggle", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		c.selected = true
		updated, cmd := c.Update(tea.KeyMsg{Type: tea.KeyTab})
		assert.False(t, updated.selected)
		assert.Nil(t, cmd)

		updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyTab})
		assert.True(t, updated.selected)
	})

	t.Run("handles enter to confirm selection", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		c.selected = true
		updated, cmd := c.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.True(t, updated.confirmed)
		assert.NotNil(t, cmd)

		msg := cmd()
		result, ok := msg.(ConfirmResultMsg)
		assert.True(t, ok)
		assert.True(t, result.Confirmed)
	})

	t.Run("handles escape to cancel", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		updated, cmd := c.Update(tea.KeyMsg{Type: tea.KeyEscape})
		assert.True(t, updated.cancelled)
		assert.NotNil(t, cmd)

		msg := cmd()
		result, ok := msg.(ConfirmResultMsg)
		assert.True(t, ok)
		assert.True(t, result.Cancelled)
	})

	t.Run("handles ctrl+c to cancel", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		updated, cmd := c.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		assert.True(t, updated.cancelled)
		assert.NotNil(t, cmd)
	})

	t.Run("handles h for left", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		c.selected = false
		updated, _ := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
		assert.True(t, updated.selected)
	})

	t.Run("handles l for right", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		c.selected = true
		updated, _ := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
		assert.False(t, updated.selected)
	})
}

func TestConfirmView(t *testing.T) {
	t.Parallel()

	t.Run("renders inline style with default no", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm(WithConfirmPrompt("Continue?"))
		view := c.View()
		assert.Contains(t, view, "Continue?")
	})

	t.Run("renders inline style with default yes", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm(WithDefaultYes())
		view := c.View()
		assert.NotEmpty(t, view)
	})

	t.Run("renders button style", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm(WithShowButtons())
		view := c.View()
		assert.Contains(t, view, "Yes")
		assert.Contains(t, view, "No")
	})

	t.Run("renders with yes selected in buttons", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm(WithShowButtons(), WithDefaultYes())
		view := c.View()
		assert.Contains(t, view, "Yes")
	})

	t.Run("renders with no selected in buttons", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm(WithShowButtons(), WithDefaultNo())
		view := c.View()
		assert.Contains(t, view, "No")
	})
}

func TestConfirmMethods(t *testing.T) {
	t.Parallel()

	t.Run("Confirmed returns true when yes selected", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		c.confirmed = true
		c.selected = true
		assert.True(t, c.Confirmed())
	})

	t.Run("Confirmed returns false when no selected", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		c.confirmed = true
		c.selected = false
		assert.False(t, c.Confirmed())
	})

	t.Run("Cancelled returns correct value", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		assert.False(t, c.Cancelled())
		c.cancelled = true
		assert.True(t, c.Cancelled())
	})

	t.Run("Selected returns correct value", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm()
		assert.False(t, c.Selected())
		c.selected = true
		assert.True(t, c.Selected())
	})
}

func TestConfirmInit(t *testing.T) {
	t.Parallel()

	c := NewConfirm()
	cmd := c.Init()
	assert.Nil(t, cmd) // Confirm doesn't need to initialize anything
}

func TestConfirmWrapper(t *testing.T) {
	t.Parallel()

	t.Run("wrapper Init calls model Init", func(t *testing.T) {
		t.Parallel()
		m := NewConfirm()
		w := &confirmWrapper{model: m}
		cmd := w.Init()
		assert.Nil(t, cmd)
	})

	t.Run("wrapper Update quits on ConfirmResultMsg", func(t *testing.T) {
		t.Parallel()
		m := NewConfirm()
		w := &confirmWrapper{model: m}
		_, cmd := w.Update(ConfirmResultMsg{Confirmed: true})
		assert.NotNil(t, cmd)
	})

	t.Run("wrapper Update propagates to model", func(t *testing.T) {
		t.Parallel()
		m := NewConfirm()
		w := &confirmWrapper{model: m}
		_, _ = w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
		assert.True(t, w.model.confirmed)
	})

	t.Run("wrapper View renders model", func(t *testing.T) {
		t.Parallel()
		m := NewConfirm(WithConfirmPrompt("Test?"))
		w := &confirmWrapper{model: m}
		view := w.View()
		assert.Contains(t, view, "Test?")
	})
}

func TestConfirmViewVariations(t *testing.T) {
	t.Parallel()

	t.Run("inline default no with selected=false", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm(WithDefaultNo())
		c.selected = false
		view := c.View()
		assert.NotEmpty(t, view)
	})

	t.Run("inline default no with selected=true", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm(WithDefaultNo())
		c.selected = true
		view := c.View()
		assert.NotEmpty(t, view)
	})

	t.Run("inline default yes with selected=false", func(t *testing.T) {
		t.Parallel()
		c := NewConfirm(WithDefaultYes())
		c.selected = false
		view := c.View()
		assert.NotEmpty(t, view)
	})
}
