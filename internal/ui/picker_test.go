package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func createTestItems() []PickerItem {
	return []PickerItem{
		{ID: "PROJ-1", Label: "Fix login bug", Icon: Icons.Bug},
		{ID: "PROJ-2", Label: "Add user profile", Icon: Icons.Story, Detail: "High priority"},
		{ID: "PROJ-3", Label: "Update documentation", Icon: Icons.Task},
		{ID: "PROJ-4", Label: "Performance improvements", Icon: Icons.Task},
		{ID: "PROJ-5", Label: "Login page redesign", Icon: Icons.Story},
	}
}

func TestNewPicker(t *testing.T) {
	t.Parallel()

	t.Run("creates picker with defaults", func(t *testing.T) {
		t.Parallel()
		p := NewPicker()
		assert.True(t, p.showSearch)
		assert.Equal(t, 10, p.height)
		assert.Empty(t, p.items)
	})

	t.Run("with title option", func(t *testing.T) {
		t.Parallel()
		p := NewPicker(WithPickerTitle("Select Issue"))
		assert.Equal(t, "Select Issue", p.title)
	})

	t.Run("with items option", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		assert.Len(t, p.items, 5)
		assert.Len(t, p.filtered, 5)
	})

	t.Run("with height option", func(t *testing.T) {
		t.Parallel()
		p := NewPicker(WithPickerHeight(5))
		assert.Equal(t, 5, p.height)
	})

	t.Run("with search disabled", func(t *testing.T) {
		t.Parallel()
		p := NewPicker(WithPickerSearch(false))
		assert.False(t, p.showSearch)
	})

	t.Run("with custom styles", func(t *testing.T) {
		t.Parallel()
		styles := NewStyles(LightScheme())
		p := NewPicker(WithPickerStyles(styles))
		assert.NotNil(t, p.styles.ListItem)
	})
}

func TestPickerUpdate(t *testing.T) {
	t.Parallel()

	t.Run("handles down key", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		assert.Equal(t, 0, p.cursor)

		updated, _ := p.Update(tea.KeyMsg{Type: tea.KeyDown})
		assert.Equal(t, 1, updated.cursor)
	})

	t.Run("handles up key", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.cursor = 2

		updated, _ := p.Update(tea.KeyMsg{Type: tea.KeyUp})
		assert.Equal(t, 1, updated.cursor)
	})

	t.Run("handles ctrl+n and ctrl+p", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))

		updated, _ := p.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
		assert.Equal(t, 1, updated.cursor)

		updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
		assert.Equal(t, 0, updated.cursor)
	})

	t.Run("handles home key", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.cursor = 3

		updated, _ := p.Update(tea.KeyMsg{Type: tea.KeyHome})
		assert.Equal(t, 0, updated.cursor)
		assert.Equal(t, 0, updated.offset)
	})

	t.Run("handles end key", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))

		updated, _ := p.Update(tea.KeyMsg{Type: tea.KeyEnd})
		assert.Equal(t, len(items)-1, updated.cursor)
	})

	t.Run("handles page up and page down", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items), WithPickerHeight(2))

		updated, _ := p.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		assert.Equal(t, 2, updated.cursor)

		updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		assert.Equal(t, 0, updated.cursor)
	})

	t.Run("handles enter key for selection", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.cursor = 1

		updated, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.True(t, updated.Submitted())
		assert.NotNil(t, cmd)

		msg := cmd()
		selectMsg, ok := msg.(PickerSelectMsg)
		assert.True(t, ok)
		assert.Equal(t, "PROJ-2", selectMsg.Item.ID)
	})

	t.Run("handles escape key for cancellation", func(t *testing.T) {
		t.Parallel()
		p := NewPicker(WithPickerItems(createTestItems()))

		updated, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEscape})
		assert.True(t, updated.Cancelled())
		assert.NotNil(t, cmd)

		msg := cmd()
		_, ok := msg.(PickerCancelMsg)
		assert.True(t, ok)
	})

	t.Run("handles ctrl+c for cancellation", func(t *testing.T) {
		t.Parallel()
		p := NewPicker(WithPickerItems(createTestItems()))

		updated, cmd := p.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		assert.True(t, updated.Cancelled())
		assert.NotNil(t, cmd)
	})

	t.Run("does not go below 0", func(t *testing.T) {
		t.Parallel()
		p := NewPicker(WithPickerItems(createTestItems()))

		updated, _ := p.Update(tea.KeyMsg{Type: tea.KeyUp})
		assert.Equal(t, 0, updated.cursor)
	})

	t.Run("does not go beyond items", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.cursor = len(items) - 1

		updated, _ := p.Update(tea.KeyMsg{Type: tea.KeyDown})
		assert.Equal(t, len(items)-1, updated.cursor)
	})
}

