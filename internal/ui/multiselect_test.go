package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestReviewers() []PickerItem {
	return []PickerItem{
		{ID: "alice", Label: "Alice Smith"},
		{ID: "bob", Label: "Bob Jones"},
		{ID: "charlie", Label: "Charlie Brown"},
		{ID: "diana", Label: "Diana Prince"},
		{ID: "edward", Label: "Edward Elric"},
	}
}

func TestNewMultiSelect(t *testing.T) {
	t.Parallel()

	t.Run("creates multi-select with defaults", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect()
		assert.True(t, m.showSearch)
		assert.Equal(t, 10, m.height)
		assert.Empty(t, m.items)
	})

	t.Run("with title option", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect(WithMultiSelectTitle("Select Reviewers"))
		assert.Equal(t, "Select Reviewers", m.title)
	})

	t.Run("with items option", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))
		assert.Len(t, m.items, 5)
	})

	t.Run("with height option", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect(WithMultiSelectHeight(3))
		assert.Equal(t, 3, m.height)
	})

	t.Run("with search disabled", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect(WithMultiSelectSearch(false))
		assert.False(t, m.showSearch)
	})

	t.Run("with minimum selection", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect(WithMultiSelectMinimum(2))
		assert.Equal(t, 2, m.minSelect)
	})

	t.Run("with maximum selection", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect(WithMultiSelectMaximum(3))
		assert.Equal(t, 3, m.maxSelect)
	})

	t.Run("with preselected items", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(
			WithMultiSelectItems(items),
			WithPreselected([]string{"alice", "charlie"}),
		)
		selected := m.SelectedItems()
		assert.Len(t, selected, 2)
	})

	t.Run("with custom styles", func(t *testing.T) {
		t.Parallel()
		styles := NewStyles(LightScheme())
		m := NewMultiSelect(WithMultiSelectStyles(styles))
		assert.NotNil(t, m.styles.CheckboxChecked)
	})
}

func TestMultiSelectUpdate(t *testing.T) {
	t.Parallel()

	t.Run("handles space key to toggle", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))

		// Toggle first item
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
		assert.True(t, updated.items[0].Selected)

		// Toggle again
		updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeySpace})
		assert.False(t, updated.items[0].Selected)
	})

	t.Run("handles navigation keys", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		assert.Equal(t, 1, updated.cursor)

		updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyUp})
		assert.Equal(t, 0, updated.cursor)
	})

	t.Run("handles enter key for confirmation", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))
		m.items[0].Selected = true
		m.items[2].Selected = true

		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.True(t, updated.Submitted())
		assert.NotNil(t, cmd)

		msg := cmd()
		result, ok := msg.(MultiSelectResultMsg)
		assert.True(t, ok)
		assert.Len(t, result.Items, 2)
	})

	t.Run("respects minimum selection", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(
			WithMultiSelectItems(items),
			WithMultiSelectMinimum(2),
		)
		m.items[0].Selected = true // Only 1 selected

		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.False(t, updated.Submitted())
		assert.Nil(t, cmd)
	})

	t.Run("respects maximum selection", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(
			WithMultiSelectItems(items),
			WithMultiSelectMaximum(2),
		)
		m.items[0].Selected = true
		m.items[1].Selected = true

		// Try to select a third item
		m.cursor = 2
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
		assert.False(t, updated.items[2].Selected)
	})

	t.Run("handles escape key for cancellation", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect(WithMultiSelectItems(createTestReviewers()))

		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
		assert.True(t, updated.Cancelled())
		assert.NotNil(t, cmd)

		msg := cmd()
		_, ok := msg.(MultiSelectCancelMsg)
		assert.True(t, ok)
	})

	t.Run("handles ctrl+c for cancellation", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect(WithMultiSelectItems(createTestReviewers()))

		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		assert.True(t, updated.Cancelled())
		assert.NotNil(t, cmd)
	})

	t.Run("handles ctrl+a to select all", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
		selected := updated.SelectedItems()
		assert.Len(t, selected, len(items))
	})

	t.Run("ctrl+a respects max selection", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(
			WithMultiSelectItems(items),
			WithMultiSelectMaximum(2),
		)

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
		selected := updated.SelectedItems()
		assert.Len(t, selected, 2)
	})
}

