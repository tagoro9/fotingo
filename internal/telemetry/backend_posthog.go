package telemetry

import (
	"io"
	"log"
	"strings"
	"time"

	"github.com/posthog/posthog-go"
)

var (
	// posthogAPIKey is injected at build time via ldflags for release builds.
	posthogAPIKey = ""
	// posthogEndpoint defaults to PostHog US ingest.
	posthogEndpoint = "https://us.i.posthog.com"
)

type posthogBackend struct{}

// IsConfigured reports whether this build has a usable PostHog API key.
func (posthogBackend) IsConfigured() bool {
	return strings.TrimSpace(posthogAPIKey) != ""
}

// NewRecorder builds a PostHog-backed recorder tuned for short-lived CLI execution.
func (posthogBackend) NewRecorder(shutdownTimeout time.Duration) (recorder, error) {
	disableGeoIP := false
	client, err := posthog.NewWithConfig(strings.TrimSpace(posthogAPIKey), posthog.Config{
		Endpoint:        strings.TrimSpace(posthogEndpoint),
		DisableGeoIP:    &disableGeoIP,
		BatchSize:       20,
		Interval:        1500 * time.Millisecond,
		ShutdownTimeout: nonNegativeDuration(shutdownTimeout, 1200*time.Millisecond),
		Logger:          posthog.StdLogger(log.New(io.Discard, "", 0), false),
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}
