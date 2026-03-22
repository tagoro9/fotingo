package commands

import (
	tea "charm.land/bubbletea/v2"
	"github.com/tagoro9/fotingo/internal/commandruntime"
)

// Custom message types for Bubble Tea apps across commands

// statusMsg represents a status update message for the TUI
type statusMsg string

// doneMsg signals that the process has completed
type doneMsg bool

// updateStatus creates a status update command
func updateStatus(status string) tea.Msg {
	return statusMsg(status)
}

// finishProcess signals completion
func finishProcess() tea.Msg {
	return doneMsg(true)
}

type statusEventKind = commandruntime.StatusEventKind

const (
	statusEventKindAppend  statusEventKind = commandruntime.StatusEventKindAppend
	statusEventKindStart   statusEventKind = commandruntime.StatusEventKindStart
	statusEventKindUpdate  statusEventKind = commandruntime.StatusEventKindUpdate
	statusEventKindSuccess statusEventKind = commandruntime.StatusEventKindSuccess
	statusEventKindError   statusEventKind = commandruntime.StatusEventKindError
)

type statusEvent = commandruntime.StatusEvent

func decodeStatusEvent(raw string) (statusEvent, bool) {
	event, ok := commandruntime.DecodeStatusEvent(raw)
	if !ok {
		return statusEvent{}, false
	}
	return event, true
}

func isKnownOutputLevel(level commandruntime.OutputLevel) bool {
	return commandruntime.IsKnownOutputLevel(level)
}
