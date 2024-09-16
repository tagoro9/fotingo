package commandruntime

import (
	"encoding/json"
	"strings"
)

// OutputLevel controls status-message visibility granularity.
type OutputLevel int

const (
	OutputLevelInfo OutputLevel = iota
	OutputLevelVerbose
	OutputLevelDebug
)

// LogEmoji is the icon associated with a status message.
type LogEmoji string

const (
	LogEmojiNone      LogEmoji = ""
	LogEmojiSuccess   LogEmoji = "✨"
	LogEmojiRocket    LogEmoji = "🚀"
	LogEmojiWarning   LogEmoji = "⚠"
	LogEmojiError     LogEmoji = "💥"
	LogEmojiLink      LogEmoji = "🔗"
	LogEmojiBrowser   LogEmoji = "🌍"
	LogEmojiPackage   LogEmoji = "📦"
	LogEmojiBookmark  LogEmoji = "🔖"
	LogEmojiCheck     LogEmoji = "✅"
	LogEmojiUser      LogEmoji = "👤"
	LogEmojiGit       LogEmoji = "🌿"
	LogEmojiInfo      LogEmoji = "ℹ"
	LogEmojiPrompt    LogEmoji = "❓"
	LogEmojiDebug     LogEmoji = "🔍"
	LogEmojiVerbose   LogEmoji = "›"
	LogEmojiBranch    LogEmoji = "🌱"
	LogEmojiJira      LogEmoji = "🎫"
	LogEmojiGitHub    LogEmoji = "🐙"
	LogEmojiReview    LogEmoji = "👀"
	LogEmojiLabel     LogEmoji = "🏷"
	LogEmojiComment   LogEmoji = "💬"
	LogEmojiIssue     LogEmoji = "🧩"
	LogEmojiRelease   LogEmoji = "📦"
	LogEmojiChangelog LogEmoji = "📝"
	LogEmojiCommit    LogEmoji = "🧾"
	LogEmojiProgress  LogEmoji = "⏳"
	LogEmojiAuth      LogEmoji = "🔐"
	LogEmojiConfigure LogEmoji = "⚙"
	LogEmojiClipboard LogEmoji = "📋"
	LogEmojiReviewer  LogEmoji = "👥"
	LogEmojiMilestone LogEmoji = "🎯"
	LogEmojiLifecycle LogEmoji = "🔄"
	LogEmojiExternal  LogEmoji = "↗"
	LogEmojiTerminal  LogEmoji = "🖥"
	LogEmojiOperation LogEmoji = "⚡"
)

// DefaultEmojiForLevel returns the default emoji for a level when no explicit icon is provided.
func DefaultEmojiForLevel(level OutputLevel) LogEmoji {
	switch level {
	case OutputLevelVerbose:
		return LogEmojiVerbose
	case OutputLevelDebug:
		return LogEmojiDebug
	default:
		return LogEmojiNone
	}
}

// StatusEventKind identifies the lifecycle stage of a status event.
type StatusEventKind string

const (
	StatusEventKindAppend  StatusEventKind = "append"
	StatusEventKindStart   StatusEventKind = "start"
	StatusEventKindUpdate  StatusEventKind = "update"
	StatusEventKindSuccess StatusEventKind = "success"
	StatusEventKindError   StatusEventKind = "error"
)

// StatusEventPrefix marks encoded status events in the shared status channel.
const StatusEventPrefix = "__fotingo_status_event__:"

// StatusEvent is the structured representation of status updates crossing channels.
type StatusEvent struct {
	Kind        StatusEventKind `json:"kind"`
	OperationID string          `json:"operation_id,omitempty"`
	Message     string          `json:"message"`
	Level       OutputLevel     `json:"level,omitempty"`
	Emoji       LogEmoji        `json:"emoji,omitempty"`
}

// EncodeStatusEvent serializes a status event with the shared envelope prefix.
func EncodeStatusEvent(
	kind StatusEventKind,
	operationID string,
	message string,
	level OutputLevel,
	emoji LogEmoji,
) string {
	payload, err := json.Marshal(StatusEvent{
		Kind:        kind,
		OperationID: operationID,
		Message:     message,
		Level:       level,
		Emoji:       emoji,
	})
	if err != nil {
		return message
	}

	return StatusEventPrefix + string(payload)
}

// DecodeStatusEvent parses an encoded status event from a raw string payload.
func DecodeStatusEvent(raw string) (StatusEvent, bool) {
	if !strings.HasPrefix(raw, StatusEventPrefix) {
		return StatusEvent{}, false
	}

	var event StatusEvent
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, StatusEventPrefix)), &event); err != nil {
		return StatusEvent{}, false
	}

	if event.Kind == "" {
		return StatusEvent{}, false
	}

	if event.Kind != StatusEventKindAppend && strings.TrimSpace(event.OperationID) == "" {
		return StatusEvent{}, false
	}

	if !IsKnownOutputLevel(event.Level) {
		event.Level = OutputLevelInfo
	}

	return event, true
}

// OperationIDOrDefault normalizes operation IDs used by event streams.
func OperationIDOrDefault(operationID string) string {
	trimmed := strings.TrimSpace(operationID)
	if trimmed != "" {
		return trimmed
	}

	return "default"
}

// IsKnownOutputLevel reports whether a level belongs to the known output-level set.
func IsKnownOutputLevel(level OutputLevel) bool {
	switch level {
	case OutputLevelInfo, OutputLevelVerbose, OutputLevelDebug:
		return true
	default:
		return false
	}
}
