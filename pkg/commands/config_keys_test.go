package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/config"
)

func TestLookupConfigKeySpec(t *testing.T) {
	t.Parallel()

	spec, ok := lookupConfigKeySpec("git.remote")
	require.True(t, ok)
	assert.Equal(t, configValueTypeString, spec.ValueType)
	assert.False(t, spec.Sensitive)
}

func TestLookupConfigKeySpec_GitHubUserProfilesTTL(t *testing.T) {
	t.Parallel()

	spec, ok := lookupConfigKeySpec("github.cache.userProfilesTTL")
	require.True(t, ok)
	assert.Equal(t, configValueTypeDuration, spec.ValueType)
	assert.Equal(t, "TTL for GitHub user profile cache", spec.Description)
}

func TestParseConfigKeyValue_Duration(t *testing.T) {
	t.Parallel()

	spec := configKeySpec{Key: "github.cache.labelsTTL", ValueType: configValueTypeDuration}
	parsed, err := parseConfigKeyValue(spec, " 1h ")
	require.NoError(t, err)
	assert.Equal(t, "1h0m0s", parsed)
}

func TestParseConfigKeyValue_AllowedValue(t *testing.T) {
	t.Parallel()

	spec := configKeySpec{
		Key:       "locale",
		ValueType: configValueTypeString,
		Allowed:   []string{"en"},
	}

	parsed, err := parseConfigKeyValue(spec, "en")
	require.NoError(t, err)
	assert.Equal(t, "en", parsed)

	_, err = parseConfigKeyValue(spec, "es")
	require.Error(t, err)
}

func TestSetConfigEntry_PersistsValue(t *testing.T) {
	origConfig := fotingoConfig
	fotingoConfig = newTempWritableConfig(t)
	t.Cleanup(func() { fotingoConfig = origConfig })

	entry, err := setConfigEntry("git.remote", "upstream")
	require.NoError(t, err)
	assert.Equal(t, "git.remote", entry.Key)
	assert.Equal(t, "upstream", fotingoConfig.GetString("git.remote"))
}

func TestSetConfigEntry_DefaultRemoteIsNotPersisted(t *testing.T) {
	origConfig := fotingoConfig
	fotingoConfig = newTempWritableConfig(t)
	t.Cleanup(func() { fotingoConfig = origConfig })

	_, err := setConfigEntry("git.remote", "upstream")
	require.NoError(t, err)
	_, err = setConfigEntry("git.remote", "origin")
	require.NoError(t, err)

	contents, err := os.ReadFile(fotingoConfig.ConfigFileUsed())
	require.NoError(t, err)
	assert.NotContains(t, string(contents), "remote:")
}

func TestSetConfigEntry_RedactsSensitiveValue(t *testing.T) {
	origConfig := fotingoConfig
	fotingoConfig = newTempWritableConfig(t)
	t.Cleanup(func() { fotingoConfig = origConfig })

	entry, err := setConfigEntry("github.token", "ghp_123456")
	require.NoError(t, err)
	assert.Equal(t, "<redacted>", entry.Value)
	assert.Equal(t, "ghp_123456", fotingoConfig.GetString("github.token"))
}

func TestConfigValueAsString_SensitiveEmptyValue(t *testing.T) {
	t.Parallel()

	spec := configKeySpec{Key: "github.token", ValueType: configValueTypeString, Sensitive: true}
	assert.Equal(t, "", configValueAsString(spec, ""))
	assert.Equal(t, "", configValueAsString(spec, nil))
	assert.Equal(t, "<redacted>", configValueAsString(spec, "ghp_123456"))
}

func TestConfigValueAsString_StringSpecIgnoresNonStringValues(t *testing.T) {
	t.Parallel()

	spec := configKeySpec{Key: "jira.user", ValueType: configValueTypeString}
	value := map[string]any{"login": "user@example.com", "token": "secret"}

	assert.Equal(t, "", configValueAsString(spec, value))
}

func TestCompleteConfigKeys(t *testing.T) {
	t.Parallel()

	keys, directive := completeConfigKeys(nil, []string{}, "jira.")
	assert.NotEmpty(t, keys)
	assert.Contains(t, keys, "jira.root")
	assert.Contains(t, keys, "jira.user.login")
	assert.Contains(t, keys, "jira.user.token")
	assert.NotContains(t, keys, "jira.user")
	assert.NotContains(t, keys, "jira.token")
	assert.NotContains(t, keys, "jira.siteId")
	assert.NotContains(t, keys, "jira.siteRoot")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)

	telemetryKeys, telemetryDirective := completeConfigKeys(nil, []string{}, "telemetry.")
	assert.Contains(t, telemetryKeys, "telemetry.enabled")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, telemetryDirective)
}

func TestSetConfigEntry_LegacyJiraKeyReturnsCanonicalHint(t *testing.T) {
	t.Parallel()

	_, err := setConfigEntry("jira.siteId", "abc123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use jira.root")
}

func newTempWritableConfig(t *testing.T) *viper.Viper {
	t.Helper()

	cfg := config.NewDefaultConfig()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg.SetConfigFile(configPath)
	require.NoError(t, cfg.WriteConfigAs(configPath))
	require.NoError(t, cfg.ReadInConfig())

	return cfg
}
