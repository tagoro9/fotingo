// Package ui provides reusable Bubble Tea UI components for the fotingo CLI.
package ui

import (
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
)

// ColorScheme defines the color palette for UI components.
type ColorScheme struct {
	// Primary colors
	Primary   color.Color
	Secondary color.Color
	Accent    color.Color

	// Status colors
	Success color.Color
	Warning color.Color
	Error   color.Color
	Info    color.Color

	// UI colors
	Border     color.Color
	Muted      color.Color
	Background color.Color
	Foreground color.Color

	// Selection colors
	Selected   color.Color
	Unselected color.Color
	Cursor     color.Color
}

// DarkScheme returns the color scheme for dark terminals.
func DarkScheme() ColorScheme {
	return ColorScheme{
		Primary:   lipgloss.Color("45"),  // Cyan blue
		Secondary: lipgloss.Color("110"), // Steel blue
		Accent:    lipgloss.Color("220"), // Amber

		Success: lipgloss.Color("42"),  // Green
		Warning: lipgloss.Color("214"), // Orange
		Error:   lipgloss.Color("203"), // Red
		Info:    lipgloss.Color("81"),  // Blue

		Border:     lipgloss.Color("240"), // Gray
		Muted:      lipgloss.Color("245"), // Light gray
		Background: lipgloss.Color("235"), // Dark gray
		Foreground: lipgloss.Color("252"), // White-ish

		Selected:   lipgloss.Color("45"),  // Cyan blue
		Unselected: lipgloss.Color("245"), // Light gray
		Cursor:     lipgloss.Color("45"),  // Cyan blue
	}
}

// LightScheme returns the color scheme for light terminals.
func LightScheme() ColorScheme {
	return ColorScheme{
		Primary:   lipgloss.Color("24"),  // Navy blue
		Secondary: lipgloss.Color("31"),  // Ocean blue
		Accent:    lipgloss.Color("130"), // Amber brown

		Success: lipgloss.Color("28"),  // Dark green
		Warning: lipgloss.Color("166"), // Dark orange
		Error:   lipgloss.Color("124"), // Dark red
		Info:    lipgloss.Color("25"),  // Dark blue

		Border:     lipgloss.Color("250"), // Gray
		Muted:      lipgloss.Color("242"), // Medium gray
		Background: lipgloss.Color("255"), // Light gray
		Foreground: lipgloss.Color("235"), // Dark

		Selected:   lipgloss.Color("24"),  // Navy blue
		Unselected: lipgloss.Color("242"), // Medium gray
		Cursor:     lipgloss.Color("24"),  // Navy blue
	}
}

// DefaultScheme returns the appropriate color scheme based on terminal settings.
// It respects NO_COLOR environment variable.
func DefaultScheme() ColorScheme {
	// Respect NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		return noColorScheme()
	}

	// For now, default to dark scheme
	// Could be enhanced to detect terminal background color
	return DarkScheme()
}

// noColorScheme returns a scheme with no colors for accessibility.
func noColorScheme() ColorScheme {
	return ColorScheme{
		Primary:    lipgloss.NoColor{},
		Secondary:  lipgloss.NoColor{},
		Accent:     lipgloss.NoColor{},
		Success:    lipgloss.NoColor{},
		Warning:    lipgloss.NoColor{},
		Error:      lipgloss.NoColor{},
		Info:       lipgloss.NoColor{},
		Border:     lipgloss.NoColor{},
		Muted:      lipgloss.NoColor{},
		Background: lipgloss.NoColor{},
		Foreground: lipgloss.NoColor{},
		Selected:   lipgloss.NoColor{},
		Unselected: lipgloss.NoColor{},
		Cursor:     lipgloss.NoColor{},
	}
}

// Styles provides a collection of reusable Lipgloss styles.
type Styles struct {
	scheme ColorScheme

	// Base styles
	Base lipgloss.Style

	// Title and header styles
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Header   lipgloss.Style

	// Content styles
	Normal    lipgloss.Style
	Bold      lipgloss.Style
	Muted     lipgloss.Style
	Highlight lipgloss.Style

	// Status styles
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
	Info    lipgloss.Style

	// List/Picker styles
	ListItem         lipgloss.Style
	ListItemSelected lipgloss.Style
	ListCursor       lipgloss.Style

	// Input styles
	InputPrompt      lipgloss.Style
	InputText        lipgloss.Style
	InputPlaceholder lipgloss.Style

	// Border styles
	BorderedBox lipgloss.Style

	// Spinner styles
	Spinner lipgloss.Style

	// Button styles
	ButtonActive   lipgloss.Style
	ButtonInactive lipgloss.Style

	// Checkbox styles
	CheckboxChecked   lipgloss.Style
	CheckboxUnchecked lipgloss.Style

	// Help styles
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
}

// DefaultStyles returns a new Styles instance with the default color scheme.
func DefaultStyles() Styles {
	return NewStyles(DefaultScheme())
}

