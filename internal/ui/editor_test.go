package ui

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultEditorConfig(t *testing.T) {
	t.Parallel()

	config := DefaultEditorConfig()

	assert.Equal(t, ".md", config.Extension)
	assert.Equal(t, "fotingo-", config.Prefix)
	assert.True(t, config.DeleteOnClose)
	assert.False(t, config.AllowEmpty)
	assert.Empty(t, config.InitialContent)
}

func TestFindEditor(t *testing.T) {
	t.Run("respects VISUAL env var", func(t *testing.T) {
		t.Setenv("VISUAL", "/usr/bin/code")
		t.Setenv("EDITOR", "")

		editor := findEditor()
		assert.Equal(t, "/usr/bin/code", editor)
	})

	t.Run("falls back to EDITOR env var", func(t *testing.T) {
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "/usr/bin/vim")

		editor := findEditor()
		assert.Equal(t, "/usr/bin/vim", editor)
	})

	t.Run("supports editor command with args", func(t *testing.T) {
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "code --wait")

		editor := findEditor()
		assert.Equal(t, "code --wait", editor)
	})
}

func TestHasEditor(t *testing.T) {
	t.Run("returns true when editor is set", func(t *testing.T) {
		t.Setenv("EDITOR", "/usr/bin/vim")
		assert.True(t, HasEditor())
	})
}

func TestGetEditor(t *testing.T) {
	t.Run("returns editor path", func(t *testing.T) {
		t.Setenv("EDITOR", "/usr/bin/nano")
		assert.Equal(t, "/usr/bin/nano", GetEditor())
	})
}

func TestNewEditor(t *testing.T) {
	t.Run("succeeds when editor is available", func(t *testing.T) {
		t.Setenv("EDITOR", "/usr/bin/vim")

		config := DefaultEditorConfig()
		editor, err := NewEditor(config)
		assert.NoError(t, err)
		assert.NotNil(t, editor)
		require.NotEmpty(t, editor.editors)
		assert.Equal(t, "/usr/bin/vim", editor.editors[0].command)
	})

	t.Run("returns error when no editor available", func(t *testing.T) {
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "")
		t.Setenv("PATH", "") // Clear PATH to prevent fallback detection

		config := DefaultEditorConfig()
		editor, err := NewEditor(config)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNoEditor)
		assert.Nil(t, editor)
	})
}

func TestEditorCreateTempFile(t *testing.T) {
	t.Run("creates temp file with content", func(t *testing.T) {
		t.Setenv("EDITOR", "/usr/bin/cat") // Use cat as a "no-op" editor

		config := EditorConfig{
			Extension:      ".txt",
			Prefix:         "test-",
			InitialContent: "Hello, World!",
			DeleteOnClose:  true,
		}

		editor, err := NewEditor(config)
		assert.NoError(t, err)

		filePath, err := editor.createTempFile()
		assert.NoError(t, err)
		assert.NotEmpty(t, filePath)
		defer func() { _ = os.Remove(filePath) }()

		// Verify content
		content, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, "Hello, World!", string(content))
	})

	t.Run("creates temp file with correct extension", func(t *testing.T) {
		t.Setenv("EDITOR", "/usr/bin/cat")

		config := EditorConfig{
			Extension:     ".md",
			Prefix:        "fotingo-",
			DeleteOnClose: true,
		}

		editor, err := NewEditor(config)
		assert.NoError(t, err)

		filePath, err := editor.createTempFile()
		assert.NoError(t, err)
		assert.Contains(t, filePath, "fotingo-")
		assert.Contains(t, filePath, ".md")
		defer func() { _ = os.Remove(filePath) }()
	})
}

func TestEditorFilePath(t *testing.T) {
	t.Run("returns pattern", func(t *testing.T) {
		t.Setenv("EDITOR", "/usr/bin/cat")

		config := EditorConfig{
			Extension: ".md",
			Prefix:    "fotingo-",
		}

		editor, err := NewEditor(config)
		assert.NoError(t, err)

		path := editor.FilePath()
		assert.Contains(t, path, "fotingo-")
		assert.Contains(t, path, ".md")
	})
}

