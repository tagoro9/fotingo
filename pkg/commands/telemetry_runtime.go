package commands

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	ftconfig "github.com/tagoro9/fotingo/internal/config"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/telemetry"
)

const (
	// telemetryEnabledConfigKey toggles anonymous telemetry collection.
	telemetryEnabledConfigKey = "telemetry.enabled"
	// telemetryInstallationIDConfigKey stores the per-install anonymous distinct_id.
	telemetryInstallationIDConfigKey = "telemetry.installationId"
)

var installationIDPattern = regexp.MustCompile(
	`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`,
)

// telemetryExecutionContext is computed once per command execution and reused.
type telemetryExecutionContext struct {
	Persona           string
	InvocationMode    string
	GlobalFlags       map[string]bool
	HasBranchOverride bool
}

var (
	telemetryStateMu        sync.RWMutex
	telemetryActiveCommand  *cobra.Command
	telemetryExecutionState telemetryExecutionContext
)

// configureTelemetryRuntime initializes the telemetry subsystem for the current process.
func configureTelemetryRuntime() {
	enabled := fotingoConfig.GetBool(telemetryEnabledConfigKey)
	distinctID := ""
	if enabled && telemetry.IsDefaultBackendConfigured() {
		distinctID = resolveTelemetryDistinctID(fotingoConfig)
	}

	_ = telemetry.Configure(telemetry.Config{
		Enabled:         enabled,
		DistinctID:      distinctID,
		BuildInfo:       currentTelemetryBuildInfo(),
		ShutdownTimeout: 1200 * time.Millisecond,
	})
}

// currentTelemetryBuildInfo converts build metadata into telemetry-safe shape.
func currentTelemetryBuildInfo() telemetry.BuildInfo {
	build := telemetry.BuildInfo{
		Version:  strings.TrimSpace(Version),
		Platform: strings.TrimSpace(Platform),
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}

	platformParts := strings.SplitN(strings.TrimSpace(Platform), "/", 2)
	if len(platformParts) == 2 {
		if strings.TrimSpace(platformParts[0]) != "" {
			build.OS = strings.TrimSpace(platformParts[0])
		}
		if strings.TrimSpace(platformParts[1]) != "" {
			build.Arch = strings.TrimSpace(platformParts[1])
		}
	}

	return build
}

// resolveTelemetryDistinctID returns a persisted installation-scoped anonymous ID.
func resolveTelemetryDistinctID(cfg *viper.Viper) string {
	if cfg == nil {
		return ""
	}

	existing := strings.TrimSpace(cfg.GetString(telemetryInstallationIDConfigKey))
	if isValidInstallationID(existing) {
		return existing
	}

	generated, err := generateInstallationID()
	if err != nil {
		return ""
	}

	cfg.Set(telemetryInstallationIDConfigKey, generated)
	if err := ftconfig.PersistConfigValue(cfg, telemetryInstallationIDConfigKey, generated); err != nil {
		return ""
	}
	return generated
}

// generateInstallationID creates an RFC-4122 compatible UUIDv4 string.
func generateInstallationID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80

	encoded := hex.EncodeToString(buf)
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		encoded[0:8],
		encoded[8:12],
		encoded[12:16],
		encoded[16:20],
		encoded[20:32],
	), nil
}

// isValidInstallationID validates persisted telemetry installation IDs.
func isValidInstallationID(value string) bool {
	return installationIDPattern.MatchString(strings.TrimSpace(strings.ToLower(value)))
}

// startTelemetryForCommand captures execution context and emits command.started.
func startTelemetryForCommand(cmd *cobra.Command) {
	if cmd == nil || isShellCompletionCommand(cmd) {
		return
	}

	setActiveTelemetryCommand(cmd)
	setTelemetryExecutionContext(newTelemetryExecutionContext())
	telemetry.TrackCommandStarted(buildTelemetryCommandContext(cmd))
}

