// Package template provides a simple template engine for PR descriptions and release notes.
// It supports {placeholder} syntax with nested placeholders like {issue.key}.
package template

import (
	"regexp"
	"strings"
)

// placeholderPattern matches placeholders in the format {name} or {name.subname}
var placeholderPattern = regexp.MustCompile(`\{([a-zA-Z][a-zA-Z0-9]*(?:\.[a-zA-Z][a-zA-Z0-9]*)*)\}`)

// Template represents a template string with placeholders.
type Template struct {
	content string
}

// New creates a new Template from the given content string.
func New(content string) *Template {
	return &Template{content: content}
}

// Render replaces all placeholders in the template with values from the data map.
// Missing placeholders are kept as-is (e.g., "{missing}" stays "{missing}").
func (t *Template) Render(data map[string]string) string {
	return t.RenderWithDefaults(data, nil)
}

// RenderWithDefaults replaces placeholders with values from data, falling back to defaults.
// If a placeholder is not found in data or defaults, it is kept as-is.
func (t *Template) RenderWithDefaults(data, defaults map[string]string) string {
	if t.content == "" {
		return ""
	}

	result := placeholderPattern.ReplaceAllStringFunc(t.content, func(match string) string {
		// Extract the placeholder name (without braces)
		name := match[1 : len(match)-1]

		// Try data first
		if data != nil {
			if value, ok := data[name]; ok {
				return value
			}
		}

		// Try defaults
		if defaults != nil {
			if value, ok := defaults[name]; ok {
				return value
			}
		}

		// Keep placeholder as-is if not found
		return match
	})

	return result
}

// Content returns the raw template content.
func (t *Template) Content() string {
	return t.content
}

// HasPlaceholder checks if the template contains a specific placeholder.
func (t *Template) HasPlaceholder(name string) bool {
	placeholder := "{" + name + "}"
	return strings.Contains(t.content, placeholder)
}

// Placeholders returns a list of all placeholder names found in the template.
func (t *Template) Placeholders() []string {
	matches := placeholderPattern.FindAllStringSubmatch(t.content, -1)
	if matches == nil {
		return nil
	}

	// Use a map to deduplicate
	seen := make(map[string]struct{})
	var result []string

	for _, match := range matches {
		name := match[1]
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}

	return result
}

// Common placeholder names used in PR descriptions and release notes.
const (
	PlaceholderBranchName            = "branchName"
	PlaceholderSummary               = "summary"
	PlaceholderDescription           = "description"
	PlaceholderIssueKey              = "issue.key"
	PlaceholderIssueSummary          = "issue.summary"
	PlaceholderIssueDescription      = "issue.description"
	PlaceholderIssueURL              = "issue.url"
	PlaceholderChanges               = "changes"
	PlaceholderFixedIssues           = "fixedIssues"
	PlaceholderFotingoBanner         = "fotingo.banner"
	PlaceholderVersion               = "version"
	PlaceholderFixedIssuesByCategory = "fixedIssuesByCategory"
	PlaceholderJiraRelease           = "jira.release"
)

// DefaultFotingoBanner is the default attribution text for fotingo.
const DefaultFotingoBanner = "🚀 PR created with [fotingo](https://github.com/tagoro9/fotingo)"
