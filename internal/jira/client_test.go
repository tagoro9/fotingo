package jira

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	jiraClient "github.com/andygrunwald/go-jira"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tagoro9/fotingo/internal/config"
	"github.com/tagoro9/fotingo/internal/tracker"
)

// JiraTestSuite is the test suite for jira package
type JiraTestSuite struct {
	suite.Suite
	server *httptest.Server
	client *jira
}

// mockJiraIssue creates a mock Jira API response for an issue
func mockJiraIssue(id, key, summary, status, issueType string) map[string]interface{} {
	return map[string]interface{}{
		"id":  id,
		"key": key,
		"fields": map[string]interface{}{
			"summary": summary,
			"status": map[string]interface{}{
				"name": status,
			},
			"issuetype": map[string]interface{}{
				"name": issueType,
			},
		},
	}
}

// mockJiraTransition creates a mock Jira API response for a transition
func mockJiraTransition(id, name string) map[string]interface{} {
	return map[string]interface{}{
		"id":   id,
		"name": name,
		"to": map[string]interface{}{
			"name": name,
		},
	}
}

// mockJiraUser creates a mock Jira API response for a user
func mockJiraUser(accountID, displayName, email string) map[string]interface{} {
	return map[string]interface{}{
		"accountId":    accountID,
		"displayName":  displayName,
		"emailAddress": email,
		"avatarUrls": map[string]interface{}{
			"48x48": "https://example.com/avatar.png",
		},
	}
}

// mockJiraSearchResult creates a mock Jira API search result
func mockJiraSearchResult(issues []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"startAt":    0,
		"maxResults": 50,
		"total":      len(issues),
		"issues":     issues,
	}
}

// mockJiraComment creates a mock Jira API response for a comment
func mockJiraComment(id, body string) map[string]interface{} {
	return map[string]interface{}{
		"id":   id,
		"body": body,
	}
}

func (suite *JiraTestSuite) SetupTest() {
	// Create a new mock HTTP server for each test
	suite.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Default 404 response
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))

	// Create a Jira client pointing to the mock server
	httpClient := suite.server.Client()
	jClient, err := jiraClient.NewClient(httpClient, suite.server.URL)
	assert.NoError(suite.T(), err)

	suite.client = &jira{
		client:                   jClient,
		ViperConfigurableService: &config.ViperConfigurableService{Config: viper.New(), Prefix: "jira"},
		jiraRootURL:              "https://tagoro9.atlassian.net",
		allowPrompt:              false,
	}
}

func (suite *JiraTestSuite) TearDownTest() {
	if suite.server != nil {
		suite.server.Close()
	}
}

// setupMockServer creates a new mock server with the given handler
func (suite *JiraTestSuite) setupMockServer(handler http.HandlerFunc) {
	suite.server.Close()
	suite.server = httptest.NewServer(handler)

	httpClient := suite.server.Client()
	jClient, err := jiraClient.NewClient(httpClient, suite.server.URL)
	assert.NoError(suite.T(), err)
	suite.client.client = jClient
}

// TestGetIssue tests fetching an issue by ID
func (suite *JiraTestSuite) TestGetIssue() {
	tests := []struct {
		name           string
		issueID        string
		mockResponse   map[string]interface{}
		mockStatusCode int
		wantErr        bool
		wantIssue      *tracker.Issue
	}{
		{
			name:           "success - fetch story issue",
			issueID:        "TEST-123",
			mockResponse:   mockJiraIssue("10001", "TEST-123", "Test Story", "In Progress", "Story"),
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantIssue: &tracker.Issue{
				ID:      "10001",
				Key:     "TEST-123",
				Summary: "Test Story",
				Status:  tracker.IssueStatusInProgress,
				Type:    tracker.IssueTypeStory,
				URL:     "https://tagoro9.atlassian.net/browse/TEST-123",
			},
		},
		{
			name:           "success - fetch bug issue",
			issueID:        "BUG-456",
			mockResponse:   mockJiraIssue("10002", "BUG-456", "Test Bug", "Backlog", "Bug"),
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantIssue: &tracker.Issue{
				ID:      "10002",
				Key:     "BUG-456",
				Summary: "Test Bug",
				Status:  tracker.IssueStatusBacklog,
				Type:    tracker.IssueTypeBug,
				URL:     "https://tagoro9.atlassian.net/browse/BUG-456",
			},
		},
		{
			name:           "error - issue not found",
			issueID:        "NOTFOUND-999",
			mockResponse:   map[string]interface{}{"errorMessages": []string{"Issue does not exist"}},
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			wantIssue:      nil,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/rest/api/2/issue/%s", tt.issueID)
				if r.URL.Path == expectedPath && r.Method == http.MethodGet {
					w.WriteHeader(tt.mockStatusCode)
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			issue, err := suite.client.GetIssue(tt.issueID)

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), issue)
			} else {
				assert.NoError(suite.T(), err)
				assert.NotNil(suite.T(), issue)
				assert.Equal(suite.T(), tt.wantIssue.ID, issue.ID)
				assert.Equal(suite.T(), tt.wantIssue.Key, issue.Key)
				assert.Equal(suite.T(), tt.wantIssue.Summary, issue.Summary)
				assert.Equal(suite.T(), tt.wantIssue.Status, issue.Status)
				assert.Equal(suite.T(), tt.wantIssue.Type, issue.Type)
				assert.Equal(suite.T(), tt.wantIssue.URL, issue.URL)
			}
		})
	}
}