// finishTelemetryForSuccess emits completion telemetry for successful runs.
func finishTelemetryForSuccess(cmd *cobra.Command, startedAt time.Time) {
	if IsShellCompletionRequest() || isInvocationShellCompletion() {
		clearRuntimeTelemetryState()
		return
	}
	trackCommandCompletion(cmd, startedAt, nil, fterrors.ExitSuccess)
	clearRuntimeTelemetryState()
}

// finishTelemetryForError emits completion and error telemetry for handled failures.
func finishTelemetryForError(cmd *cobra.Command, startedAt time.Time, err error, exitCode int) {
	if IsShellCompletionRequest() || isInvocationShellCompletion() {
		clearRuntimeTelemetryState()
		return
	}
	trackCommandCompletion(cmd, startedAt, err, exitCode)
	clearRuntimeTelemetryState()
}

// finishTelemetryForCrash emits crash telemetry from recover() paths.
func finishTelemetryForCrash(cmd *cobra.Command, startedAt time.Time, recovered any) {
	if IsShellCompletionRequest() || isInvocationShellCompletion() {
		clearRuntimeTelemetryState()
		return
	}

	panicType, fingerprint, topFrame := buildCrashTelemetry(recovered)
	ctx := buildFallbackTelemetryContext(cmd)
	telemetry.TrackCommandCrashed(ctx, telemetry.CommandCrash{
		Duration:         time.Since(startedAt),
		ExitCode:         fterrors.ExitGeneral,
		PanicType:        panicType,
		CrashFingerprint: fingerprint,
		TopFrame:         topFrame,
	})
	clearRuntimeTelemetryState()
}

func clearRuntimeTelemetryState() {
	telemetry.ClearActiveCommand()
	setActiveTelemetryCommand(nil)
	clearTelemetryExecutionContext()
}

func trackCommandCompletion(cmd *cobra.Command, startedAt time.Time, err error, exitCode int) {
	ctx := buildFallbackTelemetryContext(cmd)
	duration := time.Since(startedAt)

	telemetry.TrackCommandCompleted(ctx, telemetry.CommandCompletion{
		Duration: duration,
		ExitCode: exitCode,
	})

	if err == nil || exitCode == fterrors.ExitSuccess {
		return
	}

	telemetry.TrackCommandError(ctx, telemetry.CommandError{
		Duration:         duration,
		ExitCode:         exitCode,
		ErrorFamily:      telemetryErrorFamily(exitCode),
		ErrorFingerprint: err.Error(),
	})
}

func buildFallbackTelemetryContext(cmd *cobra.Command) telemetry.CommandContext {
	if cmd != nil {
		return buildTelemetryCommandContext(cmd)
	}

	execution := currentOrNewTelemetryExecutionContext()
	return telemetry.CommandContext{
		CommandName:          "root",
		CommandPath:          "fotingo",
		CommandSchemaVersion: "root@1.0.0",
		Persona:              execution.Persona,
		InvocationMode:       execution.InvocationMode,
		GlobalFlags:          execution.GlobalFlags,
		HasBranchOverride:    execution.HasBranchOverride,
		OptionFlags:          map[string]bool{},
		OptionCounts:         map[string]int{},
		OptionEnums:          map[string]string{},
	}
}

func buildTelemetryCommandContext(cmd *cobra.Command) telemetry.CommandContext {
	commandName := "root"
	commandPath := "fotingo"
	if cmd != nil {
		if strings.TrimSpace(cmd.Name()) != "" {
			commandName = strings.TrimSpace(cmd.Name())
		}
		if strings.TrimSpace(cmd.CommandPath()) != "" {
			commandPath = strings.TrimSpace(cmd.CommandPath())
		}
	}

	execution := currentOrNewTelemetryExecutionContext()
	optionFlags, optionCounts, optionEnums := telemetryCommandOptions(cmd, execution.InvocationMode)

	return telemetry.CommandContext{
		CommandName:          commandName,
		CommandPath:          commandPath,
		CommandSchemaVersion: telemetryCommandSchemaVersion(commandName),
		Persona:              execution.Persona,
		InvocationMode:       execution.InvocationMode,
		GlobalFlags:          execution.GlobalFlags,
		HasBranchOverride:    execution.HasBranchOverride,
		OptionFlags:          optionFlags,
		OptionCounts:         optionCounts,
		OptionEnums:          optionEnums,
	}
}

