package commandruntime

import (
	"fmt"

	"github.com/tagoro9/fotingo/internal/i18n"
)

// LocalizeFunc renders translation keys into localized text.
type LocalizeFunc func(key i18n.Key, args ...any) string

// LocalizedEmitter wraps StatusEmitter with localization-aware helpers used by command flows.
type LocalizedEmitter struct {
	emitter  StatusEmitter
	localize LocalizeFunc
}

// NewLocalizedEmitter creates a localization-aware status emitter for command workflows.
func NewLocalizedEmitter(
	statusCh chan<- string,
	shouldEmit ShouldEmitLevelFunc,
	localize LocalizeFunc,
) LocalizedEmitter {
	return LocalizedEmitter{
		emitter:  NewStatusEmitter(statusCh, shouldEmit, RedactSensitiveDebug),
		localize: localize,
	}
}

// BridgeChannel returns a temporary status channel that forwards raw status
// messages to this emitter destination. Call the returned cleanup function to
// close the bridge and wait for forwarding to finish.
func (e LocalizedEmitter) BridgeChannel() (*chan string, func()) {
	return e.emitter.BridgeChannel()
}

// Info emits an info-level message from a localization key.
func (e LocalizedEmitter) Info(emoji LogEmoji, key i18n.Key, args ...any) {
	e.InfoRaw(emoji, e.render(key, args...))
}

// InfoRaw emits an info-level raw message.
func (e LocalizedEmitter) InfoRaw(emoji LogEmoji, message string) {
	e.emitter.Emit(OutputLevelInfo, emoji, message)
}

// Verbose emits a verbose-level message from a localization key.
func (e LocalizedEmitter) Verbose(key i18n.Key, args ...any) {
	e.VerboseRaw(e.render(key, args...))
}

// VerboseRaw emits a verbose-level raw message.
func (e LocalizedEmitter) VerboseRaw(message string) {
	e.emitter.Emit(OutputLevelVerbose, LogEmojiNone, message)
}

// Debug emits a debug-level message from a localization key.
func (e LocalizedEmitter) Debug(key i18n.Key, args ...any) {
	e.DebugRaw(e.render(key, args...))
}

// DebugRaw emits a debug-level raw message.
func (e LocalizedEmitter) DebugRaw(message string) {
	e.emitter.Emit(OutputLevelDebug, LogEmojiNone, message)
}

// Debugf emits a formatted debug-level message.
func (e LocalizedEmitter) Debugf(format string, args ...any) {
	e.DebugRaw(fmt.Sprintf(format, args...))
}

// Start emits a start lifecycle event from a localization key.
func (e LocalizedEmitter) Start(operationID string, emoji LogEmoji, key i18n.Key, args ...any) {
	e.StartRaw(operationID, emoji, e.render(key, args...))
}

// StartRaw emits a raw start lifecycle event.
func (e LocalizedEmitter) StartRaw(operationID string, emoji LogEmoji, message string) {
	e.emitter.EmitEvent(OutputLevelInfo, StatusEventKindStart, operationID, emoji, message)
}

// Update emits an update lifecycle event from a localization key.
func (e LocalizedEmitter) Update(operationID string, emoji LogEmoji, key i18n.Key, args ...any) {
	e.UpdateRaw(operationID, emoji, e.render(key, args...))
}

// UpdateRaw emits a raw update lifecycle event.
func (e LocalizedEmitter) UpdateRaw(operationID string, emoji LogEmoji, message string) {
	e.emitter.EmitEvent(OutputLevelInfo, StatusEventKindUpdate, operationID, emoji, message)
}

// Success emits a success lifecycle event from a localization key.
func (e LocalizedEmitter) Success(operationID string, emoji LogEmoji, key i18n.Key, args ...any) {
	e.SuccessRaw(operationID, emoji, e.render(key, args...))
}

// SuccessRaw emits a raw success lifecycle event.
func (e LocalizedEmitter) SuccessRaw(operationID string, emoji LogEmoji, message string) {
	e.emitter.EmitEvent(OutputLevelInfo, StatusEventKindSuccess, operationID, emoji, message)
}

// Error emits an error lifecycle event from a localization key.
func (e LocalizedEmitter) Error(operationID string, emoji LogEmoji, key i18n.Key, args ...any) {
	e.ErrorRaw(operationID, emoji, e.render(key, args...))
}

// ErrorRaw emits a raw error lifecycle event.
func (e LocalizedEmitter) ErrorRaw(operationID string, emoji LogEmoji, message string) {
	e.emitter.EmitEvent(OutputLevelInfo, StatusEventKindError, operationID, emoji, message)
}

func (e LocalizedEmitter) render(key i18n.Key, args ...any) string {
	if e.localize != nil {
		return e.localize(key, args...)
	}

	return i18n.T(key, args...)
}
