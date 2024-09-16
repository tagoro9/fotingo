package telemetry

import (
	"net/http"
	"strings"
	"time"
)

// OperationResolver maps an outbound HTTP request to a stable logical operation name.
type OperationResolver func(*http.Request) string

// WrapHTTPTransport instruments an HTTP transport and emits integration-call telemetry.
//
// Service packages are responsible for supplying a low-cardinality operation resolver
// (with allowlisted operations and "other" fallback).
func WrapHTTPTransport(service string, base http.RoundTripper, resolver OperationResolver) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if existing, ok := base.(*instrumentedRoundTripper); ok && strings.EqualFold(existing.service, service) {
		return base
	}
	return &instrumentedRoundTripper{
		service: service,
		base:    base,
		resolve: resolver,
	}
}

type instrumentedRoundTripper struct {
	service string
	base    http.RoundTripper
	resolve OperationResolver
}

func (t *instrumentedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	duration := time.Since(start)

	statusBucket := "network_error"
	success := false
	if resp != nil {
		statusBucket = StatusCodeBucket(resp.StatusCode)
		success = resp.StatusCode >= 200 && resp.StatusCode < 400 && err == nil
	}
	if resp == nil && err == nil {
		statusBucket = "unknown"
	}

	operation := "other"
	if t.resolve != nil {
		resolved := strings.TrimSpace(t.resolve(req))
		if resolved != "" {
			operation = resolved
		}
	}

	TrackIntegrationCall(IntegrationCall{
		Service:          t.service,
		Operation:        operation,
		Duration:         duration,
		Success:          success,
		RetryCount:       0,
		CacheHit:         false,
		StatusCodeBucket: statusBucket,
	})
	return resp, err
}

// StatusCodeBucket maps an HTTP status code into a stable bucket label.
func StatusCodeBucket(statusCode int) string {
	switch {
	case statusCode >= 100 && statusCode < 200:
		return "1xx"
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500 && statusCode < 600:
		return "5xx"
	default:
		return "unknown"
	}
}
