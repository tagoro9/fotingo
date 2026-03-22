package ui

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func textKey(text string) tea.KeyPressMsg {
	r, _ := utf8.DecodeRuneInString(text)
	return tea.KeyPressMsg{Text: text, Code: r}
}