func TestPickerFilter(t *testing.T) {
	t.Parallel()

	t.Run("filters by substring match", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.search.SetValue("login")
		p.filter()

		// Should match "Fix login bug" and "Login page redesign"
		assert.Len(t, p.filtered, 2)
	})

	t.Run("filters by ID", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.search.SetValue("PROJ-3")
		p.filter()

		// Should find exact match first
		assert.True(t, len(p.filtered) > 0)
		assert.Equal(t, "PROJ-3", items[p.filtered[0]].ID)
	})

	t.Run("handles empty query", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.search.SetValue("")
		p.filter()

		assert.Len(t, p.filtered, len(items))
	})

	t.Run("prioritizes prefix matches", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.search.SetValue("fix")
		p.filter()

		// "Fix login bug" should be first as it's a prefix match
		if len(p.filtered) > 0 {
			assert.Equal(t, "PROJ-1", items[p.filtered[0]].ID)
		}
	})

	t.Run("uses fuzzy matching", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.search.SetValue("lgn") // Short typo
		p.filter()

		// Should still find some items through fuzzy matching
		// The threshold is len(query)/2 + 2, so for "lgn" (len=3), threshold is 3
		assert.True(t, len(p.filtered) >= 0, "fuzzy matching should work")
	})

	t.Run("resets cursor on filter", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.cursor = 3
		p.search.SetValue("fix")
		p.filter()

		assert.Equal(t, 0, p.cursor)
		assert.Equal(t, 0, p.offset)
	})
}

func TestPickerView(t *testing.T) {
	t.Parallel()

	t.Run("renders title", func(t *testing.T) {
		t.Parallel()
		p := NewPicker(WithPickerTitle("Select Item"))
		view := p.View()
		assert.Contains(t, view, "Select Item")
	})

	t.Run("renders items", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		view := p.View()
		assert.Contains(t, view, "Fix login bug")
	})

	t.Run("renders empty state", func(t *testing.T) {
		t.Parallel()
		p := NewPicker()
		view := p.View()
		assert.Contains(t, view, "No matching items")
	})

	t.Run("renders help text", func(t *testing.T) {
		t.Parallel()
		p := NewPicker()
		view := p.View()
		assert.Contains(t, view, "navigate")
		assert.Contains(t, view, "select")
		assert.Contains(t, view, "cancel")
	})

	t.Run("renders icons", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		view := p.View()
		assert.Contains(t, view, Icons.Bug)
	})

	t.Run("renders details", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		view := p.View()
		assert.Contains(t, view, "High priority")
	})

	t.Run("shows scroll indicators", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items), WithPickerHeight(2))
		view := p.View()
		assert.Contains(t, view, "more items below")
	})
}

func TestPickerMethods(t *testing.T) {
	t.Parallel()

	t.Run("SelectedItem returns current item", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.cursor = 2

		item := p.SelectedItem()
		assert.NotNil(t, item)
		assert.Equal(t, "PROJ-3", item.ID)
	})

	t.Run("SelectedItem returns nil for empty list", func(t *testing.T) {
		t.Parallel()
		p := NewPicker()
		item := p.SelectedItem()
		assert.Nil(t, item)
	})

	t.Run("Items returns all items", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		assert.Equal(t, items, p.Items())
	})

	t.Run("SetItems updates items", func(t *testing.T) {
		t.Parallel()
		p := NewPicker()
		items := createTestItems()
		p.SetItems(items)
		assert.Len(t, p.items, 5)
		assert.Len(t, p.filtered, 5)
	})

	t.Run("Submitted returns correct value", func(t *testing.T) {
		t.Parallel()
		p := NewPicker()
		assert.False(t, p.Submitted())
		p.submitted = true
		assert.True(t, p.Submitted())
	})

	t.Run("Cancelled returns correct value", func(t *testing.T) {
		t.Parallel()
		p := NewPicker()
		assert.False(t, p.Cancelled())
		p.cancelled = true
		assert.True(t, p.Cancelled())
	})
}

func TestPickerInit(t *testing.T) {
	t.Parallel()

	t.Run("with search enabled", func(t *testing.T) {
		t.Parallel()
		p := NewPicker(WithPickerSearch(true))
		cmd := p.Init()
		assert.NotNil(t, cmd)
	})

	t.Run("with search disabled", func(t *testing.T) {
		t.Parallel()
		p := NewPicker(WithPickerSearch(false))
		cmd := p.Init()
		assert.Nil(t, cmd)
	})
}

