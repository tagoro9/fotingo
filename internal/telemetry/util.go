package telemetry

import "time"

func nonNegativeDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		if fallback <= 0 {
			return 0
		}
		return fallback
	}
	return value
}
