package telemetry

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/posthog/posthog-go"
)

const (
	// EventSchemaVersion tracks the global telemetry schema contract version.
	EventSchemaVersion = "1.0.0"
)

const (
	// EventCommandStarted is emitted when command execution begins.
	EventCommandStarted = "fotingo.command.started"
	// EventCommandCompleted is emitted after command completion with duration/exit code.
	EventCommandCompleted = "fotingo.command.completed"
	// EventCommandError is emitted for handled command failures.
	EventCommandError = "fotingo.command.error"
	// EventCommandCrashed is emitted when execution recovers from a panic.
	EventCommandCrashed = "fotingo.command.crashed"
	// EventIntegrationCall is emitted for instrumented GitHub/Jira HTTP calls.
	EventIntegrationCall = "fotingo.integration.call"
	// EventUpdateBannerShown is emitted when the startup update banner is shown.
	EventUpdateBannerShown = "fotingo.ui.update_banner.shown"
)

// BuildInfo describes the binary metadata included in telemetry events.
type BuildInfo struct {
	Version  string
	Platform string
	OS       string
	Arch     string
}

// Config controls telemetry runtime initialization.
type Config struct {
	Enabled         bool
	DistinctID      string
	BuildInfo       BuildInfo
	ShutdownTimeout time.Duration
	// backend is an internal override primarily for tests.
	backend backend
}

// CommandContext captures safe command metadata.
type CommandContext struct {
	CommandName          string
	CommandPath          string
	CommandSchemaVersion string
	Persona              string
	InvocationMode       string
	GlobalFlags          map[string]bool
	HasBranchOverride    bool
	OptionFlags          map[string]bool
	OptionCounts         map[string]int
	OptionEnums          map[string]string
}

// CommandCompletion captures completion outcome data.
type CommandCompletion struct {
	Duration time.Duration
	ExitCode int
}

// CommandError captures error telemetry details.
type CommandError struct {
	Duration         time.Duration
	ExitCode         int
	ErrorFamily      string
	ErrorFingerprint string
}

// CommandCrash captures panic/crash telemetry details.
type CommandCrash struct {
	Duration         time.Duration
	ExitCode         int
	PanicType        string
	CrashFingerprint string
	TopFrame         string
}

// IntegrationCall captures normalized integration call metrics.
type IntegrationCall struct {
	Service          string
	Operation        string
	Duration         time.Duration
	Success          bool
	RetryCount       int
	CacheHit         bool
	StatusCodeBucket string
}

// UpdateBannerEvent captures startup update-banner telemetry.
type UpdateBannerEvent struct {
	CurrentVersion string
	LatestVersion  string
	Trigger        string
	Persona        string
	InvocationMode string
}

type recorder interface {
	Enqueue(posthog.Message) error
	CloseWithContext(context.Context) error
}

type activeCommandContext struct {
	CommandName          string
	CommandPath          string
	CommandSchemaVersion string
	Persona              string
	InvocationMode       string
}

type runtimeState struct {
	mu              sync.RWMutex
	enabled         bool
	distinctID      string
	buildInfo       BuildInfo
	shutdownTimeout time.Duration
	recorder        recorder
	active          activeCommandContext
}

var state = &runtimeState{}

// Configure initializes telemetry runtime. It is safe to call multiple times.
func Configure(cfg Config) error {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.enabled = false
	state.distinctID = ""
	state.buildInfo = cfg.BuildInfo
	state.shutdownTimeout = cfg.ShutdownTimeout
	state.active = activeCommandContext{}
	oldRecorder := state.recorder
	state.recorder = nil

	if oldRecorder != nil {
		closeRecorder(oldRecorder, cfg.ShutdownTimeout)
	}

	backend := cfg.backend
	if backend == nil {
		backend = currentDefaultBackend()
	}
	if backend == nil {
		return nil
	}

	if !cfg.Enabled || strings.TrimSpace(cfg.DistinctID) == "" || !backend.IsConfigured() {
		return nil
	}

	client, err := backend.NewRecorder(cfg.ShutdownTimeout)
	if err != nil {
		return err
	}

	state.enabled = true
	state.distinctID = strings.TrimSpace(cfg.DistinctID)
	state.recorder = client
	return nil
}

// Shutdown closes telemetry delivery resources.
func Shutdown() {
	state.mu.Lock()
	defer state.mu.Unlock()

	oldRecorder := state.recorder
	timeout := state.shutdownTimeout
	state.recorder = nil
	state.enabled = false
	state.active = activeCommandContext{}
	if oldRecorder != nil {
		closeRecorder(oldRecorder, timeout)
	}
}

