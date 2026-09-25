package jira

import (
	"bytes"
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
	if cfg == nil {
		return "", nil
	}
	commentTemplate := strings.TrimSpace(cfg.GetString(configPullRequestCommentBody))
	if override != nil {
		commentTemplate = strings.TrimSpace(*override)
	}
	if override == nil && !cfg.IsSet(configPullRequestCommentBody) {
		commentTemplate = "Pull request created: {{.PullRequest.URL}}"
	}
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
