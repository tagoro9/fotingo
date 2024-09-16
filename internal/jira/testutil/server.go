// Package testutil provides mock HTTP server utilities for testing Jira client code.
package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// MockJiraServer provides a configurable HTTP test server that mimics Jira API responses.
// It supports configuring users, issues, projects, transitions, and versions for testing.
type MockJiraServer struct {
	*httptest.Server

	mu                sync.RWMutex
	users             map[string]*MockUser
	issues            map[string]*MockIssue
	projects          map[string]*MockProject
	projectIssueTypes map[string][]MockIssueType
	transitions       map[string][]MockTransition
	versions          map[string]*MockVersion
	comments          map[string][]MockComment

	// CurrentUser is the user returned by GET /rest/api/2/myself
	CurrentUser *MockUser

	// ErrorResponses allows configuring specific endpoints to return errors
	ErrorResponses map[string]*ErrorResponse

	// RequestLog records all requests made to the server
	RequestLog []RecordedRequest
}

// RecordedRequest stores information about a request made to the mock server.
type RecordedRequest struct {
	Method string
	Path   string
	Body   string
}

// ErrorResponse configures an error response for a specific endpoint.
type ErrorResponse struct {
	StatusCode    int
	ErrorMessages []string
}

// NewMockJiraServer creates a new mock Jira server with default configuration.
// The server starts automatically and should be closed with Close() when done.
func NewMockJiraServer() *MockJiraServer {
	m := &MockJiraServer{
		users:             make(map[string]*MockUser),
		issues:            make(map[string]*MockIssue),
		projects:          make(map[string]*MockProject),
		projectIssueTypes: make(map[string][]MockIssueType),
		transitions:       make(map[string][]MockTransition),
		versions:          make(map[string]*MockVersion),
		comments:          make(map[string][]MockComment),
		ErrorResponses:    make(map[string]*ErrorResponse),
		RequestLog:        make([]RecordedRequest, 0),
	}

	m.Server = httptest.NewServer(http.HandlerFunc(m.handleRequest))
	return m
}

// URL returns the base URL of the mock server.
func (m *MockJiraServer) URL() string {
	return m.Server.URL
}

// Close shuts down the mock server.
func (m *MockJiraServer) Close() {
	m.Server.Close()
}

// Reset clears all configured data and request log.
func (m *MockJiraServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.users = make(map[string]*MockUser)
	m.issues = make(map[string]*MockIssue)
	m.projects = make(map[string]*MockProject)
	m.projectIssueTypes = make(map[string][]MockIssueType)
	m.transitions = make(map[string][]MockTransition)
	m.versions = make(map[string]*MockVersion)
	m.comments = make(map[string][]MockComment)
	m.ErrorResponses = make(map[string]*ErrorResponse)
	m.RequestLog = make([]RecordedRequest, 0)
	m.CurrentUser = nil
}

// SetCurrentUser configures the user returned by GET /rest/api/2/myself.
func (m *MockJiraServer) SetCurrentUser(user *MockUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CurrentUser = user
}

// AddUser adds a user to the mock server.
func (m *MockJiraServer) AddUser(user *MockUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.AccountID] = user
}

// AddIssue adds an issue to the mock server.
func (m *MockJiraServer) AddIssue(issue *MockIssue) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.issues[issue.Key] = issue
}

// AddIssues adds multiple issues to the mock server.
func (m *MockJiraServer) AddIssues(issues ...*MockIssue) {
	for _, issue := range issues {
		m.AddIssue(issue)
	}
}

// GetIssue retrieves an issue by key for inspection in tests.
func (m *MockJiraServer) GetIssue(key string) *MockIssue {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.issues[key]
}

// AddProject adds a project to the mock server.
func (m *MockJiraServer) AddProject(project *MockProject) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.projects[project.Key] = project
}

// SetProjectIssueTypes configures available issue types for a project key.
func (m *MockJiraServer) SetProjectIssueTypes(projectKey string, issueTypes []MockIssueType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.projectIssueTypes[projectKey] = issueTypes
}

// SetTransitions configures the available transitions for an issue.
func (m *MockJiraServer) SetTransitions(issueKey string, transitions []MockTransition) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transitions[issueKey] = transitions
}

// AddVersion adds a version to the mock server.
func (m *MockJiraServer) AddVersion(version *MockVersion) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.versions[version.ID] = version
}

// SetErrorResponse configures an error response for a specific path pattern.
// The pattern should be the full path (e.g., "/rest/api/2/issue/TEST-123").
func (m *MockJiraServer) SetErrorResponse(pathPattern string, err *ErrorResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ErrorResponses[pathPattern] = err
}

