package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/telemetry"
)

func TestResolveTelemetryDistinctID_PersistsGeneratedID(t *testing.T) {
	cfg := newTempWritableConfig(t)

	distinctID := resolveTelemetryDistinctID(cfg)
	require.NotEmpty(t, distinctID)
	assert.True(t, isValidInstallationID(distinctID))
	assert.Equal(t, distinctID, cfg.GetString(telemetryInstallationIDConfigKey))

	resolvedAgain := resolveTelemetryDistinctID(cfg)
	assert.Equal(t, distinctID, resolvedAgain)
}

func TestConfigureTelemetryRuntime_DisabledSkipsInstallationID(t *testing.T) {
	origConfig := fotingoConfig
	fotingoConfig = newTempWritableConfig(t)
	restoreBackend := telemetry.SetDefaultBackendConfiguredForTesting(true)
	fotingoConfig.Set(telemetryEnabledConfigKey, false)
	t.Cleanup(func() {
		fotingoConfig = origConfig
		restoreBackend()
		telemetry.Shutdown()
	})

	configureTelemetryRuntime()

	assert.Empty(t, strings.TrimSpace(fotingoConfig.GetString(telemetryInstallationIDConfigKey)))
}

func TestConfigureTelemetryRuntime_UnconfiguredBackendSkipsInstallationID(t *testing.T) {
	origConfig := fotingoConfig
	fotingoConfig = newTempWritableConfig(t)
	restoreBackend := telemetry.SetDefaultBackendConfiguredForTesting(false)
	fotingoConfig.Set(telemetryEnabledConfigKey, true)
	t.Cleanup(func() {
		fotingoConfig = origConfig
		restoreBackend()
		telemetry.Shutdown()
	})

	configureTelemetryRuntime()

	assert.Empty(t, strings.TrimSpace(fotingoConfig.GetString(telemetryInstallationIDConfigKey)))
}

func TestTelemetryCommandOptions_ReviewEmitsCountsAndFlagsOnly(t *testing.T) {
	origReviewFlags := reviewCmdFlags
	t.Cleanup(func() { reviewCmdFlags = origReviewFlags })

	cmd := &cobra.Command{Use: "review"}
	cmd.Flags().Bool("draft", false, "")
	cmd.Flags().StringSlice("labels", []string{}, "")
	cmd.Flags().StringSlice("reviewers", []string{}, "")
	cmd.Flags().StringSlice("assignee", []string{}, "")
	cmd.Flags().Bool("simple", false, "")
	cmd.Flags().String("title", "", "")
	cmd.Flags().String("description", "", "")
	cmd.Flags().String("template-summary", "", "")
	cmd.Flags().String("template-description", "", "")
	cmd.Flags().String("experimental", "", "")

	require.NoError(t, cmd.ParseFlags([]string{
		"--reviewers", "alice,bob",
		"--assignee", "charlie",
		"--labels", "bug,priority",
		"--experimental", "on",
	}))

	reviewCmdFlags = reviewFlags{
		draft:               true,
		reviewers:           []string{"alice", "bob"},
		assignees:           []string{"charlie"},
		labels:              []string{"bug", "priority"},
		templateSummary:     "summary override",
		templateDescription: "description override",
	}

	flags, counts, enums := telemetryCommandOptions(cmd, "interactive")

	assert.True(t, flags["is_draft"])
	assert.True(t, flags["has_template_summary"])
	assert.True(t, flags["has_template_description"])
	assert.Equal(t, 2, counts["reviewers"])
	assert.Equal(t, 1, counts["assignees"])
	assert.Equal(t, 2, counts["labels"])
	assert.Equal(t, 1, counts["unknown_option_count"])
	assert.NotEmpty(t, enums["output_mode"])
}
