package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockJiraServer_NewAndClose(t *testing.T) {
	server := NewMockJiraServer()
	require.NotNil(t, server)
	require.NotEmpty(t, server.URL())

	server.Close()
}

func TestMockJiraServer_GetCurrentUser(t *testing.T) {
	server := NewMockJiraServer()
	defer server.Close()

	t.Run("returns unauthorized when no user configured", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/rest/api/2/myself")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns user when configured", func(t *testing.T) {
		user := DefaultUser()
		server.SetCurrentUser(user)

		resp, err := http.Get(server.URL() + "/rest/api/2/myself")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, user.AccountID, result["accountId"])
		assert.Equal(t, user.DisplayName, result["displayName"])
		assert.Equal(t, user.EmailAddress, result["emailAddress"])
	})
}

func TestMockJiraServer_GetIssue(t *testing.T) {
	server := NewMockJiraServer()
	defer server.Close()

	t.Run("returns 404 for non-existent issue", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/rest/api/2/issue/NOTFOUND-123")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("returns issue when configured", func(t *testing.T) {
		issue := NewStoryIssue("TEST-123", "Test Story", "In Progress")
		server.AddIssue(issue)

		resp, err := http.Get(server.URL() + "/rest/api/2/issue/TEST-123")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "TEST-123", result["key"])
		fields := result["fields"].(map[string]interface{})
		assert.Equal(t, "Test Story", fields["summary"])
	})
}

func TestMockJiraServer_Search(t *testing.T) {
	server := NewMockJiraServer()
	defer server.Close()

	user := DefaultUser()
	server.SetCurrentUser(user)

	// Add issues with different assignees and resolutions
	issue1 := NewIssueWithAssignee("TEST-1", "My Issue", "In Progress", "Story", user)
	issue2 := NewStoryIssue("TEST-2", "Unassigned Issue", "Backlog")
	issue3 := NewIssueWithAssignee("TEST-3", "Another My Issue", "To Do", "Bug", user)
	issue4 := NewIssueWithAssignee("TEST-4", "Resolved Issue", "Done", "Story", user)
	issue4.Resolution = "Done" // This issue is resolved

	server.AddIssues(issue1, issue2, issue3, issue4)

	t.Run("returns all issues without filter", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/rest/api/2/search")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		issues := result["issues"].([]interface{})
		assert.Len(t, issues, 4) // All issues
	})

	t.Run("filters by assignee and resolution", func(t *testing.T) {
		// Use URL encoding for the JQL query
		resp, err := http.Get(server.URL() + "/rest/api/2/search?jql=assignee%20%3D%20currentUser%28%29%20AND%20resolution%20%3D%20Unresolved")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		issues := result["issues"].([]interface{})
		// Should return only unresolved issues assigned to current user (TEST-1 and TEST-3)
		assert.Len(t, issues, 2)
	})
}

func TestMockJiraServer_Transitions(t *testing.T) {
	server := NewMockJiraServer()
	defer server.Close()

	issue := NewStoryIssue("TEST-123", "Test Story", "Backlog")
	server.AddIssue(issue)
	server.SetTransitions("TEST-123", InProgressTransitions())

	t.Run("get transitions returns configured transitions", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/rest/api/2/issue/TEST-123/transitions")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		transitions := result["transitions"].([]interface{})
		assert.Len(t, transitions, 2)
	})

	t.Run("do transition updates issue status", func(t *testing.T) {
		client := &http.Client{}
		req, _ := http.NewRequest(http.MethodPost,
			server.URL()+"/rest/api/2/issue/TEST-123/transitions",
			jsonBody(map[string]interface{}{
				"transition": map[string]interface{}{
					"id": "21", // Start Progress
				},
			}))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		// Verify the status changed
		updatedIssue := server.GetIssue("TEST-123")
		assert.Equal(t, "In Progress", updatedIssue.Status.Name)
	})
}

