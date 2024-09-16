package io

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// Editor defines an interface for editing text content.
// This abstraction allows for easy testing by swapping the real
// editor implementation with a mock.
type Editor interface {
	// Edit opens the editor with the given content and returns
	// the edited content when the editor is closed.
	// Returns an error if the editor cannot be opened or the content
	// cannot be read.
	Edit(content string) (string, error)
}

// DefaultEditor implements Editor using the system's default editor
// as defined by the $EDITOR environment variable.
type DefaultEditor struct {
	// EditorEnvVar is the name of the environment variable to check
	// for the editor command. Defaults to "EDITOR".
	EditorEnvVar string
	// FallbackEditor is the editor to use if the environment variable
	// is not set. Defaults to "vi".
	FallbackEditor string
}

// NewDefaultEditor creates a new DefaultEditor instance with sensible defaults.
func NewDefaultEditor() *DefaultEditor {
	return &DefaultEditor{
		EditorEnvVar:   "EDITOR",
		FallbackEditor: "vi",
	}
}

// getEditorCommand returns the editor command from the environment
// or the fallback editor if not set.
func (e *DefaultEditor) getEditorCommand() string {
	envVar := e.EditorEnvVar
	if envVar == "" {
		envVar = "EDITOR"
	}
	editor := os.Getenv(envVar)
	if editor == "" {
		if e.FallbackEditor != "" {
			return e.FallbackEditor
		}
		return "vi"
	}
	return editor
}

// Edit opens the configured editor with the given content in a temporary file.
// When the editor is closed, the modified content is read and returned.
func (e *DefaultEditor) Edit(content string) (string, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "fotingo-edit-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	// Write the initial content
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("failed to write to temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Open the editor
	editorCmd := e.getEditorCommand()
	cmd := exec.Command(editorCmd, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor command failed: %w", err)
	}

	// Read the edited content
	editedContent, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read edited content: %w", err)
	}

	return string(editedContent), nil
}

// MockEditor implements Editor for testing purposes.
// It can be configured to return predefined content or errors.
type MockEditor struct {
	mu sync.Mutex
	// ReturnContent is the content to return from Edit calls.
	ReturnContent string
	// ReturnError is the error to return from Edit calls (if any).
	ReturnError error
	// ReceivedContent records the content passed to the last Edit call.
	ReceivedContent string
	// EditCalls counts how many times Edit was called.
	EditCalls int
	// EditHistory records all content passed to Edit calls.
	EditHistory []string
}

// NewMockEditor creates a new MockEditor instance.
func NewMockEditor() *MockEditor {
	return &MockEditor{
		EditHistory: make([]string, 0),
	}
}

// Edit records the input content and returns the configured content/error.
func (e *MockEditor) Edit(content string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ReceivedContent = content
	e.EditCalls++
	e.EditHistory = append(e.EditHistory, content)

	if e.ReturnError != nil {
		return "", e.ReturnError
	}

	// If no return content is configured, return the input unchanged
	if e.ReturnContent == "" {
		return content, nil
	}

	return e.ReturnContent, nil
}

// SetReturnContent configures the mock to return the specified content.
func (e *MockEditor) SetReturnContent(content string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ReturnContent = content
}

// SetError configures the mock to return the specified error on Edit calls.
func (e *MockEditor) SetError(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ReturnError = err
}

// GetReceivedContent returns the content from the last Edit call.
func (e *MockEditor) GetReceivedContent() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ReceivedContent
}

// GetEditCalls returns the number of times Edit was called.
func (e *MockEditor) GetEditCalls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.EditCalls
}

// GetEditHistory returns a copy of all content passed to Edit calls.
func (e *MockEditor) GetEditHistory() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	history := make([]string, len(e.EditHistory))
	copy(history, e.EditHistory)
	return history
}

// Reset clears all recorded state and resets configuration.
func (e *MockEditor) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ReturnContent = ""
	e.ReturnError = nil
	e.ReceivedContent = ""
	e.EditCalls = 0
	e.EditHistory = make([]string, 0)
}
