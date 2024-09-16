package jira

import (
	"net/http"
	"strings"

	"github.com/tagoro9/fotingo/internal/telemetry"
)

const jiraServiceName = "jira"

// wrapJiraHTTPClient returns a shallow client clone with telemetry instrumentation attached.
func wrapJiraHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}

	cloned := *client
	cloned.Transport = telemetry.WrapHTTPTransport(
		jiraServiceName,
		client.Transport,
		resolveJiraOperation,
	)
	return &cloned
}

// resolveJiraOperation maps outbound Jira API requests to low-cardinality operation names.
func resolveJiraOperation(req *http.Request) string {
	if req == nil || req.URL == nil {
		return "other"
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	path := normalizeJiraPath(req.URL.Path)

	switch {
	case method == http.MethodGet && strings.HasSuffix(path, "/myself"):
		return "get_current_user"
	case method == http.MethodGet && strings.HasPrefix(path, "/rest/api/") && strings.Contains(path, "/issue/") && strings.HasSuffix(path, "/transitions"):
		return "get_issue_transitions"
	case method == http.MethodPost && strings.HasPrefix(path, "/rest/api/") && strings.Contains(path, "/issue/") && strings.HasSuffix(path, "/transitions"):
		return "transition_issue"
	case method == http.MethodGet && strings.HasPrefix(path, "/rest/api/") && strings.Contains(path, "/issue/") && strings.HasSuffix(path, "/editmeta"):
		return "get_issue_editmeta"
	case method == http.MethodGet && strings.HasPrefix(path, "/rest/api/") && strings.Contains(path, "/issue/"):
		return "get_issue"
	case method == http.MethodPut && strings.HasPrefix(path, "/rest/api/") && strings.Contains(path, "/issue/"):
		return "update_issue"
	case method == http.MethodPost && strings.HasPrefix(path, "/rest/api/") && strings.Contains(path, "/issue/") && strings.HasSuffix(path, "/comment"):
		return "add_issue_comment"
	case (method == http.MethodGet || method == http.MethodPost) && strings.HasPrefix(path, "/rest/api/") &&
		(strings.Contains(path, "/search") || strings.Contains(path, "/search/jql")):
		return "search_issues"
	case method == http.MethodPost && strings.HasPrefix(path, "/rest/api/") && strings.HasSuffix(path, "/issue"):
		return "create_issue"
	case method == http.MethodGet && strings.HasPrefix(path, "/rest/api/") && strings.Contains(path, "/issue/createmeta"):
		return "get_create_meta"
	case method == http.MethodGet && strings.HasPrefix(path, "/rest/api/") && strings.HasSuffix(path, "/field"):
		return "list_fields"
	case method == http.MethodGet && strings.HasPrefix(path, "/rest/api/") && strings.HasSuffix(path, "/project"):
		return "list_projects"
	case method == http.MethodGet && strings.HasPrefix(path, "/rest/api/") && strings.Contains(path, "/project/"):
		return "get_project"
	case method == http.MethodPost && strings.HasPrefix(path, "/rest/api/") && strings.HasSuffix(path, "/version"):
		return "create_version"
	case method == http.MethodGet && strings.HasPrefix(path, "/oauth/token/accessible-resources"):
		return "resolve_accessible_resources"
	default:
		return "other"
	}
}

// normalizeJiraPath normalizes Jira API request paths for operation resolution.
func normalizeJiraPath(path string) string {
	normalized := "/" + strings.Trim(strings.ToLower(strings.TrimSpace(path)), "/")
	trimmed := strings.Trim(normalized, "/")
	segments := strings.Split(trimmed, "/")
	// OAuth-authenticated Jira API requests go through:
	// /ex/jira/{siteID}/rest/api/{version}/...
	// Strip that prefix so operation mapping stays stable across auth modes.
	if len(segments) >= 4 && segments[0] == "ex" && segments[1] == "jira" {
		normalized = "/" + strings.Join(segments[3:], "/")
	}
	if normalized == "" {
		return "/"
	}
	return normalized
}
