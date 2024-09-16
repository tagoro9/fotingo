package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/texttheater/golang-levenshtein/levenshtein"
)

// PickerItem represents an item in the picker.
type PickerItem struct {
	ID       string // Unique identifier
	Label    string // Display text
	Detail   string // Additional detail (displayed after label)
	Icon     string // Optional icon prefix
	Selected bool   // For multi-select mode
	Value    any    // Optional associated value
}

// PickerModel represents a list picker with fuzzy search filtering.
type PickerModel struct {
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
}

// PickerOption configures a PickerModel.
type PickerOption func(*PickerModel)

// WithPickerTitle sets the picker title.
func WithPickerTitle(title string) PickerOption {
	return func(m *PickerModel) {
		m.title = title
	}
}

// WithPickerItems sets the initial items.
func WithPickerItems(items []PickerItem) PickerOption {
	return func(m *PickerModel) {
		m.items = items
		m.filtered = make([]int, len(items))
		for i := range items {
			m.filtered[i] = i
		}
	}
}

// WithPickerHeight sets the maximum visible items.
func WithPickerHeight(height int) PickerOption {
	return func(m *PickerModel) {
		m.height = height
	}
}

// WithPickerSearch enables or disables the search input.
func WithPickerSearch(enabled bool) PickerOption {
	return func(m *PickerModel) {
		m.showSearch = enabled
	}
}

// WithPickerStyles sets custom styles.
func WithPickerStyles(styles Styles) PickerOption {
	return func(m *PickerModel) {
		m.styles = styles
	}
}