// SetRecorderForTesting replaces the telemetry recorder for tests.
func SetRecorderForTesting(r recorder, build BuildInfo, distinctID string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.enabled = true
	state.recorder = r
	state.buildInfo = build
	state.distinctID = strings.TrimSpace(distinctID)
	state.shutdownTimeout = time.Second
	state.active = activeCommandContext{}
}

// ResetForTesting resets telemetry runtime state for tests.
func ResetForTesting() {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.enabled = false
	state.recorder = nil
	state.distinctID = ""
	state.buildInfo = BuildInfo{}
	state.shutdownTimeout = 0
	state.active = activeCommandContext{}
}

// TrackCommandStarted emits a command-start event and stores active command context.
func TrackCommandStarted(ctx CommandContext) {
	props := buildCommandBaseProperties(ctx)
	enqueueCapture(EventCommandStarted, props)
	setActiveCommand(ctx)
}

// TrackCommandCompleted emits command-completed telemetry.
func TrackCommandCompleted(ctx CommandContext, completion CommandCompletion) {
	props := buildCommandBaseProperties(resolveCommandContext(ctx))
	props["duration_ms"] = nonNegativeDuration(completion.Duration, 0).Milliseconds()
	props["exit_code"] = completion.ExitCode
	enqueueCapture(EventCommandCompleted, props)
}

// TrackCommandError emits command-error telemetry.
func TrackCommandError(ctx CommandContext, commandError CommandError) {
	props := buildCommandBaseProperties(resolveCommandContext(ctx))
	props["duration_ms"] = nonNegativeDuration(commandError.Duration, 0).Milliseconds()
	props["exit_code"] = commandError.ExitCode
	props["error_family"] = normalizeEnum(commandError.ErrorFamily)
	props["error_fingerprint"] = sanitizeFingerprint(commandError.ErrorFingerprint)
	enqueueCapture(EventCommandError, props)
}

// TrackCommandCrashed emits command-crashed telemetry.
func TrackCommandCrashed(ctx CommandContext, crash CommandCrash) {
	resolved := resolveCommandContext(ctx)
	props := posthog.Properties{
		"event_schema_version": EventSchemaVersion,
		"persona":              normalizeEnum(resolved.Persona),
		"invocation_mode":      normalizeEnum(resolved.InvocationMode),
		"version":              strings.TrimSpace(snapshotBuildInfo().Version),
		"platform":             strings.TrimSpace(snapshotBuildInfo().Platform),
		"os":                   nonEmptyOrDefault(snapshotBuildInfo().OS, runtime.GOOS),
		"arch":                 nonEmptyOrDefault(snapshotBuildInfo().Arch, runtime.GOARCH),
		"command_name":         strings.TrimSpace(resolved.CommandName),
		"command_path":         strings.TrimSpace(resolved.CommandPath),
		"duration_ms":          nonNegativeDuration(crash.Duration, 0).Milliseconds(),
		"exit_code":            crash.ExitCode,
		"panic_type":           normalizeEnum(crash.PanicType),
		"crash_fingerprint":    sanitizeFingerprint(crash.CrashFingerprint),
		"top_frame":            sanitizeTopFrame(crash.TopFrame),
	}
	enqueueCapture(EventCommandCrashed, props)
}

// TrackIntegrationCall emits normalized external-service call telemetry.
func TrackIntegrationCall(call IntegrationCall) {
	active := snapshotActiveCommand()
	build := snapshotBuildInfo()
	operation := normalizeEnum(call.Operation)
	if operation == "" {
		operation = "other"
	}
	props := posthog.Properties{
		"event_schema_version": EventSchemaVersion,
		"persona":              normalizeEnum(active.Persona),
		"invocation_mode":      normalizeEnum(active.InvocationMode),
		"version":              strings.TrimSpace(build.Version),
		"platform":             strings.TrimSpace(build.Platform),
		"os":                   nonEmptyOrDefault(build.OS, runtime.GOOS),
		"arch":                 nonEmptyOrDefault(build.Arch, runtime.GOARCH),
		"command_name":         strings.TrimSpace(active.CommandName),
		"command_path":         strings.TrimSpace(active.CommandPath),
		"service":              normalizeEnum(call.Service),
		"operation":            operation,
		"duration_ms":          nonNegativeDuration(call.Duration, 0).Milliseconds(),
		"success":              call.Success,
		"retry_count":          maxInt(call.RetryCount, 0),
		"cache_hit":            call.CacheHit,
		"status_code_bucket":   normalizeEnum(call.StatusCodeBucket),
	}
	enqueueCapture(EventIntegrationCall, props)
}

