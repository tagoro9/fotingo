package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserModel_EnterShowsDetailAndEscapeReturnsToList(t *testing.T) {
	t.Parallel()

	model := newBrowserModel(
		"Browser",
		[]PickerItem{{ID: "one", Label: "One", Value: "value-one"}},
		func(item PickerItem) string { return item.Value.(string) },
	)

	updated, _ := model.Update(PickerSelectMsg{Item: PickerItem{ID: "one", Label: "One", Value: "value-one"}})
	browser, ok := updated.(*browserModel)
	require.True(t, ok)
	assert.True(t, browser.inDetail)
	assert.Equal(t, "One", browser.detailTitle)
	assert.Equal(t, "value-one", browser.detailBody)

	updated, _ = browser.Update(tea.KeyMsg{Type: tea.KeyEscape})
	browser, ok = updated.(*browserModel)
	require.True(t, ok)
	assert.False(t, browser.inDetail)
	assert.Empty(t, browser.detailBody)
}

func TestBrowserModel_EscapeOnListCancels(t *testing.T) {
	t.Parallel()

	model := newBrowserModel(
		"Browser",
		[]PickerItem{{ID: "one", Label: "One"}},
		nil,
	)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	browser, ok := updated.(*browserModel)
	require.True(t, ok)
	require.NotNil(t, cmd)

	msg := cmd()
	require.IsType(t, PickerCancelMsg{}, msg)

	updated, quitCmd := browser.Update(msg)
	browser, ok = updated.(*browserModel)
	require.True(t, ok)
	assert.True(t, browser.cancelled)
	require.NotNil(t, quitCmd)
	assert.IsType(t, tea.QuitMsg{}, quitCmd())
}

func TestHardWrapText(t *testing.T) {
	t.Parallel()

	input := strings.Repeat("a", 11) + "\n" + strings.Repeat("b", 4)
	wrapped := hardWrapText(input, 5)
	assert.Equal(t, "aaaaa\naaaaa\na\nbbbb", wrapped)
}

func TestBrowserModel_DetailScrolling(t *testing.T) {
	t.Parallel()

	model := newBrowserModel(
		"Browser",
		[]PickerItem{{ID: "one", Label: "One"}},
		func(item PickerItem) string {
			return strings.Repeat("line\n", 60)
		},
	)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	browser, ok := updated.(*browserModel)
	require.True(t, ok)

	updated, _ = browser.Update(PickerSelectMsg{Item: PickerItem{ID: "one", Label: "One"}})
	browser, ok = updated.(*browserModel)
	require.True(t, ok)
	require.True(t, browser.inDetail)
	assert.Equal(t, 0, browser.detailOffset)

	updated, _ = browser.Update(tea.KeyMsg{Type: tea.KeyDown})
	browser, ok = updated.(*browserModel)
	require.True(t, ok)
	assert.Equal(t, 1, browser.detailOffset)

	updated, _ = browser.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	browser, ok = updated.(*browserModel)
	require.True(t, ok)
	assert.Greater(t, browser.detailOffset, 1)

	updated, _ = browser.Update(tea.KeyMsg{Type: tea.KeyHome})
	browser, ok = updated.(*browserModel)
	require.True(t, ok)
	assert.Equal(t, 0, browser.detailOffset)

	updated, _ = browser.Update(tea.KeyMsg{Type: tea.KeyEnd})
	browser, ok = updated.(*browserModel)
	require.True(t, ok)
	assert.Greater(t, browser.detailOffset, 0)
}