// NewPicker creates a new PickerModel.
func NewPicker(opts ...PickerOption) PickerModel {
	ti := textinput.New()
	ti.Placeholder = i18n.T(i18n.UIPickerFilterPrompt)
	ti.Focus()

	styles := DefaultStyles()
	ti.PromptStyle = styles.InputPrompt
	ti.TextStyle = styles.InputText
	ti.PlaceholderStyle = styles.InputPlaceholder

	m := PickerModel{
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

// Init initializes the picker model.
func (m PickerModel) Init() tea.Cmd {
	if m.showSearch {
		return textinput.Blink
	}
	return nil
}

// PickerSelectMsg is sent when an item is selected.
type PickerSelectMsg struct {
	Item PickerItem
}

// PickerCancelMsg is sent when the picker is cancelled.
type PickerCancelMsg struct{}

// Update handles messages for the picker model.
func (m PickerModel) Update(msg tea.Msg) (PickerModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp, tea.KeyCtrlP:
			if m.cursor > 0 {
				m.cursor--
				// Scroll up if needed
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
			return m, nil

		case tea.KeyDown, tea.KeyCtrlN:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				// Scroll down if needed
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

		case tea.KeyEnter:
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				m.submitted = true
				item := m.items[m.filtered[m.cursor]]
				return m, func() tea.Msg { return PickerSelectMsg{Item: item} }
			}
			return m, nil

		case tea.KeyEscape, tea.KeyCtrlC:
			m.cancelled = true
			return m, func() tea.Msg { return PickerCancelMsg{} }
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

// filter filters items based on the search query using fuzzy matching.
func (m *PickerModel) filter() {
	query := strings.ToLower(strings.TrimSpace(m.search.Value()))

	if query == "" {
		// No filter, show all items
		m.filtered = make([]int, len(m.items))
		for i := range m.items {
			m.filtered[i] = i
		}
	} else {
		// Score and filter items
		type scored struct {
			index int
			score int
		}

		var matches []scored
		for i, item := range m.items {
			label := strings.ToLower(item.Label)
			id := strings.ToLower(item.ID)

			// Exact substring match gets highest priority
			if strings.Contains(label, query) || strings.Contains(id, query) {
				// Lower score is better for substring match
				score := 0
				if strings.HasPrefix(label, query) || strings.HasPrefix(id, query) {
					score = -1 // Prefix match is even better
				}
				matches = append(matches, scored{index: i, score: score})
			} else {
				// Fuzzy match using Levenshtein distance
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

				// Use the better match
				dist := labelDist
				if idDist < dist {
					dist = idDist
				}

				// Only include if distance is reasonable (less than half the query length + some slack)
				threshold := len(query)/2 + 2
				if dist <= threshold {
					matches = append(matches, scored{index: i, score: dist})
				}
			}
		}

		// Sort by score (lower is better)
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

	// Reset cursor and offset
	m.cursor = 0
	m.offset = 0
}

// View renders the picker.
func (m PickerModel) View() string {
	var sb strings.Builder

	// Title
	if m.title != "" {
		sb.WriteString(m.styles.Title.Render(m.title))
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
		// Calculate visible range
		start := m.offset
		end := start + m.height
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		// Show scroll indicator if needed
		if start > 0 {
			sb.WriteString(m.styles.Muted.Render("  " + Icons.Arrow + " more items above"))
			sb.WriteString("\n")
		}

		for i := start; i < end; i++ {
			item := m.items[m.filtered[i]]
			isSelected := i == m.cursor

			// Cursor
			if isSelected {
				sb.WriteString(m.styles.ListCursor.Render(Icons.Cursor + " "))
			} else {
				sb.WriteString("  ")
			}

			// Icon
			if item.Icon != "" {
				if isSelected {
					sb.WriteString(m.styles.ListItemSelected.Render(item.Icon))
				} else {
					sb.WriteString(m.styles.ListItem.Render(m.styles.Muted.Render(item.Icon)))
				}
			}

			// Label
			if isSelected {
				sb.WriteString(m.styles.ListItemSelected.Render(item.Label))
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

		// Show scroll indicator if needed
		if end < len(m.filtered) {
			sb.WriteString(m.styles.Muted.Render("  " + Icons.Arrow + " more items below"))
			sb.WriteString("\n")
		}
	}

	// Help
	sb.WriteString("\n")
	sb.WriteString(m.styles.HelpKey.Render("up/down"))
	sb.WriteString(m.styles.HelpDesc.Render(" navigate "))
	sb.WriteString(m.styles.HelpKey.Render("enter"))
	sb.WriteString(m.styles.HelpDesc.Render(" select "))
	sb.WriteString(m.styles.HelpKey.Render("esc"))
	sb.WriteString(m.styles.HelpDesc.Render(" cancel"))
	sb.WriteString("\n")

	return sb.String()
}

// SelectedItem returns the currently highlighted item, or nil if none.
func (m PickerModel) SelectedItem() *PickerItem {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.items[m.filtered[m.cursor]]
}

// Items returns all items.
func (m PickerModel) Items() []PickerItem {
	return m.items
}

// SetItems updates the items and resets the filter.
func (m *PickerModel) SetItems(items []PickerItem) {
	m.items = items
	m.filter()
}

// Submitted returns whether an item was selected.
func (m PickerModel) Submitted() bool {
	return m.submitted
}

// Cancelled returns whether the picker was cancelled.
func (m PickerModel) Cancelled() bool {
	return m.cancelled
}

// PickerProgram wraps a PickerModel in a tea.Program for standalone use.
type PickerProgram struct {
	program *tea.Program
	model   *pickerWrapper
}

// pickerWrapper wraps PickerModel to implement tea.Model.
type pickerWrapper struct {
	model PickerModel
}

func (w *pickerWrapper) Init() tea.Cmd {
	return w.model.Init()
}

func (w *pickerWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case PickerSelectMsg, PickerCancelMsg:
		return w, tea.Quit
	}

	var cmd tea.Cmd
	w.model, cmd = w.model.Update(msg)
	return w, cmd
}

func (w *pickerWrapper) View() string {
	return w.model.View()
}

// NewPickerProgram creates a new picker program for standalone operation.
func NewPickerProgram(opts ...PickerOption) *PickerProgram {
	m := NewPicker(opts...)
	w := &pickerWrapper{model: m}
	p := tea.NewProgram(w)
	return &PickerProgram{
		program: p,
		model:   w,
	}
}

// Run runs the picker program and returns the selected item.
func (pp *PickerProgram) Run() (*PickerItem, error) {
	err := runTeaProgram(pp.program)
	if err != nil {
		return nil, err
	}

	if pp.model.model.Cancelled() {
		return nil, nil
	}

	return pp.model.model.SelectedItem(), nil
}