func telemetryCommandSchemaVersion(commandName string) string {
	commandName = strings.TrimSpace(commandName)
	if commandName == "" {
		return "unknown@1.0.0"
	}
	return fmt.Sprintf("%s@1.0.0", commandName)
}

// currentOrNewTelemetryExecutionContext returns cached execution context, computing it once if missing.
func currentOrNewTelemetryExecutionContext() telemetryExecutionContext {
	telemetryStateMu.RLock()
	cached := telemetryExecutionState
	telemetryStateMu.RUnlock()

	if cached.InvocationMode != "" {
		return cached
	}

	computed := newTelemetryExecutionContext()
	setTelemetryExecutionContext(computed)
	return computed
}

func setTelemetryExecutionContext(ctx telemetryExecutionContext) {
	telemetryStateMu.Lock()
	defer telemetryStateMu.Unlock()
	telemetryExecutionState = ctx
}

func clearTelemetryExecutionContext() {
	telemetryStateMu.Lock()
	defer telemetryStateMu.Unlock()
	telemetryExecutionState = telemetryExecutionContext{}
}

func newTelemetryExecutionContext() telemetryExecutionContext {
	return telemetryExecutionContext{
		Persona:           resolveTelemetryPersona(),
		InvocationMode:    resolveTelemetryInvocationMode(),
		GlobalFlags:       snapshotTelemetryGlobalFlags(),
		HasBranchOverride: strings.TrimSpace(Global.Branch) != "",
	}
}

func snapshotTelemetryGlobalFlags() map[string]bool {
	return map[string]bool{
		"json":     Global.JSON,
		"yes":      Global.Yes,
		"quiet":    Global.Quiet,
		"verbose":  Global.Verbose,
		"debug":    Global.Debug,
		"no_color": Global.NoColor,
	}
}

func resolveTelemetryPersona() string {
	if isCIEnvironment() {
		return "ci_automation"
	}
	return "interactive_local"
}

func resolveTelemetryInvocationMode() string {
	if Global.JSON {
		return "json"
	}
	if Global.Yes || !isInputTerminalFn() {
		return "non_interactive"
	}
	return "interactive"
}

func isCIEnvironment() bool {
	ci := strings.TrimSpace(strings.ToLower(os.Getenv("CI")))
	if ci == "true" || ci == "1" {
		return true
	}

	markers := []string{
		"GITHUB_ACTIONS",
		"BUILDKITE",
		"CIRCLECI",
		"GITLAB_CI",
		"JENKINS_URL",
		"TEAMCITY_VERSION",
		"TF_BUILD",
		"TRAVIS",
	}
	for _, marker := range markers {
		if strings.TrimSpace(os.Getenv(marker)) != "" {
			return true
		}
	}
	return false
}

