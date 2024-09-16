package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/posthog/posthog-go"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/telemetry"
)

func TestResolveJiraOperation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		method    string
		path      string
		operation string
	}{
		{name: "get issue", method: http.MethodGet, path: "/rest/api/3/issue/PROJ-123", operation: "get_issue"},
		{name: "get issue oauth proxy path", method: http.MethodGet, path: "/ex/jira/site-123/rest/api/3/issue/PROJ-123", operation: "get_issue"},
		{name: "transition issue", method: http.MethodPost, path: "/rest/api/3/issue/PROJ-123/transitions", operation: "transition_issue"},
		{name: "search jql", method: http.MethodGet, path: "/rest/api/3/search/jql", operation: "search_issues"},
		{name: "accessible resources", method: http.MethodGet, path: "/oauth/token/accessible-resources", operation: "resolve_accessible_resources"},
		{name: "unknown", method: http.MethodDelete, path: "/rest/api/3/project/10000", operation: "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, "https://example.atlassian.net"+tt.path, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.operation, resolveJiraOperation(req))
		})
	}
}

func TestNewWithHTTPClient_TracksJiraIntegrationCalls(t *testing.T) {
	telemetry.ResetForTesting()
	defer telemetry.ResetForTesting()

	recorder := &telemetryRecorderStub{}
	telemetry.SetRecorderForTesting(
		recorder,
		telemetry.BuildInfo{Version: "v5.0.0", Platform: "darwin/arm64"},
		"install-123",
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/myself" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"accountId":    "abc123",
			"displayName":  "Test User",
			"emailAddress": "test@example.com",
		})
	}))
	defer server.Close()

	jiraClient, err := NewWithHTTPClient(viper.New(), server.Client(), server.URL)
	require.NoError(t, err)

	_, err = jiraClient.GetCurrentUser()
	require.NoError(t, err)

	capture, ok := findIntegrationCapture(recorder.messages)
	require.True(t, ok, "expected at least one integration telemetry capture")
	assert.Equal(t, telemetry.EventIntegrationCall, capture.Event)
	assert.Equal(t, "jira", capture.Properties["service"])
	assert.Equal(t, "get_current_user", capture.Properties["operation"])
	assert.Equal(t, true, capture.Properties["success"])
	assert.Equal(t, "2xx", capture.Properties["status_code_bucket"])
}

type telemetryRecorderStub struct {
	messages []posthog.Message
}

func (r *telemetryRecorderStub) Enqueue(msg posthog.Message) error {
	r.messages = append(r.messages, msg)
	return nil
}

func (r *telemetryRecorderStub) CloseWithContext(context.Context) error {
	return nil
}

func findIntegrationCapture(messages []posthog.Message) (posthog.Capture, bool) {
	for _, message := range messages {
		capture, ok := message.(posthog.Capture)
		if !ok {
			continue
		}
		if capture.Event == telemetry.EventIntegrationCall {
			return capture, true
		}
	}
	return posthog.Capture{}, false
}
