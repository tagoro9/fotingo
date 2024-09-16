package start

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

type startTimingEmitter struct {
	debugRaw []string
}

func (e *startTimingEmitter) Info(string, i18n.Key, ...any) {}
func (e *startTimingEmitter) Verbose(i18n.Key, ...any)      {}
func (e *startTimingEmitter) Debug(i18n.Key, ...any)        {}
func (e *startTimingEmitter) DebugRaw(message string) {
	e.debugRaw = append(e.debugRaw, strings.TrimSpace(message))
}

func TestWorkflowRunnerRunWithResult_ValidatesDeps(t *testing.T) {
	runner := WorkflowRunner{}
	result := runner.RunWithResult(&cobra.Command{}, nil, "TEST-1", nil)
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "NormalizeFlags")
}

func TestWorkflowRunnerRunWithResult_PropagatesNormalizeError(t *testing.T) {
	expectedErr := errors.New("normalize failed")
	runner := WorkflowRunner{
		Config:  viper.New(),
		Options: WorkflowOptions{},
		Deps: WorkflowDeps{
			NormalizeFlags: func(*cobra.Command, string) error { return expectedErr },
			NewJiraClient:  func(*viper.Viper) (jira.Jira, error) { return nil, nil },
			CreateNewIssue: func(WorkflowEmitter, jira.Jira) (*jira.Issue, error) { return nil, nil },
			SelectIssueWithPicker: func([]tracker.Issue) (*tracker.Issue, error) {
				return nil, nil
			},
			RunWithSpinner:       func(func(WorkflowEmitter) error) error { return nil },
			ResolveIssueAssignee: func(WorkflowEmitter, jira.Jira, string) {},
			NewGitClient: func(*viper.Viper, *chan string) (git.Git, error) {
				return nil, nil
			},
			StashChanges: func(WorkflowEmitter, git.Git) error { return nil },
		},
	}

	result := runner.RunWithResult(&cobra.Command{}, nil, "TEST-1", nil)
	require.Error(t, result.Err)
	assert.ErrorIs(t, result.Err, expectedErr)
}

func TestWorkflowRunnerLocalizeFallback(t *testing.T) {
	runner := WorkflowRunner{}
	assert.Equal(t, i18n.T(i18n.StartStatusClean), runner.localize(i18n.StartStatusClean))
}

func TestLogStartPhaseTiming_EmitsDebugTiming(t *testing.T) {
	emitter := &startTimingEmitter{}
	logStartPhaseTiming(emitter, "phase_x", time.Now().Add(-120*time.Millisecond))

	require.Len(t, emitter.debugRaw, 1)
	assert.Contains(t, emitter.debugRaw[0], "start timing phase=phase_x duration=")
}
