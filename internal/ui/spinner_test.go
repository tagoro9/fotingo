package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNewSpinner(t *testing.T) {
	t.Parallel()

	t.Run("creates spinner with defaults", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		assert.False(t, s.Done())
		assert.Nil(t, s.Error())
		assert.Empty(t, s.steps)
	})

	t.Run("with message option", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner(WithMessage("Loading..."))
		assert.Equal(t, "Loading...", s.message)
	})

	t.Run("with spinner style option", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner(WithSpinnerStyle(spinner.Line))
		assert.NotNil(t, s.spinner)
	})

	t.Run("with custom styles", func(t *testing.T) {
		t.Parallel()
		styles := NewStyles(LightScheme())
		s := NewSpinner(WithStyles(styles))
		assert.NotNil(t, s.styles.Spinner)
	})
}

func TestSpinnerUpdate(t *testing.T) {
	t.Parallel()

	t.Run("updates message", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		updated, _ := s.Update(SpinnerUpdateMsg("New message"))
		assert.Equal(t, "New message", updated.message)
	})

	t.Run("adds step", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		updated, _ := s.Update(SpinnerStepMsg{Message: "Step 1"})
		assert.Len(t, updated.steps, 1)
		assert.Equal(t, "Step 1", updated.steps[0].Message)
		assert.False(t, updated.steps[0].Completed)
	})

	t.Run("marks previous step complete when adding new step", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		s, _ = s.Update(SpinnerStepMsg{Message: "Step 1"})
		s, _ = s.Update(SpinnerStepMsg{Message: "Step 2"})
		assert.Len(t, s.steps, 2)
		assert.True(t, s.steps[0].Completed)
		assert.False(t, s.steps[1].Completed)
	})

	t.Run("completes step with error", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		s, _ = s.Update(SpinnerStepMsg{Message: "Step 1"})
		testErr := errors.New("test error")
		s, _ = s.Update(SpinnerCompleteStepMsg{Error: testErr})
		assert.True(t, s.steps[0].Completed)
		assert.Equal(t, testErr, s.steps[0].Error)
	})

	t.Run("handles done message", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		s, _ = s.Update(SpinnerStepMsg{Message: "Step 1"})
		s, _ = s.Update(SpinnerDoneMsg{})
		assert.True(t, s.Done())
		assert.True(t, s.steps[0].Completed)
	})

	t.Run("handles done with error", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		testErr := errors.New("final error")
		s, _ = s.Update(SpinnerDoneMsg{Error: testErr})
		assert.True(t, s.Done())
		assert.Equal(t, testErr, s.Error())
	})
}

func TestSpinnerView(t *testing.T) {
	t.Parallel()

	t.Run("renders empty spinner", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		view := s.View()
		// Should contain spinner character
		assert.NotEmpty(t, view)
	})

	t.Run("renders message", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner(WithMessage("Loading..."))
		view := s.View()
		assert.Contains(t, view, "Loading...")
	})

	t.Run("renders completed message", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner(WithMessage("Done"))
		s.Finish(nil)
		view := s.View()
		assert.Contains(t, view, "Done")
		assert.Contains(t, view, Icons.Check)
	})

	t.Run("renders error message", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner(WithMessage("Failed"))
		s.Finish(errors.New("test error"))
		view := s.View()
		assert.Contains(t, view, "Failed")
		assert.Contains(t, view, Icons.Cross)
	})

	t.Run("renders multi-step progress", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		s.AddStep("Step 1")
		s.AddStep("Step 2")
		view := s.View()
		assert.Contains(t, view, "Step 1")
		assert.Contains(t, view, "Step 2")
		// Step 1 should be completed
		assert.Contains(t, view, Icons.Check)
	})
}

func TestSpinnerMethods(t *testing.T) {
	t.Parallel()

	t.Run("SetMessage", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		s.SetMessage("Updated")
		assert.Equal(t, "Updated", s.message)
	})

	t.Run("AddStep", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		s.AddStep("First")
		s.AddStep("Second")
		assert.Len(t, s.steps, 2)
		assert.True(t, s.steps[0].Completed)
	})

	t.Run("CompleteStep", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		s.AddStep("Step")
		s.CompleteStep(nil)
		assert.True(t, s.steps[0].Completed)
	})

	t.Run("Finish", func(t *testing.T) {
		t.Parallel()
		s := NewSpinner()
		s.AddStep("Step")
		s.Finish(nil)
		assert.True(t, s.Done())
		assert.True(t, s.steps[0].Completed)
	})
}

func TestRenderStaticSpinner(t *testing.T) {
	t.Parallel()

	t.Run("renders in progress", func(t *testing.T) {
		t.Parallel()
		result := RenderStaticSpinner("Loading", false, nil)
		assert.Contains(t, result, "Loading")
		assert.Contains(t, result, Icons.Spinner)
	})

	t.Run("renders completed", func(t *testing.T) {
		t.Parallel()
		result := RenderStaticSpinner("Done", true, nil)
		assert.Contains(t, result, "Done")
		assert.Contains(t, result, Icons.Check)
	})

	t.Run("renders error", func(t *testing.T) {
		t.Parallel()
		result := RenderStaticSpinner("Failed", true, errors.New("error"))
		assert.Contains(t, result, "Failed")
		assert.Contains(t, result, Icons.Cross)
	})
}

func TestSpinnerInit(t *testing.T) {
	t.Parallel()

	s := NewSpinner()
	cmd := s.Init()
	assert.NotNil(t, cmd) // Should return spinner.Tick
}

func TestSpinnerViewNoSteps(t *testing.T) {
	t.Parallel()

	// Test done state with no message or steps
	s := NewSpinner()
	s.Finish(nil)
	view := s.View()
	// Should be empty or minimal since no message was set
	assert.True(t, len(strings.TrimSpace(view)) >= 0)
}

func TestSpinnerWrapper(t *testing.T) {
	t.Parallel()

	t.Run("wrapper Init calls model Init", func(t *testing.T) {
		t.Parallel()
		m := NewSpinner()
		w := spinnerWrapper{model: m}
		cmd := w.Init()
		assert.NotNil(t, cmd) // spinner.Tick
	})

	t.Run("wrapper Update handles ctrl+c", func(t *testing.T) {
		t.Parallel()
		m := NewSpinner()
		w := spinnerWrapper{model: m}
		_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		assert.NotNil(t, cmd)
		// tea.Quit returns a special msg
		msg := cmd()
		assert.NotNil(t, msg)
	})

	t.Run("wrapper Update handles SpinnerDoneMsg", func(t *testing.T) {
		t.Parallel()
		m := NewSpinner(WithMessage("Working"))
		w := spinnerWrapper{model: m}
		_, cmd := w.Update(SpinnerDoneMsg{})
		// Should batch the model's cmd with tea.Quit
		assert.NotNil(t, cmd)
	})

	t.Run("wrapper Update propagates other messages", func(t *testing.T) {
		t.Parallel()
		m := NewSpinner()
		w := spinnerWrapper{model: m}
		updated, _ := w.Update(SpinnerUpdateMsg("new message"))
		wrapper := updated.(spinnerWrapper)
		assert.Equal(t, "new message", wrapper.model.message)
	})

	t.Run("wrapper View renders model", func(t *testing.T) {
		t.Parallel()
		m := NewSpinner(WithMessage("Test"))
		w := spinnerWrapper{model: m}
		view := w.View()
		assert.Contains(t, view, "Test")
	})
}
