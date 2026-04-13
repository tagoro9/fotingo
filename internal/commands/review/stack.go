package review

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tagoro9/fotingo/internal/github"
)

const (
	StackedPRsSection = "stacked-prs"

	stackStatusOpen    = "open"
	stackStatusClosed  = "closed"
	stackStatusMerged  = "merged"
	stackStatusUnknown = "unknown"
)

var (
	stackIDPattern      = regexp.MustCompile(`<!--\s*fotingo:stack\s+id="((?:\\"|[^"])*)"\s+version="1"\s*-->`)
	stackIssueIDPattern = regexp.MustCompile(`\b([A-Z][A-Z0-9_]+-\d+)\b`)
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
	builder.WriteString("| Order | Jira | Pull request |\n")
	builder.WriteString("| --- | --- | --- |\n")
	for index, item := range opts.Items {
		fmt.Fprintf(&builder, "| %s | %s | %s |\n",
			StackOrderLabel(index+1, item.Current),
			formatStackJiraLink(item),
			formatStackPRLink(item))
	}
	builder.WriteString("\n")
	return builder.String()
}

// ExtractStackID returns the fotingo stack id embedded in a PR body.
func ExtractStackID(body string) string {
	matches := stackIDPattern.FindStringSubmatch(body)
	if len(matches) < 2 {
		return ""
	}
	value := strings.ReplaceAll(matches[1], `\"`, `"`)
	return strings.ReplaceAll(value, `\\`, `\`)
}

// StackIDForRootPR derives a stable stack id from the root pull request URL.
func StackIDForRootPR(prNumber int, htmlURL string) string {
	htmlURL = strings.TrimSpace(htmlURL)
	if htmlURL == "" {
		return fmt.Sprintf("pr-%d", prNumber)
	}

	trimmed := strings.TrimSuffix(htmlURL, "/")
	marker := "/pull/"
	index := strings.LastIndex(trimmed, marker)
	if index < 0 {
		return fmt.Sprintf("pr-%d", prNumber)
	}

	repoURL := strings.TrimPrefix(trimmed[:index], "https://github.com/")
	repoURL = strings.TrimPrefix(repoURL, "http://github.com/")
	if repoURL == "" {
		return fmt.Sprintf("pr-%d", prNumber)
	}
	return fmt.Sprintf("%s#%d", repoURL, prNumber)
}

// DeriveStackJiraKey extracts a Jira issue key from PR metadata.
func DeriveStackJiraKey(values ...string) string {
	for _, value := range values {
		matches := stackIssueIDPattern.FindStringSubmatch(strings.ToUpper(value))
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

// OrderStackPullRequests orders a linear stack from root to leaf.
func OrderStackPullRequests(members []github.PullRequest) ([]github.PullRequest, error) {
	if len(members) <= 1 {
		return append([]github.PullRequest(nil), members...), nil
	}

	headBranches := make(map[string]struct{}, len(members))
	byBaseBranch := make(map[string][]github.PullRequest, len(members))
	byNumber := make(map[int]github.PullRequest, len(members))
	for _, member := range members {
		byNumber[member.Number] = member
		if head := strings.TrimSpace(member.HeadRef); head != "" {
			headBranches[head] = struct{}{}
		}
		if base := strings.TrimSpace(member.BaseRef); base != "" {
			byBaseBranch[base] = append(byBaseBranch[base], member)
		}
	}

	var roots []github.PullRequest
	for _, member := range byNumber {
		if _, ok := headBranches[strings.TrimSpace(member.BaseRef)]; !ok {
			roots = append(roots, member)
		}
	}
	if len(roots) != 1 {
		return nil, fmt.Errorf("stack must be a single linear chain, found %d roots", len(roots))
	}

	ordered := make([]github.PullRequest, 0, len(members))
	seen := map[int]struct{}{}
	current := roots[0]
	for {
		if _, ok := seen[current.Number]; ok {
			return nil, fmt.Errorf("stack contains a cycle at pull request #%d", current.Number)
		}
		seen[current.Number] = struct{}{}
		ordered = append(ordered, current)

		children := byBaseBranch[strings.TrimSpace(current.HeadRef)]
		filtered := children[:0]
		for _, child := range children {
			if child.Number != current.Number {
				filtered = append(filtered, child)
			}
		}
		if len(filtered) == 0 {
			break
		}
		if len(filtered) > 1 {
			return nil, fmt.Errorf("branching stacks are not supported: pull request #%d has %d children", current.Number, len(filtered))
		}
		current = filtered[0]
	}

	if len(ordered) != len(members) {
		return nil, fmt.Errorf("stack must be a single linear chain, ordered %d of %d pull requests", len(ordered), len(members))
	}
	return ordered, nil
}

// StackStatusEmoji returns emoji-only PR state for stack JSON output.
func StackStatusEmoji(item StackPullRequest) string {
	return stackBaseStatusEmoji(item)
}

// StackOrderLabel returns the order cell shown in stack tables.
func StackOrderLabel(order int, current bool) string {
	label := fmt.Sprintf("%d", order)
	if current {
		return label + " 👉"
	}
	return label + "  "
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
