package testutil

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// CtrlKey constructs a Ctrl+<key> press message for Bubble Tea tests.
func CtrlKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
}

// ViewString strips ANSI escapes from a rendered Bubble Tea view.
func ViewString(view tea.View) string {
	return ansi.Strip(view.Content)
}
