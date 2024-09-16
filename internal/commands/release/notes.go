package release

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/template"
	"github.com/tagoro9/fotingo/internal/tracker"
)

// NotesText carries localized release-note strings used by rendering helpers.
type NotesText struct {
	NoIssues              string
	HeadingCategoryFormat string
	IssueBulletFormat     string
	CategoryBugFixes      string
	CategoryFeatures      string
	CategoryTasks         string
	CategorySubtasks      string
	CategoryEpics         string
}

// FetchIssueDetails fetches issue details for release-note categorization.
func FetchIssueDetails(jiraClient jira.Jira, issueIDs []string) ([]*tracker.Issue, error) {
	var issues []*tracker.Issue
	var lastErr error

	for _, id := range issueIDs {
		issue, err := jiraClient.GetIssue(id)
		if err != nil {
			lastErr = err
			continue
		}
		issues = append(issues, issue)
	}

	return issues, lastErr
}

// BuildReleaseNotes renders release notes using the provided template and localized text.
func BuildReleaseNotes(
	releaseName string,
	issues []*tracker.Issue,
	jiraRelease *tracker.Release,
	jiraClient jira.Jira,
	releaseTemplate string,
	text NotesText,
) string {
	tmpl := template.New(releaseTemplate)

	data := map[string]string{
		template.PlaceholderVersion:               releaseName,
		template.PlaceholderFotingoBanner:         template.DefaultFotingoBanner,
		template.PlaceholderFixedIssuesByCategory: FormatIssuesByCategory(issues, jiraClient, text),
	}

	if jiraRelease != nil {
		data[template.PlaceholderJiraRelease] = jiraRelease.URL
	}

	return tmpl.Render(data)
}

// FormatIssuesByCategory groups and renders issue bullets grouped by issue type.
func FormatIssuesByCategory(issues []*tracker.Issue, jiraClient jira.Jira, text NotesText) string {
	if len(issues) == 0 {
		return text.NoIssues
	}

	grouped := make(map[tracker.IssueType][]*tracker.Issue)
	for _, issue := range issues {
		grouped[issue.Type] = append(grouped[issue.Type], issue)
	}

	categoryOrder := []tracker.IssueType{
		tracker.IssueTypeBug,
		tracker.IssueTypeStory,
		tracker.IssueTypeTask,
		tracker.IssueTypeSubTask,
		tracker.IssueTypeEpic,
	}
	categoryLabels := map[tracker.IssueType]string{
		tracker.IssueTypeBug:     text.CategoryBugFixes,
		tracker.IssueTypeStory:   text.CategoryFeatures,
		tracker.IssueTypeTask:    text.CategoryTasks,
		tracker.IssueTypeSubTask: text.CategorySubtasks,
		tracker.IssueTypeEpic:    text.CategoryEpics,
	}

	var sb strings.Builder

	for _, issueType := range categoryOrder {
		issueList, ok := grouped[issueType]
		if !ok || len(issueList) == 0 {
			continue
		}

		sort.Slice(issueList, func(i, j int) bool {
			return issueList[i].Key < issueList[j].Key
		})

		label := categoryLabels[issueType]
		fmt.Fprintf(&sb, text.HeadingCategoryFormat, label)

		for _, issue := range issueList {
			url := issue.URL
			if url == "" && jiraClient != nil {
				url = jiraClient.GetIssueURL(issue.Key)
			}
			fmt.Fprintf(&sb, text.IssueBulletFormat, issue.Key, url, issue.Summary)
		}
		sb.WriteString("\n")
	}

	return strings.TrimSuffix(sb.String(), "\n")
}
