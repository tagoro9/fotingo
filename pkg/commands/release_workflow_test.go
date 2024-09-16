package commands

import (
	"errors"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/i18n"
)

func TestReleaseWorkflowEmoji(t *testing.T) {
	assert.Equal(t, commandruntime.LogEmojiWarning, releaseWorkflowEmoji("warning"))
	assert.Equal(t, commandruntime.LogEmojiIssue, releaseWorkflowEmoji("issue"))
	assert.Equal(t, commandruntime.LogEmojiRelease, releaseWorkflowEmoji("release"))
	assert.Equal(t, commandruntime.LogEmojiBookmark, releaseWorkflowEmoji("bookmark"))
	assert.Equal(t, commandruntime.LogEmojiRocket, releaseWorkflowEmoji("rocket"))
	assert.Equal(t, commandruntime.LogEmojiSuccess, releaseWorkflowEmoji("success"))
	assert.Equal(t, commandruntime.LogEmojiInfo, releaseWorkflowEmoji("unknown"))
}

func TestReleaseWorkflowEmitterBridgesToStatusEmitter(t *testing.T) {
	setDefaultOutputFlags(t)
	Global.Verbose = true

	statusCh := make(chan string, 4)
	emitter := releaseWorkflowEmitter{out: commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)}
	emitter.Info("warning", i18n.ReleaseStatusNoIssues)
	emitter.Verbose(i18n.ReleaseStatusExtractIssues)

	infoEvent, ok := decodeStatusEvent(<-statusCh)
	require.True(t, ok)
	assert.Equal(t, statusEventKindAppend, infoEvent.Kind)
	assert.Equal(t, commandruntime.LogEmojiWarning, infoEvent.Emoji)

	verboseEvent, ok := decodeStatusEvent(<-statusCh)
	require.True(t, ok)
	assert.Equal(t, commandruntime.OutputLevelVerbose, verboseEvent.Level)
}

func TestRunReleaseWithStatus_ReturnsGitInitError(t *testing.T) {
	origNewGitClient := newGitClient
	t.Cleanup(func() { newGitClient = origNewGitClient })

	newGitClient = func(*viper.Viper, *chan string) (git.Git, error) {
		return nil, errors.New("git unavailable")
	}

	statusCh := make(chan string, 8)
	err := runReleaseWithStatus(&statusCh, "v1.2.3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git unavailable")
}

func TestRunRelease_SignalsProcessDone(t *testing.T) {
	origNewGitClient := newGitClient
	t.Cleanup(func() { newGitClient = origNewGitClient })

	newGitClient = func(*viper.Viper, *chan string) (git.Git, error) {
		return nil, errors.New("git unavailable")
	}

	statusCh := make(chan string, 8)
	done := make(chan bool, 1)
	_ = runRelease(&statusCh, done, "v1.2.3")

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected processDone signal")
	}
}
