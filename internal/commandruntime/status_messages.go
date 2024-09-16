package commandruntime

import tea "github.com/charmbracelet/bubbletea"

// StatusMsg carries one status-channel update to the status model.
type StatusMsg string

// DoneMsg signals that the status model should stop rendering.
type DoneMsg bool

// UpdateStatus creates a status update message for the shared status model.
func UpdateStatus(status string) tea.Msg {
	return StatusMsg(status)
}

// FinishProcess creates a completion message for the shared status model.
func FinishProcess() tea.Msg {
	return DoneMsg(true)
}
