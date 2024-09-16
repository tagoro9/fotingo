package commandruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeStatusEvent(t *testing.T) {
	raw := EncodeStatusEvent(
		StatusEventKindUpdate,
		"open-browser",
		"opening",
		OutputLevelInfo,
		LogEmojiBrowser,
	)

	event, ok := DecodeStatusEvent(raw)
	require.True(t, ok)
	assert.Equal(t, StatusEventKindUpdate, event.Kind)
	assert.Equal(t, "open-browser", event.OperationID)
	assert.Equal(t, "opening", event.Message)
	assert.Equal(t, OutputLevelInfo, event.Level)
	assert.Equal(t, LogEmojiBrowser, event.Emoji)
}

func TestDecodeStatusEventRejectsInvalidPayloads(t *testing.T) {
	_, ok := DecodeStatusEvent("plain text")
	assert.False(t, ok)

	_, ok = DecodeStatusEvent(StatusEventPrefix + "{invalid")
	assert.False(t, ok)
}

func TestDefaultEmojiForLevel(t *testing.T) {
	assert.Equal(t, LogEmojiVerbose, DefaultEmojiForLevel(OutputLevelVerbose))
	assert.Equal(t, LogEmojiDebug, DefaultEmojiForLevel(OutputLevelDebug))
	assert.Equal(t, LogEmojiNone, DefaultEmojiForLevel(OutputLevelInfo))
}
