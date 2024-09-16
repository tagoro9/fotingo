package commands

import (
	"errors"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
)

func TestEnsureConfigRequirements_NonInteractiveFailsFast(t *testing.T) {
	cfg := viper.New()
	fotingoConfig = cfg

	restore := savePreflightState()
	defer restore()

	Global.JSON = true

	err := ensureConfigRequirements(configRequirement{
		Key:    "jira.root",
		EnvVar: "FOTINGO_JIRA_ROOT",
	})
	require.Error(t, err)
	assert.Equal(t, fterrors.ExitConfig, fterrors.GetExitCode(err))
	assert.Contains(t, err.Error(), "missing required config")
	assert.Contains(t, err.Error(), "jira.root")
}

func TestEnsureConfigRequirements_InteractivePromptsAndPersists(t *testing.T) {
	cfg := viper.New()
	fotingoConfig = cfg

	restore := savePreflightState()
	defer restore()

	Global.JSON = false
	Global.Yes = false
	isInputTerminalFn = func() bool { return true }
	promptForConfigValue = func(requirement configRequirement) (string, error) {
		return "https://example.atlassian.net", nil
	}

	var writes int
	writeConfigFn = func(key string, value string) error {
		writes++
		fotingoConfig.Set(key, value)
		return nil
	}

	err := ensureConfigRequirements(configRequirement{
		Key:      "jira.root",
		Validate: normalizeJiraRootConfigValue,
	})
	require.NoError(t, err)
	assert.Equal(t, "https://example.atlassian.net", fotingoConfig.GetString("jira.root"))
	assert.Equal(t, 1, writes)
}

func TestEnsureConfigRequirements_CancelPromptReturnsConfigError(t *testing.T) {
	cfg := viper.New()
	fotingoConfig = cfg

	restore := savePreflightState()
	defer restore()

	Global.JSON = false
	Global.Yes = false
	isInputTerminalFn = func() bool { return true }
	promptForConfigValue = func(requirement configRequirement) (string, error) {
		return "", errors.New("input cancelled")
	}

	err := ensureConfigRequirements(configRequirement{
		Key:      "jira.root",
		Validate: normalizeJiraRootConfigValue,
	})
	require.Error(t, err)
	assert.Equal(t, fterrors.ExitConfig, fterrors.GetExitCode(err))
	assert.Contains(t, err.Error(), "input cancelled")
}

func TestEnsureConfigRequirements_ExistingValueSkipsPrompt(t *testing.T) {
	cfg := viper.New()
	cfg.Set("jira.root", "https://already.atlassian.net")
	fotingoConfig = cfg

	restore := savePreflightState()
	defer restore()

	called := false
	promptForConfigValue = func(requirement configRequirement) (string, error) {
		called = true
		return "", nil
	}

	err := ensureConfigRequirements(configRequirement{
		Key:      "jira.root",
		Validate: normalizeJiraRootConfigValue,
	})
	require.NoError(t, err)
	assert.False(t, called)
}

func TestNormalizeJiraRootConfigValue(t *testing.T) {
	normalized, err := normalizeJiraRootConfigValue("mycompany.atlassian.net")
	require.NoError(t, err)
	assert.Equal(t, "https://mycompany.atlassian.net", normalized)

	_, err = normalizeJiraRootConfigValue("http://mycompany.atlassian.net")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scheme must be https")

	_, err = normalizeJiraRootConfigValue("https://mycompany.atlassian.net/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path is not allowed")
}

func savePreflightState() func() {
	savedGlobal := Global
	savedPrompt := promptForConfigValue
	savedTTY := isInputTerminalFn
	savedWrite := writeConfigFn

	return func() {
		Global = savedGlobal
		promptForConfigValue = savedPrompt
		isInputTerminalFn = savedTTY
		writeConfigFn = savedWrite
	}
}