func telemetryCommandOptions(cmd *cobra.Command, invocationMode string) (map[string]bool, map[string]int, map[string]string) {
	optionFlags := map[string]bool{}
	optionCounts := map[string]int{}
	optionEnums := map[string]string{}

	if cmd == nil {
		return optionFlags, optionCounts, optionEnums
	}

	switch cmd.Name() {
	case "review":
		optionFlags["has_title"] = strings.TrimSpace(reviewCmdFlags.title) != ""
		optionFlags["has_description"] = strings.TrimSpace(reviewCmdFlags.description) != "" && strings.TrimSpace(reviewCmdFlags.description) != "-"
		optionFlags["description_from_stdin"] = strings.TrimSpace(reviewCmdFlags.description) == "-"
		optionFlags["has_template_summary"] = strings.TrimSpace(reviewCmdFlags.templateSummary) != ""
		optionFlags["has_template_description"] = strings.TrimSpace(reviewCmdFlags.templateDescription) != ""
		optionFlags["is_draft"] = reviewCmdFlags.draft
		optionFlags["is_simple"] = reviewCmdFlags.simple
		optionCounts["labels"] = len(reviewCmdFlags.labels)
		optionCounts["reviewers"] = len(reviewCmdFlags.reviewers)
		optionCounts["assignees"] = len(reviewCmdFlags.assignees)
		optionEnums["output_mode"] = invocationMode

		allowlisted := map[string]struct{}{
			"draft": {}, "labels": {}, "reviewers": {}, "assignee": {}, "simple": {},
			"title": {}, "description": {}, "template-summary": {}, "template-description": {},
		}
		optionCounts["unknown_option_count"] = countUnknownChangedLocalFlags(cmd, allowlisted)
	case "start":
		optionFlags["has_title"] = cmd.Flags().Changed("title")
		optionFlags["has_description"] = cmd.Flags().Changed("description")
		optionFlags["has_project"] = cmd.Flags().Changed("project")
		optionFlags["has_kind"] = cmd.Flags().Changed("kind")
		optionFlags["has_parent"] = cmd.Flags().Changed("parent")
		optionFlags["has_epic"] = cmd.Flags().Changed("epic")
		optionFlags["no_branch"] = startCmdFlags.noBranch
		optionFlags["worktree"] = startWorktreeEnabled(fotingoConfig)
		optionFlags["interactive"] = startCmdFlags.interactive
		optionCounts["labels"] = len(startCmdFlags.labels)
	default:
		optionCounts["unknown_option_count"] = countUnknownChangedLocalFlags(cmd, map[string]struct{}{})
	}

	return optionFlags, optionCounts, optionEnums
}

func countUnknownChangedLocalFlags(cmd *cobra.Command, allowlisted map[string]struct{}) int {
	if cmd == nil {
		return 0
	}

	globalAllowlisted := map[string]struct{}{
		"branch":   {},
		"yes":      {},
		"json":     {},
		"quiet":    {},
		"verbose":  {},
		"debug":    {},
		"no-color": {},
		"help":     {},
	}

	unknown := 0
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if _, ok := globalAllowlisted[flag.Name]; ok {
			return
		}
		if _, ok := allowlisted[flag.Name]; ok {
			return
		}
		unknown++
	})
	return unknown
}

func setActiveTelemetryCommand(cmd *cobra.Command) {
	telemetryStateMu.Lock()
	defer telemetryStateMu.Unlock()
	telemetryActiveCommand = cmd
}

func currentTelemetryCommand() *cobra.Command {
	telemetryStateMu.RLock()
	defer telemetryStateMu.RUnlock()
	return telemetryActiveCommand
}

func telemetryErrorFamily(exitCode int) string {
	switch exitCode {
	case fterrors.ExitConfig:
		return "config"
	case fterrors.ExitAuth:
		return "auth"
	case fterrors.ExitGit:
		return "git"
	case fterrors.ExitJira:
		return "jira"
	case fterrors.ExitGitHub:
		return "github"
	case fterrors.ExitUserCancelled:
		return "cancelled"
	default:
		return "general"
	}
}

func buildCrashTelemetry(recovered any) (string, string, string) {
	panicType := "unknown"
	if recovered != nil {
		panicType = fmt.Sprintf("%T", recovered)
	}

	stack := debug.Stack()
	topFrame := resolveTopFrame(stack)
	fingerprintInput := strings.TrimSpace(fmt.Sprintf("%s|%v|%s|%s", panicType, recovered, topFrame, stack))
	return panicType, fingerprintInput, topFrame
}

func resolveTopFrame(stack []byte) string {
	lines := strings.Split(string(stack), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "goroutine ") {
			continue
		}
		if strings.Contains(trimmed, "runtime/debug.Stack") {
			continue
		}
		if strings.HasPrefix(trimmed, "runtime.") {
			continue
		}
		if strings.HasPrefix(trimmed, "runtime/") {
			continue
		}
		if strings.HasPrefix(trimmed, "testing.") {
			continue
		}
		return trimmed
	}
	return ""
}