func TestOpenEditorOrFallback(t *testing.T) {
	t.Run("returns fallback when no editor", func(t *testing.T) {
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "")
		t.Setenv("PATH", "")

		result := OpenEditorOrFallback("initial", "fallback")
		assert.Equal(t, "fallback", result)
	})
}

func TestEditorEdit(t *testing.T) {
	t.Run("returns content from editor", func(t *testing.T) {
		// Use "cat" as editor - it reads stdin and writes to stdout, leaving the file unchanged
		// We set initial content and use "true" which does nothing (leaves file as-is)
		t.Setenv("EDITOR", "true")

		config := EditorConfig{
			Extension:      ".txt",
			Prefix:         "test-edit-",
			InitialContent: "initial content",
			DeleteOnClose:  true,
		}

		editor, err := NewEditor(config)
		assert.NoError(t, err)

		content, err := editor.Edit()
		assert.NoError(t, err)
		assert.Equal(t, "initial content", content)
	})

	t.Run("returns error for empty content when AllowEmpty is false", func(t *testing.T) {
		t.Setenv("EDITOR", "true")

		config := EditorConfig{
			Extension:     ".txt",
			Prefix:        "test-empty-",
			DeleteOnClose: true,
			AllowEmpty:    false,
		}

		editor, err := NewEditor(config)
		assert.NoError(t, err)

		_, err = editor.Edit()
		assert.ErrorIs(t, err, ErrEditorEmpty)
	})

	t.Run("allows empty content when AllowEmpty is true", func(t *testing.T) {
		t.Setenv("EDITOR", "true")

		config := EditorConfig{
			Extension:     ".txt",
			Prefix:        "test-allow-empty-",
			DeleteOnClose: true,
			AllowEmpty:    true,
		}

		editor, err := NewEditor(config)
		assert.NoError(t, err)

		content, err := editor.Edit()
		assert.NoError(t, err)
		assert.Empty(t, content)
	})

	t.Run("returns error when editor fails", func(t *testing.T) {
		t.Setenv("EDITOR", "false") // "false" exits with code 1

		config := EditorConfig{
			Extension:     ".txt",
			Prefix:        "test-fail-",
			DeleteOnClose: true,
		}

		editor, err := NewEditor(config)
		assert.NoError(t, err)

		_, err = editor.Edit()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrEditorFailed)
	})

	t.Run("keeps file when DeleteOnClose is false", func(t *testing.T) {
		t.Setenv("EDITOR", "true")

		config := EditorConfig{
			Extension:      ".txt",
			Prefix:         "test-keep-",
			InitialContent: "keep me",
			DeleteOnClose:  false,
		}

		editor, err := NewEditor(config)
		assert.NoError(t, err)

		content, err := editor.Edit()
		assert.NoError(t, err)
		assert.Equal(t, "keep me", content)
	})
}

func TestOpenEditorConvenienceFunctions(t *testing.T) {
	t.Run("OpenEditor returns content", func(t *testing.T) {
		t.Setenv("EDITOR", "true")

		content, err := OpenEditor("test content")
		assert.NoError(t, err)
		assert.Equal(t, "test content", content)
	})

	t.Run("OpenEditorWithConfig returns content", func(t *testing.T) {
		t.Setenv("EDITOR", "true")

		config := EditorConfig{
			Extension:      ".txt",
			Prefix:         "test-",
			InitialContent: "custom config",
			DeleteOnClose:  true,
		}

		content, err := OpenEditorWithConfig(config)
		assert.NoError(t, err)
		assert.Equal(t, "custom config", content)
	})

	t.Run("OpenEditorOrFallback returns editor content when available", func(t *testing.T) {
		t.Setenv("EDITOR", "true")

		result := OpenEditorOrFallback("editor content", "fallback")
		assert.Equal(t, "editor content", result)
	})

	t.Run("OpenEditorOrFallback returns fallback on error", func(t *testing.T) {
		t.Setenv("EDITOR", "false")

		result := OpenEditorOrFallback("", "fallback")
		assert.Equal(t, "fallback", result)
	})
}

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("error messages are set", func(t *testing.T) {
		t.Parallel()
		assert.NotEmpty(t, ErrNoEditor.Error())
		assert.NotEmpty(t, ErrEditorFailed.Error())
		assert.NotEmpty(t, ErrEditorEmpty.Error())
	})
}
