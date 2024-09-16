package ui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tagoro9/fotingo/internal/i18n"
)

// ErrNoEditor is returned when no editor is available.
var ErrNoEditor = errors.New(i18n.T(i18n.UIErrNoEditor))

// ErrEditorFailed is returned when the editor exits with an error.
var ErrEditorFailed = errors.New(i18n.T(i18n.UIErrEditorFailed))

// ErrEditorEmpty is returned when the editor content is empty.
var ErrEditorEmpty = errors.New(i18n.T(i18n.UIErrEditorEmpty))

// EditorConfig configures the editor behavior.
type EditorConfig struct {
	// Extension is the file extension for the temp file (default: ".md")
	Extension string

	// InitialContent is the initial content to show in the editor
	InitialContent string

	// Prefix is a prefix for the temp file name
	Prefix string

	// AllowEmpty if true, allows empty content to be returned
	AllowEmpty bool

	// DeleteOnClose if true, deletes the temp file after editing
	DeleteOnClose bool
}

// DefaultEditorConfig returns the default editor configuration.
func DefaultEditorConfig() EditorConfig {
	return EditorConfig{
		Extension:     ".md",
		Prefix:        "fotingo-",
		DeleteOnClose: true,
	}
}

// Editor provides functionality to open an external editor.
type Editor struct {
	config  EditorConfig
	editors []editorCommand
}

type editorCommand struct {
	command string
	args    []string
	display string
}

// NewEditor creates a new Editor instance.
func NewEditor(config EditorConfig) (*Editor, error) {
	editors := findEditors()
	if len(editors) == 0 {
		return nil, ErrNoEditor
	}

	return &Editor{
		config:  config,
		editors: editors,
	}, nil
}

// findEditor finds the editor command to use.
func findEditor() string {
	editors := findEditors()
	if len(editors) == 0 {
		return ""
	}
	return editors[0].display
}

func findEditors() []editorCommand {
	var candidates []editorCommand

	// Check $VISUAL first (preferred for visual editors)
	if editor := os.Getenv("VISUAL"); editor != "" {
		if parsed, ok := parseEditorCommand(editor); ok {
			candidates = append(candidates, parsed)
		}
	}

	// Then check $EDITOR
	if editor := os.Getenv("EDITOR"); editor != "" {
		if parsed, ok := parseEditorCommand(editor); ok {
			candidates = append(candidates, parsed)
		}
	}

	// Fallback to common editors
	fallbacks := []string{"vim", "nano", "vi", "code", "notepad"}
	if runtime.GOOS == "windows" {
		fallbacks = []string{"notepad", "code"}
	}

	for _, editor := range fallbacks {
		if path, err := exec.LookPath(editor); err == nil {
			candidates = append(candidates, editorCommand{
				command: path,
				args:    nil,
				display: path,
			})
		}
	}

	seen := make(map[string]bool)
	result := make([]editorCommand, 0, len(candidates))
	for _, candidate := range candidates {
		key := candidate.command + "\x00" + strings.Join(candidate.args, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, candidate)
	}

	return result
}

func parseEditorCommand(raw string) (editorCommand, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return editorCommand{}, false
	}

	parts := strings.Fields(value)
	if len(parts) == 0 {
		return editorCommand{}, false
	}

	command := parts[0]
	if path, err := exec.LookPath(command); err == nil {
		command = path
	}

	return editorCommand{
		command: command,
		args:    parts[1:],
		display: value,
	}, true
}

// HasEditor returns true if an editor is available.
func HasEditor() bool {
	return findEditor() != ""
}

// GetEditor returns the path to the editor command.
func GetEditor() string {
	return findEditor()
}

// Edit opens the editor with the initial content and returns the edited content.
func (e *Editor) Edit() (string, error) {
	// Create temp file
	tempFile, err := e.createTempFile()
	if err != nil {
		return "", fmt.Errorf(i18n.T(i18n.UIErrCreateTempFile), err)
	}

	if e.config.DeleteOnClose {
		defer func() { _ = os.Remove(tempFile) }()
	}

	// Open editor
	if err := e.openEditor(tempFile); err != nil {
		return "", err
	}

	// Read content
	content, err := os.ReadFile(tempFile)
	if err != nil {
		return "", fmt.Errorf(i18n.T(i18n.UIErrReadEdited), err)
	}

	// Check if empty
	if len(content) == 0 && !e.config.AllowEmpty {
		return "", ErrEditorEmpty
	}

	return string(content), nil
}

// createTempFile creates a temp file with initial content.
func (e *Editor) createTempFile() (string, error) {
	// Create temp dir if needed
	tempDir := os.TempDir()

	// Create temp file
	pattern := e.config.Prefix + "*" + e.config.Extension
	tempFile, err := os.CreateTemp(tempDir, pattern)
	if err != nil {
		return "", err
	}
	defer func() { _ = tempFile.Close() }()

	// Write initial content
	if e.config.InitialContent != "" {
		if _, err := tempFile.WriteString(e.config.InitialContent); err != nil {
			_ = os.Remove(tempFile.Name())
			return "", err
		}
	}

	return tempFile.Name(), nil
}

// openEditor opens the editor and waits for it to close.
func (e *Editor) openEditor(filePath string) error {
	var lastErr error
	for _, candidate := range e.editors {
		args := append([]string{}, candidate.args...)
		args = append(args, filePath)

		cmd := exec.Command(candidate.command, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return fmt.Errorf("%w: exit code %d", ErrEditorFailed, exitErr.ExitCode())
			}
			lastErr = fmt.Errorf(i18n.T(i18n.UIErrRunEditor), err)
			continue
		}
		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return ErrNoEditor
}

// FilePath returns the path of the temp file (useful if DeleteOnClose is false).
func (e *Editor) FilePath() string {
	return filepath.Join(os.TempDir(), e.config.Prefix+"*"+e.config.Extension)
}

// OpenEditor is a convenience function to open an editor with content and return the result.
func OpenEditor(initialContent string) (string, error) {
	config := DefaultEditorConfig()
	config.InitialContent = initialContent

	editor, err := NewEditor(config)
	if err != nil {
		return "", err
	}

	return editor.Edit()
}

// OpenEditorWithConfig is a convenience function to open an editor with custom config.
func OpenEditorWithConfig(config EditorConfig) (string, error) {
	editor, err := NewEditor(config)
	if err != nil {
		return "", err
	}

	return editor.Edit()
}

// OpenEditorOrFallback opens an editor if available, otherwise returns the fallback content.
func OpenEditorOrFallback(initialContent string, fallback string) string {
	if !HasEditor() {
		return fallback
	}

	content, err := OpenEditor(initialContent)
	if err != nil {
		return fallback
	}

	return content
}
