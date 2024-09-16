package io

import (
	"sync"

	"github.com/pkg/browser"
)

// BrowserOpener is an interface for opening URLs in a browser.
type BrowserOpener interface {
	Open(url string) error
}

// DefaultBrowser opens URLs using the system default browser.
type DefaultBrowser struct{}

// NewDefaultBrowser creates a new DefaultBrowser.
func NewDefaultBrowser() *DefaultBrowser {
	return &DefaultBrowser{}
}

// Open opens a URL in the system default browser.
func (b *DefaultBrowser) Open(url string) error {
	return browser.OpenURL(url)
}

// MockBrowser records opened URLs for testing.
type MockBrowser struct {
	OpenedURLs []string
	OpenError  error
	mu         sync.Mutex
}

// NewMockBrowser creates a new MockBrowser.
func NewMockBrowser() *MockBrowser {
	return &MockBrowser{}
}

// Open records the URL and returns the configured error.
func (m *MockBrowser) Open(url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.OpenedURLs = append(m.OpenedURLs, url)
	return m.OpenError
}

// SetError configures the error to return on Open calls.
func (m *MockBrowser) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.OpenError = err
}

// GetOpenedURLs returns a copy of the opened URLs.
func (m *MockBrowser) GetOpenedURLs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.OpenedURLs))
	copy(result, m.OpenedURLs)
	return result
}

// WasOpened checks if a specific URL was opened.
func (m *MockBrowser) WasOpened(url string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.OpenedURLs {
		if u == url {
			return true
		}
	}
	return false
}

// Reset clears the recorded URLs and error.
func (m *MockBrowser) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.OpenedURLs = nil
	m.OpenError = nil
}