// TestGetJiraIssue tests fetching a Jira-specific issue
func (suite *JiraTestSuite) TestGetJiraIssue() {
	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/issue/TEST-123" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockJiraIssue("10001", "TEST-123", "Test Issue", "In Progress", "Story"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	issue, err := suite.client.GetJiraIssue("TEST-123")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), issue)
	assert.Equal(suite.T(), "10001", issue.Id)
	assert.Equal(suite.T(), "TEST-123", issue.Key)
	assert.Equal(suite.T(), "Test Issue", issue.Summary)
	assert.Equal(suite.T(), "In Progress", issue.Status)
	assert.Equal(suite.T(), "Story", issue.Type)
	assert.Equal(suite.T(), "", issue.ParentKey)
	assert.Equal(suite.T(), "", issue.EpicKey)
}

// TestGetUserOpenIssues tests searching for user's open issues
func (suite *JiraTestSuite) TestGetUserOpenIssues() {
	tests := []struct {
		name           string
		mockResponse   map[string]interface{}
		mockStatusCode int
		wantErr        bool
		wantCount      int
	}{
		{
			name: "success - multiple open issues",
			mockResponse: mockJiraSearchResult([]map[string]interface{}{
				mockJiraIssue("10001", "TEST-123", "First Issue", "In Progress", "Story"),
				mockJiraIssue("10002", "TEST-124", "Second Issue", "To Do", "Bug"),
				mockJiraIssue("10003", "TEST-125", "Third Issue", "In Review", "Task"),
			}),
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantCount:      3,
		},
		{
			name:           "success - no open issues",
			mockResponse:   mockJiraSearchResult([]map[string]interface{}{}),
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantCount:      0,
		},
		{
			name: "success - issue with missing status/type fields",
			mockResponse: mockJiraSearchResult([]map[string]interface{}{
				{
					"id":  "10010",
					"key": "TEST-999",
					"fields": map[string]interface{}{
						"summary": "Missing nested fields",
					},
				},
			}),
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantCount:      1,
		},
		{
			name:           "error - search fails",
			mockResponse:   map[string]interface{}{"errorMessages": []string{"Search failed"}},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
			wantCount:      0,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				if (r.URL.Path == "/rest/api/2/search" || r.URL.Path == "/rest/api/2/search/jql") && r.Method == http.MethodGet {
					w.WriteHeader(tt.mockStatusCode)
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			issues, err := suite.client.GetUserOpenIssues()

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), issues)
			} else {
				assert.NoError(suite.T(), err)
				assert.Len(suite.T(), issues, tt.wantCount)
			}
		})
	}
}

func (suite *JiraTestSuite) TestNewIssue_NilSafety() {
	suite.Run("nil issue", func() {
		got := newIssue(nil)
		assert.NotNil(suite.T(), got)
		assert.Equal(suite.T(), "", got.Id)
		assert.Equal(suite.T(), "", got.Key)
		assert.Equal(suite.T(), "", got.Summary)
		assert.Equal(suite.T(), "", got.Status)
		assert.Equal(suite.T(), "", got.Type)
	})

	suite.Run("missing fields", func() {
		got := newIssue(&jiraClient.Issue{
			ID:  "10010",
			Key: "TEST-999",
		})
		assert.NotNil(suite.T(), got)
		assert.Equal(suite.T(), "10010", got.Id)
		assert.Equal(suite.T(), "TEST-999", got.Key)
		assert.Equal(suite.T(), "", got.Summary)
		assert.Equal(suite.T(), "", got.Status)
		assert.Equal(suite.T(), "", got.Type)
	})

	suite.Run("missing status", func() {
		got := newIssue(&jiraClient.Issue{
			ID:  "10011",
			Key: "TEST-1000",
			Fields: &jiraClient.IssueFields{
				Summary: "Partial issue",
				Type:    jiraClient.IssueType{Name: "Story"},
			},
		})
		assert.NotNil(suite.T(), got)
		assert.Equal(suite.T(), "Partial issue", got.Summary)
		assert.Equal(suite.T(), "", got.Status)
		assert.Equal(suite.T(), "Story", got.Type)
	})

	suite.Run("extracts parent and epic keys", func() {
		got := newIssue(&jiraClient.Issue{
			ID:  "10012",
			Key: "TEST-1001",
			Fields: &jiraClient.IssueFields{
				Summary: "Hierarchy issue",
				Status:  &jiraClient.Status{Name: "In Progress"},
				Type:    jiraClient.IssueType{Name: "Story"},
				Parent:  &jiraClient.Parent{Key: "TEST-500"},
				Epic:    &jiraClient.Epic{Key: "TEST-900"},
			},
		})
		assert.Equal(suite.T(), "TEST-500", got.ParentKey)
		assert.Equal(suite.T(), "TEST-900", got.EpicKey)
	})

	suite.Run("epic fallback uses parent for non-subtasks", func() {
		got := newIssue(&jiraClient.Issue{
			ID:  "10013",
			Key: "TEST-1002",
			Fields: &jiraClient.IssueFields{
				Summary: "Epic via parent",
				Status:  &jiraClient.Status{Name: "To Do"},
				Type:    jiraClient.IssueType{Name: "Story"},
				Parent:  &jiraClient.Parent{Key: "TEST-901"},
			},
		})
		assert.Equal(suite.T(), "TEST-901", got.ParentKey)
		assert.Equal(suite.T(), "TEST-901", got.EpicKey)
	})

	suite.Run("epic fallback skipped for subtasks", func() {
		got := newIssue(&jiraClient.Issue{
			ID:  "10014",
			Key: "TEST-1003",
			Fields: &jiraClient.IssueFields{
				Summary: "Subtask",
				Status:  &jiraClient.Status{Name: "To Do"},
				Type:    jiraClient.IssueType{Name: "Sub-task"},
				Parent:  &jiraClient.Parent{Key: "TEST-100"},
			},
		})
		assert.Equal(suite.T(), "TEST-100", got.ParentKey)
		assert.Equal(suite.T(), "", got.EpicKey)
	})
}

