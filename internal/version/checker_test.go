package version

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/cache"
)

func TestIsVersionNewer(t *testing.T) {
	t.Parallel()

	assert.True(t, IsVersionNewer("v1.2.0", "v1.1.9"))
	assert.False(t, IsVersionNewer("v1.2.0", "v1.2.0"))
	assert.False(t, IsVersionNewer("v1.1.9", "v1.2.0"))
	assert.False(t, IsVersionNewer("dev", "v1.2.0"))
}

func TestNewChecker_UsesDefaultLatestVersionFetcher(t *testing.T) {
	t.Parallel()

	checker := NewChecker("v1.0.0", nil)
	require.NotNil(t, checker.fetchLatest)
}

func TestChecker_UsesCachedStateWithinCadence(t *testing.T) {
	store := newStore(t)
	now := time.Date(2026, time.February, 17, 10, 0, 0, 0, time.UTC)

	require.NoError(t, store.SetWithTTL(defaultCheckStateKey, CheckState{
		CheckedAt:     now.Add(-2 * time.Hour),
		LatestVersion: "v1.2.0",
	}, 0))

	fetchCalls := 0
	checker := NewChecker(
		"v1.0.0",
		store,
		WithLatestVersionFetcher(func(context.Context) (string, error) {
			fetchCalls++
			return "v1.3.0", nil
		}),
	)
	checker.now = func() time.Time { return now }

	result, err := checker.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, fetchCalls)
	assert.True(t, result.UsedCached)
	assert.True(t, result.UpdateIsAvailable)
	assert.Equal(t, "v1.2.0", result.LatestVersion)
}

func TestChecker_FetchesAndStoresWhenStateIsStale(t *testing.T) {
	store := newStore(t)
	now := time.Date(2026, time.February, 17, 10, 0, 0, 0, time.UTC)

	require.NoError(t, store.SetWithTTL(defaultCheckStateKey, CheckState{
		CheckedAt:     now.Add(-48 * time.Hour),
		LatestVersion: "v1.1.0",
	}, 0))

	fetchCalls := 0
	checker := NewChecker(
		"v1.0.0",
		store,
		WithLatestVersionFetcher(func(context.Context) (string, error) {
			fetchCalls++
			return "v1.3.0", nil
		}),
	)
	checker.now = func() time.Time { return now }

	result, err := checker.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, fetchCalls)
	assert.False(t, result.UsedCached)
	assert.True(t, result.UpdateIsAvailable)
	assert.Equal(t, "v1.3.0", result.LatestVersion)

	var stored CheckState
	hit, err := store.Get(defaultCheckStateKey, &stored)
	require.NoError(t, err)
	require.True(t, hit)
	assert.Equal(t, "v1.3.0", stored.LatestVersion)
	assert.Equal(t, now.UTC(), stored.CheckedAt)
}

func TestChecker_FetchErrorFallsBackToCachedState(t *testing.T) {
	store := newStore(t)
	now := time.Date(2026, time.February, 17, 10, 0, 0, 0, time.UTC)

	require.NoError(t, store.SetWithTTL(defaultCheckStateKey, CheckState{
		CheckedAt:     now.Add(-48 * time.Hour),
		LatestVersion: "v1.4.0",
	}, 0))

	checker := NewChecker(
		"v1.0.0",
		store,
		WithLatestVersionFetcher(func(context.Context) (string, error) {
			return "", errors.New("boom")
		}),
	)
	checker.now = func() time.Time { return now }

	result, err := checker.Check(context.Background())
	require.Error(t, err)
	assert.True(t, result.UsedCached)
	assert.True(t, result.UpdateIsAvailable)
	assert.Equal(t, "v1.4.0", result.LatestVersion)
}

func TestChecker_SkipsNonSemanticCurrentVersion(t *testing.T) {
	store := newStore(t)

	fetchCalls := 0
	checker := NewChecker(
		"dev",
		store,
		WithLatestVersionFetcher(func(context.Context) (string, error) {
			fetchCalls++
			return "v1.3.0", nil
		}),
	)

	result, err := checker.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, fetchCalls)
	assert.Empty(t, result.LatestVersion)
	assert.False(t, result.UpdateIsAvailable)
}

func newStore(t *testing.T) cache.Store {
	t.Helper()

	store, err := cache.New(cache.WithPath(filepath.Join(t.TempDir(), "cache.db")))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	return store
}
