package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/commands/ai"
	"github.com/tagoro9/fotingo/internal/ui"
)

func TestAICommandStructure(t *testing.T) {
	cmd, _, err := Fotingo.Find([]string{"ai"})
	require.NoError(t, err)
	require.NotNil(t, cmd)
	assert.Equal(t, "ai", cmd.Name())

	setupCmd, _, err := Fotingo.Find([]string{"ai", "setup"})
	require.NoError(t, err)
	require.NotNil(t, setupCmd)
	assert.Equal(t, "setup", setupCmd.Name())
}

func TestAISetup_JSONDryRunAll(t *testing.T) {
	restoreGlobal := saveGlobalFlags()
	t.Cleanup(restoreGlobal)
	setDefaultOutputFlags(t)

	origFlags := aiSetupCmdFlags
	origGetwd := aiSetupGetwdFn
	origHome := aiSetupUserHomeDirFn
	origGetenv := aiSetupGetenvFn
	t.Cleanup(func() {
		aiSetupCmdFlags = origFlags
		aiSetupGetwdFn = origGetwd
		aiSetupUserHomeDirFn = origHome
		aiSetupGetenvFn = origGetenv
	})
	aiSetupCmdFlags = aiSetupFlags{}

	temp := t.TempDir()
	aiSetupGetwdFn = func() (string, error) { return temp, nil }
	aiSetupUserHomeDirFn = func() (string, error) { return temp, nil }
	aiSetupGetenvFn = func(string) string { return "" }

	Fotingo.SetArgs([]string{"ai", "setup", "--json", "--all", "--dry-run", "--scope", "project"})
	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		require.NoError(t, err)
	})

	var result AISetupOutput
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should be valid JSON, got: %s", output)

	assert.True(t, result.Success)
	assert.Equal(t, "project", result.Scope)
	assert.True(t, result.DryRun)
	assert.ElementsMatch(t, []string{"codex", "cursor", "claude-code"}, result.Providers)
	require.Len(t, result.Results, 3)
	for _, item := range result.Results {
		assert.Equal(t, "planned", item.Status)
	}
}

func TestAISetup_JSONWithoutProvidersFails(t *testing.T) {
	restoreGlobal := saveGlobalFlags()
	t.Cleanup(restoreGlobal)
	setDefaultOutputFlags(t)

	origFlags := aiSetupCmdFlags
	t.Cleanup(func() {
		aiSetupCmdFlags = origFlags
	})
	aiSetupCmdFlags = aiSetupFlags{}

	Fotingo.SetArgs([]string{"ai", "setup", "--json"})
	output := captureStdout(t, func() {
		err := Fotingo.Execute()
		require.Error(t, err)
	})

	var result AISetupOutput
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should be valid JSON, got: %s", output)

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "--agent")
}

func TestResolveAISetupProviders_InteractiveFallback(t *testing.T) {
	restoreGlobal := saveGlobalFlags()
	t.Cleanup(restoreGlobal)
	setDefaultOutputFlags(t)

	origInteractive := isInteractiveTerminalFn
	origSelect := aiSetupSelectProvidersFn
	origFlags := aiSetupCmdFlags
	t.Cleanup(func() {
		isInteractiveTerminalFn = origInteractive
		aiSetupSelectProvidersFn = origSelect
		aiSetupCmdFlags = origFlags
	})

	isInteractiveTerminalFn = func() bool { return true }
	aiSetupSelectProvidersFn = func(_ string, _ []ui.MultiSelectItem, _ int) ([]string, error) {
		return []string{"cursor", "codex"}, nil
	}
	aiSetupCmdFlags = aiSetupFlags{}

	providers, err := resolveAISetupProviders()
	require.NoError(t, err)
	assert.Equal(t, []ai.Provider{ai.ProviderCodex, ai.ProviderCursor}, providers)
}

func TestRunAISetup_CreateSkipForce(t *testing.T) {
	restoreGlobal := saveGlobalFlags()
	t.Cleanup(restoreGlobal)
	setDefaultOutputFlags(t)

	origFlags := aiSetupCmdFlags
	origGetwd := aiSetupGetwdFn
	origHome := aiSetupUserHomeDirFn
	origGetenv := aiSetupGetenvFn
	t.Cleanup(func() {
		aiSetupCmdFlags = origFlags
		aiSetupGetwdFn = origGetwd
		aiSetupUserHomeDirFn = origHome
		aiSetupGetenvFn = origGetenv
	})

	temp := t.TempDir()
	aiSetupGetwdFn = func() (string, error) { return temp, nil }
	aiSetupUserHomeDirFn = func() (string, error) { return temp, nil }
	aiSetupGetenvFn = func(string) string { return "" }

	aiSetupCmdFlags = aiSetupFlags{
		agents: []string{"codex"},
		scope:  "project",
	}

	result, err := runAISetup()
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.Equal(t, "created", string(result.Results[0].Status))
	firstPath := result.Results[0].Target.SkillPath

	_, statErr := os.Stat(firstPath)
	require.NoError(t, statErr)

	result, err = runAISetup()
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.Equal(t, "skipped", string(result.Results[0].Status))

	aiSetupCmdFlags.force = true
	result, err = runAISetup()
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.Equal(t, "updated", string(result.Results[0].Status))

	assert.Equal(
		t,
		filepath.Join(temp, ".codex", "skills", "fotingo", "SKILL.md"),
		firstPath,
	)
}
