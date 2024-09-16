package ai

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyInstallPlan_DryRun(t *testing.T) {
	t.Parallel()

	target := InstallTarget{
		Provider: ProviderCodex,
		Scope:    ScopeProject,
		SkillPath: filepath.Join(
			t.TempDir(),
			".codex",
			"skills",
			"fotingo",
			"SKILL.md",
		),
	}

	results, err := ApplyInstallPlan([]InstallTarget{target}, InstallOptions{DryRun: true}, func(Provider) (string, error) {
		return "content\n", nil
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, InstallStatusPlanned, results[0].Status)

	_, statErr := os.Stat(target.SkillPath)
	require.True(t, os.IsNotExist(statErr))
}

func TestApplyInstallPlan_CreateSkipForce(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	target := InstallTarget{
		Provider: ProviderCodex,
		Scope:    ScopeProject,
		SkillPath: filepath.Join(
			temp,
			".codex",
			"skills",
			"fotingo",
			"SKILL.md",
		),
	}

	render := func(Provider) (string, error) {
		return "one\n", nil
	}

	results, err := ApplyInstallPlan([]InstallTarget{target}, InstallOptions{}, render)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, InstallStatusCreated, results[0].Status)

	bytes, err := os.ReadFile(target.SkillPath)
	require.NoError(t, err)
	assert.Equal(t, "one\n", string(bytes))

	results, err = ApplyInstallPlan([]InstallTarget{target}, InstallOptions{}, func(Provider) (string, error) {
		return "two\n", nil
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, InstallStatusSkipped, results[0].Status)
	assert.Contains(t, results[0].Reason, "--force")

	bytes, err = os.ReadFile(target.SkillPath)
	require.NoError(t, err)
	assert.Equal(t, "one\n", string(bytes))

	results, err = ApplyInstallPlan([]InstallTarget{target}, InstallOptions{Force: true}, func(Provider) (string, error) {
		return "three\n", nil
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, InstallStatusUpdated, results[0].Status)

	bytes, err = os.ReadFile(target.SkillPath)
	require.NoError(t, err)
	assert.Equal(t, "three\n", string(bytes))
}

func TestCountInstallResults(t *testing.T) {
	t.Parallel()

	results := []InstallResult{
		{Status: InstallStatusPlanned},
		{Status: InstallStatusCreated},
		{Status: InstallStatusCreated},
		{Status: InstallStatusUpdated},
		{Status: InstallStatusSkipped},
	}

	planned, created, updated, skipped := CountInstallResults(results)
	assert.Equal(t, 1, planned)
	assert.Equal(t, 2, created)
	assert.Equal(t, 1, updated)
	assert.Equal(t, 1, skipped)
}
