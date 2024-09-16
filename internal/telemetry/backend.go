package telemetry

import (
	"sync"
	"time"
)

// backend abstracts telemetry transport setup behind a vendor-agnostic boundary.
type backend interface {
	IsConfigured() bool
	NewRecorder(shutdownTimeout time.Duration) (recorder, error)
}

var (
	defaultBackendMu sync.RWMutex
	defaultBackendV  backend = posthogBackend{}

	defaultBackendConfiguredOverride *bool
)

func currentDefaultBackend() backend {
	defaultBackendMu.RLock()
	defer defaultBackendMu.RUnlock()
	return defaultBackendV
}

// IsDefaultBackendConfigured reports whether the process default telemetry backend is configured.
func IsDefaultBackendConfigured() bool {
	defaultBackendMu.RLock()
	override := defaultBackendConfiguredOverride
	defaultBackendMu.RUnlock()
	if override != nil {
		return *override
	}

	backend := currentDefaultBackend()
	if backend == nil {
		return false
	}
	return backend.IsConfigured()
}

// SetDefaultBackendConfiguredForTesting forces IsDefaultBackendConfigured for tests.
func SetDefaultBackendConfiguredForTesting(configured bool) func() {
	defaultBackendMu.Lock()
	previous := defaultBackendConfiguredOverride
	override := configured
	defaultBackendConfiguredOverride = &override
	defaultBackendMu.Unlock()

	return func() {
		defaultBackendMu.Lock()
		defer defaultBackendMu.Unlock()
		defaultBackendConfiguredOverride = previous
	}
}
