package commands

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/cache"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/ui"
)

type fakeCacheBrowser struct {
	runErr error
	called *bool
}

func (f fakeCacheBrowser) Run() error {
	if f.called != nil {
		*f.called = true
	}
	return f.runErr
}

func TestListCacheEntries_RedactsSensitiveValues(t *testing.T) {
	withTempUtilityCacheStore(t)

	store, err := newUtilityCacheStore()
	require.NoError(t, err)
	require.NoError(t, store.SetWithTTL("github:token", map[string]string{"token": "ghp_secret"}, time.Hour))
	require.NoError(t, store.SetWithTTL("github:labels", []string{"bug", "feature"}, time.Hour))
	require.NoError(t, store.Close())

	entries, err := listCacheEntries("github:")
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, "github:labels", entries[0].Key)
	assert.Contains(t, entries[0].Value, "bug")
	assert.Equal(t, "github:token", entries[1].Key)
	assert.Equal(t, "<redacted>", entries[1].Value)
}

func TestListCacheEntries_DecodesURLEncodedValues(t *testing.T) {
	withTempUtilityCacheStore(t)

	store, err := newUtilityCacheStore()
	require.NoError(t, err)
	require.NoError(
		t,
		store.SetWithTTL(
			"jira:issue-types:https%3A%2F%2Fexample.atlassian.net:FOTINGO",
			map[string]string{"url": "https%3A%2F%2Fexample.atlassian.net%2Fbrowse%2FPROJ-1"},
			time.Hour,
		),
	)
	require.NoError(t, store.Close())

	entries, err := listCacheEntries("jira:")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "jira:issue-types:https%3A%2F%2Fexample.atlassian.net:FOTINGO", entries[0].Key)
	assert.Equal(t, "jira:issue-types:https://example.atlassian.net:FOTINGO", entries[0].DisplayKey)
	assert.Contains(t, entries[0].Value, "https://example.atlassian.net/browse/PROJ-1")
	assert.Contains(t, entries[0].Detail, "https://example.atlassian.net/browse/PROJ-1")
}

func TestListCacheEntries_UsesFullValueForJSONAndPreviewForUI(t *testing.T) {
	withTempUtilityCacheStore(t)

	store, err := newUtilityCacheStore()
	require.NoError(t, err)
	longValue := strings.Repeat("x", 300)
	require.NoError(t, store.SetWithTTL("github:long", map[string]string{"value": longValue}, time.Hour))
	require.NoError(t, store.Close())

	entries, err := listCacheEntries("github:long")
	require.NoError(t, err)
	require.Len(t, entries, 1)

	assert.Contains(t, entries[0].Value, longValue)
	assert.Contains(t, entries[0].Preview, "...")
	assert.NotEqual(t, entries[0].Value, entries[0].Preview)
}

func TestResolveCacheEntriesToDelete_InteractiveSelection(t *testing.T) {
	origInteractiveFn := isInteractiveTerminalFn
	origSelectFn := runCacheMultiSelectFn
	cacheClearAll = false
	t.Cleanup(func() {
		isInteractiveTerminalFn = origInteractiveFn
		runCacheMultiSelectFn = origSelectFn
		cacheClearAll = false
	})

	isInteractiveTerminalFn = func() bool { return true }
	runCacheMultiSelectFn = func(entries []cache.Entry) ([]string, error) {
		require.Len(t, entries, 2)
		return []string{entries[1].Key}, nil
	}

	keys, err := resolveCacheEntriesToDelete(
		[]cache.Entry{
			{Key: "key:one"},
			{Key: "key:two"},
		},
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"key:two"}, keys)
}

func TestPickCacheEntriesForDelete_InteractiveCancel(t *testing.T) {
	origSelectFn := runCacheMultiSelectFn
	t.Cleanup(func() {
		runCacheMultiSelectFn = origSelectFn
	})

	runCacheMultiSelectFn = func(entries []cache.Entry) ([]string, error) {
		require.Len(t, entries, 2)
		return nil, fterrors.ErrUserCancelled
	}

	_, err := runCacheMultiSelectFn([]cache.Entry{
		{Key: "key:one"},
		{Key: "key:two"},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, fterrors.ErrUserCancelled)
}

func TestPickCacheEntriesForDelete_UsesUISelector(t *testing.T) {
	origSelect := cacheSelectIDsFn
	t.Cleanup(func() {
		cacheSelectIDsFn = origSelect
	})

	cacheSelectIDsFn = func(_ string, items []ui.MultiSelectItem, minimum int) ([]string, error) {
		require.Equal(t, 1, minimum)
		require.Len(t, items, 2)
		return []string{items[0].ID}, nil
	}

	keys, err := pickCacheEntriesForDelete([]cache.Entry{
		{Key: "key:one", Value: []byte("123")},
		{Key: "key:two", Value: []byte("9999")},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"key:one"}, keys)
}

func TestPickCacheEntriesForDelete_EmptySelectionCancels(t *testing.T) {
	origSelect := cacheSelectIDsFn
	t.Cleanup(func() {
		cacheSelectIDsFn = origSelect
	})

	cacheSelectIDsFn = func(_ string, _ []ui.MultiSelectItem, _ int) ([]string, error) {
		return []string{}, nil
	}

	_, err := pickCacheEntriesForDelete([]cache.Entry{{Key: "key:one"}})
	require.Error(t, err)
	assert.ErrorIs(t, err, fterrors.ErrUserCancelled)
}

func TestRunCacheBrowserUsesBrowserFactory(t *testing.T) {
	origFactory := newCacheBrowserFn
	t.Cleanup(func() {
		newCacheBrowserFn = origFactory
	})

	called := false
	newCacheBrowserFn = func(title string, items []ui.PickerItem, detailRenderer func(ui.PickerItem) string) cacheBrowser {
		require.NotEmpty(t, title)
		require.Len(t, items, 1)
		assert.Contains(t, detailRenderer(items[0]), "Key:")
		return fakeCacheBrowser{called: &called}
	}

	err := runCacheBrowser([]cacheEntryView{{
		Key:        "github:labels",
		DisplayKey: "github:labels",
		Detail:     "value",
		Metadata:   "meta",
		SizeBytes:  10,
	}})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestRunCacheBrowserFactoryError(t *testing.T) {
	origFactory := newCacheBrowserFn
	t.Cleanup(func() {
		newCacheBrowserFn = origFactory
	})

	expectedErr := errors.New("boom")
	newCacheBrowserFn = func(string, []ui.PickerItem, func(ui.PickerItem) string) cacheBrowser {
		return fakeCacheBrowser{runErr: expectedErr}
	}

	err := runCacheBrowser([]cacheEntryView{{Key: "x", DisplayKey: "x"}})
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func withTempUtilityCacheStore(t *testing.T) {
	t.Helper()

	cachePath := filepath.Join(t.TempDir(), "cache.db")
	origFactory := newUtilityCacheStore
	newUtilityCacheStore = func() (cache.Store, error) {
		return cache.New(cache.WithPath(cachePath), cache.WithLogger(nil))
	}
	t.Cleanup(func() {
		newUtilityCacheStore = origFactory
	})
}