// ClearErrorResponse removes an error response configuration.
func (m *MockJiraServer) ClearErrorResponse(pathPattern string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.ErrorResponses, pathPattern)
}

// GetRequestLog returns a copy of the request log.
func (m *MockJiraServer) GetRequestLog() []RecordedRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	log := make([]RecordedRequest, len(m.RequestLog))
	copy(log, m.RequestLog)
	return log
}

// handleRequest routes requests to the appropriate handler.
func (m *MockJiraServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Record the request
	m.recordRequest(r)

	// Check for configured error response
	if err := m.getErrorResponse(r.URL.Path); err != nil {
		m.writeErrorResponse(w, err.StatusCode, err.ErrorMessages)
		return
	}

	path := r.URL.Path

	switch {
	case path == "/rest/api/2/myself" && r.Method == http.MethodGet:
		m.handleGetCurrentUser(w, r)

	case (path == "/rest/api/2/search" || path == "/rest/api/2/search/jql") && r.Method == http.MethodGet:
		m.handleSearch(w, r)

	case strings.HasPrefix(path, "/rest/api/2/issue/") && strings.HasSuffix(path, "/transitions"):
		issueKey := extractIssueKey(path, "/transitions")
		switch r.Method {
		case http.MethodGet:
			m.handleGetTransitions(w, r, issueKey)
		case http.MethodPost:
			m.handleDoTransition(w, r, issueKey)
		default:
			m.writeNotFound(w)
		}

	case strings.HasPrefix(path, "/rest/api/2/issue/") && strings.HasSuffix(path, "/comment"):
		issueKey := extractIssueKey(path, "/comment")
		if r.Method == http.MethodPost {
			m.handleAddComment(w, r, issueKey)
		} else {
			m.writeNotFound(w)
		}

	case strings.HasPrefix(path, "/rest/api/2/issue/createmeta") && r.Method == http.MethodGet:
		m.handleGetCreateMeta(w, r)

	case strings.HasPrefix(path, "/rest/api/2/issue/"):
		issueKey := strings.TrimPrefix(path, "/rest/api/2/issue/")
		switch r.Method {
		case http.MethodGet:
			m.handleGetIssue(w, r, issueKey)
		case http.MethodPut:
			m.handleUpdateIssue(w, r, issueKey)
		default:
			m.writeNotFound(w)
		}

	case path == "/rest/api/2/issue" && r.Method == http.MethodPost:
		m.handleCreateIssue(w, r)

	case path == "/rest/api/2/version" && r.Method == http.MethodPost:
		m.handleCreateVersion(w, r)

	case strings.HasPrefix(path, "/rest/api/2/project/"):
		projectKey := strings.TrimPrefix(path, "/rest/api/2/project/")
		if r.Method == http.MethodGet {
			m.handleGetProject(w, r, projectKey)
		} else {
			m.writeNotFound(w)
		}

	default:
		m.writeNotFound(w)
	}
}

func (m *MockJiraServer) recordRequest(r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	recorded := RecordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
	}

	// Note: We don't read the body here to avoid consuming it
	// If body logging is needed, it should be done after reading in handlers

	m.RequestLog = append(m.RequestLog, recorded)
}

func (m *MockJiraServer) getErrorResponse(path string) *ErrorResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ErrorResponses[path]
}

func (m *MockJiraServer) handleGetCurrentUser(w http.ResponseWriter, _ *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.CurrentUser == nil {
		m.writeErrorResponse(w, http.StatusUnauthorized, []string{"Unauthorized"})
		return
	}

	m.writeJSON(w, http.StatusOK, m.CurrentUser.ToAPIResponse())
}

func (m *MockJiraServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// For simplicity, return all issues that match basic JQL criteria
	// In a real implementation, you'd parse the JQL query
	jql := r.URL.Query().Get("jql")
	var matchingIssues []map[string]interface{}

	for _, issue := range m.issues {
		// Simple JQL parsing - check if looking for unresolved issues
		if strings.Contains(jql, "resolution = Unresolved") {
			if issue.Resolution != "" {
				continue
			}
		}

		// Check assignee filter
		if strings.Contains(jql, "assignee = currentUser()") && m.CurrentUser != nil {
			if issue.Assignee == nil || issue.Assignee.AccountID != m.CurrentUser.AccountID {
				continue
			}
		}

		matchingIssues = append(matchingIssues, issue.ToAPIResponse())
	}

	response := map[string]interface{}{
		"startAt":    0,
		"maxResults": 50,
		"total":      len(matchingIssues),
		"issues":     matchingIssues,
	}

	m.writeJSON(w, http.StatusOK, response)
}

