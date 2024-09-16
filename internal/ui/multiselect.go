package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/texttheater/golang-levenshtein/levenshtein"
)

// MultiSelectModel represents a multi-select picker with fuzzy search filtering.
type MultiSelectModel struct {
	styles     Styles
	title      string
	items      []PickerItem
	filtered   []int // Indices of filtered items
	cursor     int   // Current cursor position in filtered list
	search     textinput.Model
	showSearch bool
	height     int // Maximum visible items
	offset     int // Scroll offset
	submitted  bool
	cancelled  bool
	minSelect  int // Minimum required selections (0 = no minimum)
	maxSelect  int // Maximum allowed selections (0 = no limit)
}

// MultiSelectOption configures a MultiSelectModel.
type MultiSelectOption func(*MultiSelectModel)

// WithMultiSelectTitle sets the title.
func WithMultiSelectTitle(title string) MultiSelectOption {
	return func(m *MultiSelectModel) {
		m.title = title
	}
}

// WithMultiSelectItems sets the initial items.
func WithMultiSelectItems(items []PickerItem) MultiSelectOption {
	return func(m *MultiSelectModel) {
		m.items = items
		m.filtered = make([]int, len(items))
		for i := range items {
			m.filtered[i] = i
		}
	}
}

// WithMultiSelectHeight sets the maximum visible items.
func WithMultiSelectHeight(height int) MultiSelectOption {
	return func(m *MultiSelectModel) {
		m.height = height
	}
}

// WithMultiSelectSearch enables or disables the search input.
func WithMultiSelectSearch(enabled bool) MultiSelectOption {
	return func(m *MultiSelectModel) {
		m.showSearch = enabled
	}
}

// WithMultiSelectMinimum sets the minimum required selections.
func WithMultiSelectMinimum(min int) MultiSelectOption {
	return func(m *MultiSelectModel) {
		m.minSelect = min
	}
}

// WithMultiSelectMaximum sets the maximum allowed selections.
func WithMultiSelectMaximum(max int) MultiSelectOption {
	return func(m *MultiSelectModel) {
		m.maxSelect = max
	}
}

// WithMultiSelectStyles sets custom styles.
func WithMultiSelectStyles(styles Styles) MultiSelectOption {
	return func(m *MultiSelectModel) {
		m.styles = styles
	}
}

// WithPreselected marks specific items as selected by their IDs.
func WithPreselected(ids []string) MultiSelectOption {
	return func(m *MultiSelectModel) {
		idSet := make(map[string]bool)
		for _, id := range ids {
			idSet[id] = true
		}
		for i := range m.items {
			if idSet[m.items[i].ID] {
				m.items[i].Selected = true
			}
		}
	}
}

// NewMultiSelect creates a new MultiSelectModel.
func NewMultiSelect(opts ...MultiSelectOption) MultiSelectModel {
	ti := textinput.New()
	ti.Placeholder = i18n.T(i18n.UIMultiFilterPrompt)
	ti.Focus()

	styles := DefaultStyles()
	ti.PromptStyle = styles.InputPrompt
	ti.TextStyle = styles.InputText
	ti.PlaceholderStyle = styles.InputPlaceholder

	m := MultiSelectModel{
		styles:     styles,
		search:     ti,
		showSearch: true,
		height:     10,
		items:      []PickerItem{},
		filtered:   []int{},
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

// Init initializes the multi-select model.
func (m MultiSelectModel) Init() tea.Cmd {
	if m.showSearch {
		return textinput.Blink
	}
	return nil
}

// MultiSelectResultMsg is sent when the multi-select is confirmed.
type MultiSelectResultMsg struct {
	Items []PickerItem
}

// MultiSelectCancelMsg is sent when the multi-select is cancelled.
type MultiSelectCancelMsg struct{}

// Update handles messages for the multi-select model.
func (m MultiSelectModel) Update(msg tea.Msg) (MultiSelectModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp, tea.KeyCtrlP:
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
			return m, nil

		case tea.KeyDown, tea.KeyCtrlN:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				if m.cursor >= m.offset+m.height {
					m.offset = m.cursor - m.height + 1
				}
			}
			return m, nil

		case tea.KeyHome:
			m.cursor = 0
			m.offset = 0
			return m, nil

		case tea.KeyEnd:
			m.cursor = len(m.filtered) - 1
			if m.cursor >= m.height {
				m.offset = m.cursor - m.height + 1
			}
			return m, nil

		case tea.KeyPgUp:
			m.cursor -= m.height
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.offset = m.cursor
			return m, nil

		case tea.KeyPgDown:
			m.cursor += m.height
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			if m.cursor >= m.offset+m.height {
				m.offset = m.cursor - m.height + 1
			}
			return m, nil

		case tea.KeySpace:
			// Toggle selection
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				idx := m.filtered[m.cursor]
				item := &m.items[idx]

				// Check max limit before selecting
				if !item.Selected && m.maxSelect > 0 && m.selectedCount() >= m.maxSelect {
					// Already at max, don't select
					return m, nil
				}

				item.Selected = !item.Selected
			}
			return m, nil

		case tea.KeyEnter:
			// Check minimum requirement
			if m.minSelect > 0 && m.selectedCount() < m.minSelect {
				return m, nil
			}
			m.submitted = true
			return m, func() tea.Msg {
				return MultiSelectResultMsg{Items: m.SelectedItems()}
			}

		case tea.KeyEscape, tea.KeyCtrlC:
			m.cancelled = true
			return m, func() tea.Msg { return MultiSelectCancelMsg{} }

		case tea.KeyCtrlA:
			// Select all visible
			for _, idx := range m.filtered {
				if m.maxSelect > 0 && m.selectedCount() >= m.maxSelect {
					break
				}
				m.items[idx].Selected = true
			}
			return m, nil
		}

		// Handle 'a' key for select all (when not in search mode or search is empty)
		if msg.String() == "a" && (!m.showSearch || m.search.Value() == "") {
			for _, idx := range m.filtered {
				if m.maxSelect > 0 && m.selectedCount() >= m.maxSelect {
					break
				}
				m.items[idx].Selected = true
			}
			return m, nil
		}
	}

	// Handle search input
	if m.showSearch {
		prevValue := m.search.Value()
		m.search, cmd = m.search.Update(msg)

		// Re-filter if search changed
		if m.search.Value() != prevValue {
			m.filter()
		}
	}

	return m, cmd
}