func TestMultiSelectFilter(t *testing.T) {
	t.Parallel()

	t.Run("filters by substring match", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))
		m.search.SetValue("brown")
		m.filter()

		// Should find "Charlie Brown" first
		assert.True(t, len(m.filtered) > 0)
		assert.Equal(t, "charlie", m.items[m.filtered[0]].ID)
	})

	t.Run("filters by ID", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))
		m.search.SetValue("alice")
		m.filter()

		// Should find alice first due to exact ID match
		assert.True(t, len(m.filtered) > 0)
		assert.Equal(t, "alice", m.items[m.filtered[0]].ID)
	})

	t.Run("handles empty query", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))
		m.search.SetValue("")
		m.filter()

		assert.Len(t, m.filtered, len(items))
	})
}

func TestMultiSelectView(t *testing.T) {
	t.Parallel()

	t.Run("renders title with selection count", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(
			WithMultiSelectItems(items),
			WithMultiSelectTitle("Reviewers"),
		)
		m.items[0].Selected = true
		m.items[1].Selected = true

		view := m.View()
		assert.Contains(t, view, "Reviewers")
		assert.Contains(t, view, "2 selected")
	})

	t.Run("renders checkboxes", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))
		m.items[0].Selected = true

		view := m.View()
		assert.Contains(t, view, Icons.Selected)
		assert.Contains(t, view, Icons.Checkbox)
	})

	t.Run("renders items", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))

		view := m.View()
		assert.Contains(t, view, "Alice Smith")
		assert.Contains(t, view, "Bob Jones")
	})

	t.Run("renders help text", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect()
		view := m.View()
		assert.Contains(t, view, "toggle")
		assert.Contains(t, view, "confirm")
	})

	t.Run("renders minimum warning", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(
			WithMultiSelectItems(items),
			WithMultiSelectMinimum(2),
		)

		view := m.View()
		assert.Contains(t, view, "Select at least 2")
	})

	t.Run("renders empty state", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect()
		view := m.View()
		assert.Contains(t, view, "No matching items")
	})

	t.Run("selected non-cursor row keeps label alignment", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		styles := NewStyles(ColorScheme{})

		base := NewMultiSelect(
			WithMultiSelectItems(items),
			WithMultiSelectStyles(styles),
		)
		base.cursor = 1
		baseLine := findLineContaining(base.View(), "Alice Smith")
		baseLabelIndex := strings.Index(baseLine, "Alice Smith")
		require.GreaterOrEqual(t, baseLabelIndex, 0)

		selected := NewMultiSelect(
			WithMultiSelectItems(items),
			WithMultiSelectStyles(styles),
		)
		selected.cursor = 1
		selected.items[0].Selected = true
		selectedLine := findLineContaining(selected.View(), "Alice Smith")
		selectedLabelIndex := strings.Index(selectedLine, "Alice Smith")
		require.GreaterOrEqual(t, selectedLabelIndex, 0)

		assert.Equal(t, baseLabelIndex, selectedLabelIndex)
	})

	t.Run("cursor row keeps icon alignment", func(t *testing.T) {
		t.Parallel()
		styles := NewStyles(ColorScheme{})
		items := []PickerItem{
			{ID: "codex", Label: "Codex", Icon: "⚙"},
			{ID: "cursor", Label: "Cursor", Icon: "⚙"},
		}

		nonCursor := NewMultiSelect(
			WithMultiSelectItems(items),
			WithMultiSelectStyles(styles),
		)
		nonCursor.cursor = 1
		nonCursorLine := findLineContaining(nonCursor.View(), "Codex")
		nonCursorIconIndex := strings.Index(nonCursorLine, "⚙")
		require.GreaterOrEqual(t, nonCursorIconIndex, 0)

		cursor := NewMultiSelect(
			WithMultiSelectItems(items),
			WithMultiSelectStyles(styles),
		)
		cursor.cursor = 0
		cursorLine := findLineContaining(cursor.View(), "Codex")
		cursorIconIndex := strings.Index(cursorLine, "⚙")
		require.GreaterOrEqual(t, cursorIconIndex, 0)

		assert.Equal(t, nonCursorIconIndex, cursorIconIndex)
	})
}

