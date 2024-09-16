package commandruntime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/version"
)

func TestVersionAnnouncementProvider_EmitsWhenNewerVersionExists(t *testing.T) {
	provider := NewVersionAnnouncementProvider(VersionAnnouncementProviderConfig{
		CurrentVersion: func() string { return "v1.0.0" },
		BuildChecker: func() (*version.Checker, cache.Store, func(), error) {
			checker := version.NewChecker(
				"v1.0.0",
				nil,
				version.WithLatestVersionFetcher(func(context.Context) (string, error) {
					return "v1.1.0", nil
				}),
			)
			return checker, nil, func() {}, nil
		},
	})

	announcements, err := provider.Announcements()
	require.NoError(t, err)
	require.Len(t, announcements, 1)
	assert.Equal(t, LogEmojiRocket, announcements[0].Emoji)
	assert.Contains(t, announcements[0].Message, "v1.1.0")
}

func TestVersionAnnouncementProvider_SkipsDevVersion(t *testing.T) {
	provider := NewVersionAnnouncementProvider(VersionAnnouncementProviderConfig{
		CurrentVersion: func() string { return "dev" },
		BuildChecker: func() (*version.Checker, cache.Store, func(), error) {
			return nil, nil, func() {}, errors.New("should not be called")
		},
	})

	announcements, err := provider.Announcements()
	require.NoError(t, err)
	assert.Empty(t, announcements)
}

func TestVersionAnnouncementProvider_ThrottlesToOncePerDay(t *testing.T) {
	store, err := cache.New(cache.WithPath(filepath.Join(t.TempDir(), "cache.db")), cache.WithLogger(nil))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	provider := NewVersionAnnouncementProvider(VersionAnnouncementProviderConfig{
		CurrentVersion: func() string { return "v1.0.0" },
		BuildChecker: func() (*version.Checker, cache.Store, func(), error) {
			checker := version.NewChecker(
				"v1.0.0",
				store,
				version.WithLatestVersionFetcher(func(context.Context) (string, error) {
					return "v1.1.0", nil
				}),
			)
			return checker, store, func() {}, nil
		},
	})

	first, err := provider.Announcements()
	require.NoError(t, err)
	require.Len(t, first, 1)

	second, err := provider.Announcements()
	require.NoError(t, err)
	assert.Empty(t, second)
}

func TestVersionAnnouncementThrottleStateHelpers(t *testing.T) {
	store, err := cache.New(cache.WithPath(filepath.Join(t.TempDir(), "cache.db")), cache.WithLogger(nil))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	assert.True(t, shouldEmitVersionAnnouncement(store))
	require.NoError(t, markVersionAnnouncementShown(store))
	assert.False(t, shouldEmitVersionAnnouncement(store))

	require.NoError(t, store.SetWithTTL(versionAnnouncementStateKey, versionAnnouncementState{
		LastShownAt: time.Now().UTC().Add(-25 * time.Hour),
	}, 0))
	assert.True(t, shouldEmitVersionAnnouncement(store))
}
