// Package testutil provides mock HTTP server utilities for testing GitHub client code.
package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"time"
)

// MockGitHubServer provides a configurable HTTP test server that mimics GitHub API responses.
// It supports configuring users, repositories, pull requests, collaborators, labels, and releases.
type MockGitHubServer struct {
	*httptest.Server

	mu            sync.RWMutex
	currentUser   *MockUser
	repositories  map[string]*MockRepository    // key: "owner/repo"
	pullRequests  map[string][]*MockPullRequest // key: "owner/repo"
	collaborators map[string][]*MockUser        // key: "owner/repo"
	orgMembers    map[string][]*MockUser        // key: "org"
	teams         map[string][]*MockTeam        // key: "org"
	labels        map[string][]*MockLabel       // key: "owner/repo"
	releases      map[string][]*MockRelease     // key: "owner/repo"

	// ErrorResponses allows configuring specific endpoints to return errors.
	// Info format: "METHOD /path" (e.g., "GET /user" or "POST /repos/owner/repo/pulls")
	ErrorResponses map[string]*ErrorResponse

	// RequestLog records all requests made to the server.
	RequestLog []RecordedRequest
}

// RecordedRequest stores information about a request made to the mock server.
type RecordedRequest struct {
	Method      string
	Path        string
	QueryParams map[string]string
	Body        string
}

// ErrorResponse configures an error response for a specific endpoint.
type ErrorResponse struct {
	StatusCode int
	Message    string
	Errors     []map[string]interface{}
}

// NewMockGitHubServer creates a new mock GitHub server with default configuration.
// The server starts automatically and should be closed with Close() when done.
func NewMockGitHubServer() *MockGitHubServer {
	m := &MockGitHubServer{
		repositories:   make(map[string]*MockRepository),
		pullRequests:   make(map[string][]*MockPullRequest),
		collaborators:  make(map[string][]*MockUser),
		orgMembers:     make(map[string][]*MockUser),
		teams:          make(map[string][]*MockTeam),
		labels:         make(map[string][]*MockLabel),
		releases:       make(map[string][]*MockRelease),
		ErrorResponses: make(map[string]*ErrorResponse),
		RequestLog:     make([]RecordedRequest, 0),
	}

	m.Server = httptest.NewServer(http.HandlerFunc(m.handleRequest))
	return m
}

// URL returns the base URL of the mock server.
func (m *MockGitHubServer) URL() string {
	return m.Server.URL
}

// Close shuts down the mock server.
func (m *MockGitHubServer) Close() {
	m.Server.Close()
}

// Reset clears all configured data and request log.
func (m *MockGitHubServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.currentUser = nil
	m.repositories = make(map[string]*MockRepository)
	m.pullRequests = make(map[string][]*MockPullRequest)
	m.collaborators = make(map[string][]*MockUser)
	m.orgMembers = make(map[string][]*MockUser)
	m.teams = make(map[string][]*MockTeam)
	m.labels = make(map[string][]*MockLabel)
	m.releases = make(map[string][]*MockRelease)
	m.ErrorResponses = make(map[string]*ErrorResponse)
	m.RequestLog = make([]RecordedRequest, 0)
}

// -----------------------------------------------------------------------
// Configuration Methods
// -----------------------------------------------------------------------

// SetCurrentUser configures the authenticated user returned by GET /user.
func (m *MockGitHubServer) SetCurrentUser(user *MockUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentUser = user
}

// AddRepository adds a repository to the mock server.
func (m *MockGitHubServer) AddRepository(repo *MockRepository) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.repositories[repo.FullName] = repo
}

// AddPullRequest adds a pull request to a repository.
func (m *MockGitHubServer) AddPullRequest(owner, repo string, pr *MockPullRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := owner + "/" + repo
	m.pullRequests[key] = append(m.pullRequests[key], pr)
}

// AddPullRequests adds multiple pull requests to a repository.
func (m *MockGitHubServer) AddPullRequests(owner, repo string, prs ...*MockPullRequest) {
	for _, pr := range prs {
		m.AddPullRequest(owner, repo, pr)
	}
}

// SetPullRequests sets all pull requests for a repository (replaces existing).
func (m *MockGitHubServer) SetPullRequests(owner, repo string, prs []*MockPullRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := owner + "/" + repo
	m.pullRequests[key] = prs
}

