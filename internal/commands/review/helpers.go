package review

import (
	"fmt"
	"strings"

	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/template"
)

// TemplateOptions configures review template placeholder overrides.
type TemplateOptions struct {
	TitleOverride         string
	TemplateSummary       string
	TemplateDescription   string
	EmptyPlaceholderValue string
	LinkedIssueIDs        []string
}

// BuildTemplateData returns template placeholder values for review PR bodies.
func BuildTemplateData(
	branch string,
	issue *jira.Issue,
	jiraClient jira.Jira,
	commits []git.Commit,
	opts TemplateOptions,
) map[string]string {
	empty := opts.EmptyPlaceholderValue
	data := map[string]string{
		template.PlaceholderBranchName:    branch,
		template.PlaceholderSummary:       DeriveSummary(branch, issue, commits),
		template.PlaceholderDescription:   DeriveDescription(issue, commits, empty),
		template.PlaceholderFixedIssues:   empty,
		template.PlaceholderChanges:       FormatChanges(commits),
		template.PlaceholderFotingoBanner: template.DefaultFotingoBanner,
	}

	if issue != nil {
		data[template.PlaceholderIssueKey] = issue.Key
		data[template.PlaceholderIssueSummary] = issue.Summary
		data[template.PlaceholderIssueDescription] = issue.Description
		data[template.PlaceholderIssueURL] = jiraIssueURL(jiraClient, issue.Key)
	}

	linkedIssueIDs := opts.LinkedIssueIDs
	if len(linkedIssueIDs) == 0 && issue != nil && strings.TrimSpace(issue.Key) != "" {
		linkedIssueIDs = []string{issue.Key}
	}
	if fixedIssues := FormatFixedIssues(linkedIssueIDs, jiraClient); fixedIssues != "" {
		data[template.PlaceholderFixedIssues] = fixedIssues
	}

	if opts.TemplateSummary != "" {
		data[template.PlaceholderSummary] = NormalizeTemplateOverride(opts.TemplateSummary)
	}
	if opts.TemplateDescription != "" {
		data[template.PlaceholderDescription] = NormalizeTemplateOverride(opts.TemplateDescription)
	}

	return data
}

// NormalizeTemplateOverride expands escaped formatting sequences used in CLI
// flag values and normalizes the final content to LF line endings.
func NormalizeTemplateOverride(content string) string {
	if content == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(content))

	for index := 0; index < len(content); index++ {
		current := content[index]
		if current != '\\' || index == len(content)-1 {
			builder.WriteByte(current)
			continue
		}

		next := content[index+1]
		switch next {
		case '\\':
			builder.WriteByte('\\')
			index++
		case 'n':
			builder.WriteByte('\n')
			index++
		case 'r':
			if index+2 < len(content) && content[index+2] == 'n' {
				builder.WriteByte('\n')
				index += 2
				continue
			}
			builder.WriteByte('\n')
			index++
		case 't':
			builder.WriteByte('\t')
			index++
		default:
			builder.WriteByte(current)
		}
	}

	return NormalizeLineEndings(builder.String())
}

// CollectLinkedIssueIDs keeps the branch issue first and appends commit-linked
// issues in first-seen order without duplicates.
func CollectLinkedIssueIDs(issue *jira.Issue, commitIssueIDs []string) []string {
	linked := make([]string, 0, len(commitIssueIDs)+1)
	if issue != nil && strings.TrimSpace(issue.Key) != "" {
		linked = append(linked, strings.TrimSpace(issue.Key))
	}
	for _, issueID := range commitIssueIDs {
		issueID = strings.TrimSpace(issueID)
		if issueID == "" {
			continue
		}
		linked = append(linked, issueID)
	}
	return DedupeStringsPreserveOrder(linked)
}

// FormatFixedIssues renders the fixed-issues template content for one or more issues.
func FormatFixedIssues(issueIDs []string, jiraClient jira.Jira) string {
	lines := make([]string, 0, len(issueIDs))
	for _, issueID := range DedupeStringsPreserveOrder(issueIDs) {
		issueID = strings.TrimSpace(issueID)
		if issueID == "" {
			continue
		}

		issueReference := issueID
		if issueURL := jiraIssueURL(jiraClient, issueID); issueURL != "" {
			issueReference = fmt.Sprintf("[%s](%s)", issueID, issueURL)
		}
		lines = append(lines, "Fixes "+issueReference)
	}

	return strings.Join(lines, "\n")
}