// TestCreateIssue tests creating a new issue
func (suite *JiraTestSuite) TestCreateIssue() {
	tests := []struct {
		name           string
		input          tracker.CreateIssueInput
		mockResponse   map[string]interface{}
		mockStatusCode int
		wantErr        bool
		wantKey        string
	}{
		{
			name: "success - create story",
			input: tracker.CreateIssueInput{
				Title:       "New Story",
				Description: "This is a new story",
				Project:     "TEST",
				Type:        tracker.IssueTypeStory,
			},
			mockResponse:   mockJiraIssue("10001", "TEST-999", "New Story", "Backlog", "Story"),
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
			wantKey:        "TEST-999",
		},
		{
			name: "success - create bug with labels",
			input: tracker.CreateIssueInput{
				Title:   "Bug Report",
				Project: "TEST",
				Type:    tracker.IssueTypeBug,
				Labels:  []string{"bug", "critical"},
			},
			mockResponse:   mockJiraIssue("10002", "TEST-1000", "Bug Report", "Backlog", "Bug"),
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
			wantKey:        "TEST-1000",
		},
		{
			name: "success - create sub-task with parent",
			input: tracker.CreateIssueInput{
				Title:    "Sub-task",
				Project:  "TEST",
				Type:     tracker.IssueTypeSubTask,
				ParentID: "TEST-100",
			},
			mockResponse:   mockJiraIssue("10003", "TEST-1001", "Sub-task", "Backlog", "Sub-task"),
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
			wantKey:        "TEST-1001",
		},
		{
			name: "error - invalid project",
			input: tracker.CreateIssueInput{
				Title:   "Invalid Issue",
				Project: "INVALID",
				Type:    tracker.IssueTypeTask,
			},
			mockResponse:   map[string]interface{}{"errorMessages": []string{"Project not found"}},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
			wantKey:        "",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/2/issue" && r.Method == http.MethodPost {
					w.WriteHeader(tt.mockStatusCode)
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			issue, err := suite.client.CreateIssue(tt.input)

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), issue)
			} else {
				assert.NoError(suite.T(), err)
				assert.NotNil(suite.T(), issue)
				assert.Equal(suite.T(), tt.wantKey, issue.Key)
			}
		})
	}

	suite.Run("success - create issue with epic uses parent fallback when epic link field missing", func() {
		suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/rest/api/2/field" && r.Method == http.MethodGet:
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode([]map[string]interface{}{
					{"id": "summary", "name": "Summary", "schema": map[string]interface{}{"system": "summary"}},
				})
			case r.URL.Path == "/rest/api/2/issue" && r.Method == http.MethodPost:
				var payload map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&payload)
				require.NoError(suite.T(), err)

				fields, ok := payload["fields"].(map[string]interface{})
				require.True(suite.T(), ok)
				parent, ok := fields["parent"].(map[string]interface{})
				require.True(suite.T(), ok, "expected parent fallback field in create payload")
				assert.Equal(suite.T(), "TEST-200", parent["key"])

				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(mockJiraIssue("10004", "TEST-1002", "Task linked to epic", "Backlog", "Task"))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})

		issue, err := suite.client.CreateIssue(tracker.CreateIssueInput{
			Title:   "Task linked to epic",
			Project: "TEST",
			Type:    tracker.IssueTypeTask,
			EpicID:  "TEST-200",
		})

		assert.NoError(suite.T(), err)
		assert.NotNil(suite.T(), issue)
		assert.Equal(suite.T(), "TEST-1002", issue.Key)
	})

	suite.Run("success - resolve epic field by schema custom id", func() {
		suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/rest/api/2/field" && r.Method == http.MethodGet {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode([]map[string]interface{}{
					{
						"id":   "customfield_10008",
						"name": "Vinculo",
						"schema": map[string]interface{}{
							"custom": jiraEpicLinkSchemaCustom,
						},
					},
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		fieldID, err := suite.client.resolveEpicLinkFieldID()
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), "customfield_10008", fieldID)
	})

	suite.Run("success - create subtask using preferred issue type name", func() {
		suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/rest/api/2/issue" && r.Method == http.MethodPost {
				var payload map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&payload)
				require.NoError(suite.T(), err)

				fields, ok := payload["fields"].(map[string]interface{})
				require.True(suite.T(), ok)
				issueType, ok := fields["issuetype"].(map[string]interface{})
				require.True(suite.T(), ok)
				assert.Equal(suite.T(), "Subtask", issueType["name"])

				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(mockJiraIssue("10005", "TEST-1003", "Subtask from preferred", "Backlog", "Subtask"))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		issue, err := suite.client.CreateIssue(tracker.CreateIssueInput{
			Title:    "Subtask from preferred",
			Project:  "TEST",
			Type:     tracker.IssueTypeSubTask,
			TypeName: "Subtask",
			ParentID: "TEST-100",
		})

		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), issue)
		assert.Equal(suite.T(), "TEST-1003", issue.Key)
	})
}