// AddCollaborator adds a collaborator to a repository.
func (m *MockGitHubServer) AddCollaborator(owner, repo string, user *MockUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := owner + "/" + repo
	m.collaborators[key] = append(m.collaborators[key], user)
}

// AddCollaborators adds multiple collaborators to a repository.
func (m *MockGitHubServer) AddCollaborators(owner, repo string, users ...*MockUser) {
	for _, user := range users {
		m.AddCollaborator(owner, repo, user)
	}
}

// SetCollaborators sets all collaborators for a repository (replaces existing).
func (m *MockGitHubServer) SetCollaborators(owner, repo string, users []*MockUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := owner + "/" + repo
	m.collaborators[key] = users
}

// AddOrgMember adds an organization member.
func (m *MockGitHubServer) AddOrgMember(org string, user *MockUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orgMembers[org] = append(m.orgMembers[org], user)
}

// AddOrgMembers adds multiple organization members.
func (m *MockGitHubServer) AddOrgMembers(org string, users ...*MockUser) {
	for _, user := range users {
		m.AddOrgMember(org, user)
	}
}

// SetOrgMembers sets all members for an organization (replaces existing).
func (m *MockGitHubServer) SetOrgMembers(org string, users []*MockUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orgMembers[org] = users
}

// AddTeam adds an organization team.
func (m *MockGitHubServer) AddTeam(org string, team *MockTeam) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.teams[org] = append(m.teams[org], team)
}

// AddTeams adds multiple organization teams.
func (m *MockGitHubServer) AddTeams(org string, teams ...*MockTeam) {
	for _, team := range teams {
		m.AddTeam(org, team)
	}
}

// SetTeams sets all teams for an organization (replaces existing).
func (m *MockGitHubServer) SetTeams(org string, teams []*MockTeam) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.teams[org] = teams
}

// AddLabel adds a label to a repository.
func (m *MockGitHubServer) AddLabel(owner, repo string, label *MockLabel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := owner + "/" + repo
	m.labels[key] = append(m.labels[key], label)
}

// AddLabels adds multiple labels to a repository.
func (m *MockGitHubServer) AddLabels(owner, repo string, labels ...*MockLabel) {
	for _, label := range labels {
		m.AddLabel(owner, repo, label)
	}
}

// SetLabels sets all labels for a repository (replaces existing).
func (m *MockGitHubServer) SetLabels(owner, repo string, labels []*MockLabel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := owner + "/" + repo
	m.labels[key] = labels
}

// AddRelease adds a release to a repository.
func (m *MockGitHubServer) AddRelease(owner, repo string, release *MockRelease) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := owner + "/" + repo
	m.releases[key] = append(m.releases[key], release)
}

// SetReleases sets all releases for a repository (replaces existing).
func (m *MockGitHubServer) SetReleases(owner, repo string, releases []*MockRelease) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := owner + "/" + repo
	m.releases[key] = releases
}

// SetErrorResponse configures an error response for a specific endpoint.
// The key format is "METHOD /path" (e.g., "GET /user" or "POST /repos/owner/repo/pulls").
func (m *MockGitHubServer) SetErrorResponse(key string, err *ErrorResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ErrorResponses[key] = err
}

// ClearErrorResponse removes an error response configuration.
func (m *MockGitHubServer) ClearErrorResponse(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.ErrorResponses, key)
}

// GetRequestLog returns a copy of the request log.
func (m *MockGitHubServer) GetRequestLog() []RecordedRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	log := make([]RecordedRequest, len(m.RequestLog))
	copy(log, m.RequestLog)
	return log
}

// GetPullRequest retrieves a pull request by number for inspection in tests.
func (m *MockGitHubServer) GetPullRequest(owner, repo string, number int) *MockPullRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := owner + "/" + repo
	for _, pr := range m.pullRequests[key] {
		if pr.Number == number {
			return pr
		}
	}
	return nil
}

// -----------------------------------------------------------------------
// Request Handling
// -----------------------------------------------------------------------

