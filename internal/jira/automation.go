package jira

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/spf13/viper"
)

const (
	configLabelsInProgress       = "tracker.labels.inProgress"
	configLabelsInReview         = "tracker.labels.inReview"
	configPullRequestCommentBody = "tracker.comments.pullRequestCreated"
)

type labelAdder interface {
	AddLabels(issueID string, labels []string) error
}

// ApplyLabels adds configured and command-supplied labels for a status transition
// when the Jira client supports label updates.
func ApplyLabels(client Jira, cfg *viper.Viper, issueID string, status IssueStatus, commandLabels []string) error {
	labels := mergeLabels(LabelsForStatus(cfg, status), commandLabels)
	if len(labels) == 0 {
		return nil
	}
	adder, ok := client.(labelAdder)
	if !ok {
		return nil
	}
	return adder.AddLabels(issueID, labels)
}

func mergeLabels(groups ...[]string) []string {
	labels := make([]string, 0)
	seen := make(map[string]struct{})
	for _, group := range groups {
		for _, rawLabel := range group {
			label := strings.TrimSpace(rawLabel)
			if label == "" {
				continue
			}
			normalized := strings.ToLower(label)
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			labels = append(labels, label)
		}
	}
	return labels
}

// LabelsForStatus returns the configured labels for an issue-tracker workflow transition.
func LabelsForStatus(cfg *viper.Viper, status IssueStatus) []string {
	if cfg == nil {
		return nil
	}
	key := configLabelsInProgress
	if status == StatusInReview {
		key = configLabelsInReview
	}
	parts := strings.Split(cfg.GetString(key), ",")
	labels := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		label := strings.TrimSpace(part)
		if label == "" {
			continue
		}
		normalized := strings.ToLower(label)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		labels = append(labels, label)
	}
	return labels
}

// RenderPullRequestComment renders the configured issue-tracker comment for a newly created pull request.
// An empty template disables the automatic issue-tracker comment.
func RenderPullRequestComment(cfg *viper.Viper, issue *Issue, pullRequestURL string, override *string) (string, error) {
	commentTemplate := pullRequestCommentTemplate(cfg, override)
	if commentTemplate == "" {
		return "", nil
	}
	tmpl, err := template.New("tracker-comment").Option("missingkey=error").Parse(commentTemplate)
	if err != nil {
		return "", err
	}
	var output bytes.Buffer
	err = tmpl.Execute(&output, map[string]any{
		"Issue":       issue,
		"PullRequest": map[string]string{"URL": pullRequestURL},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output.String()), nil
}

// PreparePullRequestComments validates comment rendering using fetched issue data before PR mutations.
// The supplied URL may be a placeholder when the PR has not been created yet.
func PreparePullRequestComments(client Jira, cfg *viper.Viper, keys []string, known *Issue, url string, override *string) (map[string]*Issue, error) {
	issues := make(map[string]*Issue, len(keys))
	if pullRequestCommentTemplate(cfg, override) == "" {
		return issues, nil
	}
	for _, key := range keys {
		issue, err := issueForPullRequestComment(client, key, known)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch %s for issue-tracker comment: %w", key, err)
		}
		if _, err := RenderPullRequestComment(cfg, issue, url, override); err != nil {
			return nil, fmt.Errorf("invalid issue-tracker comment template for %s: %w", key, err)
		}
		issues[key] = issue
	}
	return issues, nil
}

// issueForPullRequestComment retrieves issue data independently of status transitions.
func issueForPullRequestComment(client Jira, key string, known *Issue) (*Issue, error) {
	if known != nil && strings.EqualFold(known.Key, key) {
		return known, nil
	}
	issue, err := client.GetJiraIssue(key)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, fmt.Errorf("no issue data returned for %s", key)
	}
	return issue, nil
}

func pullRequestCommentTemplate(cfg *viper.Viper, override *string) string {
	if cfg == nil {
		return ""
	}
	commentTemplate := strings.TrimSpace(cfg.GetString(configPullRequestCommentBody))
	if override != nil {
		commentTemplate = strings.TrimSpace(*override)
	}
	if override == nil && !cfg.IsSet(configPullRequestCommentBody) {
		commentTemplate = "Pull request created: {{.PullRequest.URL}}"
	}
	return commentTemplate
}