// TestAddComment tests adding a comment to an issue
func (suite *JiraTestSuite) TestAddComment() {
	tests := []struct {
		name           string
		issueID        string
		comment        string
		mockResponse   map[string]interface{}
		mockStatusCode int
		wantErr        bool
	}{
		{
			name:           "success - add comment",
			issueID:        "TEST-123",
			comment:        "This is a test comment",
			mockResponse:   mockJiraComment("1001", "This is a test comment"),
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
		},
		{
			name:           "error - issue not found",
			issueID:        "NOTFOUND-999",
			comment:        "Comment on non-existent issue",
			mockResponse:   map[string]interface{}{"errorMessages": []string{"Issue does not exist"}},
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/rest/api/2/issue/%s/comment", tt.issueID)
				if r.URL.Path == expectedPath && r.Method == http.MethodPost {
					w.WriteHeader(tt.mockStatusCode)
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			err := suite.client.AddComment(tt.issueID, tt.comment)

			if tt.wantErr {
				assert.Error(suite.T(), err)
			} else {
				assert.NoError(suite.T(), err)
			}
		})
	}
}

// TestSetIssueStatus tests transitioning issue status
func (suite *JiraTestSuite) TestSetIssueStatus() {
	tests := []struct {
		name            string
		issueID         string
		status          tracker.IssueStatus
		mockTransitions []map[string]interface{}
		mockIssue       map[string]interface{}
		mockStatusCode  int
		wantErr         bool
		wantFinalStatus tracker.IssueStatus
	}{
		{
			name:    "success - transition to in progress",
			issueID: "TEST-123",
			status:  tracker.IssueStatusInProgress,
			mockTransitions: []map[string]interface{}{
				mockJiraTransition("21", "Start Progress"),
				mockJiraTransition("31", "Done"),
			},
			mockIssue:       mockJiraIssue("10001", "TEST-123", "Test Issue", "In Progress", "Story"),
			mockStatusCode:  http.StatusOK,
			wantErr:         false,
			wantFinalStatus: tracker.IssueStatusInProgress,
		},
		{
			name:    "success - transition to done",
			issueID: "TEST-124",
			status:  tracker.IssueStatusDone,
			mockTransitions: []map[string]interface{}{
				mockJiraTransition("21", "Start Progress"),
				mockJiraTransition("31", "Complete"),
			},
			mockIssue:       mockJiraIssue("10002", "TEST-124", "Test Issue", "Done", "Story"),
			mockStatusCode:  http.StatusOK,
			wantErr:         false,
			wantFinalStatus: tracker.IssueStatusDone,
		},
		{
			name:    "success - exact match transition",
			issueID: "TEST-125",
			status:  tracker.IssueStatusInProgress,
			mockTransitions: []map[string]interface{}{
				mockJiraTransition("21", "In Progress"),
			},
			mockIssue:       mockJiraIssue("10003", "TEST-125", "Test Issue", "In Progress", "Story"),
			mockStatusCode:  http.StatusOK,
			wantErr:         false,
			wantFinalStatus: tracker.IssueStatusInProgress,
		},
		{
			name:    "error - no matching transition",
			issueID: "TEST-126",
			status:  tracker.IssueStatusInReview,
			mockTransitions: []map[string]interface{}{
				mockJiraTransition("21", "Open"),
				mockJiraTransition("31", "Close"),
			},
			mockIssue:      mockJiraIssue("10004", "TEST-126", "Test Issue", "Open", "Story"),
			mockStatusCode: http.StatusOK,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				// Handle transitions request
				transitionsPath := fmt.Sprintf("/rest/api/2/issue/%s/transitions", tt.issueID)
				if r.URL.Path == transitionsPath && r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"transitions": tt.mockTransitions,
					})
					return
				}

				// Handle transition POST request
				if r.URL.Path == transitionsPath && r.Method == http.MethodPost {
					w.WriteHeader(http.StatusNoContent)
					return
				}

				// Handle issue GET request (after transition)
				issuePath := fmt.Sprintf("/rest/api/2/issue/%s", tt.issueID)
				if r.URL.Path == issuePath && r.Method == http.MethodGet {
					w.WriteHeader(tt.mockStatusCode)
					_ = json.NewEncoder(w).Encode(tt.mockIssue)
					return
				}

				w.WriteHeader(http.StatusNotFound)
			})

			issue, err := suite.client.SetIssueStatus(tt.issueID, tt.status)

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), issue)
			} else {
				assert.NoError(suite.T(), err)
				assert.NotNil(suite.T(), issue)
				assert.Equal(suite.T(), tt.wantFinalStatus, issue.Status)
			}
		})
	}
}

// TestSetJiraIssueStatus tests the Jira-specific status transition
func (suite *JiraTestSuite) TestSetJiraIssueStatus() {
	suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		// Handle transitions request
		if r.URL.Path == "/rest/api/2/issue/TEST-123/transitions" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"transitions": []map[string]interface{}{
					mockJiraTransition("21", "In Progress"),
				},
			})
			return
		}

		// Handle transition POST request
		if r.URL.Path == "/rest/api/2/issue/TEST-123/transitions" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Handle issue GET request
		if r.URL.Path == "/rest/api/2/issue/TEST-123" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockJiraIssue("10001", "TEST-123", "Test Issue", "In Progress", "Story"))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	issue, err := suite.client.SetJiraIssueStatus("TEST-123", StatusInProgress)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), issue)
	assert.Equal(suite.T(), "In Progress", issue.Status)
}