// filter filters items based on the search query.
func (m *MultiSelectModel) filter() {
	query := strings.ToLower(strings.TrimSpace(m.search.Value()))

	if query == "" {
		m.filtered = make([]int, len(m.items))
		for i := range m.items {
			m.filtered[i] = i
		}
	} else {
		type scored struct {
			index int
			score int
		}

		var matches []scored
		for i, item := range m.items {
			label := strings.ToLower(item.Label)
			id := strings.ToLower(item.ID)

			if strings.Contains(label, query) || strings.Contains(id, query) {
				score := 0
				if strings.HasPrefix(label, query) || strings.HasPrefix(id, query) {
					score = -1
				}
				matches = append(matches, scored{index: i, score: score})
			} else {
				labelDist := levenshtein.DistanceForStrings(
					[]rune(query),
					[]rune(label),
					levenshtein.DefaultOptions,
				)
				idDist := levenshtein.DistanceForStrings(
					[]rune(query),
					[]rune(id),
					levenshtein.DefaultOptions,
				)

				dist := labelDist
				if idDist < dist {
					dist = idDist
				}

				threshold := len(query)/2 + 2
				if dist <= threshold {
					matches = append(matches, scored{index: i, score: dist})
				}
			}
		}

		// Sort by score
		for i := 0; i < len(matches)-1; i++ {
			for j := i + 1; j < len(matches); j++ {
				if matches[j].score < matches[i].score {
					matches[i], matches[j] = matches[j], matches[i]
				}
			}
		}

		m.filtered = make([]int, len(matches))
		for i, match := range matches {
			m.filtered[i] = match.index
		}
	}

	m.cursor = 0
	m.offset = 0
}

// selectedCount returns the number of selected items.
func (m MultiSelectModel) selectedCount() int {
	count := 0
	for _, item := range m.items {
		if item.Selected {
			count++
		}
	}
	return count
}

