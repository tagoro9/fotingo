package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMockGitHubServer(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	assert.NotNil(t, server)
	assert.NotEmpty(t, server.URL())
}

func TestMockGitHubServer_GetCurrentUser(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	t.Run("returns error when no user configured", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/user")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns user when configured", func(t *testing.T) {
		user := DefaultUser()
		server.SetCurrentUser(user)

		resp, err := http.Get(server.URL() + "/user")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, user.Login, result["login"])
		assert.Equal(t, user.Name, result["name"])
	})
}

func TestMockGitHubServer_GetRepository(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	t.Run("returns 404 for non-existent repo", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/repos/owner/nonexistent")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("returns repository when configured", func(t *testing.T) {
		repo := NewRepository(1, "testowner", "testrepo")
		server.AddRepository(repo)

		resp, err := http.Get(server.URL() + "/repos/testowner/testrepo")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, repo.Name, result["name"])
		assert.Equal(t, repo.FullName, result["full_name"])
	})
}

func TestMockGitHubServer_PullRequests(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	owner := "testowner"
	repo := "testrepo"

	t.Run("list returns empty array when no PRs", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/repos/" + owner + "/" + repo + "/pulls")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Empty(t, result)
	})

	t.Run("list returns configured PRs", func(t *testing.T) {
		pr1 := NewPullRequest(1, "First PR", "feature-1", "main", "open")
		pr2 := NewPullRequest(2, "Second PR", "feature-2", "main", "open")
		server.AddPullRequests(owner, repo, pr1, pr2)

		resp, err := http.Get(server.URL() + "/repos/" + owner + "/" + repo + "/pulls")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Len(t, result, 2)
	})

	t.Run("list filters by head", func(t *testing.T) {
		server.Reset()
		pr1 := NewPullRequest(1, "First PR", "feature-1", "main", "open")
		pr2 := NewPullRequest(2, "Second PR", "feature-2", "main", "open")
		server.AddPullRequests(owner, repo, pr1, pr2)

		resp, err := http.Get(server.URL() + "/repos/" + owner + "/" + repo + "/pulls?head=" + owner + ":feature-1")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Len(t, result, 1)
		assert.Equal(t, "First PR", result[0]["title"])
	})

	t.Run("list filters by state", func(t *testing.T) {
		server.Reset()
		prOpen := NewPullRequest(1, "Open PR", "feature-1", "main", "open")
		prClosed := NewPullRequest(2, "Closed PR", "feature-2", "main", "closed")
		server.AddPullRequests(owner, repo, prOpen, prClosed)

		resp, err := http.Get(server.URL() + "/repos/" + owner + "/" + repo + "/pulls?state=open")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		var result []map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Len(t, result, 1)
		assert.Equal(t, "Open PR", result[0]["title"])
	})

	t.Run("create PR succeeds", func(t *testing.T) {
		server.Reset()
		server.SetCurrentUser(DefaultUser())

		body := map[string]interface{}{
			"title": "New PR",
			"body":  "PR description",
			"head":  "new-feature",
			"base":  "main",
			"draft": false,
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(server.URL()+"/repos/"+owner+"/"+repo+"/pulls", "application/json", bytes.NewReader(jsonBody))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "New PR", result["title"])
		assert.Equal(t, "open", result["state"])
	})

	t.Run("create PR fails when PR already exists for head", func(t *testing.T) {
		server.Reset()
		server.SetCurrentUser(DefaultUser())
		existingPR := NewPullRequest(1, "Existing PR", "feature-branch", "main", "open")
		server.AddPullRequest(owner, repo, existingPR)

		body := map[string]interface{}{
			"title": "Duplicate PR",
			"body":  "PR description",
			"head":  "feature-branch",
			"base":  "main",
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(server.URL()+"/repos/"+owner+"/"+repo+"/pulls", "application/json", bytes.NewReader(jsonBody))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("get PR by number", func(t *testing.T) {
		server.Reset()
		pr := NewPullRequest(42, "Test PR", "feature", "main", "open")
		server.AddPullRequest(owner, repo, pr)

		resp, err := http.Get(server.URL() + "/repos/" + owner + "/" + repo + "/pulls/42")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, float64(42), result["number"])
		assert.Equal(t, "Test PR", result["title"])
	})
}

func TestMockGitHubServer_PullRequestDiscussion(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	owner := "testowner"
	repo := "testrepo"
	pr := NewPullRequest(1, "Discussion PR", "feature", "main", "open")
	pr.IssueComments = []*MockIssueComment{
		NewIssueComment(101, "Top-level comment", "alice"),
	}
	pr.Reviews = []*MockPullRequestReview{
		NewPullRequestReview(201, "COMMENTED", "Review body", "bob"),
	}
	pr.ReviewComments = []*MockPullRequestReviewComment{
		NewPullRequestReviewComment(301, 201, 0, "Inline comment", "bob"),
	}
	server.AddPullRequest(owner, repo, pr)

	t.Run("list issue comments", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/repos/" + owner + "/" + repo + "/issues/1/comments")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.Len(t, result, 1)
		assert.Equal(t, float64(101), result[0]["id"])
		assert.Equal(t, "Top-level comment", result[0]["body"])
	})

	t.Run("list reviews", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/repos/" + owner + "/" + repo + "/pulls/1/reviews")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.Len(t, result, 1)
		assert.Equal(t, float64(201), result[0]["id"])
		assert.Equal(t, "COMMENTED", result[0]["state"])
	})

	t.Run("list review comments", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/repos/" + owner + "/" + repo + "/pulls/1/comments")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.Len(t, result, 1)
		assert.Equal(t, float64(301), result[0]["id"])
		assert.Equal(t, "Inline comment", result[0]["body"])
	})
}

