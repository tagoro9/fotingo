package commands

import (
	"context"
	"testing"

	"github.com/posthog/posthog-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	"github.com/tagoro9/fotingo/internal/telemetry"
)

type startupTelemetryRecorder struct {
	messages []posthog.Message
}

func (r *startupTelemetryRecorder) Enqueue(msg posthog.Message) error {
	r.messages = append(r.messages, msg)
	return nil
}

func (r *startupTelemetryRecorder) CloseWithContext(context.Context) error {
	return nil
}

func TestEmitPendingStartupAnnouncements_TracksUpdateBanner(t *testing.T) {
	telemetry.ResetForTesting()
	defer telemetry.ResetForTesting()

	recorder := &startupTelemetryRecorder{}
	telemetry.SetRecorderForTesting(
		recorder,
		telemetry.BuildInfo{Version: "v5.0.0", Platform: "darwin/arm64"},
		"install-123",
	)

	setPendingStartupAnnouncements([]startupAnnouncement{{
		Emoji:   commandruntime.LogEmojiRocket,
		Message: "update available",
		Kind:    commandruntime.AnnouncementKindUpdateBanner,
		Meta: map[string]string{
			"current_version": "v5.0.0",
			"latest_version":  "v5.1.0",
			"trigger":         "startup",
		},
	}})

	statusCh := make(chan string, 2)
	out := commandruntime.NewLocalizedEmitter(statusCh, func(commandruntime.OutputLevel) bool { return false }, localizer.T)
	emitPendingStartupAnnouncements(out)
	close(statusCh)

	require.Len(t, recorder.messages, 1)
	msg, ok := recorder.messages[0].(posthog.Capture)
	require.True(t, ok)
	assert.Equal(t, telemetry.EventUpdateBannerShown, msg.Event)
	assert.Equal(t, "v5.0.0", msg.Properties["current_version"])
	assert.Equal(t, "v5.1.0", msg.Properties["latest_version"])
}