// TestGetCurrentUser tests fetching the current user
func (suite *JiraTestSuite) TestGetCurrentUser() {
	tests := []struct {
		name           string
		mockResponse   map[string]interface{}
		mockStatusCode int
		wantErr        bool
		wantUser       *tracker.User
	}{
		{
			name:           "success - get current user",
			mockResponse:   mockJiraUser("user-123", "John Doe", "john@example.com"),
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantUser: &tracker.User{
				ID:        "user-123",
				Name:      "John Doe",
				Email:     "john@example.com",
				AvatarURL: "https://example.com/avatar.png",
			},
		},
		{
			name:           "error - unauthorized",
			mockResponse:   map[string]interface{}{"message": "Unauthorized"},
			mockStatusCode: http.StatusUnauthorized,
			wantErr:        true,
			wantUser:       nil,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/2/myself" && r.Method == http.MethodGet {
					w.WriteHeader(tt.mockStatusCode)
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			user, err := suite.client.GetCurrentUser()

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), user)
			} else {
				assert.NoError(suite.T(), err)
				assert.NotNil(suite.T(), user)
				assert.Equal(suite.T(), tt.wantUser.ID, user.ID)
				assert.Equal(suite.T(), tt.wantUser.Name, user.Name)
				assert.Equal(suite.T(), tt.wantUser.Email, user.Email)
				assert.Equal(suite.T(), tt.wantUser.AvatarURL, user.AvatarURL)
			}
		})
	}
}

// TestIsValidIssueID tests issue ID validation (pure unit test, no HTTP)
func (suite *JiraTestSuite) TestIsValidIssueID() {
	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		// Valid issue IDs
		{name: "valid - simple", id: "TEST-123", valid: true},
		{name: "valid - lowercase converted", id: "test-123", valid: true},
		{name: "valid - long project key", id: "MYPROJECT-1", valid: true},
		{name: "valid - with numbers in key", id: "TEST2-456", valid: true},
		{name: "valid - with underscore", id: "TEST_PROJ-789", valid: true},
		{name: "valid - large number", id: "ABC-99999", valid: true},

		// Invalid issue IDs
		{name: "invalid - no hyphen", id: "TEST123", valid: false},
		{name: "invalid - no number", id: "TEST-", valid: false},
		{name: "invalid - starts with number", id: "123-TEST", valid: false},
		{name: "invalid - empty", id: "", valid: false},
		{name: "invalid - only numbers", id: "123-456", valid: false},
		{name: "invalid - special chars", id: "TEST@-123", valid: false},
		{name: "invalid - spaces", id: "TEST -123", valid: false},
		{name: "invalid - double hyphen", id: "TEST--123", valid: false},
		{name: "invalid - just text", id: "TESTPROJECT", valid: false},
		{name: "invalid - just number", id: "12345", valid: false},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := suite.client.IsValidIssueID(tt.id)
			assert.Equal(suite.T(), tt.valid, result, "IsValidIssueID(%q) = %v, want %v", tt.id, result, tt.valid)
		})
	}
}

// TestGetIssueURL tests the issue URL generation
func (suite *JiraTestSuite) TestGetIssueURL() {
	tests := []struct {
		name     string
		issueID  string
		expected string
	}{
		{
			name:     "uppercase issue ID",
			issueID:  "TEST-123",
			expected: "https://tagoro9.atlassian.net/browse/TEST-123",
		},
		{
			name:     "lowercase issue ID - converted to uppercase",
			issueID:  "test-456",
			expected: "https://tagoro9.atlassian.net/browse/TEST-456",
		},
		{
			name:     "mixed case issue ID",
			issueID:  "Test-789",
			expected: "https://tagoro9.atlassian.net/browse/TEST-789",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			url := suite.client.GetIssueURL(tt.issueID)
			assert.Equal(suite.T(), tt.expected, url)
		})
	}
}

// TestIssueShortName tests the Issue ShortName method
func (suite *JiraTestSuite) TestIssueShortName() {
	tests := []struct {
		issueType string
		expected  string
	}{
		{issueType: "Bug", expected: "b"},
		{issueType: "bug", expected: "b"},
		{issueType: "BUG", expected: "b"},
		{issueType: "Feature", expected: "f"},
		{issueType: "Story", expected: "f"},
		{issueType: "Sub-task", expected: "f"},
		{issueType: "Spike", expected: "s"},
		{issueType: "Task", expected: "c"},
		{issueType: "Tech Debt", expected: "d"},
		{issueType: "Unknown", expected: "f"},
		{issueType: "", expected: "f"},
	}

	for _, tt := range tests {
		suite.Run(tt.issueType, func() {
			issue := &Issue{Type: tt.issueType}
			assert.Equal(suite.T(), tt.expected, issue.ShortName())
		})
	}
}

// TestIssueSanitizedSummary tests the Issue SanitizedSummary method
func (suite *JiraTestSuite) TestIssueSanitizedSummary() {
	tests := []struct {
		name     string
		summary  string
		expected string
	}{
		{
			name:     "simple summary",
			summary:  "Add new feature",
			expected: "add_new_feature",
		},
		{
			name:     "summary with special chars",
			summary:  "Fix: bug in the 'login' page",
			expected: "fix_bug_in_the_login_page",
		},
		{
			name:     "summary with slashes",
			summary:  "Update user/profile endpoint",
			expected: "update_user-profile_endpoint",
		},
		{
			name:     "summary with parentheses",
			summary:  "Add feature (beta)",
			expected: "add_feature__beta",
		},
		{
			name:     "summary exceeding 72 chars",
			summary:  "This is a very long summary that exceeds the seventy two character limit and should be truncated",
			expected: "this_is_a_very_long_summary_that_exceeds_the_seventy_two_character_limit",
		},
		{
			name:     "summary with trailing underscore",
			summary:  "Fix bug ",
			expected: "fix_bug",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			issue := &Issue{Summary: tt.summary}
			assert.Equal(suite.T(), tt.expected, issue.SanitizedSummary())
		})
	}
}

