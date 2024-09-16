package commandruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusEmitterEmit(t *testing.T) {
	ch := make(chan string, 1)
	emitter := NewStatusEmitter(ch, func(level OutputLevel) bool {
		return level == OutputLevelInfo
	}, RedactSensitiveDebug)

	emitter.Emit(OutputLevelInfo, LogEmojiNone, "hello")
	raw := <-ch

	event, ok := DecodeStatusEvent(raw)
	require.True(t, ok)
	assert.Equal(t, StatusEventKindAppend, event.Kind)
	assert.Equal(t, LogEmojiNone, event.Emoji)
}

func TestStatusEmitterEmitEvent(t *testing.T) {
	ch := make(chan string, 1)
	emitter := NewStatusEmitter(ch, nil, nil)

	emitter.EmitEvent(
		OutputLevelInfo,
		StatusEventKindStart,
		"test-op",
		LogEmojiBrowser,
		"starting",
	)

	event, ok := DecodeStatusEvent(<-ch)
	require.True(t, ok)
	assert.Equal(t, StatusEventKindStart, event.Kind)
	assert.Equal(t, "test-op", event.OperationID)
}

func TestStatusEmitterBridgeChannel(t *testing.T) {
	target := make(chan string, 1)
	emitter := NewStatusEmitter(target, nil, nil)
	bridge, done := emitter.BridgeChannel()

	*bridge <- "raw"
	done()

	assert.Equal(t, "raw", <-target)
}
