package commandruntime

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type startupProviderStub struct {
	announcements []StartupAnnouncement
	err           error
}

func (s startupProviderStub) Announcements() ([]StartupAnnouncement, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.announcements, nil
}

func TestStartupAnnouncementManagerPrepareDisabled(t *testing.T) {
	manager := NewStartupAnnouncementManager()
	manager.SetPending([]StartupAnnouncement{{Message: "existing"}})

	err := manager.Prepare(false, []StartupAnnouncementProvider{
		startupProviderStub{announcements: []StartupAnnouncement{{Message: "new"}}},
	})
	require.NoError(t, err)
	assert.Empty(t, manager.ConsumePending())
}

func TestStartupAnnouncementManagerPrepareCollectsAndIgnoresProviderErrors(t *testing.T) {
	manager := NewStartupAnnouncementManager()
	err := manager.Prepare(true, []StartupAnnouncementProvider{
		startupProviderStub{announcements: []StartupAnnouncement{{Emoji: LogEmojiRocket, Message: "update"}}},
		startupProviderStub{err: errors.New("boom")},
		startupProviderStub{announcements: []StartupAnnouncement{{Emoji: LogEmojiInfo, Message: "tip"}}},
	})
	require.NoError(t, err)

	pending := manager.ConsumePending()
	require.Len(t, pending, 2)
	assert.Equal(t, "update", pending[0].Message)
	assert.Equal(t, "tip", pending[1].Message)
	assert.Empty(t, manager.ConsumePending())
}

func TestShouldSkipStartupCommand(t *testing.T) {
	assert.True(t, ShouldSkipStartupCommand("version", false, false))
	assert.True(t, ShouldSkipStartupCommand("completion", false, false))
	assert.True(t, ShouldSkipStartupCommand("inspect", false, false))
	assert.True(t, ShouldSkipStartupCommand("start", false, false))
	assert.True(t, ShouldSkipStartupCommand("review", false, false))
	assert.True(t, ShouldSkipStartupCommand("review", true, false))
	assert.True(t, ShouldSkipStartupCommand("start", false, true))
	assert.True(t, ShouldSkipStartupCommand("help", false, false))
	assert.False(t, ShouldSkipStartupCommand("open", false, false))
}