// TestJiraIssueStatusToTracker tests status conversion from Jira to tracker
func (suite *JiraTestSuite) TestJiraIssueStatusToTracker() {
	tests := []struct {
		jiraStatus    string
		trackerStatus tracker.IssueStatus
	}{
		{jiraStatus: "Backlog", trackerStatus: tracker.IssueStatusBacklog},
		{jiraStatus: "Product Backlog", trackerStatus: tracker.IssueStatusBacklog},
		{jiraStatus: "To Do", trackerStatus: tracker.IssueStatusToDo},
		{jiraStatus: "Todo", trackerStatus: tracker.IssueStatusToDo},
		{jiraStatus: "Selected for Development", trackerStatus: tracker.IssueStatusToDo},
		{jiraStatus: "In Progress", trackerStatus: tracker.IssueStatusInProgress},
		{jiraStatus: "Work in Progress", trackerStatus: tracker.IssueStatusInProgress},
		{jiraStatus: "In Review", trackerStatus: tracker.IssueStatusInReview},
		{jiraStatus: "Code Review", trackerStatus: tracker.IssueStatusInReview},
		{jiraStatus: "Done", trackerStatus: tracker.IssueStatusDone},
		{jiraStatus: "Complete", trackerStatus: tracker.IssueStatusDone},
		{jiraStatus: "Resolved", trackerStatus: tracker.IssueStatusDone},
		{jiraStatus: "Unknown Status", trackerStatus: tracker.IssueStatusBacklog},
	}

	for _, tt := range tests {
		suite.Run(tt.jiraStatus, func() {
			result := jiraIssueStatusToTracker(tt.jiraStatus)
			assert.Equal(suite.T(), tt.trackerStatus, result)
		})
	}
}

// TestTrackerStatusToJira tests status conversion from tracker to Jira
func (suite *JiraTestSuite) TestTrackerStatusToJira() {
	tests := []struct {
		trackerStatus tracker.IssueStatus
		jiraStatus    IssueStatus
	}{
		{trackerStatus: tracker.IssueStatusBacklog, jiraStatus: StatusBacklog},
		{trackerStatus: tracker.IssueStatusToDo, jiraStatus: StatusSelectedForDevelopment},
		{trackerStatus: tracker.IssueStatusInProgress, jiraStatus: StatusInProgress},
		{trackerStatus: tracker.IssueStatusInReview, jiraStatus: StatusInReview},
		{trackerStatus: tracker.IssueStatusDone, jiraStatus: StatusDone},
	}

	for _, tt := range tests {
		suite.Run(string(tt.trackerStatus), func() {
			result := trackerStatusToJira(tt.trackerStatus)
			assert.Equal(suite.T(), tt.jiraStatus, result)
		})
	}
}

// TestJiraIssueTypeToTracker tests issue type conversion from Jira to tracker
func (suite *JiraTestSuite) TestJiraIssueTypeToTracker() {
	tests := []struct {
		jiraType    string
		trackerType tracker.IssueType
	}{
		{jiraType: "Story", trackerType: tracker.IssueTypeStory},
		{jiraType: "Feature", trackerType: tracker.IssueTypeStory},
		{jiraType: "Bug", trackerType: tracker.IssueTypeBug},
		{jiraType: "Task", trackerType: tracker.IssueTypeTask},
		{jiraType: "Sub-task", trackerType: tracker.IssueTypeSubTask},
		{jiraType: "Subtask", trackerType: tracker.IssueTypeSubTask},
		{jiraType: "Epic", trackerType: tracker.IssueTypeEpic},
		{jiraType: "Unknown", trackerType: tracker.IssueTypeTask},
	}

	for _, tt := range tests {
		suite.Run(tt.jiraType, func() {
			result := jiraIssueTypeToTracker(tt.jiraType)
			assert.Equal(suite.T(), tt.trackerType, result)
		})
	}
}

// TestTrackerIssueTypeToJira tests issue type conversion from tracker to Jira
func (suite *JiraTestSuite) TestTrackerIssueTypeToJira() {
	tests := []struct {
		trackerType tracker.IssueType
		jiraType    string
	}{
		{trackerType: tracker.IssueTypeStory, jiraType: "Story"},
		{trackerType: tracker.IssueTypeBug, jiraType: "Bug"},
		{trackerType: tracker.IssueTypeTask, jiraType: "Task"},
		{trackerType: tracker.IssueTypeSubTask, jiraType: "Sub-task"},
		{trackerType: tracker.IssueTypeEpic, jiraType: "Epic"},
	}

	for _, tt := range tests {
		suite.Run(string(tt.trackerType), func() {
			result := trackerIssueTypeToJira(tt.trackerType)
			assert.Equal(suite.T(), tt.jiraType, result)
		})
	}
}

// TestName tests the Name method
func (suite *JiraTestSuite) TestName() {
	assert.Equal(suite.T(), "Jira", suite.client.Name())
}

// TestGetIssueUrl tests the deprecated GetIssueUrl method
func (suite *JiraTestSuite) TestGetIssueUrl() {
	tests := []struct {
		name     string
		issueID  string
		expected string
	}{
		{
			name:     "uppercase issue ID",
			issueID:  "TEST-123",
			expected: "https://tagoro9.atlassian.net/browse/TEST-123",
		},
		{
			name:     "lowercase issue ID",
			issueID:  "test-456",
			expected: "https://tagoro9.atlassian.net/browse/TEST-456",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			url, err := suite.client.GetIssueUrl(tt.issueID)
			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), tt.expected, url)
		})
	}
}

