package io

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultBrowser_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ BrowserOpener = &DefaultBrowser{}
	var _ BrowserOpener = NewDefaultBrowser()
}

func TestMockBrowser_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ BrowserOpener = &MockBrowser{}
	var _ BrowserOpener = NewMockBrowser()
}

func TestNewMockBrowser(t *testing.T) {
	t.Parallel()
	mock := NewMockBrowser()

	assert.NotNil(t, mock)
	assert.Empty(t, mock.OpenedURLs)
	assert.Nil(t, mock.OpenError)
}

func TestMockBrowser_Open_RecordsURL(t *testing.T) {
	t.Parallel()
	mock := NewMockBrowser()

	err := mock.Open("https://example.com")

	require.NoError(t, err)
	assert.Equal(t, []string{"https://example.com"}, mock.GetOpenedURLs())
}

func TestMockBrowser_Open_RecordsMultipleURLs(t *testing.T) {
	t.Parallel()
	mock := NewMockBrowser()

	err1 := mock.Open("https://example.com")
	err2 := mock.Open("https://github.com")
	err3 := mock.Open("https://google.com")

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)
	assert.Equal(t, []string{
		"https://example.com",
		"https://github.com",
		"https://google.com",
	}, mock.GetOpenedURLs())
}

func TestMockBrowser_Open_ReturnsConfiguredError(t *testing.T) {
	t.Parallel()
	mock := NewMockBrowser()
	expectedErr := errors.New("browser not available")
	mock.SetError(expectedErr)

	err := mock.Open("https://example.com")

	assert.Equal(t, expectedErr, err)
	// URL should still be recorded even when error is returned
	assert.Equal(t, []string{"https://example.com"}, mock.GetOpenedURLs())
}

func TestMockBrowser_WasOpened(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		openURLs   []string
		checkURL   string
		wantOpened bool
	}{
		{
			name:       "URL was opened",
			openURLs:   []string{"https://example.com", "https://github.com"},
			checkURL:   "https://example.com",
			wantOpened: true,
		},
		{
			name:       "URL was not opened",
			openURLs:   []string{"https://example.com"},
			checkURL:   "https://github.com",
			wantOpened: false,
		},
		{
			name:       "no URLs opened",
			openURLs:   []string{},
			checkURL:   "https://example.com",
			wantOpened: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mock := NewMockBrowser()
			for _, url := range tt.openURLs {
				_ = mock.Open(url)
			}

			got := mock.WasOpened(tt.checkURL)

			assert.Equal(t, tt.wantOpened, got)
		})
	}
}

func TestMockBrowser_Reset(t *testing.T) {
	t.Parallel()
	mock := NewMockBrowser()
	mock.SetError(errors.New("some error"))
	_ = mock.Open("https://example.com")

	mock.Reset()

	assert.Empty(t, mock.GetOpenedURLs())
	assert.Nil(t, mock.OpenError)
}

func TestMockBrowser_GetOpenedURLs_ReturnsCopy(t *testing.T) {
	t.Parallel()
	mock := NewMockBrowser()
	_ = mock.Open("https://example.com")

	urls := mock.GetOpenedURLs()
	urls[0] = "modified"

	// Original should be unchanged
	assert.Equal(t, []string{"https://example.com"}, mock.GetOpenedURLs())
}

func TestMockBrowser_ThreadSafety(t *testing.T) {
	t.Parallel()
	mock := NewMockBrowser()
	var wg sync.WaitGroup
	numGoroutines := 100

	// Test concurrent Open calls
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(n int) {
			defer wg.Done()
			_ = mock.Open("https://example.com/" + string(rune('a'+n%26)))
		}(i)
	}
	wg.Wait()

	assert.Len(t, mock.GetOpenedURLs(), numGoroutines)
}
