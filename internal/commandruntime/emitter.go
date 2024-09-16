package commandruntime

import "sync"

// ShouldEmitLevelFunc decides whether a message should be emitted for a level.
type ShouldEmitLevelFunc func(level OutputLevel) bool

// StatusEmitter writes structured status events into a shared channel.
type StatusEmitter struct {
	statusCh    chan<- string
	shouldEmit  ShouldEmitLevelFunc
	redactDebug func(message string) string
}

// NewStatusEmitter creates a StatusEmitter with visibility callbacks.
func NewStatusEmitter(
	statusCh chan<- string,
	shouldEmit ShouldEmitLevelFunc,
	redactDebug func(message string) string,
) StatusEmitter {
	return StatusEmitter{
		statusCh:    statusCh,
		shouldEmit:  shouldEmit,
		redactDebug: redactDebug,
	}
}

// BridgeChannel returns a temporary status channel that forwards raw messages
// to this emitter destination. Call the returned cleanup function to close the
// bridge and wait for forwarding to finish.
func (e StatusEmitter) BridgeChannel() (*chan string, func()) {
	bridge := make(chan string)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for msg := range bridge {
			if e.statusCh == nil {
				continue
			}
			e.statusCh <- msg
		}
	}()

	var once sync.Once
	return &bridge, func() {
		once.Do(func() {
			close(bridge)
			<-done
		})
	}
}

// Emit appends a status message event.
func (e StatusEmitter) Emit(level OutputLevel, emoji LogEmoji, message string) {
	if e.statusCh == nil {
		return
	}
	if e.shouldEmit != nil && !e.shouldEmit(level) {
		return
	}

	if level == OutputLevelDebug && e.redactDebug != nil {
		message = e.redactDebug(message)
	}

	resolvedEmoji := emoji
	if resolvedEmoji == LogEmojiNone {
		resolvedEmoji = DefaultEmojiForLevel(level)
	}

	e.statusCh <- EncodeStatusEvent(
		StatusEventKindAppend,
		OperationIDOrDefault("append"),
		message,
		level,
		resolvedEmoji,
	)
}

// EmitEvent emits a lifecycle status event for an operation.
func (e StatusEmitter) EmitEvent(
	level OutputLevel,
	kind StatusEventKind,
	operationID string,
	emoji LogEmoji,
	message string,
) {
	if e.statusCh == nil {
		return
	}
	if e.shouldEmit != nil && !e.shouldEmit(level) {
		return
	}

	resolvedEmoji := emoji
	if resolvedEmoji == LogEmojiNone {
		resolvedEmoji = DefaultEmojiForLevel(level)
	}

	e.statusCh <- EncodeStatusEvent(
		kind,
		OperationIDOrDefault(operationID),
		message,
		level,
		resolvedEmoji,
	)
}
