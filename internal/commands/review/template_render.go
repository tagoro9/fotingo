package review

import (
	"strings"

	"github.com/tagoro9/fotingo/internal/template"
)

var legacyManagedSectionPlaceholders = map[string]string{
	ManagedSectionSummary:     template.PlaceholderSummary,
	ManagedSectionDescription: template.PlaceholderDescription,
	ManagedSectionFixedIssues: template.PlaceholderFixedIssues,
	ManagedSectionChanges:     template.PlaceholderChanges,
}

// RenderTemplate renders a review template using marker-only managed sections
// when possible, while keeping legacy managed placeholders working for
// backward compatibility.
func RenderTemplate(content string, data map[string]string) (string, bool, error) {
	rendered := template.New(content).Render(data)
	usedLegacyManagedPlaceholders := false

	for _, section := range ManagedSections() {
		placeholderName, ok := legacyManagedSectionPlaceholders[section]
		if !ok {
			continue
		}

		if strings.Contains(content, "{"+placeholderName+"}") {
			usedLegacyManagedPlaceholders = true
			continue
		}

		rawSectionContent, err := ExtractManagedSectionContent(content, section)
		if err != nil {
			continue
		}
		if strings.TrimSpace(rawSectionContent) != "" {
			continue
		}

		replacement := managedSectionReplacement(data[placeholderName])
		rendered, err = ReplaceManagedSectionContent(rendered, section, replacement)
		if err != nil {
			return "", false, err
		}
	}

	return rendered, usedLegacyManagedPlaceholders, nil
}

func managedSectionReplacement(content string) string {
	trimmed := strings.Trim(content, "\n")
	if trimmed == "" {
		return "\n"
	}

	return "\n" + trimmed + "\n"
}
