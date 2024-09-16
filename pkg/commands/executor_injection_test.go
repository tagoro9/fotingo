package commands

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ftgit "github.com/tagoro9/fotingo/internal/git"
)

func TestStartExecutorRunWithResult_PropagatesNormalizeError(t *testing.T) {
	expectedErr := errors.New("normalize failed")

	executor := newStartExecutorWithDeps(startExecutorDeps{
		normalizeFlags: func(_ *cobra.Command, _ string) error {
			return expectedErr
		},
	})

	statusCh := make(chan string, 1)
	result := executor.runWithResult(&cobra.Command{}, &statusCh, "TEST-123")
	require.Error(t, result.err)
	assert.ErrorIs(t, result.err, expectedErr)
}

func TestReviewExecutorRunWithOptions_PropagatesGitInitError(t *testing.T) {
	expectedErr := errors.New("git init failed")
	deps := defaultReviewExecutorDeps()
	deps.newGitClient = func(_ *viper.Viper, _ *chan string) (ftgit.Git, error) {
		return nil, expectedErr
	}

	executor := reviewExecutor{deps: deps}
	statusCh := make(chan string, 1)
	result := executor.runWithOptions(&statusCh, false)
	require.Error(t, result.err)
	assert.ErrorIs(t, result.err, expectedErr)
}
