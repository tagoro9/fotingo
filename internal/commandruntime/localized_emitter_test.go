package commandruntime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/i18n"
)

func TestLocalizedEmitterRedactsDebug(t *testing.T) {
	ch := make(chan string, 1)
	emitter := NewLocalizedEmitter(
		ch,
		func(level OutputLevel) bool { return level == OutputLevelDebug },
		i18n.T,
	)
	emitter.DebugRaw("token=abc123 password:secret bearer mytoken")

	var raw string
	select {
	case raw = <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected debug message to be emitted")
	}

	event, ok := DecodeStatusEvent(raw)
	require.True(t, ok)
	assert.Equal(t, OutputLevelDebug, event.Level)
	assert.Equal(t, LogEmojiDebug, event.Emoji)
	assert.Contains(t, event.Message, "token=<redacted>")
	assert.Contains(t, event.Message, "password=<redacted>")
	assert.Contains(t, event.Message, "bearer <redacted>")
	assert.NotContains(t, event.Message, "abc123")
	assert.NotContains(t, event.Message, "secret")
	assert.NotContains(t, event.Message, "mytoken")
}

func TestLocalizedEmitterHonorsLevels(t *testing.T) {
	ch := make(chan string, 4)
	emitter := NewLocalizedEmitter(
		ch,
		func(level OutputLevel) bool { return level == OutputLevelInfo },
		i18n.T,
	)

	emitter.InfoRaw(LogEmojiCheck, "k")
	emitter.VerboseRaw("v")
	emitter.DebugRaw("d")

	close(ch)
	var events []StatusEvent
	for raw := range ch {
		event, ok := DecodeStatusEvent(raw)
		require.True(t, ok)
		events = append(events, event)
	}

	require.Len(t, events, 1)
	assert.Equal(t, StatusEventKindAppend, events[0].Kind)
	assert.Equal(t, OutputLevelInfo, events[0].Level)
	assert.Equal(t, LogEmojiCheck, events[0].Emoji)
	assert.Equal(t, "k", events[0].Message)
}

func TestLocalizedEmitterOperationLifecycleEncoding(t *testing.T) {
	ch := make(chan string, 4)
	emitter := NewLocalizedEmitter(ch, nil, i18n.T)

	emitter.StartRaw("open-browser", LogEmojiBrowser, "opening pr")
	emitter.UpdateRaw("open-browser", LogEmojiBrowser, "opening https://github.com/o/r/pull/1")
	emitter.SuccessRaw("open-browser", LogEmojiBrowser, "opened")
	close(ch)

	var events []StatusEvent
	for raw := range ch {
		event, ok := DecodeStatusEvent(raw)
		require.True(t, ok)
		events = append(events, event)
	}

	require.Len(t, events, 3)
	assert.Equal(t, StatusEventKindStart, events[0].Kind)
	assert.Equal(t, StatusEventKindUpdate, events[1].Kind)
	assert.Equal(t, StatusEventKindSuccess, events[2].Kind)
	assert.Equal(t, "open-browser", events[0].OperationID)
	assert.Equal(t, LogEmojiBrowser, events[0].Emoji)
	assert.Equal(t, OutputLevelInfo, events[0].Level)
}

func TestLocalizedEmitterErrorEncoding(t *testing.T) {
	ch := make(chan string, 2)
	emitter := NewLocalizedEmitter(ch, nil, i18n.T)

	emitter.ErrorRaw("op-1", LogEmojiError, "boom")
	emitter.Error("op-2", LogEmojiError, i18n.RootShort)
	close(ch)

	var events []StatusEvent
	for raw := range ch {
		event, ok := DecodeStatusEvent(raw)
		require.True(t, ok)
		events = append(events, event)
	}

	require.Len(t, events, 2)
	assert.Equal(t, OutputLevelInfo, events[0].Level)
	assert.Equal(t, OutputLevelInfo, events[1].Level)
	assert.Equal(t, LogEmojiError, events[0].Emoji)
	assert.Equal(t, LogEmojiError, events[1].Emoji)
	assert.Equal(t, StatusEventKindError, events[0].Kind)
	assert.Equal(t, StatusEventKindError, events[1].Kind)
	assert.Equal(t, "op-1", events[0].OperationID)
	assert.Equal(t, "op-2", events[1].OperationID)
	assert.Equal(t, "boom", events[0].Message)
	assert.NotEmpty(t, events[1].Message)
}