func findLineContaining(view, needle string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func TestMultiSelectMethods(t *testing.T) {
	t.Parallel()

	t.Run("SelectedItems returns selected", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))
		m.items[0].Selected = true
		m.items[2].Selected = true

		selected := m.SelectedItems()
		assert.Len(t, selected, 2)
		assert.Equal(t, "alice", selected[0].ID)
		assert.Equal(t, "charlie", selected[1].ID)
	})

	t.Run("Items returns all items", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))
		assert.Equal(t, items, m.Items())
	})

	t.Run("SetItems updates items", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect()
		items := createTestReviewers()
		m.SetItems(items)
		assert.Len(t, m.items, 5)
	})

	t.Run("Submitted returns correct value", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect()
		assert.False(t, m.Submitted())
		m.submitted = true
		assert.True(t, m.Submitted())
	})

	t.Run("Cancelled returns correct value", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect()
		assert.False(t, m.Cancelled())
		m.cancelled = true
		assert.True(t, m.Cancelled())
	})
}

func TestMultiSelectInit(t *testing.T) {
	t.Parallel()

	t.Run("with search enabled", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect(WithMultiSelectSearch(true))
		cmd := m.Init()
		assert.NotNil(t, cmd)
	})

	t.Run("with search disabled", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect(WithMultiSelectSearch(false))
		cmd := m.Init()
		assert.Nil(t, cmd)
	})
}

func TestMultiSelectSelectedCount(t *testing.T) {
	t.Parallel()

	items := createTestReviewers()
	m := NewMultiSelect(WithMultiSelectItems(items))

	assert.Equal(t, 0, m.selectedCount())

	m.items[0].Selected = true
	assert.Equal(t, 1, m.selectedCount())

	m.items[2].Selected = true
	m.items[4].Selected = true
	assert.Equal(t, 3, m.selectedCount())
}

func TestMultiSelectScrolling(t *testing.T) {
	t.Parallel()

	t.Run("handles page up and page down", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items), WithMultiSelectHeight(2))

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		assert.Equal(t, 2, m.cursor)

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		assert.Equal(t, 0, m.cursor)
	})

	t.Run("handles home and end keys", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
		assert.Equal(t, len(items)-1, m.cursor)

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
		assert.Equal(t, 0, m.cursor)
	})

	t.Run("handles ctrl+n and ctrl+p", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
		assert.Equal(t, 1, m.cursor)

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
		assert.Equal(t, 0, m.cursor)
	})

	t.Run("page down with boundary", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items), WithMultiSelectHeight(2))
		m.cursor = len(items) - 2

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		assert.Equal(t, len(items)-1, m.cursor)
	})

	t.Run("scrolls down when moving past visible area", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items), WithMultiSelectHeight(2))
		m.cursor = 0
		m.offset = 0

		// Move down multiple times to go beyond visible area
		for i := 0; i < 3; i++ {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		}
		// Should have scrolled
		assert.True(t, m.offset > 0)
	})

	t.Run("scrolls up when moving before visible area", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items), WithMultiSelectHeight(2))
		m.cursor = 3
		m.offset = 2

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
		assert.Equal(t, 2, m.cursor)
		assert.Equal(t, 2, m.offset)
	})
}