// NewStyles creates a new Styles instance with the given color scheme.
func NewStyles(scheme ColorScheme) Styles {
	s := Styles{scheme: scheme}

	// Base styles
	s.Base = lipgloss.NewStyle()

	// Title and header styles
	s.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(scheme.Primary).
		MarginBottom(1)

	s.Subtitle = lipgloss.NewStyle().
		Foreground(scheme.Secondary).
		MarginBottom(1)

	s.Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(scheme.Primary).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(scheme.Border).
		MarginBottom(1).
		PaddingBottom(1)

	// Content styles
	s.Normal = lipgloss.NewStyle().
		Foreground(scheme.Foreground)

	s.Bold = lipgloss.NewStyle().
		Bold(true).
		Foreground(scheme.Foreground)

	s.Muted = lipgloss.NewStyle().
		Foreground(scheme.Muted)

	s.Highlight = lipgloss.NewStyle().
		Foreground(scheme.Accent).
		Bold(true)

	// Status styles
	s.Success = lipgloss.NewStyle().
		Foreground(scheme.Success)

	s.Warning = lipgloss.NewStyle().
		Foreground(scheme.Warning)

	s.Error = lipgloss.NewStyle().
		Foreground(scheme.Error)

	s.Info = lipgloss.NewStyle().
		Foreground(scheme.Info)

	// List/Picker styles
	s.ListItem = lipgloss.NewStyle().
		PaddingLeft(1)

	s.ListItemSelected = lipgloss.NewStyle().
		Foreground(scheme.Selected).
		Bold(true).
		PaddingLeft(1)

	s.ListCursor = lipgloss.NewStyle().
		Foreground(scheme.Cursor).
		Bold(true)

	// Input styles
	s.InputPrompt = lipgloss.NewStyle().
		Foreground(scheme.Primary).
		Bold(true)

	s.InputText = lipgloss.NewStyle().
		Foreground(scheme.Foreground)

	s.InputPlaceholder = lipgloss.NewStyle().
		Foreground(scheme.Muted)

	// Border styles
	s.BorderedBox = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(scheme.Border).
		Padding(1, 2)

	// Spinner styles
	s.Spinner = lipgloss.NewStyle().
		Foreground(scheme.Primary)

	// Button styles
	s.ButtonActive = lipgloss.NewStyle().
		Foreground(scheme.Background).
		Background(scheme.Primary).
		Padding(0, 2).
		Bold(true)

	s.ButtonInactive = lipgloss.NewStyle().
		Foreground(scheme.Muted).
		Background(scheme.Background).
		Padding(0, 2)

	// Checkbox styles
	s.CheckboxChecked = lipgloss.NewStyle().
		Foreground(scheme.Success).
		Bold(true)

	s.CheckboxUnchecked = lipgloss.NewStyle().
		Foreground(scheme.Muted)

	// Help styles
	s.HelpKey = lipgloss.NewStyle().
		Foreground(scheme.Muted)

	s.HelpDesc = lipgloss.NewStyle().
		Foreground(scheme.Muted)

	return s
}

// Scheme returns the color scheme used by these styles.
func (s Styles) Scheme() ColorScheme {
	return s.scheme
}

// Icons provides consistent unicode icons for UI elements.
var Icons = struct {
	// Issue type icons
	Bug         string
	Story       string
	Task        string
	Epic        string
	Subtask     string
	Improvement string
	NewFeature  string
	Unknown     string

	// Status icons
	Check    string
	Cross    string
	Warning  string
	Info     string
	Question string

	// UI icons
	Cursor    string
	Checkbox  string
	Selected  string
	Arrow     string
	Spinner   string
	Ellipsis  string
	Separator string
}{
	// Issue type icons
	Bug:         "[B]",
	Story:       "[S]",
	Task:        "[T]",
	Epic:        "[E]",
	Subtask:     "[s]",
	Improvement: "[I]",
	NewFeature:  "[N]",
	Unknown:     "[?]",

	// Status icons
	Check:    "[v]",
	Cross:    "[x]",
	Warning:  "[!]",
	Info:     "[i]",
	Question: "[?]",

	// UI icons
	Cursor:    ">",
	Checkbox:  "[ ]",
	Selected:  "[x]",
	Arrow:     "->",
	Spinner:   "*",
	Ellipsis:  "...",
	Separator: "|",
}

// IssueTypeIcon returns the appropriate icon for a Jira issue type.
func IssueTypeIcon(issueType string) string {
	switch issueType {
	case "Bug":
		return Icons.Bug
	case "Story":
		return Icons.Story
	case "Task":
		return Icons.Task
	case "Epic":
		return Icons.Epic
	case "Sub-task", "Subtask":
		return Icons.Subtask
	case "Improvement":
		return Icons.Improvement
	case "New Feature":
		return Icons.NewFeature
	default:
		return Icons.Unknown
	}
}

// Spacing provides consistent spacing values.
var Spacing = struct {
	None   int
	Small  int
	Medium int
	Large  int
}{
	None:   0,
	Small:  1,
	Medium: 2,
	Large:  4,
}