// TrackUpdateBannerShown emits update-banner impression telemetry.
func TrackUpdateBannerShown(event UpdateBannerEvent) {
	build := snapshotBuildInfo()
	props := posthog.Properties{
		"event_schema_version": EventSchemaVersion,
		"persona":              normalizeEnum(event.Persona),
		"invocation_mode":      normalizeEnum(event.InvocationMode),
		"version":              strings.TrimSpace(build.Version),
		"platform":             strings.TrimSpace(build.Platform),
		"os":                   nonEmptyOrDefault(build.OS, runtime.GOOS),
		"arch":                 nonEmptyOrDefault(build.Arch, runtime.GOARCH),
		"current_version":      sanitizeVersion(event.CurrentVersion),
		"latest_version":       sanitizeVersion(event.LatestVersion),
		"trigger":              normalizeEnum(event.Trigger),
	}
	enqueueCapture(EventUpdateBannerShown, props)
}

// ClearActiveCommand clears active command context for integration event correlation.
func ClearActiveCommand() {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.active = activeCommandContext{}
}

func buildCommandBaseProperties(ctx CommandContext) posthog.Properties {
	resolved := resolveCommandContext(ctx)
	build := snapshotBuildInfo()
	return posthog.Properties{
		"event_schema_version":   EventSchemaVersion,
		"persona":                normalizeEnum(resolved.Persona),
		"invocation_mode":        normalizeEnum(resolved.InvocationMode),
		"version":                strings.TrimSpace(build.Version),
		"platform":               strings.TrimSpace(build.Platform),
		"os":                     nonEmptyOrDefault(build.OS, runtime.GOOS),
		"arch":                   nonEmptyOrDefault(build.Arch, runtime.GOARCH),
		"command_name":           strings.TrimSpace(resolved.CommandName),
		"command_path":           strings.TrimSpace(resolved.CommandPath),
		"command_schema_version": sanitizeSchemaVersion(resolved.CommandSchemaVersion),
		"global_flags":           sanitizeBoolMap(ctx.GlobalFlags, 16),
		"has_branch_override":    ctx.HasBranchOverride,
		"option_flags":           sanitizeBoolMap(ctx.OptionFlags, 64),
		"option_counts":          sanitizeIntMap(ctx.OptionCounts, 64),
		"option_enums":           sanitizeEnumMap(ctx.OptionEnums, 32),
	}
}

func resolveCommandContext(ctx CommandContext) CommandContext {
	if strings.TrimSpace(ctx.CommandName) != "" {
		return ctx
	}
	active := snapshotActiveCommand()
	return CommandContext{
		CommandName:          active.CommandName,
		CommandPath:          active.CommandPath,
		CommandSchemaVersion: active.CommandSchemaVersion,
		Persona:              active.Persona,
		InvocationMode:       active.InvocationMode,
		GlobalFlags:          ctx.GlobalFlags,
		HasBranchOverride:    ctx.HasBranchOverride,
		OptionFlags:          ctx.OptionFlags,
		OptionCounts:         ctx.OptionCounts,
		OptionEnums:          ctx.OptionEnums,
	}
}

func setActiveCommand(ctx CommandContext) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.active = activeCommandContext{
		CommandName:          strings.TrimSpace(ctx.CommandName),
		CommandPath:          strings.TrimSpace(ctx.CommandPath),
		CommandSchemaVersion: sanitizeSchemaVersion(ctx.CommandSchemaVersion),
		Persona:              normalizeEnum(ctx.Persona),
		InvocationMode:       normalizeEnum(ctx.InvocationMode),
	}
}

func snapshotActiveCommand() activeCommandContext {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.active
}

func snapshotBuildInfo() BuildInfo {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.buildInfo
}

func enqueueCapture(event string, props posthog.Properties) {
	event = strings.TrimSpace(event)
	if event == "" {
		return
	}

	normalizedProps := sanitizeAndValidateProperties(event, props)
	if normalizedProps == nil {
		return
	}

	state.mu.RLock()
	rec := state.recorder
	enabled := state.enabled
	distinctID := state.distinctID
	state.mu.RUnlock()
	if !enabled || rec == nil || strings.TrimSpace(distinctID) == "" {
		return
	}

	_ = rec.Enqueue(posthog.Capture{
		DistinctId: strings.TrimSpace(distinctID),
		Event:      event,
		Timestamp:  time.Now().UTC(),
		Properties: normalizedProps,
	})
}

func closeRecorder(rec recorder, timeout time.Duration) {
	if rec == nil {
		return
	}
	timeout = nonNegativeDuration(timeout, time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_ = rec.CloseWithContext(ctx)
}
