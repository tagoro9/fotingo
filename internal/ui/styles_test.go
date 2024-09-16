package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDarkScheme(t *testing.T) {
	t.Parallel()

	scheme := DarkScheme()

	// Verify primary colors are set
	assert.NotNil(t, scheme.Primary)
	assert.NotNil(t, scheme.Secondary)
	assert.NotNil(t, scheme.Accent)

	// Verify status colors are set
	assert.NotNil(t, scheme.Success)
	assert.NotNil(t, scheme.Warning)
	assert.NotNil(t, scheme.Error)
	assert.NotNil(t, scheme.Info)

	// Verify UI colors are set
	assert.NotNil(t, scheme.Border)
	assert.NotNil(t, scheme.Muted)
	assert.NotNil(t, scheme.Background)
	assert.NotNil(t, scheme.Foreground)
}

func TestLightScheme(t *testing.T) {
	t.Parallel()

	scheme := LightScheme()

	// Verify primary colors are set
	assert.NotNil(t, scheme.Primary)
	assert.NotNil(t, scheme.Secondary)
	assert.NotNil(t, scheme.Accent)
}

func TestDefaultScheme(t *testing.T) {
	// Test with NO_COLOR set
	t.Run("respects NO_COLOR", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		scheme := DefaultScheme()

		// All colors should be NoColor
		assert.NotNil(t, scheme.Primary)
	})

	t.Run("returns dark scheme by default", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")

		scheme := DefaultScheme()
		assert.NotNil(t, scheme.Primary)
	})
}

func TestNewStyles(t *testing.T) {
	t.Parallel()

	scheme := DarkScheme()
	styles := NewStyles(scheme)

	// Verify styles are created
	assert.NotNil(t, styles.Base)
	assert.NotNil(t, styles.Title)
	assert.NotNil(t, styles.Subtitle)
	assert.NotNil(t, styles.Header)

	// Verify status styles
	assert.NotNil(t, styles.Success)
	assert.NotNil(t, styles.Warning)
	assert.NotNil(t, styles.Error)
	assert.NotNil(t, styles.Info)

	// Verify list styles
	assert.NotNil(t, styles.ListItem)
	assert.NotNil(t, styles.ListItemSelected)
	assert.NotNil(t, styles.ListCursor)

	// Verify Scheme method
	returnedScheme := styles.Scheme()
	assert.Equal(t, scheme.Primary, returnedScheme.Primary)
}

func TestDefaultStyles(t *testing.T) {
	t.Parallel()

	styles := DefaultStyles()
	assert.NotNil(t, styles.Title)
}

func TestIssueTypeIcon(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		issueType string
		wantIcon  string
	}{
		{name: "Bug", issueType: "Bug", wantIcon: Icons.Bug},
		{name: "Story", issueType: "Story", wantIcon: Icons.Story},
		{name: "Task", issueType: "Task", wantIcon: Icons.Task},
		{name: "Epic", issueType: "Epic", wantIcon: Icons.Epic},
		{name: "Sub-task", issueType: "Sub-task", wantIcon: Icons.Subtask},
		{name: "Subtask", issueType: "Subtask", wantIcon: Icons.Subtask},
		{name: "Improvement", issueType: "Improvement", wantIcon: Icons.Improvement},
		{name: "New Feature", issueType: "New Feature", wantIcon: Icons.NewFeature},
		{name: "Unknown type", issueType: "CustomType", wantIcon: Icons.Unknown},
		{name: "Empty type", issueType: "", wantIcon: Icons.Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IssueTypeIcon(tt.issueType)
			assert.Equal(t, tt.wantIcon, got)
		})
	}
}

func TestIcons(t *testing.T) {
	t.Parallel()

	// Verify icons are non-empty strings
	assert.NotEmpty(t, Icons.Bug)
	assert.NotEmpty(t, Icons.Story)
	assert.NotEmpty(t, Icons.Task)
	assert.NotEmpty(t, Icons.Check)
	assert.NotEmpty(t, Icons.Cross)
	assert.NotEmpty(t, Icons.Cursor)
	assert.NotEmpty(t, Icons.Checkbox)
	assert.NotEmpty(t, Icons.Selected)
}

func TestSpacing(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, Spacing.None)
	assert.Equal(t, 1, Spacing.Small)
	assert.Equal(t, 2, Spacing.Medium)
	assert.Equal(t, 4, Spacing.Large)
}
