package start

import (
	"strings"

	"github.com/tagoro9/fotingo/internal/tracker"
)

// ParseIssueKind maps the CLI kind value to a tracker issue type.
func ParseIssueKind(kind string) (tracker.IssueType, bool) {
	switch strings.ToLower(kind) {
	case "story":
		return tracker.IssueTypeStory, true
	case "bug":
		return tracker.IssueTypeBug, true
	case "task":
		return tracker.IssueTypeTask, true
	case "subtask", "sub-task":
		return tracker.IssueTypeSubTask, true
	case "epic":
		return tracker.IssueTypeEpic, true
	default:
		return "", false
	}
}

// ParseInteractiveLabels parses comma-separated labels from interactive prompts.
func ParseInteractiveLabels(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	parts := strings.Split(raw, ",")
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized := strings.TrimSpace(part)
		if normalized != "" {
			labels = append(labels, normalized)
		}
	}
	return labels
}

// IssueMatchesAllowedTypes reports whether the issue type is allowed.
func IssueMatchesAllowedTypes(issueType tracker.IssueType, allowed []tracker.IssueType) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if issueType == candidate {
			return true
		}
	}
	return false
}

// ProjectIssueTypeNames extracts non-empty issue-type names.
func ProjectIssueTypeNames(issueTypes []tracker.ProjectIssueType) []string {
	names := make([]string, 0, len(issueTypes))
	for _, issueType := range issueTypes {
		issueTypeName := strings.TrimSpace(issueType.Name)
		if issueTypeName == "" {
			continue
		}
		names = append(names, issueTypeName)
	}
	return names
}
