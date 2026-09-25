package jira

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/config"
)

func TestLabelsForStatus(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Set("tracker.labels.inProgress", " active, backend,Active, ")
	cfg.Set("tracker.labels.inReview", " review, backend ")

	assert.Equal(t, []string{"active", "backend"}, LabelsForStatus(cfg, StatusInProgress))
	assert.Equal(t, []string{"review", "backend"}, LabelsForStatus(cfg, StatusInReview))
}

func TestRenderPullRequestComment(t *testing.T) {
	cfg := config.NewDefaultConfig()
	issue := &Issue{Key: "FOTINGO-53", Summary: "Configure tracker comments"}

	comment, err := RenderPullRequestComment(cfg, issue, "https://github.com/tagoro9/fotingo/pull/53", nil)
	require.NoError(t, err)
	assert.Equal(t, "Pull request created: https://github.com/tagoro9/fotingo/pull/53", comment)

	cfg.Set("tracker.comments.pullRequestCreated", "{{.Issue.Key}}: {{.PullRequest.URL}}")
	comment, err = RenderPullRequestComment(cfg, issue, "https://github.com/tagoro9/fotingo/pull/53", nil)
	require.NoError(t, err)
	assert.Equal(t, "FOTINGO-53: https://github.com/tagoro9/fotingo/pull/53", comment)

	cfg.Set("tracker.comments.pullRequestCreated", "")
	comment, err = RenderPullRequestComment(cfg, issue, "https://github.com/tagoro9/fotingo/pull/53", nil)
	require.NoError(t, err)
	assert.Empty(t, comment)

	override := "Review: {{.PullRequest.URL}}"
	comment, err = RenderPullRequestComment(cfg, issue, "https://github.com/tagoro9/fotingo/pull/53", &override)
	require.NoError(t, err)
	assert.Equal(t, "Review: https://github.com/tagoro9/fotingo/pull/53", comment)
}

func TestMergeLabels(t *testing.T) {
	assert.Equal(t, []string{"active", "backend", "review"}, mergeLabels(
		[]string{"active", "backend"},
		[]string{"Backend", "review", ""},
	))
}