func TestMockJiraServer_CreateIssue(t *testing.T) {
	server := NewMockJiraServer()
	defer server.Close()

	project := DefaultProject()
	server.AddProject(project)

	t.Run("creates issue successfully", func(t *testing.T) {
		client := &http.Client{}
		req, _ := http.NewRequest(http.MethodPost,
			server.URL()+"/rest/api/2/issue",
			jsonBody(map[string]interface{}{
				"fields": map[string]interface{}{
					"summary":     "New Issue",
					"description": "Issue description",
					"project": map[string]interface{}{
						"key": "TEST",
					},
					"issuetype": map[string]interface{}{
						"name": "Story",
					},
					"labels": []string{"feature", "priority"},
				},
			}))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result["key"], "TEST-")
		fields := result["fields"].(map[string]interface{})
		assert.Equal(t, "New Issue", fields["summary"])
	})

	t.Run("returns error for invalid project", func(t *testing.T) {
		client := &http.Client{}
		req, _ := http.NewRequest(http.MethodPost,
			server.URL()+"/rest/api/2/issue",
			jsonBody(map[string]interface{}{
				"fields": map[string]interface{}{
					"summary": "New Issue",
					"project": map[string]interface{}{
						"key": "INVALID",
					},
					"issuetype": map[string]interface{}{
						"name": "Story",
					},
				},
			}))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestMockJiraServer_AddComment(t *testing.T) {
	server := NewMockJiraServer()
	defer server.Close()

	issue := NewStoryIssue("TEST-123", "Test Story", "In Progress")
	server.AddIssue(issue)

	t.Run("adds comment successfully", func(t *testing.T) {
		client := &http.Client{}
		req, _ := http.NewRequest(http.MethodPost,
			server.URL()+"/rest/api/2/issue/TEST-123/comment",
			jsonBody(map[string]interface{}{
				"body": "This is a test comment",
			}))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.NotEmpty(t, result["id"])
		assert.Equal(t, "This is a test comment", result["body"])
	})

	t.Run("returns 404 for non-existent issue", func(t *testing.T) {
		client := &http.Client{}
		req, _ := http.NewRequest(http.MethodPost,
			server.URL()+"/rest/api/2/issue/NOTFOUND-999/comment",
			jsonBody(map[string]interface{}{
				"body": "Comment on missing issue",
			}))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestMockJiraServer_CreateVersion(t *testing.T) {
	server := NewMockJiraServer()
	defer server.Close()

	t.Run("creates version successfully", func(t *testing.T) {
		client := &http.Client{}
		req, _ := http.NewRequest(http.MethodPost,
			server.URL()+"/rest/api/2/version",
			jsonBody(map[string]interface{}{
				"name":        "v1.0.0",
				"description": "First release",
				"projectId":   10000,
				"released":    true,
			}))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.NotEmpty(t, result["id"])
		assert.Equal(t, "v1.0.0", result["name"])
		assert.Equal(t, "First release", result["description"])
		assert.Equal(t, true, result["released"])
	})
}

func TestMockJiraServer_UpdateIssue(t *testing.T) {
	server := NewMockJiraServer()
	defer server.Close()

	issue := NewStoryIssue("TEST-123", "Test Story", "In Progress")
	server.AddIssue(issue)

	t.Run("updates fix version successfully", func(t *testing.T) {
		client := &http.Client{}
		req, _ := http.NewRequest(http.MethodPut,
			server.URL()+"/rest/api/2/issue/TEST-123",
			jsonBody(map[string]interface{}{
				"update": map[string]interface{}{
					"fixVersions": []map[string]interface{}{
						{
							"add": map[string]interface{}{
								"name": "v1.0.0",
							},
						},
					},
				},
			}))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		// Verify the fix version was added
		updatedIssue := server.GetIssue("TEST-123")
		require.Len(t, updatedIssue.FixVersions, 1)
		assert.Equal(t, "v1.0.0", updatedIssue.FixVersions[0].Name)
	})
}

func TestMockJiraServer_GetProject(t *testing.T) {
	server := NewMockJiraServer()
	defer server.Close()

	project := DefaultProject()
	server.AddProject(project)

	t.Run("returns project when configured", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/rest/api/2/project/TEST")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "TEST", result["key"])
		assert.Equal(t, "Test Project", result["name"])
	})

	t.Run("returns 404 for non-existent project", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/rest/api/2/project/INVALID")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestMockJiraServer_ErrorResponses(t *testing.T) {
	server := NewMockJiraServer()
	defer server.Close()

	issue := NewStoryIssue("TEST-123", "Test Story", "In Progress")
	server.AddIssue(issue)

	t.Run("returns configured error response", func(t *testing.T) {
		server.SetErrorResponse("/rest/api/2/issue/TEST-123", &ErrorResponse{
			StatusCode:    http.StatusForbidden,
			ErrorMessages: []string{"You do not have permission to view this issue"},
		})

		resp, err := http.Get(server.URL() + "/rest/api/2/issue/TEST-123")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		messages := result["errorMessages"].([]interface{})
		assert.Contains(t, messages[0], "permission")
	})

	t.Run("returns normal response after clearing error", func(t *testing.T) {
		server.ClearErrorResponse("/rest/api/2/issue/TEST-123")

		resp, err := http.Get(server.URL() + "/rest/api/2/issue/TEST-123")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestMockJiraServer_RequestLog(t *testing.T) {
	server := NewMockJiraServer()
	defer server.Close()

	server.SetCurrentUser(DefaultUser())

	// Make some requests
	_, _ = http.Get(server.URL() + "/rest/api/2/myself")
	_, _ = http.Get(server.URL() + "/rest/api/2/issue/TEST-123")

	log := server.GetRequestLog()
	assert.Len(t, log, 2)
	assert.Equal(t, http.MethodGet, log[0].Method)
	assert.Equal(t, "/rest/api/2/myself", log[0].Path)
	assert.Equal(t, "/rest/api/2/issue/TEST-123", log[1].Path)
}

func TestMockJiraServer_Reset(t *testing.T) {
	server := NewMockJiraServer()
	defer server.Close()

	// Configure some data
	server.SetCurrentUser(DefaultUser())
	server.AddIssue(NewStoryIssue("TEST-123", "Test", "In Progress"))
	server.AddProject(DefaultProject())

	// Make a request
	_, _ = http.Get(server.URL() + "/rest/api/2/myself")

	// Reset
	server.Reset()

	// Verify everything is cleared
	assert.Nil(t, server.CurrentUser)
	assert.Empty(t, server.GetRequestLog())

	// User endpoint should return unauthorized
	resp, _ := http.Get(server.URL() + "/rest/api/2/myself")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestFixtures(t *testing.T) {
	t.Run("DefaultUser", func(t *testing.T) {
		user := DefaultUser()
		assert.NotEmpty(t, user.AccountID)
		assert.NotEmpty(t, user.DisplayName)
		assert.NotEmpty(t, user.EmailAddress)
		assert.True(t, user.Active)
	})

	t.Run("DefaultProject", func(t *testing.T) {
		project := DefaultProject()
		assert.NotEmpty(t, project.ID)
		assert.NotEmpty(t, project.Key)
		assert.NotEmpty(t, project.Name)
	})

	t.Run("NewIssue", func(t *testing.T) {
		issue := NewIssue("TEST-123", "Test Summary", "In Progress", "Story")
		assert.Equal(t, "TEST-123", issue.Key)
		assert.Equal(t, "Test Summary", issue.Summary)
		assert.Equal(t, "In Progress", issue.Status.Name)
		assert.Equal(t, "Story", issue.IssueType.Name)
		assert.Equal(t, "TEST", issue.Project.Key)
	})

	t.Run("NewSubTaskIssue", func(t *testing.T) {
		issue := NewSubTaskIssue("TEST-124", "Sub Task", "To Do", "TEST-100")
		assert.Equal(t, "Sub-task", issue.IssueType.Name)
		assert.True(t, issue.IssueType.Subtask)
		assert.NotNil(t, issue.Parent)
		assert.Equal(t, "TEST-100", issue.Parent.Key)
	})

	t.Run("DefaultTransitions", func(t *testing.T) {
		transitions := DefaultTransitions()
		assert.Len(t, transitions, 4)
	})

	t.Run("SampleUsers", func(t *testing.T) {
		users := SampleUsers()
		assert.Len(t, users, 3)
		for _, u := range users {
			assert.NotEmpty(t, u.AccountID)
			assert.NotEmpty(t, u.DisplayName)
		}
	})

	t.Run("SampleProjects", func(t *testing.T) {
		projects := SampleProjects()
		assert.Len(t, projects, 3)
	})

	t.Run("SampleIssues", func(t *testing.T) {
		issues := SampleIssues()
		assert.Len(t, issues, 5)
	})
}

// jsonBody is a helper to create a JSON request body
func jsonBody(data interface{}) *jsonReader {
	return &jsonReader{data: data}
}

type jsonReader struct {
	data    interface{}
	buf     []byte
	offset  int
	encoded bool
}

func (r *jsonReader) Read(p []byte) (n int, err error) {
	if !r.encoded {
		r.buf, _ = json.Marshal(r.data)
		r.encoded = true
	}
	if r.offset >= len(r.buf) {
		return 0, io.EOF
	}
	n = copy(p, r.buf[r.offset:])
	r.offset += n
	return n, nil
}