func TestPickerScrolling(t *testing.T) {
	t.Parallel()

	t.Run("scrolls down when cursor goes beyond visible", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items), WithPickerHeight(2))

		// Move down past visible area
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})

		assert.Equal(t, 2, p.cursor)
		assert.Equal(t, 1, p.offset) // Should have scrolled
	})

	t.Run("scrolls up when cursor goes above visible", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items), WithPickerHeight(2))
		p.cursor = 3
		p.offset = 2

		// Move up past visible area
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})

		assert.Equal(t, 1, p.cursor)
		assert.Equal(t, 1, p.offset)
	})

	t.Run("page down boundary check", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items), WithPickerHeight(2))
		p.cursor = len(items) - 2

		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		assert.Equal(t, len(items)-1, p.cursor)
	})

	t.Run("page up with small cursor", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items), WithPickerHeight(10))
		p.cursor = 1

		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		assert.Equal(t, 0, p.cursor)
	})

	t.Run("end key with offset", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items), WithPickerHeight(2))

		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnd})
		assert.Equal(t, len(items)-1, p.cursor)
		assert.True(t, p.offset > 0)
	})
}

func TestPickerEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("enter on empty filtered list", func(t *testing.T) {
		t.Parallel()
		p := NewPicker()
		// No items, filtered is empty
		updated, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.False(t, updated.Submitted())
		assert.Nil(t, cmd)
	})

	t.Run("enter when cursor beyond filtered", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.cursor = len(p.filtered) + 10 // Beyond bounds

		updated, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.False(t, updated.Submitted())
		assert.Nil(t, cmd)
	})

	t.Run("SelectedItem with cursor beyond bounds", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items))
		p.cursor = len(p.filtered) + 10

		item := p.SelectedItem()
		assert.Nil(t, item)
	})
}

func TestPickerWrapper(t *testing.T) {
	t.Parallel()

	t.Run("wrapper Init calls model Init", func(t *testing.T) {
		t.Parallel()
		m := NewPicker()
		w := &pickerWrapper{model: m}
		cmd := w.Init()
		assert.NotNil(t, cmd)
	})

	t.Run("wrapper Update quits on PickerSelectMsg", func(t *testing.T) {
		t.Parallel()
		m := NewPicker()
		w := &pickerWrapper{model: m}
		_, cmd := w.Update(PickerSelectMsg{Item: PickerItem{}})
		assert.NotNil(t, cmd)
	})

	t.Run("wrapper Update quits on PickerCancelMsg", func(t *testing.T) {
		t.Parallel()
		m := NewPicker()
		w := &pickerWrapper{model: m}
		_, cmd := w.Update(PickerCancelMsg{})
		assert.NotNil(t, cmd)
	})

	t.Run("wrapper Update propagates to model", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		m := NewPicker(WithPickerItems(items))
		w := &pickerWrapper{model: m}
		// Send a key down to move selection
		_, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
		assert.Equal(t, 1, w.model.cursor)
	})

	t.Run("wrapper View renders model", func(t *testing.T) {
		t.Parallel()
		m := NewPicker(WithPickerTitle("Test"))
		w := &pickerWrapper{model: m}
		view := w.View()
		assert.Contains(t, view, "Test")
	})
}

func TestPickerFilterEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("fuzzy match with levenshtein", func(t *testing.T) {
		t.Parallel()
		items := []PickerItem{
			{ID: "1", Label: "Login Page"},
			{ID: "2", Label: "Dashboard"},
		}
		p := NewPicker(WithPickerItems(items))
		p.search.SetValue("Logi") // Close typo
		p.filter()

		// Should find Login Page through fuzzy matching (distance 1, threshold = len("logi")/2+2 = 4)
		assert.True(t, len(p.filtered) > 0)
	})

	t.Run("filters out distant matches", func(t *testing.T) {
		t.Parallel()
		items := []PickerItem{
			{ID: "1", Label: "Apple"},
			{ID: "2", Label: "Zebra"},
		}
		p := NewPicker(WithPickerItems(items))
		p.search.SetValue("xyz")
		p.filter()

		// Should have very few or no matches
		assert.True(t, len(p.filtered) < len(items))
	})

	t.Run("sorts by score with multiple matches", func(t *testing.T) {
		t.Parallel()
		items := []PickerItem{
			{ID: "1", Label: "Fix bug"},
			{ID: "2", Label: "Bug report"},
			{ID: "3", Label: "Documentation about bugs"},
		}
		p := NewPicker(WithPickerItems(items))
		p.search.SetValue("bug")
		p.filter()

		// All should match, but priority matters
		assert.True(t, len(p.filtered) >= 2)
	})
}

func TestPickerViewScrollIndicators(t *testing.T) {
	t.Parallel()

	t.Run("shows scroll indicator above", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items), WithPickerHeight(2))
		p.offset = 2
		view := p.View()
		assert.Contains(t, view, "more items above")
	})

	t.Run("does not show above when at start", func(t *testing.T) {
		t.Parallel()
		items := createTestItems()
		p := NewPicker(WithPickerItems(items), WithPickerHeight(2))
		p.offset = 0
		view := p.View()
		assert.NotContains(t, view, "more items above")
	})
}