// handleRequest routes requests to the appropriate handler.
func (m *MockGitHubServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Record the request
	m.recordRequest(r)

	// Normalize path - remove /api/v3 prefix if present (for enterprise URLs)
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/v3")

	// Check for configured error response
	errorKey := r.Method + " " + path
	if err := m.getErrorResponse(errorKey); err != nil {
		m.writeErrorResponse(w, err)
		return
	}

	// Route the request
	switch {
	// GET /user - authenticated user
	case path == "/user" && r.Method == http.MethodGet:
		m.handleGetCurrentUser(w)

	// GET /repos/{owner}/{repo} - repository info
	case matchPath(path, "/repos/{owner}/{repo}") && r.Method == http.MethodGet:
		owner, repo := extractOwnerRepo(path)
		m.handleGetRepository(w, owner, repo)

	// GET /orgs/{org}/members - list org members
	case matchPath(path, "/orgs/{org}/members") && r.Method == http.MethodGet:
		org := extractOrg(path)
		m.handleListOrgMembers(w, r, org)

	// GET /orgs/{org}/teams - list org teams
	case matchPath(path, "/orgs/{org}/teams") && r.Method == http.MethodGet:
		org := extractOrg(path)
		m.handleListTeams(w, r, org)

	// GET /repos/{owner}/{repo}/pulls - list PRs
	case matchPath(path, "/repos/{owner}/{repo}/pulls") && r.Method == http.MethodGet:
		owner, repo := extractOwnerRepo(path)
		m.handleListPullRequests(w, r, owner, repo)

	// POST /repos/{owner}/{repo}/pulls - create PR
	case matchPath(path, "/repos/{owner}/{repo}/pulls") && r.Method == http.MethodPost:
		owner, repo := extractOwnerRepo(path)
		m.handleCreatePullRequest(w, r, owner, repo)

	// GET /repos/{owner}/{repo}/pulls/{number} - get PR
	case matchPathWithNumber(path, "/repos/{owner}/{repo}/pulls/{number}") && r.Method == http.MethodGet:
		owner, repo, number := extractOwnerRepoNumber(path)
		m.handleGetPullRequest(w, owner, repo, number)

	// PATCH /repos/{owner}/{repo}/pulls/{number} - edit PR
	case matchPathWithNumber(path, "/repos/{owner}/{repo}/pulls/{number}") && r.Method == http.MethodPatch:
		owner, repo, number := extractOwnerRepoNumber(path)
		m.handleEditPullRequest(w, r, owner, repo, number)

	// POST /repos/{owner}/{repo}/pulls/{number}/requested_reviewers - request reviewers
	case matchPath(path, "/repos/{owner}/{repo}/pulls/{number}/requested_reviewers") && r.Method == http.MethodPost:
		owner, repo, number := extractOwnerRepoNumber(strings.TrimSuffix(path, "/requested_reviewers"))
		m.handleRequestReviewers(w, r, owner, repo, number)

	// POST /repos/{owner}/{repo}/issues/{number}/labels - add labels
	case matchPath(path, "/repos/{owner}/{repo}/issues/{number}/labels") && r.Method == http.MethodPost:
		owner, repo, number := extractOwnerRepoNumberFromIssues(path)
		m.handleAddLabels(w, r, owner, repo, number)

	// POST /repos/{owner}/{repo}/issues/{number}/assignees - add assignees
	case matchPath(path, "/repos/{owner}/{repo}/issues/{number}/assignees") && r.Method == http.MethodPost:
		owner, repo, number := extractOwnerRepoNumberFromIssues(path)
		m.handleAddAssignees(w, r, owner, repo, number)

	// GET /repos/{owner}/{repo}/collaborators - list collaborators
	case matchPath(path, "/repos/{owner}/{repo}/collaborators") && r.Method == http.MethodGet:
		owner, repo := extractOwnerRepo(path)
		m.handleListCollaborators(w, r, owner, repo)

	// GET /repos/{owner}/{repo}/labels - list labels
	case matchPath(path, "/repos/{owner}/{repo}/labels") && r.Method == http.MethodGet:
		owner, repo := extractOwnerRepo(path)
		m.handleListLabels(w, r, owner, repo)

	// POST /repos/{owner}/{repo}/releases - create release
	case matchPath(path, "/repos/{owner}/{repo}/releases") && r.Method == http.MethodPost:
		owner, repo := extractOwnerRepo(path)
		m.handleCreateRelease(w, r, owner, repo)

	default:
		m.writeNotFound(w)
	}
}