// TestCreateRelease tests creating a new release/version
func (suite *JiraTestSuite) TestCreateRelease() {
	tests := []struct {
		name           string
		input          tracker.CreateReleaseInput
		mockStatusCode int
		setupMock      func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		wantRelease    *tracker.Release
	}{
		{
			name: "success - create release",
			input: tracker.CreateReleaseInput{
				Name:        "v1.0.0",
				Description: "Release 1.0.0",
				IssueIDs:    []string{"TEST-123"},
			},
			mockStatusCode: http.StatusCreated,
			setupMock: func(w http.ResponseWriter, r *http.Request) {
				// Get issue to extract project
				if r.URL.Path == "/rest/api/2/issue/TEST-123" && r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(mockJiraIssue("10001", "TEST-123", "Test Issue", "Done", "Story"))
					return
				}

				// Get project
				if r.URL.Path == "/rest/api/2/project/TEST" && r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"id":   "10000",
						"key":  "TEST",
						"name": "Test Project",
					})
					return
				}

				// Create version
				if r.URL.Path == "/rest/api/2/version" && r.Method == http.MethodPost {
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"id":          "20001",
						"name":        "v1.0.0",
						"description": "Release 1.0.0",
						"projectId":   10000,
						"released":    true,
					})
					return
				}

				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: false,
			wantRelease: &tracker.Release{
				ID:          "20001",
				Name:        "v1.0.0",
				Description: "Release 1.0.0",
				URL:         "https://tagoro9.atlassian.net/projects/TEST/versions/20001",
			},
		},
		{
			name: "error - no issue IDs provided",
			input: tracker.CreateReleaseInput{
				Name:        "v1.0.0",
				Description: "Release 1.0.0",
				IssueIDs:    []string{},
			},
			wantErr:     true,
			wantRelease: nil,
		},
		{
			name: "error - issue not found",
			input: tracker.CreateReleaseInput{
				Name:        "v1.0.0",
				Description: "Release 1.0.0",
				IssueIDs:    []string{"NOTFOUND-999"},
			},
			setupMock: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/2/issue/NOTFOUND-999" && r.Method == http.MethodGet {
					w.WriteHeader(http.StatusNotFound)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"errorMessages": []string{"Issue does not exist"},
					})
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr:     true,
			wantRelease: nil,
		},
		{
			name: "error - project not found",
			input: tracker.CreateReleaseInput{
				Name:        "v1.0.0",
				Description: "Release 1.0.0",
				IssueIDs:    []string{"TEST-123"},
			},
			setupMock: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/2/issue/TEST-123" && r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(mockJiraIssue("10001", "TEST-123", "Test Issue", "Done", "Story"))
					return
				}

				if r.URL.Path == "/rest/api/2/project/TEST" && r.Method == http.MethodGet {
					w.WriteHeader(http.StatusNotFound)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"errorMessages": []string{"Project not found"},
					})
					return
				}

				w.WriteHeader(http.StatusNotFound)
			},
			wantErr:     true,
			wantRelease: nil,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			if tt.setupMock != nil {
				suite.setupMockServer(tt.setupMock)
			}

			release, err := suite.client.CreateRelease(tt.input)

			if tt.wantErr {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), release)
			} else {
				assert.NoError(suite.T(), err)
				assert.NotNil(suite.T(), release)
				assert.Equal(suite.T(), tt.wantRelease.ID, release.ID)
				assert.Equal(suite.T(), tt.wantRelease.Name, release.Name)
				assert.Equal(suite.T(), tt.wantRelease.Description, release.Description)
				assert.Equal(suite.T(), tt.wantRelease.URL, release.URL)
			}
		})
	}
}

// TestSetFixVersion tests setting fix version on issues
func (suite *JiraTestSuite) TestSetFixVersion() {
	tests := []struct {
		name      string
		issueIDs  []string
		release   *tracker.Release
		setupMock func(w http.ResponseWriter, r *http.Request)
		wantErr   bool
	}{
		{
			name:     "success - set fix version on single issue",
			issueIDs: []string{"TEST-123"},
			release: &tracker.Release{
				ID:   "20001",
				Name: "v1.0.0",
			},
			setupMock: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/2/issue/TEST-123" && r.Method == http.MethodPut {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: false,
		},
		{
			name:     "success - set fix version on multiple issues",
			issueIDs: []string{"TEST-123", "TEST-124", "TEST-125"},
			release: &tracker.Release{
				ID:   "20001",
				Name: "v1.0.0",
			},
			setupMock: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPut && (r.URL.Path == "/rest/api/2/issue/TEST-123" ||
					r.URL.Path == "/rest/api/2/issue/TEST-124" ||
					r.URL.Path == "/rest/api/2/issue/TEST-125") {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: false,
		},
		{
			name:     "error - nil release",
			issueIDs: []string{"TEST-123"},
			release:  nil,
			wantErr:  true,
		},
		{
			name:     "error - issue not found",
			issueIDs: []string{"NOTFOUND-999"},
			release: &tracker.Release{
				ID:   "20001",
				Name: "v1.0.0",
			},
			setupMock: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/2/issue/NOTFOUND-999" && r.Method == http.MethodPut {
					w.WriteHeader(http.StatusNotFound)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"errorMessages": []string{"Issue does not exist"},
					})
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			if tt.setupMock != nil {
				suite.setupMockServer(tt.setupMock)
			}

			err := suite.client.SetFixVersion(tt.issueIDs, tt.release)

			if tt.wantErr {
				assert.Error(suite.T(), err)
			} else {
				assert.NoError(suite.T(), err)
			}
		})
	}
}