func TestMockGitHubServer_Collaborators(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	owner := "testowner"
	repo := "testrepo"

	t.Run("list returns empty array when no collaborators", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/repos/" + owner + "/" + repo + "/collaborators")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Empty(t, result)
	})

	t.Run("list returns configured collaborators", func(t *testing.T) {
		collaborators := SampleCollaborators()
		server.SetCollaborators(owner, repo, collaborators)

		resp, err := http.Get(server.URL() + "/repos/" + owner + "/" + repo + "/collaborators")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Len(t, result, 3)
	})
}

func TestMockGitHubServer_Labels(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	owner := "testowner"
	repo := "testrepo"

	t.Run("list returns empty array when no labels", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/repos/" + owner + "/" + repo + "/labels")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Empty(t, result)
	})

	t.Run("list returns configured labels", func(t *testing.T) {
		labels := SampleLabels()
		server.SetLabels(owner, repo, labels)

		resp, err := http.Get(server.URL() + "/repos/" + owner + "/" + repo + "/labels")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Len(t, result, 7)
	})

	t.Run("add labels to issue", func(t *testing.T) {
		server.Reset()
		pr := NewPullRequest(1, "Test PR", "feature", "main", "open")
		server.AddPullRequest(owner, repo, pr)

		labels := SampleLabels()
		server.SetLabels(owner, repo, labels)

		body := map[string]interface{}{
			"labels": []string{"bug", "enhancement"},
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(server.URL()+"/repos/"+owner+"/"+repo+"/issues/1/labels", "application/json", bytes.NewReader(jsonBody))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Len(t, result, 2)
	})
}