func jiraIssueURL(jiraClient jira.Jira, issueID string) string {
	if jiraClient == nil {
		return ""
	}
	return jiraClient.GetIssueURL(issueID)
}

// BuildDefaultTitle derives the default PR title for a branch and optional issue.
func BuildDefaultTitle(branch string, issue *jira.Issue) string {
	if issue != nil {
		return fmt.Sprintf("[%s] %s", issue.Key, issue.Summary)
	}
	return branch
}

// DerivePRTitle returns the final PR title after applying overrides and editor content.
func DerivePRTitle(
	titleOverride string,
	branch string,
	issue *jira.Issue,
	editorTitle string,
	editorMode bool,
) string {
	if titleOverride != "" {
		return titleOverride
	}

	if editorMode {
		title := strings.TrimSpace(editorTitle)
		if title != "" {
			return title
		}
	}

	return BuildDefaultTitle(branch, issue)
}

// DeriveEditorTitle extracts the first line title from editor content.
func DeriveEditorTitle(content string) string {
	firstLine, _, _ := strings.Cut(NormalizeLineEndings(content), "\n")
	return strings.TrimSpace(firstLine)
}

// SplitEditorContent returns title/body parts from editor content.
func SplitEditorContent(content string) (string, string) {
	normalized := NormalizeLineEndings(content)
	firstLine, rest, found := strings.Cut(normalized, "\n")
	title := strings.TrimSpace(firstLine)

	if !found {
		return title, ""
	}

	return title, strings.TrimLeft(rest, "\n")
}

// BuildEditorSeedContent formats the interactive editor input so line 1 is the
// editable PR title and the remaining content is the PR body.
func BuildEditorSeedContent(title string, body string) string {
	title = strings.TrimSpace(NormalizeLineEndings(title))
	body = NormalizeLineEndings(body)

	switch {
	case title == "":
		return body
	case body == "":
		return title
	default:
		return title + "\n\n" + body
	}
}

// DeriveSummary computes the PR summary placeholder value.
func DeriveSummary(branch string, issue *jira.Issue, commits []git.Commit) string {
	if issue != nil {
		return TakePrefix(fmt.Sprintf("%s: %s", issue.Key, issue.Summary), 100)
	}

	header, _ := OldestCommitHeaderAndBody(commits)
	if header != "" {
		return header
	}

	return branch
}

// DeriveDescription computes the PR description placeholder value.
func DeriveDescription(issue *jira.Issue, commits []git.Commit, emptyPlaceholder string) string {
	if issue != nil {
		return NormalizeLineEndings(issue.Description)
	}

	_, body := OldestCommitHeaderAndBody(commits)
	if body != "" {
		return body
	}

	return emptyPlaceholder
}

// FormatChanges formats commit headers as markdown bullet list including line stats.
func FormatChanges(commits []git.Commit) string {
	var changes []string
	for i := len(commits) - 1; i >= 0; i-- {
		header, _ := SplitCommitMessage(commits[i].Message)
		if header == "" {
			continue
		}
		changes = append(
			changes,
			fmt.Sprintf(
				"* %s (+%d/-%d)",
				header,
				commits[i].Additions,
				commits[i].Deletions,
			),
		)
	}

	return strings.Join(changes, "\n")
}

// OldestCommitHeaderAndBody returns the oldest commit header and body from branch commits.
func OldestCommitHeaderAndBody(commits []git.Commit) (string, string) {
	if len(commits) == 0 {
		return "", ""
	}

	return SplitCommitMessage(commits[len(commits)-1].Message)
}

// SplitCommitMessage splits a commit message into header/body parts.
func SplitCommitMessage(message string) (string, string) {
	normalized := strings.TrimRight(NormalizeLineEndings(message), "\n")
	if normalized == "" {
		return "", ""
	}

	header, rest, found := strings.Cut(normalized, "\n")
	header = strings.TrimSpace(header)
	if !found {
		return header, ""
	}

	return header, strings.TrimLeft(rest, "\n")
}

// NormalizeLineEndings normalizes CRLF content to LF.
func NormalizeLineEndings(content string) string {
	return strings.ReplaceAll(content, "\r\n", "\n")
}

// TakePrefix returns up to n runes from content.
func TakePrefix(content string, n int) string {
	if n <= 0 {
		return ""
	}

	runes := []rune(content)
	if len(runes) <= n {
		return content
	}

	return string(runes[:n])
}
