package review

import (
	"fmt"
	"strings"
)

const (
	StackedPRsSection = "stacked-prs"

	stackStatusOpen    = "open"
	stackStatusClosed  = "closed"
	stackStatusMerged  = "merged"
	stackStatusUnknown = "unknown"
)

// StackPullRequest contains the PR metadata rendered in a stack table.
type StackPullRequest struct {
	Number  int
	Title   string
	HTMLURL string
	JiraKey string
	JiraURL string
	State   string
	Draft   bool
	Merged  bool
	Current bool
}

// StackRenderOptions configures stacked PR section rendering.
type StackRenderOptions struct {
	StackID string
	Items   []StackPullRequest
}

// StackedPRSectionMarkers returns the hidden marker pair for stack content.
func StackedPRSectionMarkers() (string, string) {
	return managedSectionStartMarker(StackedPRsSection), managedSectionEndMarker(StackedPRsSection)
}

// ExtractStackedPRSectionContent returns the content between the stack marker pair.
func ExtractStackedPRSectionContent(body string) (string, error) {
	return extractMarkedSectionContent(body, StackedPRsSection)
}

// ReplaceStackedPRSectionContent replaces the content between the stack marker pair.
func ReplaceStackedPRSectionContent(body string, replacement string) (string, error) {
	return replaceMarkedSectionContent(body, StackedPRsSection, replacement)
}

// RenderStackedPRSection renders the managed stacked PR section body.
func RenderStackedPRSection(opts StackRenderOptions) string {
	if len(opts.Items) == 0 {
		return "\n"
	}

	var builder strings.Builder
	builder.WriteString("\n")
	if stackID := strings.TrimSpace(opts.StackID); stackID != "" {
		fmt.Fprintf(&builder, "<!-- fotingo:stack id=\"%s\" version=\"1\" -->\n\n", escapeStackMetadataValue(stackID))
	}
	builder.WriteString("**Stacked PRs**\n\n")
	builder.WriteString("| Order | Jira | Pull request | Status |\n")
	builder.WriteString("| --- | --- | --- | --- |\n")
	for index, item := range opts.Items {
		fmt.Fprintf(&builder, "| %d | %s | %s | %s |\n",
			index+1,
			formatStackJiraLink(item),
			formatStackPRLink(item),
			StackStatusEmoji(item))
	}
	builder.WriteString("\n")
	return builder.String()
}

// StackStatusEmoji returns emoji-only display state for a stack table row.
func StackStatusEmoji(item StackPullRequest) string {
	status := stackBaseStatusEmoji(item)
	if item.Current {
		status = strings.TrimSpace(status + " 👀")
	}
	return status
}

func stackBaseStatusEmoji(item StackPullRequest) string {
	switch {
	case item.Merged:
		return "🟣"
	case item.Draft:
		return "📝"
	}

	switch strings.ToLower(strings.TrimSpace(item.State)) {
	case stackStatusMerged:
		return "🟣"
	case stackStatusClosed:
		return "🔴"
	case stackStatusOpen:
		return "🟢"
	case "", stackStatusUnknown:
		return "⚪"
	default:
		return "⚪"
	}
}

func formatStackJiraLink(item StackPullRequest) string {
	key := strings.TrimSpace(item.JiraKey)
	if key == "" {
		return "-"
	}
	if url := strings.TrimSpace(item.JiraURL); url != "" {
		return fmt.Sprintf("[%s](%s)", escapeMarkdownTableCell(key), url)
	}
	return escapeMarkdownTableCell(key)
}

func formatStackPRLink(item StackPullRequest) string {
	label := fmt.Sprintf("#%d", item.Number)
	if title := strings.TrimSpace(item.Title); title != "" {
		label = fmt.Sprintf("#%d %s", item.Number, title)
	}
	if url := strings.TrimSpace(item.HTMLURL); url != "" {
		return fmt.Sprintf("[%s](%s)", escapeMarkdownTableCell(label), url)
	}
	return escapeMarkdownTableCell(label)
}

func extractMarkedSectionContent(body string, section string) (string, error) {
	start, _, startIndex, endIndex, err := markedSectionRange(body, section)
	if err != nil {
		return "", err
	}

	return body[startIndex+len(start) : endIndex], nil
}

func replaceMarkedSectionContent(body string, section string, replacement string) (string, error) {
	start, _, startIndex, endIndex, err := markedSectionRange(body, section)
	if err != nil {
		return "", err
	}

	contentStart := startIndex + len(start)
	return body[:contentStart] + replacement + body[endIndex:], nil
}

func markedSectionRange(body string, section string) (string, string, int, int, error) {
	normalized := strings.ToLower(strings.TrimSpace(section))
	if normalized == "" {
		return "", "", 0, 0, fmt.Errorf("missing fotingo markers for section %q", normalized)
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

func escapeStackMetadataValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func escapeMarkdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.ReplaceAll(value, "|", "\\|")
}
