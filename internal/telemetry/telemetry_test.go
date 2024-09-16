package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/posthog/posthog-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recorderStub struct {
	messages []posthog.Message
	closed   bool
}

func (r *recorderStub) Enqueue(msg posthog.Message) error {
	r.messages = append(r.messages, msg)
	return nil
}

func (r *recorderStub) CloseWithContext(context.Context) error {
	r.closed = true
	return nil
}

func TestTrackCommandLifecycle_EmitsSchemaSafeEvents(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	recorder := &recorderStub{}
	SetRecorderForTesting(recorder, BuildInfo{
		Version:  "v5.0.0",
		Platform: "darwin/arm64",
		OS:       "darwin",
		Arch:     "arm64",
	}, "install-123")

	ctx := CommandContext{
		CommandName:          "review",
		CommandPath:          "fotingo review",
		CommandSchemaVersion: "review@1.0.0",
		Persona:              "interactive_local",
		InvocationMode:       "human",
		GlobalFlags:          map[string]bool{"yes": true},
		HasBranchOverride:    false,
		OptionFlags:          map[string]bool{"has_template_summary": true},
		OptionCounts:         map[string]int{"reviewers": 2},
		OptionEnums:          map[string]string{"output_mode": "human"},
	}

	TrackCommandStarted(ctx)
	TrackCommandCompleted(ctx, CommandCompletion{
		Duration: 500 * time.Millisecond,
		ExitCode: 0,
	})

	require.Len(t, recorder.messages, 2)

	startCapture, ok := recorder.messages[0].(posthog.Capture)
	require.True(t, ok)
	assert.Equal(t, EventCommandStarted, startCapture.Event)
	assert.NotContains(t, startCapture.Properties, "duration_ms")
	assert.NotContains(t, startCapture.Properties, "exit_code")
	assert.Equal(t, "review", startCapture.Properties["command_name"])

	completedCapture, ok := recorder.messages[1].(posthog.Capture)
	require.True(t, ok)
	assert.Equal(t, EventCommandCompleted, completedCapture.Event)
	assert.EqualValues(t, 500, completedCapture.Properties["duration_ms"])
	assert.EqualValues(t, 0, completedCapture.Properties["exit_code"])
}

func TestTrackCommandError_SanitizesFingerprint(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	recorder := &recorderStub{}
	SetRecorderForTesting(recorder, BuildInfo{Version: "v5.0.0", Platform: "linux/amd64"}, "install-123")

	ctx := CommandContext{
		CommandName:          "start",
		CommandSchemaVersion: "start@1.0.0",
		Persona:              "ci_automation",
		InvocationMode:       "json",
		GlobalFlags:          map[string]bool{},
		OptionFlags:          map[string]bool{},
		OptionCounts:         map[string]int{},
		OptionEnums:          map[string]string{},
	}

	TrackCommandError(ctx, CommandError{
		Duration:         time.Second,
		ExitCode:         6,
		ErrorFamily:      "github",
		ErrorFingerprint: "POST /requested_reviewers: 422 Review cannot be requested",
	})

	require.Len(t, recorder.messages, 1)
	msg, ok := recorder.messages[0].(posthog.Capture)
	require.True(t, ok)
	assert.Equal(t, EventCommandError, msg.Event)
	fingerprint, ok := msg.Properties["error_fingerprint"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, fingerprint)
	assert.NotContains(t, fingerprint, "requested_reviewers")
}

func TestTrackIntegrationCall_InvalidOperationFallsBackToOther(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	recorder := &recorderStub{}
	SetRecorderForTesting(recorder, BuildInfo{Version: "v5.0.0", Platform: "linux/amd64"}, "install-123")

	TrackIntegrationCall(IntegrationCall{
		Service:          "github",
		Operation:        "POST /repos/foo/bar/secrets",
		Duration:         12 * time.Millisecond,
		Success:          true,
		RetryCount:       0,
		CacheHit:         false,
		StatusCodeBucket: "2xx",
	})

	require.Len(t, recorder.messages, 1)
	msg, ok := recorder.messages[0].(posthog.Capture)
	require.True(t, ok)
	assert.Equal(t, EventIntegrationCall, msg.Event)
	assert.Equal(t, "other", msg.Properties["operation"])
}

func TestWrapHTTPTransport_TracksResolvedOperation(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	recorder := &recorderStub{}
	SetRecorderForTesting(recorder, BuildInfo{Version: "v5.0.0", Platform: "linux/amd64"}, "install-123")
	TrackCommandStarted(CommandContext{
		CommandName:          "review",
		CommandPath:          "fotingo review",
		CommandSchemaVersion: "review@1.0.0",
		Persona:              "interactive_local",
		InvocationMode:       "human",
		GlobalFlags:          map[string]bool{},
		OptionFlags:          map[string]bool{},
		OptionCounts:         map[string]int{},
		OptionEnums:          map[string]string{},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: WrapHTTPTransport("github", nil, func(req *http.Request) string {
			if req.Method == http.MethodGet {
				return "get_latest_release"
			}
			return "other"
		}),
	}
	req, err := http.NewRequest(http.MethodGet, server.URL+"/repos/tagoro9/fotingo/releases/latest", nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	require.NoError(t, err)

	require.NotEmpty(t, recorder.messages)
	last, ok := recorder.messages[len(recorder.messages)-1].(posthog.Capture)
	require.True(t, ok)
	assert.Equal(t, EventIntegrationCall, last.Event)
	assert.Equal(t, "github", last.Properties["service"])
	assert.Equal(t, "get_latest_release", last.Properties["operation"])
	assert.Equal(t, "review", last.Properties["command_name"])
}

func TestTrackUpdateBannerShown_EmitsEvent(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	recorder := &recorderStub{}
	SetRecorderForTesting(recorder, BuildInfo{Version: "v5.0.0", Platform: "darwin/arm64"}, "install-123")

	TrackUpdateBannerShown(UpdateBannerEvent{
		CurrentVersion: "v5.0.0",
		LatestVersion:  "v5.1.0",
		Trigger:        "startup",
		Persona:        "interactive_local",
		InvocationMode: "interactive",
	})

	require.Len(t, recorder.messages, 1)
	msg, ok := recorder.messages[0].(posthog.Capture)
	require.True(t, ok)
	assert.Equal(t, EventUpdateBannerShown, msg.Event)
	assert.Equal(t, "v5.0.0", msg.Properties["current_version"])
	assert.Equal(t, "v5.1.0", msg.Properties["latest_version"])
}

func TestSanitizeAndValidateProperties_DropsUnknownAndEnforcesRequired(t *testing.T) {
	props := posthog.Properties{
		"event_schema_version":   "1.0.0",
		"persona":                "interactive_local",
		"invocation_mode":        "interactive",
		"version":                "v5.0.0",
		"platform":               "darwin/arm64",
		"command_name":           "review",
		"command_schema_version": "review@1.0.0",
		"global_flags":           map[string]bool{},
		"option_flags":           map[string]bool{},
		"option_counts":          map[string]int{},
		"option_enums":           map[string]string{},
		"unknown_key":            "drop-me",
	}

	sanitized := sanitizeAndValidateProperties(EventCommandStarted, props)
	require.NotNil(t, sanitized)
	_, hasUnknown := sanitized["unknown_key"]
	assert.False(t, hasUnknown)

	delete(props, "command_name")
	assert.Nil(t, sanitizeAndValidateProperties(EventCommandStarted, props))
}
