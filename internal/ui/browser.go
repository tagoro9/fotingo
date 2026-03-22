package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// BrowserProgram provides a list/detail interactive browser experience.
// Enter opens details for the selected item. Escape goes back to the list.
// Escape on the list exits the browser.
type BrowserProgram struct {
	program *tea.Program
	model   *browserModel
}

type browserModel struct {
	picker       PickerModel
	styles       Styles
	renderDetail func(PickerItem) string
	inDetail     bool
	detailTitle  string
	detailBody   string
	width        int
	height       int
	detailOffset int
	cancelled    bool
}

func newBrowserModel(title string, items []PickerItem, renderDetail func(PickerItem) string) *browserModel {
	picker := NewPicker(
		WithPickerTitle(title),
		WithPickerItems(items),
		WithPickerSearch(true),
	)

	return &browserModel{
		picker:       picker,
		styles:       DefaultStyles(),
		renderDetail: renderDetail,
	}
}

func (m *browserModel) Init() tea.Cmd {
	return m.picker.Init()
}

func (m *browserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		if m.inDetail {
			switch msg.String() {
			case "esc":
				m.inDetail = false
				m.detailTitle = ""
				m.detailBody = ""
				m.detailOffset = 0
				return m, nil
			case "ctrl+c":
				m.cancelled = true
				return m, tea.Quit
			case "up", "k":
				m.scrollDetailBy(-1)
				return m, nil
			case "down", "j":
				m.scrollDetailBy(1)
				return m, nil
			case "pgup", "b":
				m.scrollDetailBy(-m.detailPageStep())
				return m, nil
			case "pgdown", "f", " ":
				m.scrollDetailBy(m.detailPageStep())
				return m, nil
			case "home", "g":
				m.detailOffset = 0
				return m, nil
			case "end", "G":
				m.detailOffset = m.maxDetailOffset()
				return m, nil
			default:
				return m, nil
			}
		}

	case PickerSelectMsg:
		m.inDetail = true
		m.detailTitle = msg.Item.Label
		if m.renderDetail != nil {
			m.detailBody = m.renderDetail(msg.Item)
		} else {
			m.detailBody = ""
		}
		m.detailOffset = 0
		return m, nil

	case PickerCancelMsg:
		m.cancelled = true
		return m, tea.Quit
	}

	if m.inDetail {
		return m, nil
	}

	updatedPicker, cmd := m.picker.Update(msg)
	m.picker = updatedPicker
	return m, cmd
}

func (m *browserModel) View() tea.View {
	if !m.inDetail {
		return m.picker.View()
	}

	detail := strings.TrimSpace(m.detailBody)
	if detail == "" {
		detail = m.styles.Muted.Render("No value")
	}

	lines := strings.Split(hardWrapText(detail, m.detailWrapWidth()), "\n")
	maxOffset := maxInt(0, len(lines)-m.detailViewportHeight())
	if m.detailOffset > maxOffset {
		m.detailOffset = maxOffset
	}
	if m.detailOffset < 0 {
		m.detailOffset = 0
	}
	visibleEnd := minInt(len(lines), m.detailOffset+m.detailViewportHeight())
	visibleDetail := strings.Join(lines[m.detailOffset:visibleEnd], "\n")

	var sb strings.Builder
	sb.WriteString(m.styles.Title.Render(m.detailTitle))
	sb.WriteString("\n")
	sb.WriteString(m.styles.BorderedBox.Render(visibleDetail))
	sb.WriteString("\n\n")

	if m.detailOffset > 0 {
		sb.WriteString(m.styles.Muted.Render("↑ more above"))
		sb.WriteString("\n")
	}
	if visibleEnd < len(lines) {
		sb.WriteString(m.styles.Muted.Render("↓ more below"))
		sb.WriteString("\n")
	}

	sb.WriteString(m.styles.HelpKey.Render("esc"))
	sb.WriteString(m.styles.HelpDesc.Render(" back to list "))
	sb.WriteString(m.styles.HelpKey.Render("↑/↓"))
	sb.WriteString(m.styles.HelpDesc.Render(" scroll "))
	sb.WriteString(m.styles.HelpKey.Render("ctrl+c"))
	sb.WriteString(m.styles.HelpDesc.Render(" exit"))
	sb.WriteString("\n")

	return tea.NewView(sb.String())
}

// NewBrowserProgram creates a browser program for list/detail navigation.
func NewBrowserProgram(title string, items []PickerItem, renderDetail func(PickerItem) string) *BrowserProgram {
	model := newBrowserModel(title, items, renderDetail)
	program := tea.NewProgram(model)
	return &BrowserProgram{
		program: program,
		model:   model,
	}
}

// Run executes the browser program.
func (bp *BrowserProgram) Run() error {
	return runTeaProgram(bp.program)
}

func (m *browserModel) detailWrapWidth() int {
	width := m.width
	if width <= 0 {
		width = 100
	}

	// Rounded border contributes 2 cells and box padding contributes 4.
	innerWidth := width - 6
	if innerWidth < 20 {
		innerWidth = 20
	}

	return innerWidth
}

func (m *browserModel) detailViewportHeight() int {
	height := m.height
	if height <= 0 {
		height = 24
	}

	const reservedLines = 8
	viewportHeight := height - reservedLines
	if viewportHeight < 3 {
		viewportHeight = 3
	}

	return viewportHeight
}

func (m *browserModel) detailPageStep() int {
	step := m.detailViewportHeight() - 1
	if step < 1 {
		return 1
	}
	return step
}

func (m *browserModel) maxDetailOffset() int {
	detail := strings.TrimSpace(m.detailBody)
	if detail == "" {
		detail = "No value"
	}

	lines := strings.Split(hardWrapText(detail, m.detailWrapWidth()), "\n")
	return maxInt(0, len(lines)-m.detailViewportHeight())
}

func (m *browserModel) scrollDetailBy(delta int) {
	m.detailOffset += delta
	maxOffset := m.maxDetailOffset()
	if m.detailOffset > maxOffset {
		m.detailOffset = maxOffset
	}
	if m.detailOffset < 0 {
		m.detailOffset = 0
	}
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func hardWrapText(value string, width int) string {
	if width <= 0 {
		return value
	}

	lines := strings.Split(value, "\n")
	wrappedLines := make([]string, 0, len(lines))

	for _, line := range lines {
		runes := []rune(line)
		if len(runes) == 0 {
			wrappedLines = append(wrappedLines, "")
			continue
		}

		for len(runes) > width {
			wrappedLines = append(wrappedLines, string(runes[:width]))
			runes = runes[width:]
		}

		wrappedLines = append(wrappedLines, string(runes))
	}

	return strings.Join(wrappedLines, "\n")
}
