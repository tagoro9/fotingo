package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tmpl := New("Hello {name}")
	assert.NotNil(t, tmpl)
	assert.Equal(t, "Hello {name}", tmpl.Content())
}

func TestRender_SimplePlaceholder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		data     map[string]string
		want     string
	}{
		{
			name:     "single placeholder",
			template: "Hello {name}",
			data:     map[string]string{"name": "World"},
			want:     "Hello World",
		},
		{
			name:     "multiple placeholders",
			template: "{greeting} {name}!",
			data:     map[string]string{"greeting": "Hello", "name": "World"},
			want:     "Hello World!",
		},
		{
			name:     "repeated placeholder",
			template: "{name} and {name}",
			data:     map[string]string{"name": "Alice"},
			want:     "Alice and Alice",
		},
		{
			name:     "placeholder at start",
			template: "{prefix}foo",
			data:     map[string]string{"prefix": "bar"},
			want:     "barfoo",
		},
		{
			name:     "placeholder at end",
			template: "foo{suffix}",
			data:     map[string]string{"suffix": "bar"},
			want:     "foobar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpl := New(tt.template)
			got := tmpl.Render(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRender_NestedPlaceholders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		data     map[string]string
		want     string
	}{
		{
			name:     "issue.key",
			template: "Issue: {issue.key}",
			data:     map[string]string{"issue.key": "PROJ-123"},
			want:     "Issue: PROJ-123",
		},
		{
			name:     "issue.summary",
			template: "[{issue.key}] {issue.summary}",
			data:     map[string]string{"issue.key": "PROJ-123", "issue.summary": "Fix bug"},
			want:     "[PROJ-123] Fix bug",
		},
		{
			name:     "deep nesting",
			template: "{a.b.c}",
			data:     map[string]string{"a.b.c": "value"},
			want:     "value",
		},
		{
			name:     "mixed simple and nested",
			template: "{branchName}: {issue.key} - {issue.summary}",
			data: map[string]string{
				"branchName":    "feature/test",
				"issue.key":     "PROJ-456",
				"issue.summary": "Add feature",
			},
			want: "feature/test: PROJ-456 - Add feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpl := New(tt.template)
			got := tmpl.Render(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRender_MissingPlaceholders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		data     map[string]string
		want     string
	}{
		{
			name:     "missing placeholder kept as-is",
			template: "Hello {name}",
			data:     map[string]string{},
			want:     "Hello {name}",
		},
		{
			name:     "partial data",
			template: "{greeting} {name}!",
			data:     map[string]string{"greeting": "Hello"},
			want:     "Hello {name}!",
		},
		{
			name:     "nil data",
			template: "Hello {name}",
			data:     nil,
			want:     "Hello {name}",
		},
		{
			name:     "missing nested placeholder",
			template: "Issue: {issue.key}",
			data:     map[string]string{},
			want:     "Issue: {issue.key}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpl := New(tt.template)
			got := tmpl.Render(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderWithDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		data     map[string]string
		defaults map[string]string
		want     string
	}{
		{
			name:     "data takes precedence over defaults",
			template: "Hello {name}",
			data:     map[string]string{"name": "World"},
			defaults: map[string]string{"name": "Default"},
			want:     "Hello World",
		},
		{
			name:     "fallback to defaults",
			template: "Hello {name}",
			data:     map[string]string{},
			defaults: map[string]string{"name": "Default"},
			want:     "Hello Default",
		},
		{
			name:     "nil data uses defaults",
			template: "Hello {name}",
			data:     nil,
			defaults: map[string]string{"name": "Default"},
			want:     "Hello Default",
		},
		{
			name:     "nil defaults",
			template: "Hello {name}",
			data:     map[string]string{"name": "World"},
			defaults: nil,
			want:     "Hello World",
		},
		{
			name:     "both nil",
			template: "Hello {name}",
			data:     nil,
			defaults: nil,
			want:     "Hello {name}",
		},
		{
			name:     "mixed data and defaults",
			template: "{greeting} {name}!",
			data:     map[string]string{"greeting": "Hi"},
			defaults: map[string]string{"name": "There", "greeting": "Hello"},
			want:     "Hi There!",
		},
		{
			name:     "fotingo.banner default",
			template: "{content}\n\n{fotingo.banner}",
			data:     map[string]string{"content": "PR description"},
			defaults: map[string]string{"fotingo.banner": DefaultFotingoBanner},
			want:     "PR description\n\n🚀 PR created with [fotingo](https://github.com/tagoro9/fotingo)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpl := New(tt.template)
			got := tmpl.RenderWithDefaults(tt.data, tt.defaults)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRender_EmptyTemplate(t *testing.T) {
	t.Parallel()

	tmpl := New("")
	got := tmpl.Render(map[string]string{"name": "World"})
	assert.Equal(t, "", got)
}

func TestRender_NoPlaceholders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		data     map[string]string
		want     string
	}{
		{
			name:     "plain text",
			template: "Hello World",
			data:     map[string]string{"name": "ignored"},
			want:     "Hello World",
		},
		{
			name:     "text with special chars",
			template: "Hello! How are you?",
			data:     map[string]string{},
			want:     "Hello! How are you?",
		},
		{
			name:     "text with braces but not placeholder",
			template: "JSON: {\"key\": \"value\"}",
			data:     map[string]string{},
			want:     "JSON: {\"key\": \"value\"}",
		},
		{
			name:     "empty braces",
			template: "Empty: {}",
			data:     map[string]string{},
			want:     "Empty: {}",
		},
		{
			name:     "invalid placeholder syntax",
			template: "Invalid: {123} {-name} {name-}",
			data:     map[string]string{"123": "a", "-name": "b", "name-": "c"},
			want:     "Invalid: {123} {-name} {name-}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpl := New(tt.template)
			got := tmpl.Render(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHasPlaceholder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		template    string
		placeholder string
		want        bool
	}{
		{
			name:        "has simple placeholder",
			template:    "Hello {name}",
			placeholder: "name",
			want:        true,
		},
		{
			name:        "has nested placeholder",
			template:    "Issue: {issue.key}",
			placeholder: "issue.key",
			want:        true,
		},
		{
			name:        "does not have placeholder",
			template:    "Hello {name}",
			placeholder: "greeting",
			want:        false,
		},
		{
			name:        "empty template",
			template:    "",
			placeholder: "name",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpl := New(tt.template)
			got := tmpl.HasPlaceholder(tt.placeholder)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPlaceholders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		want     []string
	}{
		{
			name:     "single placeholder",
			template: "Hello {name}",
			want:     []string{"name"},
		},
		{
			name:     "multiple placeholders",
			template: "{greeting} {name}!",
			want:     []string{"greeting", "name"},
		},
		{
			name:     "nested placeholders",
			template: "[{issue.key}] {issue.summary}",
			want:     []string{"issue.key", "issue.summary"},
		},
		{
			name:     "repeated placeholders deduplicated",
			template: "{name} and {name} again",
			want:     []string{"name"},
		},
		{
			name:     "no placeholders",
			template: "Hello World",
			want:     nil,
		},
		{
			name:     "empty template",
			template: "",
			want:     nil,
		},
		{
			name:     "all common placeholders",
			template: "{branchName} {issue.key} {issue.summary} {issue.description} {issue.url} {changes} {fixedIssues} {fotingo.banner}",
			want:     []string{"branchName", "issue.key", "issue.summary", "issue.description", "issue.url", "changes", "fixedIssues", "fotingo.banner"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpl := New(tt.template)
			got := tmpl.Placeholders()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRender_PRDescriptionTemplate(t *testing.T) {
	t.Parallel()

	// Test a realistic PR description template
	template := `## [{issue.key}] {issue.summary}

### Description
{issue.description}

### Changes
{changes}

### Fixed Issues
{fixedIssues}

---
{fotingo.banner}`

	data := map[string]string{
		"issue.key":         "PROJ-123",
		"issue.summary":     "Add user authentication",
		"issue.description": "Implement OAuth2 authentication for the API.",
		"changes":           "- Added OAuth2 provider\n- Added login endpoint\n- Added logout endpoint",
		"fixedIssues":       "- [PROJ-123](https://jira.example.com/browse/PROJ-123)",
		"fotingo.banner":    "🚀 PR created with [fotingo](https://github.com/tagoro9/fotingo)",
	}

	tmpl := New(template)
	got := tmpl.Render(data)

	expected := `## [PROJ-123] Add user authentication

### Description
Implement OAuth2 authentication for the API.

### Changes
- Added OAuth2 provider
- Added login endpoint
- Added logout endpoint

### Fixed Issues
- [PROJ-123](https://jira.example.com/browse/PROJ-123)

---
🚀 PR created with [fotingo](https://github.com/tagoro9/fotingo)`

	assert.Equal(t, expected, got)
}

func TestContent(t *testing.T) {
	t.Parallel()

	content := "Hello {name}"
	tmpl := New(content)
	assert.Equal(t, content, tmpl.Content())
}

func TestPlaceholderConstants(t *testing.T) {
	t.Parallel()

	// Verify that the constants have expected values
	assert.Equal(t, "branchName", PlaceholderBranchName)
	assert.Equal(t, "summary", PlaceholderSummary)
	assert.Equal(t, "description", PlaceholderDescription)
	assert.Equal(t, "issue.key", PlaceholderIssueKey)
	assert.Equal(t, "issue.summary", PlaceholderIssueSummary)
	assert.Equal(t, "issue.description", PlaceholderIssueDescription)
	assert.Equal(t, "issue.url", PlaceholderIssueURL)
	assert.Equal(t, "changes", PlaceholderChanges)
	assert.Equal(t, "fixedIssues", PlaceholderFixedIssues)
	assert.Equal(t, "fotingo.banner", PlaceholderFotingoBanner)
}

func TestDefaultFotingoBanner(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "🚀 PR created with [fotingo](https://github.com/tagoro9/fotingo)", DefaultFotingoBanner)
}