// TestSetJiraIssueStatusTransitionMatching tests different transition matching strategies
func (suite *JiraTestSuite) TestSetJiraIssueStatusTransitionMatching() {
	tests := []struct {
		name            string
		targetStatus    IssueStatus
		mockTransitions []map[string]interface{}
		wantErr         bool
		description     string
	}{
		{
			name:         "exact match - case insensitive",
			targetStatus: StatusInProgress,
			mockTransitions: []map[string]interface{}{
				mockJiraTransition("11", "To Do"),
				mockJiraTransition("21", "in progress"),
				mockJiraTransition("31", "Done"),
			},
			wantErr:     false,
			description: "Should match 'in progress' exactly (case insensitive)",
		},
		{
			name:         "pattern match - start progress",
			targetStatus: StatusInProgress,
			mockTransitions: []map[string]interface{}{
				mockJiraTransition("21", "Start Progress"),
				mockJiraTransition("31", "Done"),
			},
			wantErr:     false,
			description: "Should match 'Start Progress' using pattern matching",
		},
		{
			name:         "pattern match - done variants",
			targetStatus: StatusDone,
			mockTransitions: []map[string]interface{}{
				mockJiraTransition("21", "In Progress"),
				mockJiraTransition("31", "Complete"),
			},
			wantErr:     false,
			description: "Should match 'Complete' using pattern for Done",
		},
		{
			name:         "pattern match - review",
			targetStatus: StatusInReview,
			mockTransitions: []map[string]interface{}{
				mockJiraTransition("21", "In Progress"),
				mockJiraTransition("31", "Code Review"),
			},
			wantErr:     false,
			description: "Should match 'Code Review' using pattern for In Review",
		},
		{
			name:         "similarity match - close enough",
			targetStatus: StatusInProgress,
			mockTransitions: []map[string]interface{}{
				mockJiraTransition("21", "In Progres"),
			},
			wantErr:     false,
			description: "Should match 'In Progres' using similarity (typo)",
		},
		{
			name:         "no match - too different",
			targetStatus: StatusInProgress,
			mockTransitions: []map[string]interface{}{
				mockJiraTransition("21", "Open"),
				mockJiraTransition("31", "Closed"),
			},
			wantErr:     true,
			description: "Should fail to match - no similar transitions",
		},
		{
			name:            "no match - empty transitions",
			targetStatus:    StatusInProgress,
			mockTransitions: []map[string]interface{}{},
			wantErr:         true,
			description:     "Should fail with no available transitions",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			issueKey := "TEST-123"
			suite.setupMockServer(func(w http.ResponseWriter, r *http.Request) {
				transitionsPath := fmt.Sprintf("/rest/api/2/issue/%s/transitions", issueKey)
				issuePath := fmt.Sprintf("/rest/api/2/issue/%s", issueKey)

				if r.URL.Path == transitionsPath && r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"transitions": tt.mockTransitions,
					})
					return
				}

				if r.URL.Path == transitionsPath && r.Method == http.MethodPost {
					w.WriteHeader(http.StatusNoContent)
					return
				}

				if r.URL.Path == issuePath && r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(mockJiraIssue("10001", issueKey, "Test Issue", string(tt.targetStatus), "Story"))
					return
				}

				w.WriteHeader(http.StatusNotFound)
			})

			_, err := suite.client.SetJiraIssueStatus(issueKey, tt.targetStatus)

			if tt.wantErr {
				assert.Error(suite.T(), err, tt.description)
			} else {
				assert.NoError(suite.T(), err, tt.description)
			}
		})
	}
}

func TestJiraSuite(t *testing.T) {
	suite.Run(t, new(JiraTestSuite))
}

func TestResolveJiraRootURL_PromptAndPersist(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := viper.New()
	cfg.SetConfigFile(configPath)
	cfg.SetConfigType("yaml")
	require.NoError(t, cfg.WriteConfigAs(configPath))

	j := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "jira"},
		allowPrompt:              true,
		promptRoot: func() (string, error) {
			return "https://acme.atlassian.net/", nil
		},
	}

	root, err := j.resolveJiraRootURL()
	require.NoError(t, err)
	assert.Equal(t, "https://acme.atlassian.net", root)
	assert.Equal(t, "https://acme.atlassian.net", cfg.GetString("jira.root"))

	contents, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(contents), "root: https://acme.atlassian.net")
}

func TestResolveJiraRootURL_NonInteractiveMissingConfig(t *testing.T) {
	j := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: viper.New(), Prefix: "jira"},
		allowPrompt:              false,
	}

	_, err := j.resolveJiraRootURL()
	require.Error(t, err)
	assert.ErrorIs(t, err, errMissingJiraRoot)
}

func TestNormalizeJiraRootURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		allowHTTP bool
		want      string
		wantErr   bool
	}{
		{name: "https with trailing slash", input: "https://acme.atlassian.net/", want: "https://acme.atlassian.net"},
		{name: "missing scheme defaults to https", input: "acme.atlassian.net", want: "https://acme.atlassian.net"},
		{name: "path not allowed", input: "https://acme.atlassian.net/foo", wantErr: true},
		{name: "http rejected by default", input: "http://acme.atlassian.net", wantErr: true},
		{name: "http allowed for tests", input: "http://localhost:9999", allowHTTP: true, want: "http://localhost:9999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeJiraRootURL(tt.input, tt.allowHTTP)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
