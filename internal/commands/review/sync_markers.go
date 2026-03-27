package review

import (
	"fmt"
	"strings"
)

const (
	ManagedSectionSummary     = "summary"
	ManagedSectionDescription = "description"
	ManagedSectionFixedIssues = "fixed-issues"
	ManagedSectionChanges     = "changes"
)

var managedSectionOrder = []string{
	ManagedSectionSummary,
	ManagedSectionDescription,
	ManagedSectionFixedIssues,
	ManagedSectionChanges,
}

// ManagedSections returns the supported fotingo-managed PR section ids.
func ManagedSections() []string {
	return append([]string(nil), managedSectionOrder...)
}

// NormalizeManagedSections validates and normalizes requested managed section ids.
// When no sections are requested, all known sections are returned.
func NormalizeManagedSections(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return ManagedSections(), nil
	}

	normalized := make([]string, 0, len(requested))
	seen := map[string]struct{}{}
	for _, section := range requested {
		candidate := strings.ToLower(strings.TrimSpace(section))
		if candidate == "" {
			continue
		}
		if !isManagedSection(candidate) {
			return nil, fmt.Errorf("unsupported review sync section %q", section)
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		normalized = append(normalized, candidate)
	}

	if len(normalized) == 0 {
		return nil, fmt.Errorf("at least one non-empty review sync section is required")
	}

	return normalized, nil
}

func isManagedSection(section string) bool {
	for _, candidate := range managedSectionOrder {
		if candidate == section {
			return true
		}
	}
	return false
}

// ManagedSectionMarkers returns the start/end marker pair for a section.
func ManagedSectionMarkers(section string) (string, string) {
	normalized := strings.ToLower(strings.TrimSpace(section))
	return managedSectionStartMarker(normalized), managedSectionEndMarker(normalized)
}

func managedSectionStartMarker(section string) string {
	return fmt.Sprintf("<!-- fotingo:start %s -->", section)
}

func managedSectionEndMarker(section string) string {
	return fmt.Sprintf("<!-- fotingo:end %s -->", section)
}

// ExtractManagedSectionContent returns the content between the marker pair for a section.
func ExtractManagedSectionContent(body string, section string) (string, error) {
	start, _, startIndex, endIndex, err := managedSectionRange(body, section)
	if err != nil {
		return "", err
	}

	return body[startIndex+len(start) : endIndex], nil
}

// ReplaceManagedSectionContent replaces the content between a section's markers.
func ReplaceManagedSectionContent(body string, section string, replacement string) (string, error) {
	start, _, startIndex, endIndex, err := managedSectionRange(body, section)
	if err != nil {
		return "", err
	}

	contentStart := startIndex + len(start)
	return body[:contentStart] + replacement + body[endIndex:], nil
}

func managedSectionRange(body string, section string) (string, string, int, int, error) {
	normalized := strings.ToLower(strings.TrimSpace(section))
	if !isManagedSection(normalized) {
		return "", "", 0, 0, fmt.Errorf("unsupported review sync section %q", section)
	}

	start := managedSectionStartMarker(normalized)
	end := managedSectionEndMarker(normalized)
	startIndex := strings.Index(body, start)
	if startIndex < 0 {
		return "", "", 0, 0, fmt.Errorf("missing fotingo markers for section %q", normalized)
	}

	searchFrom := startIndex + len(start)
	relativeEndIndex := strings.Index(body[searchFrom:], end)
	if relativeEndIndex < 0 {
		return "", "", 0, 0, fmt.Errorf("missing fotingo markers for section %q", normalized)
	}

	endIndex := searchFrom + relativeEndIndex
	return start, end, startIndex, endIndex, nil
}
