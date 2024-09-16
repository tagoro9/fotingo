package completion

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tagoro9/fotingo/internal/tracker"
)

// ReviewMatchOption is the completion helper view over resolver matches.
type ReviewMatchOption struct {
	Resolved string
	Label    string
	Detail   string
	Fields   []string
	Kind     string
}

const (
	// ReviewMatchKindUser identifies user review-matching options.
	ReviewMatchKindUser = "user"
	// ReviewMatchKindTeam identifies team review-matching options.
	ReviewMatchKindTeam = "team"
)

// CompleteReviewMatchOptions returns sorted CSV-aware completion candidates.
func CompleteReviewMatchOptions(
	toComplete string,
	options []ReviewMatchOption,
	matchFinder func(token string, options []ReviewMatchOption) []ReviewMatchOption,
) []string {
	prefix, token := SplitCSVCompletionToken(toComplete)
	candidates := make([]string, 0, len(options))
	if strings.TrimSpace(token) == "" {
		for _, option := range options {
			candidates = append(candidates, FormatReviewCompletionCandidate(option))
		}
	} else {
		matches := matchFinder(token, options)
		for _, match := range matches {
			candidates = append(candidates, FormatReviewCompletionCandidate(match))
		}
	}

	candidates = dedupeStringsPreserveOrder(candidates)
	sort.Strings(candidates)
	return ApplyCSVCompletionPrefix(prefix, candidates)
}

// FormatReviewCompletionCandidate formats completion value plus optional detail.
func FormatReviewCompletionCandidate(option ReviewMatchOption) string {
	value := strings.TrimSpace(option.Resolved)
	detail := strings.TrimSpace(option.Detail)
	if option.Kind != ReviewMatchKindUser && option.Kind != ReviewMatchKindTeam {
		return value
	}
	if detail == "" || strings.EqualFold(value, detail) {
		return value
	}
	return fmt.Sprintf("%s\t%s", value, detail)
}

// SplitCSVCompletionToken returns the existing CSV prefix and token currently being completed.
func SplitCSVCompletionToken(input string) (string, string) {
	index := strings.LastIndex(input, ",")
	if index < 0 {
		return "", strings.TrimSpace(input)
	}
	return input[:index+1], strings.TrimSpace(input[index+1:])
}

// ApplyCSVCompletionPrefix reapplies CSV prefix to completion candidates.
func ApplyCSVCompletionPrefix(prefix string, values []string) []string {
	if prefix == "" {
		return values
	}
	withPrefix := make([]string, 0, len(values))
	for _, value := range values {
		mainValue, description, hasDescription := strings.Cut(value, "\t")
		if hasDescription {
			withPrefix = append(withPrefix, prefix+mainValue+"\t"+description)
			continue
		}
		withPrefix = append(withPrefix, prefix+value)
	}
	return withPrefix
}

// CollectProjectsFromIssues collects unique project keys from issue IDs.
func CollectProjectsFromIssues(issues []tracker.Issue) []string {
	projects := make([]string, 0, len(issues))
	for _, issue := range issues {
		key := strings.TrimSpace(issue.Key)
		if key == "" {
			continue
		}
		parts := strings.SplitN(key, "-", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		projects = append(projects, strings.ToUpper(parts[0]))
	}
	projects = dedupeStringsPreserveOrder(projects)
	sort.Strings(projects)
	return projects
}

// FilterByContainsFold filters values by case-insensitive contains matching.
func FilterByContainsFold(values []string, query string) []string {
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery == "" {
		return values
	}

	filtered := make([]string, 0, len(values))
	for _, value := range values {
		normalizedValue := strings.ToLower(strings.TrimSpace(value))
		if strings.Contains(normalizedValue, normalizedQuery) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func dedupeStringsPreserveOrder(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
