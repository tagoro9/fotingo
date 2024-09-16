package commands

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/cache"
)

func TestConfigViewInteractiveUsesBrowserSession(t *testing.T) {
	setDefaultOutputFlags(t)

	origInteractive := isInteractiveTerminalFn
	origBrowser := runConfigBrowserFn
	t.Cleanup(func() {
		isInteractiveTerminalFn = origInteractive
		runConfigBrowserFn = origBrowser
	})

	isInteractiveTerminalFn = func() bool { return true }
	called := false
	runConfigBrowserFn = func(entries []configEntry) error {
		called = true
		require.NotEmpty(t, entries)
		return nil
	}

	err := configViewCmd.RunE(configViewCmd, []string{})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestConfigViewFiltersEmptyValues(t *testing.T) {
	setDefaultOutputFlags(t)

	origInteractive := isInteractiveTerminalFn
	origBrowser := runConfigBrowserFn
	t.Cleanup(func() {
		isInteractiveTerminalFn = origInteractive
		runConfigBrowserFn = origBrowser
	})

	isInteractiveTerminalFn = func() bool { return true }
	var gotEntries []configEntry
	runConfigBrowserFn = func(entries []configEntry) error {
		gotEntries = entries
		return nil
	}

	err := configViewCmd.RunE(configViewCmd, []string{})
	require.NoError(t, err)
	require.NotEmpty(t, gotEntries)

	for _, entry := range gotEntries {
		assert.NotEmpty(t, strings.TrimSpace(entry.Value))
	}
}

func TestCacheViewInteractiveUsesBrowserSession(t *testing.T) {
	setDefaultOutputFlags(t)

	origInteractive := isInteractiveTerminalFn
	origBrowser := runCacheBrowserFn
	origCacheFactory := newUtilityCacheStore
	t.Cleanup(func() {
		isInteractiveTerminalFn = origInteractive
		runCacheBrowserFn = origBrowser
		newUtilityCacheStore = origCacheFactory
	})

	cachePath := filepath.Join(t.TempDir(), "cache.db")
	newUtilityCacheStore = func() (cache.Store, error) {
		return cache.New(cache.WithPath(cachePath), cache.WithLogger(nil))
	}

	store, err := newUtilityCacheStore()
	require.NoError(t, err)
	require.NoError(t, store.SetWithTTL("github:labels", []string{"bug"}, time.Hour))
	require.NoError(t, store.Close())

	isInteractiveTerminalFn = func() bool { return true }
	called := false
	runCacheBrowserFn = func(entries []cacheEntryView) error {
		called = true
		require.NotEmpty(t, entries)
		return nil
	}

	err = cacheViewCmd.RunE(cacheViewCmd, []string{})
	require.NoError(t, err)
	assert.True(t, called)
}