// View renders the multi-select.
func (m MultiSelectModel) View() string {
	var sb strings.Builder

	// Title with selection count
	if m.title != "" {
		count := m.selectedCount()
		titleText := m.title
		if count > 0 {
			titleText = fmt.Sprintf("%s (%d selected)", m.title, count)
		}
		sb.WriteString(m.styles.Title.Render(titleText))
		sb.WriteString("\n")
	}

	// Search input
	if m.showSearch {
		sb.WriteString(m.search.View())
		sb.WriteString("\n\n")
	}

	// Items
	if len(m.filtered) == 0 {
		sb.WriteString(m.styles.Muted.Render("  No matching items"))
		sb.WriteString("\n")
	} else {
		start := m.offset
		end := start + m.height
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		if start > 0 {
			sb.WriteString(m.styles.Muted.Render("  " + Icons.Arrow + " more items above"))
			sb.WriteString("\n")
		}

		for i := start; i < end; i++ {
			item := m.items[m.filtered[i]]
			isCursor := i == m.cursor

			// Cursor
			if isCursor {
				sb.WriteString(m.styles.ListCursor.Render(Icons.Cursor + " "))
			} else {
				sb.WriteString("  ")
			}

			// Checkbox
			if item.Selected {
				sb.WriteString(m.styles.CheckboxChecked.Render(Icons.Selected + " "))
			} else {
				sb.WriteString(m.styles.CheckboxUnchecked.Render(Icons.Checkbox + " "))
			}

			// Icon
			if item.Icon != "" {
				if isCursor {
					// Keep icon column stable when cursor moves onto a row.
					iconStyle := m.styles.ListItemSelected
					sb.WriteString(iconStyle.PaddingLeft(0).Render(item.Icon + " "))
				} else {
					sb.WriteString(m.styles.Muted.Render(item.Icon + " "))
				}
			}

			// Label
			if isCursor {
				sb.WriteString(m.styles.ListItemSelected.Render(item.Label))
			} else if item.Selected {
				// Keep selected/non-selected rows aligned by preserving list-item left padding.
				sb.WriteString(m.styles.Success.Render(" " + item.Label))
			} else {
				sb.WriteString(m.styles.ListItem.Render(item.Label))
			}

			// Detail
			if item.Detail != "" {
				sb.WriteString(" ")
				sb.WriteString(m.styles.Muted.Render(item.Detail))
			}

			sb.WriteString("\n")
		}

		if end < len(m.filtered) {
			sb.WriteString(m.styles.Muted.Render("  " + Icons.Arrow + " more items below"))
			sb.WriteString("\n")
		}
	}

	// Validation message
	if m.minSelect > 0 && m.selectedCount() < m.minSelect {
		sb.WriteString("\n")
		sb.WriteString(m.styles.Warning.Render(
			fmt.Sprintf("  Select at least %d item(s)", m.minSelect),
		))
		sb.WriteString("\n")
	}

	// Help
	sb.WriteString("\n")
	sb.WriteString(m.styles.HelpKey.Render("up/down"))
	sb.WriteString(m.styles.HelpDesc.Render(" navigate "))
	sb.WriteString(m.styles.HelpKey.Render("space"))
	sb.WriteString(m.styles.HelpDesc.Render(" toggle "))
	sb.WriteString(m.styles.HelpKey.Render("enter"))
	sb.WriteString(m.styles.HelpDesc.Render(" confirm "))
	sb.WriteString(m.styles.HelpKey.Render("esc"))
	sb.WriteString(m.styles.HelpDesc.Render(" cancel"))
	sb.WriteString("\n")

	return sb.String()
}

// SelectedItems returns all selected items.
func (m MultiSelectModel) SelectedItems() []PickerItem {
	var selected []PickerItem
	for _, item := range m.items {
		if item.Selected {
			selected = append(selected, item)
		}
	}
	return selected
}

// Items returns all items.
func (m MultiSelectModel) Items() []PickerItem {
	return m.items
}

// SetItems updates the items and resets the filter.
func (m *MultiSelectModel) SetItems(items []PickerItem) {
	m.items = items
	m.filter()
}

// Submitted returns whether the selection was confirmed.
func (m MultiSelectModel) Submitted() bool {
	return m.submitted
}

// Cancelled returns whether the selection was cancelled.
func (m MultiSelectModel) Cancelled() bool {
	return m.cancelled
}

// MultiSelectProgram wraps a MultiSelectModel in a tea.Program for standalone use.
type MultiSelectProgram struct {
	program *tea.Program
	model   *multiSelectWrapper
}

// multiSelectWrapper wraps MultiSelectModel to implement tea.Model.
type multiSelectWrapper struct {
	model MultiSelectModel
}

func (w *multiSelectWrapper) Init() tea.Cmd {
	return w.model.Init()
}

func (w *multiSelectWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case MultiSelectResultMsg, MultiSelectCancelMsg:
		return w, tea.Quit
	}

	var cmd tea.Cmd
	w.model, cmd = w.model.Update(msg)
	return w, cmd
}

func (w *multiSelectWrapper) View() string {
	return w.model.View()
}

// NewMultiSelectProgram creates a new multi-select program for standalone operation.
func NewMultiSelectProgram(opts ...MultiSelectOption) *MultiSelectProgram {
	m := NewMultiSelect(opts...)
	w := &multiSelectWrapper{model: m}
	p := tea.NewProgram(w)
	return &MultiSelectProgram{
		program: p,
		model:   w,
	}
}

// Run runs the multi-select program and returns the selected items.
func (mp *MultiSelectProgram) Run() ([]PickerItem, error) {
	err := runTeaProgram(mp.program)
	if err != nil {
		return nil, err
	}

	if mp.model.model.Cancelled() {
		return nil, nil
	}

	return mp.model.model.SelectedItems(), nil
}
