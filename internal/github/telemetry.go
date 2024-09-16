package github

import (
	"net/http"
	"strings"

	"github.com/tagoro9/fotingo/internal/telemetry"
)

const githubServiceName = "github"

// wrapGitHubHTTPClient returns a shallow client clone with telemetry instrumentation attached.
func wrapGitHubHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}

	cloned := *client
	cloned.Transport = telemetry.WrapHTTPTransport(
		githubServiceName,
		client.Transport,
		resolveGitHubOperation,
	)
	return &cloned
}

// resolveGitHubOperation maps outbound GitHub API requests to low-cardinality operation names.
func resolveGitHubOperation(req *http.Request) string {
	if req == nil || req.URL == nil {
		return "other"
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	path := normalizeGitHubPath(req.URL.Path)

	switch {
	case method == http.MethodGet && path == "/user":
		return "get_current_user"
	case method == http.MethodGet && strings.HasPrefix(path, "/repos/") && strings.HasSuffix(path, "/pulls"):
		return "list_pull_requests"
	case method == http.MethodPost && strings.HasPrefix(path, "/repos/") && strings.HasSuffix(path, "/pulls"):
		return "create_pull_request"
	case method == http.MethodGet && strings.HasPrefix(path, "/repos/") && strings.HasSuffix(path, "/labels"):
		return "list_labels"
	case method == http.MethodPost && strings.HasPrefix(path, "/repos/") && strings.Contains(path, "/issues/") && strings.HasSuffix(path, "/labels"):
		return "add_issue_labels"
	case method == http.MethodGet && strings.HasPrefix(path, "/repos/") && strings.HasSuffix(path, "/collaborators"):
		return "list_collaborators"
	case method == http.MethodGet && strings.HasPrefix(path, "/orgs/") && strings.HasSuffix(path, "/members"):
		return "list_org_members"
	case method == http.MethodGet && strings.HasPrefix(path, "/orgs/") && strings.HasSuffix(path, "/teams"):
		return "list_org_teams"
	case method == http.MethodPost && strings.HasPrefix(path, "/repos/") && strings.Contains(path, "/pulls/") && strings.HasSuffix(path, "/requested_reviewers"):
		return "request_reviewers"
	case method == http.MethodPost && strings.HasPrefix(path, "/repos/") && strings.Contains(path, "/issues/") && strings.HasSuffix(path, "/assignees"):
		return "add_assignees"
	case method == http.MethodGet && strings.HasPrefix(path, "/users/"):
		return "get_user_profile"
	case method == http.MethodPost && strings.HasPrefix(path, "/repos/") && strings.HasSuffix(path, "/releases"):
		return "create_release"
	case method == http.MethodGet && strings.HasPrefix(path, "/repos/") && strings.HasSuffix(path, "/releases/latest"):
		return "get_latest_release"
	default:
		return "other"
	}
}

// normalizeGitHubPath normalizes GitHub and GitHub Enterprise API request paths.
func normalizeGitHubPath(path string) string {
	normalized := "/" + strings.Trim(strings.ToLower(strings.TrimSpace(path)), "/")
	normalized = strings.TrimPrefix(normalized, "/api/v3")
	if normalized == "" {
		return "/"
	}
	return normalized
}
