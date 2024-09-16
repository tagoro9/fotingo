package github

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveGitHubOperation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		method    string
		path      string
		operation string
	}{
		{name: "list collaborators", method: http.MethodGet, path: "/repos/acme/fotingo/collaborators", operation: "list_collaborators"},
		{name: "request reviewers", method: http.MethodPost, path: "/repos/acme/fotingo/pulls/12/requested_reviewers", operation: "request_reviewers"},
		{name: "latest release", method: http.MethodGet, path: "/api/v3/repos/acme/fotingo/releases/latest", operation: "get_latest_release"},
		{name: "unknown", method: http.MethodDelete, path: "/repos/acme/fotingo/labels", operation: "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, "https://api.github.com"+tt.path, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.operation, resolveGitHubOperation(req))
		})
	}
}