func TestMultiSelectKeyHandlers(t *testing.T) {
	t.Parallel()

	t.Run("handles 'a' key to select all when no search", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items), WithMultiSelectSearch(false))

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		selected := updated.SelectedItems()
		assert.Len(t, selected, len(items))
	})

	t.Run("handles 'a' key when search is empty", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))
		m.search.SetValue("")

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		selected := updated.SelectedItems()
		assert.Len(t, selected, len(items))
	})

	t.Run("'a' key respects max selection", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(
			WithMultiSelectItems(items),
			WithMultiSelectMaximum(2),
			WithMultiSelectSearch(false),
		)

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		selected := updated.SelectedItems()
		assert.Len(t, selected, 2)
	})

	t.Run("space on empty filtered list", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect()
		// No items, filtered is empty
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
		// Should not panic
		assert.NotNil(t, updated)
	})

	t.Run("space when cursor beyond filtered", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))
		m.cursor = len(m.filtered) + 10 // Beyond bounds

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
		// Should not panic
		assert.NotNil(t, updated)
	})
}

func TestMultiSelectWrapper(t *testing.T) {
	t.Parallel()

	t.Run("wrapper Init calls model Init", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect()
		w := &multiSelectWrapper{model: m}
		cmd := w.Init()
		assert.NotNil(t, cmd)
	})

	t.Run("wrapper Update quits on MultiSelectResultMsg", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect()
		w := &multiSelectWrapper{model: m}
		_, cmd := w.Update(MultiSelectResultMsg{Items: []PickerItem{}})
		assert.NotNil(t, cmd)
	})

	t.Run("wrapper Update quits on MultiSelectCancelMsg", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect()
		w := &multiSelectWrapper{model: m}
		_, cmd := w.Update(MultiSelectCancelMsg{})
		assert.NotNil(t, cmd)
	})

	t.Run("wrapper Update propagates to model", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items))
		w := &multiSelectWrapper{model: m}
		// Send a key down to move selection
		_, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
		assert.Equal(t, 1, w.model.cursor)
	})

	t.Run("wrapper View renders model", func(t *testing.T) {
		t.Parallel()
		m := NewMultiSelect(WithMultiSelectTitle("Test"))
		w := &multiSelectWrapper{model: m}
		view := w.View()
		assert.Contains(t, view, "Test")
	})
}

func TestMultiSelectViewEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("renders with items above visible window", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items), WithMultiSelectHeight(2))
		m.offset = 2

		view := m.View()
		assert.Contains(t, view, "more items above")
	})

	t.Run("renders with items below visible window", func(t *testing.T) {
		t.Parallel()
		items := createTestReviewers()
		m := NewMultiSelect(WithMultiSelectItems(items), WithMultiSelectHeight(2))

		view := m.View()
		assert.Contains(t, view, "more items below")
	})

	t.Run("renders item with icon", func(t *testing.T) {
		t.Parallel()
		items := []PickerItem{
			{ID: "1", Label: "Item 1", Icon: Icons.Bug},
		}
		m := NewMultiSelect(WithMultiSelectItems(items))
		view := m.View()
		assert.Contains(t, view, Icons.Bug)
	})

	t.Run("renders item with detail", func(t *testing.T) {
		t.Parallel()
		items := []PickerItem{
			{ID: "1", Label: "Item 1", Detail: "Important"},
		}
		m := NewMultiSelect(WithMultiSelectItems(items))
		view := m.View()
		assert.Contains(t, view, "Important")
	})
}

func TestMultiSelectFilterFuzzy(t *testing.T) {
	t.Parallel()

	t.Run("fuzzy matches with levenshtein", func(t *testing.T) {
		t.Parallel()
		items := []PickerItem{
			{ID: "1", Label: "Alice Smith"},
			{ID: "2", Label: "Bob Johnson"},
		}
		m := NewMultiSelect(WithMultiSelectItems(items))
		m.search.SetValue("Alic") // Close typo
		m.filter()

		// Should still find Alice with fuzzy matching (distance 1, threshold = len("alic")/2+2 = 4)
		assert.True(t, len(m.filtered) > 0)
	})

	t.Run("filters out items beyond threshold", func(t *testing.T) {
		t.Parallel()
		items := []PickerItem{
			{ID: "1", Label: "Alice"},
			{ID: "2", Label: "Zebra"},
		}
		m := NewMultiSelect(WithMultiSelectItems(items))
		m.search.SetValue("xyz") // Very different
		m.filter()

		// Should have very few or no matches
		assert.True(t, len(m.filtered) < len(items))
	})
}
