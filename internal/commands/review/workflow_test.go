package review

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
)

type reviewTimingEmitter struct {
	debug []string
}

func (e *reviewTimingEmitter) Info(string, i18n.Key, ...any) {}
func (e *reviewTimingEmitter) InfoRaw(string, string)        {}
func (e *reviewTimingEmitter) Verbose(i18n.Key, ...any)      {}
func (e *reviewTimingEmitter) Debugf(format string, args ...any) {
	e.debug = append(e.debug, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func TestWorkflowRunnerRun_ValidatesDeps(t *testing.T) {
	runner := WorkflowRunner{}
	result := runner.Run(nil, nil, false)
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "NewGitClient")
}

func TestWorkflowRunnerRun_WrapsGitInitError(t *testing.T) {
	runner := WorkflowRunner{
		Config: viper.New(),
		Localize: func(key i18n.Key, args ...any) string {
			if key == i18n.ReviewWrapInitGit {
				return "failed to init git"
			}
			return i18n.T(key, args...)
		},
		Deps: WorkflowDeps{
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return nil, errors.New("git unavailable")
			},
			NewGitHubClient: func(git.Git, *viper.Viper) (github.Github, error) { return nil, nil },
			NewJiraClient:   func(*viper.Viper) (jira.Jira, error) { return nil, nil },
			FetchBranchIssue: func(jira.Jira, string, func(string, ...any)) (*jira.Issue, error) {
				return nil, nil
			},
			ResolvePRBody: func(*chan string, string, *jira.Issue, jira.Jira, []git.Commit, bool) (string, error) {
				return "", nil
			},
			ResolveLabels: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			ResolveReviewers: func(github.Github, []string) ([]string, []string, []string, error) {
				return nil, nil, nil, nil
			},
			ResolveAssignees: func(github.Github, []string) ([]string, []string, error) {
				return nil, nil, nil
			},
			SplitEditorContent:     SplitEditorContent,
			DerivePRTitle:          func(string, *jira.Issue, string, bool) string { return "title" },
			ToTeamSlugs:            ToTeamSlugs,
			FormatReviewersWarning: func(err error) string { return err.Error() },
			ShouldOpenReviewEditor: func(bool) bool { return false },
		},
	}

	result := runner.Run(nil, nil, false)
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "failed to init git")
	assert.Contains(t, result.Err.Error(), "git unavailable")
}

func TestShouldCollectReviewCommits(t *testing.T) {
	assert.True(t, shouldCollectReviewCommits(""))
	assert.True(t, shouldCollectReviewCommits("   "))
	assert.False(t, shouldCollectReviewCommits("manual body"))
	assert.False(t, shouldCollectReviewCommits("-"))
}

func TestLogReviewPhaseTiming_EmitsDebugTiming(t *testing.T) {
	emitter := &reviewTimingEmitter{}
	logReviewPhaseTiming(emitter, "phase_x", time.Now().Add(-150*time.Millisecond))

	require.Len(t, emitter.debug, 1)
	assert.Contains(t, emitter.debug[0], "review timing phase=phase_x duration=")
}
