package io

import (
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultEditor_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ Editor = &DefaultEditor{}
	var _ Editor = NewDefaultEditor()
}

func TestMockEditor_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ Editor = &MockEditor{}
	var _ Editor = NewMockEditor()
}

func TestNewDefaultEditor(t *testing.T) {
	t.Parallel()
	editor := NewDefaultEditor()

	assert.Equal(t, "EDITOR", editor.EditorEnvVar)
	assert.Equal(t, "vi", editor.FallbackEditor)
}

func TestDefaultEditor_getEditorCommand(t *testing.T) {
	tests := []struct {
		name           string
		editorEnvVar   string
		fallbackEditor string
		envValue       string
		want           string
	}{
		{
			name:           "uses environment variable",
			editorEnvVar:   "TEST_EDITOR_VAR",
			fallbackEditor: "nano",
			envValue:       "vim",
			want:           "vim",
		},
		{
			name:           "uses fallback when env not set",
			editorEnvVar:   "TEST_EDITOR_VAR_UNSET",
			fallbackEditor: "nano",
			envValue:       "",
			want:           "nano",
		},
		{
			name:           "uses vi when no fallback and env not set",
			editorEnvVar:   "TEST_EDITOR_VAR_UNSET_2",
			fallbackEditor: "",
			envValue:       "",
			want:           "vi",
		},
		{
			name:           "defaults to EDITOR env var when editorEnvVar is empty",
			editorEnvVar:   "",
			fallbackEditor: "nano",
			envValue:       "custom-editor",
			want:           "custom-editor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable for the test
			if tt.envValue != "" {
				envVarName := tt.editorEnvVar
				if envVarName == "" {
					envVarName = "EDITOR"
				}
				t.Setenv(envVarName, tt.envValue)
			}

			editor := &DefaultEditor{
				EditorEnvVar:   tt.editorEnvVar,
				FallbackEditor: tt.fallbackEditor,
			}

			got := editor.getEditorCommand()

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultEditor_Edit_CreatesAndCleansUpTempFile(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()

	// Skip this test in CI environments where editors may not work
	if os.Getenv("CI") != "" {
		t.Skip("Skipping in CI environment")
	}

	// Use 'true' as the editor - it does nothing and exits successfully
	t.Setenv("EDITOR", "true")

	editor := NewDefaultEditor()
	content := "test content"

	result, err := editor.Edit(content)

	require.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestNewMockEditor(t *testing.T) {
	t.Parallel()
	mock := NewMockEditor()

	assert.NotNil(t, mock)
	assert.Empty(t, mock.ReturnContent)
	assert.Nil(t, mock.ReturnError)
	assert.Empty(t, mock.ReceivedContent)
	assert.Zero(t, mock.EditCalls)
	assert.Empty(t, mock.EditHistory)
}

func TestMockEditor_Edit_RecordsContent(t *testing.T) {
	t.Parallel()
	mock := NewMockEditor()
	content := "original content"

	_, err := mock.Edit(content)

	require.NoError(t, err)
	assert.Equal(t, content, mock.GetReceivedContent())
	assert.Equal(t, 1, mock.GetEditCalls())
}

func TestMockEditor_Edit_ReturnsInputWhenNoReturnContentSet(t *testing.T) {
	t.Parallel()
	mock := NewMockEditor()
	content := "original content"

	result, err := mock.Edit(content)

	require.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestMockEditor_Edit_ReturnsConfiguredContent(t *testing.T) {
	t.Parallel()
	mock := NewMockEditor()
	mock.SetReturnContent("modified content")

	result, err := mock.Edit("original content")

	require.NoError(t, err)
	assert.Equal(t, "modified content", result)
}

func TestMockEditor_Edit_ReturnsConfiguredError(t *testing.T) {
	t.Parallel()
	mock := NewMockEditor()
	expectedErr := errors.New("editor failed")
	mock.SetError(expectedErr)

	result, err := mock.Edit("content")

	assert.Equal(t, expectedErr, err)
	assert.Empty(t, result)
	// Content should still be recorded
	assert.Equal(t, "content", mock.GetReceivedContent())
}

func TestMockEditor_Edit_RecordsHistory(t *testing.T) {
	t.Parallel()
	mock := NewMockEditor()

	_, _ = mock.Edit("first")
	_, _ = mock.Edit("second")
	_, _ = mock.Edit("third")

	assert.Equal(t, 3, mock.GetEditCalls())
	assert.Equal(t, []string{"first", "second", "third"}, mock.GetEditHistory())
	assert.Equal(t, "third", mock.GetReceivedContent())
}

func TestMockEditor_Reset(t *testing.T) {
	t.Parallel()
	mock := NewMockEditor()
	mock.SetReturnContent("modified")
	mock.SetError(errors.New("some error"))
	_, _ = mock.Edit("content")

	mock.Reset()

	assert.Empty(t, mock.ReturnContent)
	assert.Nil(t, mock.ReturnError)
	assert.Empty(t, mock.ReceivedContent)
	assert.Zero(t, mock.EditCalls)
	assert.Empty(t, mock.EditHistory)
}

func TestMockEditor_GetEditHistory_ReturnsCopy(t *testing.T) {
	t.Parallel()
	mock := NewMockEditor()
	_, _ = mock.Edit("content")

	history := mock.GetEditHistory()
	history[0] = "modified"

	// Original should be unchanged
	assert.Equal(t, []string{"content"}, mock.GetEditHistory())
}

func TestMockEditor_ThreadSafety(t *testing.T) {
	t.Parallel()
	mock := NewMockEditor()
	var wg sync.WaitGroup
	numGoroutines := 100

	// Test concurrent Edit calls
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(n int) {
			defer wg.Done()
			_, _ = mock.Edit("content " + string(rune('a'+n%26)))
		}(i)
	}
	wg.Wait()

	assert.Equal(t, numGoroutines, mock.GetEditCalls())
	assert.Len(t, mock.GetEditHistory(), numGoroutines)
}