func (m *MockGitHubServer) recordRequest(r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queryParams := make(map[string]string)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0]
		}
	}

	recorded := RecordedRequest{
		Method:      r.Method,
		Path:        r.URL.Path,
		QueryParams: queryParams,
	}

	m.RequestLog = append(m.RequestLog, recorded)
}

func (m *MockGitHubServer) getErrorResponse(key string) *ErrorResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ErrorResponses[key]
}

// -----------------------------------------------------------------------
// Handler Methods
// -----------------------------------------------------------------------

func (m *MockGitHubServer) handleGetCurrentUser(w http.ResponseWriter) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentUser == nil {
		m.writeErrorResponse(w, &ErrorResponse{
			StatusCode: http.StatusUnauthorized,
			Message:    "Bad credentials",
		})
		return
	}

	m.writeJSON(w, http.StatusOK, m.currentUser.ToAPIResponse())
}

func (m *MockGitHubServer) handleGetRepository(w http.ResponseWriter, owner, repo string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := owner + "/" + repo
	repository, exists := m.repositories[key]
	if !exists {
		m.writeErrorResponse(w, &ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("Repository %s not found", key),
		})
		return
	}

	m.writeJSON(w, http.StatusOK, repository.ToAPIResponse())
}

func (m *MockGitHubServer) handleListPullRequests(w http.ResponseWriter, r *http.Request, owner, repo string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := owner + "/" + repo
	prs := m.pullRequests[key]

	// Filter by head if specified
	headFilter := r.URL.Query().Get("head")
	stateFilter := r.URL.Query().Get("state")

	var filteredPRs []*MockPullRequest
	for _, pr := range prs {
		// Filter by head (format: "owner:branch")
		if headFilter != "" {
			expectedHead := strings.TrimPrefix(headFilter, owner+":")
			if pr.Head.Ref != expectedHead {
				continue
			}
		}

		// Filter by state
		if stateFilter != "" && stateFilter != "all" && pr.State != stateFilter {
			continue
		}

		filteredPRs = append(filteredPRs, pr)
	}

	var response []map[string]interface{}
	for _, pr := range filteredPRs {
		response = append(response, pr.ToAPIResponse())
	}

	if response == nil {
		response = []map[string]interface{}{}
	}

	m.writeJSON(w, http.StatusOK, response)
}

func (m *MockGitHubServer) handleCreatePullRequest(w http.ResponseWriter, r *http.Request, owner, repo string) {
	var createRequest struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		Head  string `json:"head"`
		Base  string `json:"base"`
		Draft bool   `json:"draft"`
	}

	if err := json.NewDecoder(r.Body).Decode(&createRequest); err != nil {
		m.writeErrorResponse(w, &ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request body",
		})
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := owner + "/" + repo

	// Check if PR already exists for this head
	for _, pr := range m.pullRequests[key] {
		if pr.Head.Ref == createRequest.Head && pr.State == "open" {
			m.writeErrorResponse(w, &ErrorResponse{
				StatusCode: http.StatusUnprocessableEntity,
				Message:    "Validation Failed",
				Errors: []map[string]interface{}{
					{"message": "A pull request already exists for " + owner + ":" + createRequest.Head},
				},
			})
			return
		}
	}

	// Generate new PR number
	newNumber := len(m.pullRequests[key]) + 1
	newPR := &MockPullRequest{
		ID:      int64(newNumber * 1000),
		Number:  newNumber,
		Title:   createRequest.Title,
		Body:    createRequest.Body,
		State:   "open",
		HTMLURL: fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, newNumber),
		URL:     fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, newNumber),
		Head:    MockPRRef{Ref: createRequest.Head, SHA: "abc123"},
		Base:    MockPRRef{Ref: createRequest.Base},
		Draft:   createRequest.Draft,
		User:    m.currentUser,
	}

	m.pullRequests[key] = append(m.pullRequests[key], newPR)

	m.writeJSON(w, http.StatusCreated, newPR.ToAPIResponse())
}

func (m *MockGitHubServer) handleGetPullRequest(w http.ResponseWriter, owner, repo string, number int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := owner + "/" + repo
	for _, pr := range m.pullRequests[key] {
		if pr.Number == number {
			m.writeJSON(w, http.StatusOK, pr.ToAPIResponse())
			return
		}
	}

	m.writeErrorResponse(w, &ErrorResponse{
		StatusCode: http.StatusNotFound,
		Message:    "Pull request not found",
	})
}

