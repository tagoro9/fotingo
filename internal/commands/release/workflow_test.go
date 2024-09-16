package release

import (
	"errors"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

func TestWorkflowRunnerRun_ValidatesDeps(t *testing.T) {
	runner := WorkflowRunner{}
	err := runner.Run(nil, nil, "v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NewGitClient")
}

func TestWorkflowRunnerRun_WrapsGitInitError(t *testing.T) {
	expected := errors.New("git unavailable")
	runner := WorkflowRunner{
		Config: viper.New(),
		Localize: func(key i18n.Key, args ...any) string {
			if key == i18n.ReleaseErrInitGit {
				return "failed to init git: %v"
			}
			return i18n.T(key, args...)
		},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return nil, expected
			},
			NewJiraClient: func(*viper.Viper) (jira.Jira, error) {
				return nil, nil
			},
			NewGitHubClient: func(git.Git, *viper.Viper) (github.Github, error) {
				return nil, nil
			},
			FetchIssueDetails: func(jira.Jira, []string) ([]*tracker.Issue, error) {
				return nil, nil
			},
			BuildReleaseNotes: func(string, []*tracker.Issue, *tracker.Release, jira.Jira) string {
				return ""
			},
		},
	}

	err := runner.Run(nil, nil, "v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to init git")
	assert.Contains(t, err.Error(), expected.Error())
}

func TestWorkflowRunner_DefaultTargetCommitish(t *testing.T) {
	runner := WorkflowRunner{}
	assert.Equal(t, "", runner.defaultTargetCommitish())

	runner.Deps.DefaultTargetCommitish = func() string { return "main" }
	assert.Equal(t, "main", runner.defaultTargetCommitish())
}

func TestWorkflowRunnerLocalizeFallback(t *testing.T) {
	runner := WorkflowRunner{}
	message := runner.localize(i18n.ReleaseNoIssues)
	assert.Equal(t, i18n.T(i18n.ReleaseNoIssues), message)
}

func TestContains(t *testing.T) {
	values := []string{"A", "B"}
	assert.True(t, contains(values, "A"))
	assert.False(t, contains(values, "C"))
}