func (m *MockJiraServer) handleGetIssue(w http.ResponseWriter, _ *http.Request, issueKey string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	issue, exists := m.issues[issueKey]
	if !exists {
		m.writeErrorResponse(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."})
		return
	}

	m.writeJSON(w, http.StatusOK, issue.ToAPIResponse())
}

func (m *MockJiraServer) handleCreateIssue(w http.ResponseWriter, r *http.Request) {
	var createRequest struct {
		Fields struct {
			Summary     string `json:"summary"`
			Description string `json:"description"`
			Project     struct {
				Key string `json:"key"`
			} `json:"project"`
			IssueType struct {
				Name string `json:"name"`
			} `json:"issuetype"`
			Labels []string `json:"labels"`
			Parent *struct {
				Key string `json:"key"`
			} `json:"parent"`
		} `json:"fields"`
	}

	if err := json.NewDecoder(r.Body).Decode(&createRequest); err != nil {
		m.writeErrorResponse(w, http.StatusBadRequest, []string{"Invalid request body"})
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify project exists
	project, exists := m.projects[createRequest.Fields.Project.Key]
	if !exists {
		m.writeErrorResponse(w, http.StatusBadRequest, []string{"Project '" + createRequest.Fields.Project.Key + "' does not exist."})
		return
	}

	// Generate new issue key
	issueNum := len(m.issues) + 1
	newKey := project.Key + "-" + itoa(issueNum)
	newID := itoa(10000 + issueNum)

	newIssue := &MockIssue{
		ID:          newID,
		Key:         newKey,
		Summary:     createRequest.Fields.Summary,
		Description: createRequest.Fields.Description,
		Status:      MockStatus{ID: "1", Name: "Backlog"},
		IssueType:   MockIssueType{ID: "1", Name: createRequest.Fields.IssueType.Name},
		Project:     MockProject{ID: project.ID, Key: project.Key, Name: project.Name},
		Labels:      createRequest.Fields.Labels,
	}

	if createRequest.Fields.Parent != nil {
		newIssue.Parent = &MockParent{Key: createRequest.Fields.Parent.Key}
	}

	m.issues[newKey] = newIssue

	m.writeJSON(w, http.StatusCreated, newIssue.ToAPIResponse())
}

func (m *MockJiraServer) handleGetTransitions(w http.ResponseWriter, _ *http.Request, issueKey string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.issues[issueKey]
	if !exists {
		m.writeErrorResponse(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."})
		return
	}

	transitions := m.transitions[issueKey]
	if transitions == nil {
		transitions = []MockTransition{}
	}

	var transitionResponses []map[string]interface{}
	for _, t := range transitions {
		transitionResponses = append(transitionResponses, t.ToAPIResponse())
	}

	response := map[string]interface{}{
		"transitions": transitionResponses,
	}

	m.writeJSON(w, http.StatusOK, response)
}

func (m *MockJiraServer) handleDoTransition(w http.ResponseWriter, r *http.Request, issueKey string) {
	var transitionRequest struct {
		Transition struct {
			ID string `json:"id"`
		} `json:"transition"`
	}

	if err := json.NewDecoder(r.Body).Decode(&transitionRequest); err != nil {
		m.writeErrorResponse(w, http.StatusBadRequest, []string{"Invalid request body"})
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	issue, exists := m.issues[issueKey]
	if !exists {
		m.writeErrorResponse(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."})
		return
	}

	// Find the transition and apply it
	transitions := m.transitions[issueKey]
	for _, t := range transitions {
		if t.ID == transitionRequest.Transition.ID {
			issue.Status = MockStatus{ID: t.To.ID, Name: t.To.Name}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	m.writeErrorResponse(w, http.StatusBadRequest, []string{"Transition with id '" + transitionRequest.Transition.ID + "' is not valid for this issue."})
}

func (m *MockJiraServer) handleAddComment(w http.ResponseWriter, r *http.Request, issueKey string) {
	var commentRequest struct {
		Body string `json:"body"`
	}

	if err := json.NewDecoder(r.Body).Decode(&commentRequest); err != nil {
		m.writeErrorResponse(w, http.StatusBadRequest, []string{"Invalid request body"})
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.issues[issueKey]
	if !exists {
		m.writeErrorResponse(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."})
		return
	}

	commentID := itoa(len(m.comments[issueKey]) + 1001)
	newComment := MockComment{
		ID:   commentID,
		Body: commentRequest.Body,
	}

	m.comments[issueKey] = append(m.comments[issueKey], newComment)

	response := map[string]interface{}{
		"id":   newComment.ID,
		"body": newComment.Body,
	}

	m.writeJSON(w, http.StatusCreated, response)
}

func (m *MockJiraServer) handleUpdateIssue(w http.ResponseWriter, r *http.Request, issueKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	issue, exists := m.issues[issueKey]
	if !exists {
		m.writeErrorResponse(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."})
		return
	}

	var updateRequest map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updateRequest); err != nil {
		m.writeErrorResponse(w, http.StatusBadRequest, []string{"Invalid request body"})
		return
	}

	// Handle fixVersions update
	if update, ok := updateRequest["update"].(map[string]interface{}); ok {
		if fixVersions, ok := update["fixVersions"].([]interface{}); ok {
			for _, fv := range fixVersions {
				if fvMap, ok := fv.(map[string]interface{}); ok {
					if addMap, ok := fvMap["add"].(map[string]interface{}); ok {
						if name, ok := addMap["name"].(string); ok {
							issue.FixVersions = append(issue.FixVersions, MockVersion{Name: name})
						}
					}
				}
			}
		}
	}

	// Handle assignee updates
	if fields, ok := updateRequest["fields"].(map[string]interface{}); ok {
		if assignee, ok := fields["assignee"].(map[string]interface{}); ok {
			accountID, _ := assignee["accountId"].(string)
			if accountID != "" {
				if user, exists := m.users[accountID]; exists {
					issue.Assignee = user
				} else {
					issue.Assignee = &MockUser{
						AccountID:   accountID,
						DisplayName: accountID,
						Active:      true,
					}
				}
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (m *MockJiraServer) handleGetCreateMeta(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	projectKey := strings.TrimSpace(r.URL.Query().Get("projectKeys"))
	if projectKey == "" {
		m.writeErrorResponse(w, http.StatusBadRequest, []string{"projectKeys query parameter is required"})
		return
	}

	project, exists := m.projects[projectKey]
	if !exists {
		m.writeErrorResponse(w, http.StatusNotFound, []string{"No project could be found with key '" + projectKey + "'."})
		return
	}

	configuredIssueTypes := m.projectIssueTypes[projectKey]
	if len(configuredIssueTypes) == 0 {
		configuredIssueTypes = []MockIssueType{
			{ID: "1", Name: "Task", Description: "Task", Subtask: false},
			{ID: "2", Name: "Story", Description: "Story", Subtask: false},
			{ID: "3", Name: "Bug", Description: "Bug", Subtask: false},
		}
	}

	issueTypes := make([]map[string]interface{}, 0, len(configuredIssueTypes))
	for _, issueType := range configuredIssueTypes {
		issueTypes = append(issueTypes, issueType.ToAPIResponse())
	}

	response := map[string]interface{}{
		"projects": []map[string]interface{}{
			{
				"id":         project.ID,
				"key":        project.Key,
				"name":       project.Name,
				"issuetypes": issueTypes,
			},
		},
	}

	m.writeJSON(w, http.StatusOK, response)
}

func (m *MockJiraServer) handleCreateVersion(w http.ResponseWriter, r *http.Request) {
	var versionRequest struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ProjectID   int    `json:"projectId"`
		Released    bool   `json:"released"`
	}

	if err := json.NewDecoder(r.Body).Decode(&versionRequest); err != nil {
		m.writeErrorResponse(w, http.StatusBadRequest, []string{"Invalid request body"})
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	versionID := itoa(20000 + len(m.versions) + 1)
	newVersion := &MockVersion{
		ID:          versionID,
		Name:        versionRequest.Name,
		Description: versionRequest.Description,
		ProjectID:   versionRequest.ProjectID,
		Released:    versionRequest.Released,
	}

	m.versions[versionID] = newVersion

	response := map[string]interface{}{
		"id":          newVersion.ID,
		"name":        newVersion.Name,
		"description": newVersion.Description,
		"projectId":   newVersion.ProjectID,
		"released":    newVersion.Released,
	}

	m.writeJSON(w, http.StatusCreated, response)
}

func (m *MockJiraServer) handleGetProject(w http.ResponseWriter, _ *http.Request, projectKey string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, exists := m.projects[projectKey]
	if !exists {
		m.writeErrorResponse(w, http.StatusNotFound, []string{"No project could be found with key '" + projectKey + "'."})
		return
	}

	m.writeJSON(w, http.StatusOK, project.ToAPIResponse())
}

func (m *MockJiraServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (m *MockJiraServer) writeErrorResponse(w http.ResponseWriter, status int, messages []string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"errorMessages": messages,
	})
}

func (m *MockJiraServer) writeNotFound(w http.ResponseWriter) {
	m.writeErrorResponse(w, http.StatusNotFound, []string{"Not found"})
}

// extractIssueKey extracts the issue key from a path like /rest/api/2/issue/TEST-123/transitions
func extractIssueKey(path, suffix string) string {
	path = strings.TrimPrefix(path, "/rest/api/2/issue/")
	path = strings.TrimSuffix(path, suffix)
	return path
}

// itoa converts an int to string without importing strconv
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
