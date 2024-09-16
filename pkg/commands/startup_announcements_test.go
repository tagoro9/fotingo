package commands

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/commandruntime"
)

type staticAnnouncementProvider struct {
	announcements []startupAnnouncement
	err           error
}

func (p staticAnnouncementProvider) Announcements() ([]startupAnnouncement, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.announcements, nil
}

func TestPrepareStartupAnnouncements_CollectsProviderMessages(t *testing.T) {
	restoreGlobals := saveGlobalFlags()
	defer restoreGlobals()

	origProviders := startupAnnouncementProviders
	startupAnnouncementProviders = []startupAnnouncementProvider{
		staticAnnouncementProvider{
			announcements: []startupAnnouncement{
				{Emoji: commandruntime.LogEmojiInfo, Message: "hello"},
			},
		},
	}
	t.Cleanup(func() {
		startupAnnouncementProviders = origProviders
		setPendingStartupAnnouncements(nil)
	})

	Global.JSON = false
	Global.Quiet = false

	cmd := &cobra.Command{Use: "open"}
	require.NoError(t, prepareStartupAnnouncements(cmd))

	announcements := consumePendingStartupAnnouncements()
	require.Len(t, announcements, 1)
	assert.Equal(t, "hello", announcements[0].Message)
}

func TestPrepareStartupAnnouncements_SkipsVersionCommand(t *testing.T) {
	restoreGlobals := saveGlobalFlags()
	defer restoreGlobals()

	origProviders := startupAnnouncementProviders
	startupAnnouncementProviders = []startupAnnouncementProvider{
		staticAnnouncementProvider{
			announcements: []startupAnnouncement{
				{Emoji: commandruntime.LogEmojiInfo, Message: "should not show"},
			},
		},
	}
	t.Cleanup(func() {
		startupAnnouncementProviders = origProviders
		setPendingStartupAnnouncements(nil)
	})

	cmd := &cobra.Command{Use: "version"}
	require.NoError(t, prepareStartupAnnouncements(cmd))
	assert.Empty(t, consumePendingStartupAnnouncements())
}

func TestPrepareStartupAnnouncements_SkipsStartAndReviewCommands(t *testing.T) {
	restoreGlobals := saveGlobalFlags()
	defer restoreGlobals()

	origProviders := startupAnnouncementProviders
	startupAnnouncementProviders = []startupAnnouncementProvider{
		staticAnnouncementProvider{
			announcements: []startupAnnouncement{
				{Emoji: commandruntime.LogEmojiInfo, Message: "should not show"},
			},
		},
	}
	t.Cleanup(func() {
		startupAnnouncementProviders = origProviders
		setPendingStartupAnnouncements(nil)
	})

	for _, commandName := range []string{"start", "review"} {
		cmd := &cobra.Command{Use: commandName}
		require.NoError(t, prepareStartupAnnouncements(cmd))
		assert.Empty(t, consumePendingStartupAnnouncements())
	}
}

func TestPrepareStartupAnnouncements_SkipsHiddenCompletionCommand(t *testing.T) {
	restoreGlobals := saveGlobalFlags()
	defer restoreGlobals()

	origProviders := startupAnnouncementProviders
	origArgsFn := completionArgsFn
	startupAnnouncementProviders = []startupAnnouncementProvider{
		staticAnnouncementProvider{
			announcements: []startupAnnouncement{
				{Emoji: commandruntime.LogEmojiInfo, Message: "should not show"},
			},
		},
	}
	completionArgsFn = func() []string { return []string{"fotingo", "__complete", "open", ""} }
	t.Cleanup(func() {
		startupAnnouncementProviders = origProviders
		completionArgsFn = origArgsFn
		setPendingStartupAnnouncements(nil)
	})

	cmd := &cobra.Command{Use: "__complete"}
	require.NoError(t, prepareStartupAnnouncements(cmd))
	assert.Empty(t, consumePendingStartupAnnouncements())
}

func TestEmitPendingStartupAnnouncements_WritesStatusEvent(t *testing.T) {
	restoreGlobals := saveGlobalFlags()
	defer restoreGlobals()

	Global.JSON = false
	Global.Quiet = false

	statusCh := make(chan string, 1)
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)

	setPendingStartupAnnouncements([]startupAnnouncement{
		{Emoji: commandruntime.LogEmojiInfo, Message: "announcement"},
	})

	emitPendingStartupAnnouncements(out)
	close(statusCh)

	raw := <-statusCh
	event, ok := decodeStatusEvent(raw)
	require.True(t, ok)
	assert.Equal(t, statusEventKindAppend, event.Kind)
	assert.Equal(t, "announcement", event.Message)
	assert.Equal(t, commandruntime.LogEmojiInfo, event.Emoji)
}

func TestBuildVersionChecker(t *testing.T) {
	origStore := newUtilityCacheStore
	t.Cleanup(func() { newUtilityCacheStore = origStore })

	newUtilityCacheStore = func() (cache.Store, error) {
		return cache.New(cache.WithPath(filepath.Join(t.TempDir(), "cache.db")), cache.WithLogger(nil))
	}

	checker, store, cleanup, err := buildVersionChecker()
	require.NoError(t, err)
	require.NotNil(t, checker)
	require.NotNil(t, store)
	require.NotNil(t, cleanup)
	cleanup()
}

func TestBuildVersionChecker_Error(t *testing.T) {
	origStore := newUtilityCacheStore
	t.Cleanup(func() { newUtilityCacheStore = origStore })

	newUtilityCacheStore = func() (cache.Store, error) {
		return nil, errors.New("cache init failed")
	}

	checker, store, cleanup, err := buildVersionChecker()
	require.Error(t, err)
	assert.Nil(t, checker)
	assert.Nil(t, store)
	require.NotNil(t, cleanup)
}