func TestMockGitHubServer_RequestReviewers(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	owner := "testowner"
	repo := "testrepo"

	t.Run("request reviewers succeeds with valid collaborators", func(t *testing.T) {
		server.Reset()
		pr := NewPullRequest(1, "Test PR", "feature", "main", "open")
		server.AddPullRequest(owner, repo, pr)

		collab1 := NewUser(1, "reviewer1", "Reviewer One")
		collab2 := NewUser(2, "reviewer2", "Reviewer Two")
		server.AddCollaborators(owner, repo, collab1, collab2)

		body := map[string]interface{}{
			"reviewers": []string{"reviewer1"},
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(server.URL()+"/repos/"+owner+"/"+repo+"/pulls/1/requested_reviewers", "application/json", bytes.NewReader(jsonBody))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		reviewers := result["requested_reviewers"].([]interface{})
		assert.Len(t, reviewers, 1)
	})

	t.Run("request reviewers fails with non-collaborator", func(t *testing.T) {
		server.Reset()
		pr := NewPullRequest(1, "Test PR", "feature", "main", "open")
		server.AddPullRequest(owner, repo, pr)

		collab := NewUser(1, "collaborator", "Collaborator")
		server.AddCollaborator(owner, repo, collab)

		body := map[string]interface{}{
			"reviewers": []string{"non-collaborator"},
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(server.URL()+"/repos/"+owner+"/"+repo+"/pulls/1/requested_reviewers", "application/json", bytes.NewReader(jsonBody))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestMockGitHubServer_OrgMetadataEndpoints(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	org := "testowner"
	server.AddOrgMembers(org, NewUser(1, "alice", "Alice"), NewUser(2, "bob", "Bob"))
	server.AddTeams(org, NewTeam(1, org, "platform", "Platform", "Platform team"))

	t.Run("list org members", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/orgs/" + org + "/members")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var payload []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
		assert.Len(t, payload, 2)
	})

	t.Run("list org teams", func(t *testing.T) {
		resp, err := http.Get(server.URL() + "/orgs/" + org + "/teams")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var payload []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
		assert.Len(t, payload, 1)
		assert.Equal(t, "platform", payload[0]["slug"])
	})
}

func TestMockGitHubServer_Assignees(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	owner := "testowner"
	repo := "testrepo"
	server.AddPullRequest(owner, repo, NewPullRequest(1, "Test PR", "feature", "main", "open"))
	server.AddCollaborator(owner, repo, NewUser(1, "alice", "Alice"))

	body := map[string]interface{}{
		"assignees": []string{"alice"},
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post(
		server.URL()+"/repos/"+owner+"/"+repo+"/issues/1/assignees",
		"application/json",
		bytes.NewReader(jsonBody),
	)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var payload map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	assignees, ok := payload["assignees"].([]interface{})
	require.True(t, ok)
	assert.Len(t, assignees, 1)
}

func TestMockGitHubServer_Releases(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	owner := "testowner"
	repo := "testrepo"

	t.Run("create release succeeds", func(t *testing.T) {
		body := map[string]interface{}{
			"tag_name":         "v1.0.0",
			"target_commitish": "main",
			"name":             "Version 1.0.0",
			"body":             "Release notes",
			"draft":            false,
			"prerelease":       false,
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(server.URL()+"/repos/"+owner+"/"+repo+"/releases", "application/json", bytes.NewReader(jsonBody))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "v1.0.0", result["tag_name"])
		assert.Equal(t, "Version 1.0.0", result["name"])
	})

	t.Run("create release fails when tag exists", func(t *testing.T) {
		server.Reset()
		existing := NewRelease(1, "v1.0.0", "Version 1.0.0", "main")
		server.AddRelease(owner, repo, existing)

		body := map[string]interface{}{
			"tag_name":         "v1.0.0",
			"target_commitish": "main",
			"name":             "Duplicate Release",
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(server.URL()+"/repos/"+owner+"/"+repo+"/releases", "application/json", bytes.NewReader(jsonBody))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestMockGitHubServer_ErrorResponses(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	t.Run("configured error response is returned", func(t *testing.T) {
		server.SetErrorResponse("GET /user", &ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    "Internal Server Error",
		})

		resp, err := http.Get(server.URL() + "/user")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "Internal Server Error", result["message"])
	})

	t.Run("error response can be cleared", func(t *testing.T) {
		server.SetCurrentUser(DefaultUser())
		server.SetErrorResponse("GET /user", &ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    "Internal Server Error",
		})
		server.ClearErrorResponse("GET /user")

		resp, err := http.Get(server.URL() + "/user")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestMockGitHubServer_RequestLog(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	server.SetCurrentUser(DefaultUser())

	// Make some requests
	_, _ = http.Get(server.URL() + "/user")
	_, _ = http.Get(server.URL() + "/repos/owner/repo/pulls?state=open")

	log := server.GetRequestLog()

	assert.Len(t, log, 2)
	assert.Equal(t, "GET", log[0].Method)
	assert.Equal(t, "/user", log[0].Path)
	assert.Equal(t, "GET", log[1].Method)
	assert.Equal(t, "/repos/owner/repo/pulls", log[1].Path)
	assert.Equal(t, "open", log[1].QueryParams["state"])
}

func TestMockGitHubServer_Reset(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	// Configure some data
	server.SetCurrentUser(DefaultUser())
	server.AddRepository(DefaultRepository())
	server.AddPullRequest("owner", "repo", NewPullRequest(1, "PR", "feature", "main", "open"))

	// Make a request to populate the log
	_, _ = http.Get(server.URL() + "/user")

	// Reset
	server.Reset()

	// Verify request log is cleared
	assert.Empty(t, server.GetRequestLog())

	// Verify user is cleared (this will add to the log, but that's expected)
	resp, _ := http.Get(server.URL() + "/user")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestMockGitHubServer_EnterpriseURL(t *testing.T) {
	server := NewMockGitHubServer()
	defer server.Close()

	server.SetCurrentUser(DefaultUser())

	// Test with /api/v3 prefix (enterprise URL format)
	resp, err := http.Get(server.URL() + "/api/v3/user")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "testuser", result["login"])
}