func (m *MockGitHubServer) handleEditPullRequest(w http.ResponseWriter, r *http.Request, owner, repo string, number int) {
	var editRequest struct {
		Title *string `json:"title"`
		Body  *string `json:"body"`
	}

	if err := json.NewDecoder(r.Body).Decode(&editRequest); err != nil {
		m.writeErrorResponse(w, &ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request body",
		})
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := owner + "/" + repo
	for _, pr := range m.pullRequests[key] {
		if pr.Number != number {
			continue
		}
		if editRequest.Title != nil {
			pr.Title = *editRequest.Title
		}
		if editRequest.Body != nil {
			pr.Body = *editRequest.Body
		}
		pr.UpdatedAt = time.Now()
		m.writeJSON(w, http.StatusOK, pr.ToAPIResponse())
		return
	}

	m.writeErrorResponse(w, &ErrorResponse{
		StatusCode: http.StatusNotFound,
		Message:    "Pull request not found",
	})
}

func (m *MockGitHubServer) handleRequestReviewers(w http.ResponseWriter, r *http.Request, owner, repo string, number int) {
	var reviewRequest struct {
		Reviewers     []string `json:"reviewers"`
		TeamReviewers []string `json:"team_reviewers"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reviewRequest); err != nil {
		m.writeErrorResponse(w, &ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request body",
		})
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	key := owner + "/" + repo
	var foundPR *MockPullRequest
	for _, pr := range m.pullRequests[key] {
		if pr.Number == number {
			foundPR = pr
			break
		}
	}

	if foundPR == nil {
		m.writeErrorResponse(w, &ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "Not Found",
		})
		return
	}

	// Validate reviewers are collaborators
	collaborators := m.collaborators[key]
	for _, reviewer := range reviewRequest.Reviewers {
		found := false
		for _, collab := range collaborators {
			if collab.Login == reviewer {
				found = true
				break
			}
		}
		if !found {
			m.writeErrorResponse(w, &ErrorResponse{
				StatusCode: http.StatusUnprocessableEntity,
				Message:    "Reviews may only be requested from collaborators",
			})
			return
		}
	}

	// Build response with requested reviewers
	var requestedReviewers []map[string]interface{}
	for _, reviewer := range reviewRequest.Reviewers {
		for _, collab := range collaborators {
			if collab.Login == reviewer {
				requestedReviewers = append(requestedReviewers, collab.ToAPIResponse())
				break
			}
		}
	}
	requestedTeams := make([]map[string]interface{}, 0, len(reviewRequest.TeamReviewers))
	for _, teamSlug := range reviewRequest.TeamReviewers {
		team := findTeamBySlug(m.teams[owner], teamSlug)
		if team == nil {
			team = NewTeam(0, owner, teamSlug, teamSlug, "")
		}
		requestedTeams = append(requestedTeams, team.ToAPIResponse())
	}

	response := foundPR.ToAPIResponse()
	response["requested_reviewers"] = requestedReviewers
	response["requested_teams"] = requestedTeams

	m.writeJSON(w, http.StatusCreated, response)
}

func (m *MockGitHubServer) handleAddLabels(w http.ResponseWriter, r *http.Request, owner, repo string, number int) {
	var labelRequest struct {
		Labels []string `json:"labels"`
	}

	if err := json.NewDecoder(r.Body).Decode(&labelRequest); err != nil {
		m.writeErrorResponse(w, &ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request body",
		})
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	key := owner + "/" + repo

	// Check if issue/PR exists
	found := false
	for _, pr := range m.pullRequests[key] {
		if pr.Number == number {
			found = true
			break
		}
	}

	if !found {
		m.writeErrorResponse(w, &ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "Not Found",
		})
		return
	}

	// Validate labels exist
	repoLabels := m.labels[key]
	var addedLabels []map[string]interface{}

	for _, labelName := range labelRequest.Labels {
		labelFound := false
		for _, label := range repoLabels {
			if label.Name == labelName {
				addedLabels = append(addedLabels, label.ToAPIResponse())
				labelFound = true
				break
			}
		}
		if !labelFound {
			// GitHub auto-creates labels if they don't exist, so we'll just add them
			addedLabels = append(addedLabels, map[string]interface{}{
				"name":  labelName,
				"color": "ededed",
			})
		}
	}

	m.writeJSON(w, http.StatusOK, addedLabels)
}

func (m *MockGitHubServer) handleAddAssignees(w http.ResponseWriter, r *http.Request, owner, repo string, number int) {
	var assigneeRequest struct {
		Assignees []string `json:"assignees"`
	}

	if err := json.NewDecoder(r.Body).Decode(&assigneeRequest); err != nil {
		m.writeErrorResponse(w, &ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request body",
		})
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := owner + "/" + repo
	var foundPR *MockPullRequest
	for _, pr := range m.pullRequests[key] {
		if pr.Number == number {
			foundPR = pr
			break
		}
	}

	if foundPR == nil {
		m.writeErrorResponse(w, &ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "Not Found",
		})
		return
	}

	assigned := make([]*MockUser, 0, len(assigneeRequest.Assignees))
	for _, login := range assigneeRequest.Assignees {
		user := findUserByLogin(m.collaborators[key], login)
		if user == nil {
			user = findUserByLogin(m.orgMembers[owner], login)
		}
		if user == nil {
			user = NewUser(0, login, login)
		}
		assigned = append(assigned, user)
	}

	foundPR.Assignees = assigned
	m.writeJSON(w, http.StatusCreated, foundPR.ToAPIResponse())
}

func (m *MockGitHubServer) handleListCollaborators(w http.ResponseWriter, r *http.Request, owner, repo string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := owner + "/" + repo
	collaborators := m.collaborators[key]

	var response []map[string]interface{}
	for _, user := range collaborators {
		response = append(response, user.ToAPIResponse())
	}

	if response == nil {
		response = []map[string]interface{}{}
	}

	// Handle pagination
	m.addPaginationHeaders(w, r, len(response))
	m.writeJSON(w, http.StatusOK, response)
}

func (m *MockGitHubServer) handleListOrgMembers(w http.ResponseWriter, r *http.Request, org string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	members := m.orgMembers[org]
	response := make([]map[string]interface{}, 0, len(members))
	for _, member := range members {
		response = append(response, member.ToAPIResponse())
	}

	m.addPaginationHeaders(w, r, len(response))
	m.writeJSON(w, http.StatusOK, response)
}

func (m *MockGitHubServer) handleListTeams(w http.ResponseWriter, r *http.Request, org string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	teams := m.teams[org]
	response := make([]map[string]interface{}, 0, len(teams))
	for _, team := range teams {
		response = append(response, team.ToAPIResponse())
	}

	m.addPaginationHeaders(w, r, len(response))
	m.writeJSON(w, http.StatusOK, response)
}

func (m *MockGitHubServer) handleListLabels(w http.ResponseWriter, r *http.Request, owner, repo string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := owner + "/" + repo
	labels := m.labels[key]

	var response []map[string]interface{}
	for _, label := range labels {
		response = append(response, label.ToAPIResponse())
	}

	if response == nil {
		response = []map[string]interface{}{}
	}

	// Handle pagination
	m.addPaginationHeaders(w, r, len(response))
	m.writeJSON(w, http.StatusOK, response)
}

func (m *MockGitHubServer) handleCreateRelease(w http.ResponseWriter, r *http.Request, owner, repo string) {
	var createRequest struct {
		TagName         string `json:"tag_name"`
		TargetCommitish string `json:"target_commitish"`
		Name            string `json:"name"`
		Body            string `json:"body"`
		Draft           bool   `json:"draft"`
		Prerelease      bool   `json:"prerelease"`
	}

	if err := json.NewDecoder(r.Body).Decode(&createRequest); err != nil {
		m.writeErrorResponse(w, &ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request body",
		})
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := owner + "/" + repo

	// Check if tag already exists
	for _, release := range m.releases[key] {
		if release.TagName == createRequest.TagName {
			m.writeErrorResponse(w, &ErrorResponse{
				StatusCode: http.StatusUnprocessableEntity,
				Message:    "Validation Failed",
				Errors: []map[string]interface{}{
					{"code": "already_exists", "field": "tag_name"},
				},
			})
			return
		}
	}

	// Generate new release
	newID := int64(len(m.releases[key]) + 1)
	newRelease := &MockRelease{
		ID:              newID,
		TagName:         createRequest.TagName,
		Name:            createRequest.Name,
		Body:            createRequest.Body,
		Draft:           createRequest.Draft,
		Prerelease:      createRequest.Prerelease,
		TargetCommitish: createRequest.TargetCommitish,
		URL:             fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/%d", owner, repo, newID),
		HTMLURL:         fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", owner, repo, createRequest.TagName),
	}

	m.releases[key] = append(m.releases[key], newRelease)

	m.writeJSON(w, http.StatusCreated, newRelease.ToAPIResponse())
}

// -----------------------------------------------------------------------
// Response Helpers
// -----------------------------------------------------------------------

func (m *MockGitHubServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (m *MockGitHubServer) writeErrorResponse(w http.ResponseWriter, err *ErrorResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode)

	response := map[string]interface{}{
		"message": err.Message,
	}
	if err.Errors != nil {
		response["errors"] = err.Errors
	}

	_ = json.NewEncoder(w).Encode(response)
}

func (m *MockGitHubServer) writeNotFound(w http.ResponseWriter) {
	m.writeErrorResponse(w, &ErrorResponse{
		StatusCode: http.StatusNotFound,
		Message:    "Not Found",
	})
}

func (m *MockGitHubServer) addPaginationHeaders(_ http.ResponseWriter, _ *http.Request, _ int) {
	// Simple pagination stub - in a real implementation, you'd handle page/per_page params
	// and add Link headers for pagination
}

// -----------------------------------------------------------------------
// Path Matching Helpers
// -----------------------------------------------------------------------

// matchPath checks if a path matches a pattern with {param} placeholders.
func matchPath(path, pattern string) bool {
	// Simple pattern matching - replace {param} with regex
	regexPattern := regexp.QuoteMeta(pattern)
	regexPattern = strings.ReplaceAll(regexPattern, "\\{owner\\}", "[^/]+")
	regexPattern = strings.ReplaceAll(regexPattern, "\\{repo\\}", "[^/]+")
	regexPattern = strings.ReplaceAll(regexPattern, "\\{org\\}", "[^/]+")
	regexPattern = strings.ReplaceAll(regexPattern, "\\{number\\}", "[0-9]+")
	regexPattern = "^" + regexPattern + "$"

	matched, _ := regexp.MatchString(regexPattern, path)
	return matched
}

// matchPathWithNumber checks if a path matches a pattern with a number at the end.
func matchPathWithNumber(path, pattern string) bool {
	return matchPath(path, pattern)
}

// extractOwnerRepo extracts owner and repo from a path like /repos/owner/repo/...
func extractOwnerRepo(path string) (owner, repo string) {
	parts := strings.Split(strings.TrimPrefix(path, "/repos/"), "/")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func extractOrg(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/orgs/"), "/")
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

// extractOwnerRepoNumber extracts owner, repo, and number from paths like /repos/owner/repo/pulls/123
func extractOwnerRepoNumber(path string) (owner, repo string, number int) {
	parts := strings.Split(strings.TrimPrefix(path, "/repos/"), "/")
	if len(parts) >= 4 {
		owner = parts[0]
		repo = parts[1]
		// parts[2] is "pulls"
		number = parseNumber(parts[3])
	}
	return
}

// extractOwnerRepoNumberFromIssues extracts owner, repo, and number from paths like /repos/owner/repo/issues/123/labels
func extractOwnerRepoNumberFromIssues(path string) (owner, repo string, number int) {
	parts := strings.Split(strings.TrimPrefix(path, "/repos/"), "/")
	if len(parts) >= 4 {
		owner = parts[0]
		repo = parts[1]
		// parts[2] is "issues"
		number = parseNumber(parts[3])
	}
	return
}

// parseNumber extracts an integer from a string, returning 0 if parsing fails.
func parseNumber(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

func findUserByLogin(users []*MockUser, login string) *MockUser {
	for _, user := range users {
		if user != nil && user.Login == login {
			return user
		}
	}
	return nil
}

func findTeamBySlug(teams []*MockTeam, slug string) *MockTeam {
	for _, team := range teams {
		if team != nil && team.Slug == slug {
			return team
		}
	}
	return nil
}
