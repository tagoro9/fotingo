package jira

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/jira/testutil"
	"github.com/tagoro9/fotingo/internal/tracker"
)

// TestClientWithMockServer tests the Jira client using the testutil mock server.
// These tests verify integration with a more realistic mock server setup.
func TestClientWithMockServer(t *testing.T) {
	t.Run("GetCurrentUser with mock server", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		user := testutil.DefaultUser()
		server.SetCurrentUser(user)

		cfg := viper.New()
		cfg.Set("jira.siteId", "test-site")

		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		result, err := client.GetCurrentUser()
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, user.AccountID, result.ID)
		assert.Equal(t, user.DisplayName, result.Name)
		assert.Equal(t, user.EmailAddress, result.Email)
	})

	t.Run("GetCurrentUser - unauthorized", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		// Don't set a current user - should return unauthorized

		cfg := viper.New()
		cfg.Set("jira.siteId", "test-site")

		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		result, err := client.GetCurrentUser()
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("GetJiraIssue with mock server", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		issue := testutil.NewStoryIssue("TEST-123", "Test Story", "In Progress")
		server.AddIssue(issue)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		result, err := client.GetJiraIssue("TEST-123")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, issue.Key, result.Key)
		assert.Equal(t, issue.Summary, result.Summary)
		assert.Equal(t, issue.Status.Name, result.Status)
		assert.Equal(t, issue.IssueType.Name, result.Type)
	})

	t.Run("GetJiraIssue - not found", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		result, err := client.GetJiraIssue("NOTFOUND-999")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get issue")
	})

	t.Run("GetIssue with mock server", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		issue := testutil.NewBugIssue("BUG-456", "Fix login bug", "Backlog")
		server.AddIssue(issue)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		result, err := client.GetIssue("BUG-456")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, issue.Key, result.Key)
		assert.Equal(t, issue.Summary, result.Summary)
		assert.Equal(t, tracker.IssueTypeBug, result.Type)
		assert.Equal(t, tracker.IssueStatusBacklog, result.Status)
	})

	t.Run("AssignIssue assigns assignee", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		user := testutil.DefaultUser()
		server.AddUser(user)
		server.SetCurrentUser(user)

		issue := testutil.NewBugIssue("BUG-789", "Assign me", "To Do")
		issue.Assignee = nil
		server.AddIssue(issue)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		result, err := client.AssignIssue("BUG-789", user.AccountID)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Assignee)
		assert.Equal(t, user.AccountID, result.Assignee.ID)

		updatedIssue := server.GetIssue("BUG-789")
		require.NotNil(t, updatedIssue)
		require.NotNil(t, updatedIssue.Assignee)
		assert.Equal(t, user.AccountID, updatedIssue.Assignee.AccountID)
	})

	t.Run("GetProjectIssueTypes returns project issue types", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		project := testutil.DefaultProject()
		server.AddProject(project)
		server.SetProjectIssueTypes(project.Key, []testutil.MockIssueType{
			{ID: "1", Name: "Task", Description: "Task", Subtask: false},
			{ID: "2", Name: "Story", Description: "Story", Subtask: false},
			{ID: "3", Name: "Sub-task", Description: "Sub task", Subtask: true},
		})

		cfg := viper.New()
		cfg.Set("cache.path", filepath.Join(t.TempDir(), "jira-cache.db"))

		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		issueTypes, err := client.GetProjectIssueTypes(project.Key)
		require.NoError(t, err)
		require.Len(t, issueTypes, 3)
		assert.Equal(t, "Task", issueTypes[0].Name)
		assert.Equal(t, "Sub-task", issueTypes[2].Name)
		assert.True(t, issueTypes[2].Subtask)
	})

	t.Run("GetProjectIssueTypes caches by project and site", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		project := testutil.DefaultProject()
		projectTwo := testutil.NewProject("10001", "TEST2", "Test Project 2")
		server.AddProject(project)
		server.AddProject(projectTwo)
		server.SetProjectIssueTypes(project.Key, []testutil.MockIssueType{
			{ID: "1", Name: "Task", Description: "Task", Subtask: false},
		})
		server.SetProjectIssueTypes(projectTwo.Key, []testutil.MockIssueType{
			{ID: "2", Name: "Bug", Description: "Bug", Subtask: false},
		})

		cfg := viper.New()
		cfg.Set("cache.path", filepath.Join(t.TempDir(), "jira-cache.db"))
		cfg.Set("jira.cache.issueTypesTTL", "1h")

		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		_, err = client.GetProjectIssueTypes(project.Key)
		require.NoError(t, err)
		_, err = client.GetProjectIssueTypes(project.Key)
		require.NoError(t, err)
		_, err = client.GetProjectIssueTypes(projectTwo.Key)
		require.NoError(t, err)

		requests := server.GetRequestLog()
		createmetaCalls := 0
		for _, request := range requests {
			if request.Method == "GET" && request.Path == "/rest/api/2/issue/createmeta" {
				createmetaCalls++
			}
		}

		assert.Equal(t, 2, createmetaCalls, "expected cache hit for repeated project key and miss for distinct project key")
	})

	t.Run("GetProjectIssueTypes cache key is partitioned by Jira site", func(t *testing.T) {
		serverA := testutil.NewMockJiraServer()
		defer serverA.Close()
		serverB := testutil.NewMockJiraServer()
		defer serverB.Close()

		projectA := testutil.DefaultProject()
		projectB := testutil.DefaultProject()
		serverA.AddProject(projectA)
		serverB.AddProject(projectB)
		serverA.SetProjectIssueTypes(projectA.Key, []testutil.MockIssueType{
			{ID: "1", Name: "Task", Description: "Task", Subtask: false},
		})
		serverB.SetProjectIssueTypes(projectB.Key, []testutil.MockIssueType{
			{ID: "2", Name: "Bug", Description: "Bug", Subtask: false},
		})

		cachePath := filepath.Join(t.TempDir(), "jira-cache.db")

		cfgA := viper.New()
		cfgA.Set("cache.path", cachePath)
		cfgA.Set("jira.cache.issueTypesTTL", "1h")
		clientA, err := NewWithHTTPClient(cfgA, serverA.Client(), serverA.URL())
		require.NoError(t, err)
		_, err = clientA.GetProjectIssueTypes("TEST")
		require.NoError(t, err)
		jiraA, ok := clientA.(*jira)
		require.True(t, ok)
		require.NoError(t, jiraA.metadataCache.Close())

		cfgB := viper.New()
		cfgB.Set("cache.path", cachePath)
		cfgB.Set("jira.cache.issueTypesTTL", "1h")
		clientB, err := NewWithHTTPClient(cfgB, serverB.Client(), serverB.URL())
		require.NoError(t, err)
		_, err = clientB.GetProjectIssueTypes("TEST")
		require.NoError(t, err)
		jiraB, ok := clientB.(*jira)
		require.True(t, ok)
		require.NoError(t, jiraB.metadataCache.Close())

		cfgARepeat := viper.New()
		cfgARepeat.Set("cache.path", cachePath)
		cfgARepeat.Set("jira.cache.issueTypesTTL", "1h")
		clientARepeat, err := NewWithHTTPClient(cfgARepeat, serverA.Client(), serverA.URL())
		require.NoError(t, err)
		_, err = clientARepeat.GetProjectIssueTypes("TEST")
		require.NoError(t, err)
		jiraARepeat, ok := clientARepeat.(*jira)
		require.True(t, ok)
		require.NoError(t, jiraARepeat.metadataCache.Close())

		serverACalls := 0
		for _, request := range serverA.GetRequestLog() {
			if request.Method == "GET" && request.Path == "/rest/api/2/issue/createmeta" {
				serverACalls++
			}
		}

		serverBCalls := 0
		for _, request := range serverB.GetRequestLog() {
			if request.Method == "GET" && request.Path == "/rest/api/2/issue/createmeta" {
				serverBCalls++
			}
		}

		assert.Equal(t, 1, serverACalls)
		assert.Equal(t, 1, serverBCalls)
	})

	t.Run("GetUserOpenIssues with mock server", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		user := testutil.DefaultUser()
		server.SetCurrentUser(user)

		// Add issues assigned to the user
		issue1 := testutil.NewIssueWithAssignee("TEST-1", "First Issue", "In Progress", "Story", user)
		issue2 := testutil.NewIssueWithAssignee("TEST-2", "Second Issue", "To Do", "Bug", user)
		issue3 := testutil.NewIssueWithAssignee("TEST-3", "Third Issue", "In Review", "Task", user)

		// Add a resolved issue - should not appear in results
		issue4 := testutil.NewIssueWithAssignee("TEST-4", "Done Issue", "Done", "Story", user)
		issue4.Resolution = "Done"

		server.AddIssues(issue1, issue2, issue3, issue4)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		results, err := client.GetUserOpenIssues()
		assert.NoError(t, err)
		assert.Len(t, results, 3) // Only unresolved issues
	})

	t.Run("CreateIssue with labels", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		project := testutil.DefaultProject()
		server.AddProject(project)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		input := tracker.CreateIssueInput{
			Title:       "New Feature",
			Description: "Feature description",
			Project:     project.Key,
			Type:        tracker.IssueTypeStory,
			Labels:      []string{"feature", "priority"},
		}

		result, err := client.CreateIssue(input)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "New Feature", result.Summary)
		assert.Equal(t, tracker.IssueTypeStory, result.Type)

		// Verify the issue was created in the mock server
		createdIssue := server.GetIssue(result.Key)
		assert.NotNil(t, createdIssue)
		assert.Equal(t, []string{"feature", "priority"}, createdIssue.Labels)
	})

	t.Run("CreateIssue - sub-task with parent", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		project := testutil.DefaultProject()
		server.AddProject(project)

		// Add parent issue
		parentIssue := testutil.NewStoryIssue("TEST-100", "Parent Story", "In Progress")
		server.AddIssue(parentIssue)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		input := tracker.CreateIssueInput{
			Title:    "Sub-task",
			Project:  project.Key,
			Type:     tracker.IssueTypeSubTask,
			ParentID: "TEST-100",
		}

		result, err := client.CreateIssue(input)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, tracker.IssueTypeSubTask, result.Type)

		// Verify the sub-task has a parent
		createdIssue := server.GetIssue(result.Key)
		assert.NotNil(t, createdIssue)
		assert.NotNil(t, createdIssue.Parent)
		assert.Equal(t, "TEST-100", createdIssue.Parent.Key)
	})

	t.Run("CreateIssue - invalid project", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		input := tracker.CreateIssueInput{
			Title:   "New Issue",
			Project: "INVALID",
			Type:    tracker.IssueTypeTask,
		}

		result, err := client.CreateIssue(input)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("AddComment success", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		issue := testutil.NewStoryIssue("TEST-123", "Test Story", "In Progress")
		server.AddIssue(issue)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		err = client.AddComment("TEST-123", "This is a test comment")
		assert.NoError(t, err)
	})

	t.Run("AddComment - issue not found", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		err = client.AddComment("NOTFOUND-999", "Comment")
		assert.Error(t, err)
	})

	t.Run("SetIssueStatus - exact match", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		issue := testutil.NewStoryIssue("TEST-123", "Test Story", "To Do")
		server.AddIssue(issue)

		transitions := []testutil.MockTransition{
			testutil.NewTransition("21", "In Progress", "10002", "In Progress"),
			testutil.NewTransition("31", "Done", "10004", "Done"),
		}
		server.SetTransitions("TEST-123", transitions)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		result, err := client.SetIssueStatus("TEST-123", tracker.IssueStatusInProgress)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, tracker.IssueStatusInProgress, result.Status)

		// Verify the issue status was updated
		updatedIssue := server.GetIssue("TEST-123")
		assert.Equal(t, "In Progress", updatedIssue.Status.Name)
	})

	t.Run("SetIssueStatus - pattern match", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		issue := testutil.NewStoryIssue("TEST-123", "Test Story", "To Do")
		server.AddIssue(issue)

		transitions := []testutil.MockTransition{
			testutil.NewTransition("21", "Start Progress", "10002", "In Progress"),
		}
		server.SetTransitions("TEST-123", transitions)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		result, err := client.SetIssueStatus("TEST-123", tracker.IssueStatusInProgress)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, tracker.IssueStatusInProgress, result.Status)
	})

	t.Run("SetIssueStatus - no matching transition", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		issue := testutil.NewStoryIssue("TEST-123", "Test Story", "To Do")
		server.AddIssue(issue)

		transitions := []testutil.MockTransition{
			testutil.NewTransition("21", "Open", "10001", "Open"),
			testutil.NewTransition("31", "Closed", "10005", "Closed"),
		}
		server.SetTransitions("TEST-123", transitions)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		result, err := client.SetIssueStatus("TEST-123", tracker.IssueStatusInProgress)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no matching transition found")
	})

	t.Run("SetJiraIssueStatus - done variants", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		issue := testutil.NewStoryIssue("TEST-123", "Test Story", "In Progress")
		server.AddIssue(issue)

		transitions := []testutil.MockTransition{
			testutil.NewTransition("41", "Complete", "10004", "Done"),
		}
		server.SetTransitions("TEST-123", transitions)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		result, err := client.SetJiraIssueStatus("TEST-123", StatusDone)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Done", result.Status)
	})

	t.Run("CreateRelease success", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		project := testutil.DefaultProject()
		server.AddProject(project)

		issue := testutil.NewStoryIssue("TEST-123", "Test Story", "Done")
		server.AddIssue(issue)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		input := tracker.CreateReleaseInput{
			Name:        "v1.0.0",
			Description: "Release 1.0.0",
			IssueIDs:    []string{"TEST-123"},
		}

		result, err := client.CreateRelease(input)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "v1.0.0", result.Name)
		assert.Equal(t, "Release 1.0.0", result.Description)
		assert.Contains(t, result.URL, "projects/TEST/versions/")
	})

	t.Run("CreateRelease - no issue IDs", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		input := tracker.CreateReleaseInput{
			Name:        "v1.0.0",
			Description: "Release 1.0.0",
			IssueIDs:    []string{},
		}

		result, err := client.CreateRelease(input)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "at least one issue ID is required")
	})

	t.Run("SetFixVersion success", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		issue1 := testutil.NewStoryIssue("TEST-123", "Story 1", "Done")
		issue2 := testutil.NewStoryIssue("TEST-124", "Story 2", "Done")
		server.AddIssues(issue1, issue2)

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		release := &tracker.Release{
			ID:   "20001",
			Name: "v1.0.0",
		}

		err = client.SetFixVersion([]string{"TEST-123", "TEST-124"}, release)
		assert.NoError(t, err)

		// Verify fix versions were set
		updatedIssue1 := server.GetIssue("TEST-123")
		assert.Len(t, updatedIssue1.FixVersions, 1)
		assert.Equal(t, "v1.0.0", updatedIssue1.FixVersions[0].Name)

		updatedIssue2 := server.GetIssue("TEST-124")
		assert.Len(t, updatedIssue2.FixVersions, 1)
		assert.Equal(t, "v1.0.0", updatedIssue2.FixVersions[0].Name)
	})

	t.Run("SetFixVersion - nil release", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		err = client.SetFixVersion([]string{"TEST-123"}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "release cannot be nil")
	})

	t.Run("SetFixVersion - issue not found", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		release := &tracker.Release{
			ID:   "20001",
			Name: "v1.0.0",
		}

		err = client.SetFixVersion([]string{"NOTFOUND-999"}, release)
		assert.Error(t, err)
	})

	t.Run("GetIssueURL - various formats", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		tests := []struct {
			input    string
			expected string
		}{
			{"TEST-123", server.URL() + "/browse/TEST-123"},
			{"test-456", server.URL() + "/browse/TEST-456"},
			{"Test-789", server.URL() + "/browse/TEST-789"},
		}

		for _, tt := range tests {
			url := client.GetIssueURL(tt.input)
			assert.Equal(t, tt.expected, url)
		}
	})

	t.Run("GetIssueUrl - deprecated method", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		url, err := client.GetIssueUrl("TEST-123")
		assert.NoError(t, err)
		assert.Equal(t, server.URL()+"/browse/TEST-123", url)
	})

	t.Run("Name returns Jira", func(t *testing.T) {
		server := testutil.NewMockJiraServer()
		defer server.Close()

		cfg := viper.New()
		client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
		require.NoError(t, err)

		assert.Equal(t, "Jira", client.Name())
	})
}

// TestIssueHelperMethods tests the Issue struct helper methods
func TestIssueHelperMethods(t *testing.T) {
	t.Run("ShortName for all issue types", func(t *testing.T) {
		tests := []struct {
			issueType string
			expected  string
		}{
			{"Bug", "b"},
			{"bug", "b"},
			{"BUG", "b"},
			{"Feature", "f"},
			{"feature", "f"},
			{"Story", "f"},
			{"story", "f"},
			{"Sub-task", "f"},
			{"Spike", "s"},
			{"spike", "s"},
			{"Task", "c"},
			{"task", "c"},
			{"Tech Debt", "d"},
			{"tech debt", "d"},
			{"Unknown Type", "f"},
			{"", "f"},
		}

		for _, tt := range tests {
			t.Run(tt.issueType, func(t *testing.T) {
				issue := &Issue{Type: tt.issueType}
				assert.Equal(t, tt.expected, issue.ShortName())
			})
		}
	})

	t.Run("SanitizedSummary - various cases", func(t *testing.T) {
		tests := []struct {
			name     string
			summary  string
			expected string
		}{
			{
				name:     "simple text",
				summary:  "Add new feature",
				expected: "add_new_feature",
			},
			{
				name:     "with special characters",
				summary:  "Fix: bug in the 'login' page",
				expected: "fix_bug_in_the_login_page",
			},
			{
				name:     "with slashes",
				summary:  "Update user/profile endpoint",
				expected: "update_user-profile_endpoint",
			},
			{
				name:     "with dots",
				summary:  "Fix api.endpoint issue",
				expected: "fix_api-endpoint_issue",
			},
			{
				name:     "with parentheses",
				summary:  "Add feature (beta)",
				expected: "add_feature__beta",
			},
			{
				name:     "with multiple spaces",
				summary:  "Fix    multiple   spaces",
				expected: "fix____multiple___spaces",
			},
			{
				name:     "with double dashes",
				summary:  "Fix--issue",
				expected: "fix--issue",
			},
			{
				name:     "very long summary",
				summary:  "This is a very long summary that exceeds the seventy two character limit and should be truncated to exactly seventy two characters",
				expected: "this_is_a_very_long_summary_that_exceeds_the_seventy_two_character_limit",
			},
			{
				name:     "trailing spaces",
				summary:  "Fix bug   ",
				expected: "fix_bug__",
			},
			{
				name:     "with quotes",
				summary:  `Add "feature" with 'quotes'`,
				expected: "add_feature_with_quotes",
			},
			{
				name:     "with ampersand",
				summary:  "Fix A & B issue",
				expected: "fix_a__b_issue",
			},
			{
				name:     "with colons",
				summary:  "Update: database schema",
				expected: "update_database_schema",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				issue := &Issue{Summary: tt.summary}
				result := issue.SanitizedSummary()
				assert.Equal(t, tt.expected, result)
				assert.LessOrEqual(t, len(result), 72, "Result should not exceed 72 characters")
			})
		}
	})
}

// TestConversionFunctions tests the status and type conversion functions
func TestConversionFunctions(t *testing.T) {
	t.Run("jiraIssueStatusToTracker", func(t *testing.T) {
		tests := []struct {
			jiraStatus    string
			trackerStatus tracker.IssueStatus
		}{
			{"Backlog", tracker.IssueStatusBacklog},
			{"Product Backlog", tracker.IssueStatusBacklog},
			{"backlog", tracker.IssueStatusBacklog},
			{"To Do", tracker.IssueStatusToDo},
			{"Todo", tracker.IssueStatusToDo},
			{"TODO", tracker.IssueStatusToDo},
			{"Selected for Development", tracker.IssueStatusToDo},
			{"In Progress", tracker.IssueStatusInProgress},
			{"Work in Progress", tracker.IssueStatusInProgress},
			{"progress ongoing", tracker.IssueStatusInProgress},
			{"In Review", tracker.IssueStatusInReview},
			{"Code Review", tracker.IssueStatusInReview},
			{"review pending", tracker.IssueStatusInReview},
			{"Done", tracker.IssueStatusDone},
			{"Complete", tracker.IssueStatusDone},
			{"Completed", tracker.IssueStatusDone},
			{"Resolved", tracker.IssueStatusDone},
			{"Unknown Status", tracker.IssueStatusBacklog},
			{"", tracker.IssueStatusBacklog},
		}

		for _, tt := range tests {
			t.Run(tt.jiraStatus, func(t *testing.T) {
				result := jiraIssueStatusToTracker(tt.jiraStatus)
				assert.Equal(t, tt.trackerStatus, result)
			})
		}
	})

	t.Run("trackerStatusToJira", func(t *testing.T) {
		tests := []struct {
			trackerStatus tracker.IssueStatus
			jiraStatus    IssueStatus
		}{
			{tracker.IssueStatusBacklog, StatusBacklog},
			{tracker.IssueStatusToDo, StatusSelectedForDevelopment},
			{tracker.IssueStatusInProgress, StatusInProgress},
			{tracker.IssueStatusInReview, StatusInReview},
			{tracker.IssueStatusDone, StatusDone},
			{tracker.IssueStatus("unknown"), StatusBacklog},
		}

		for _, tt := range tests {
			t.Run(string(tt.trackerStatus), func(t *testing.T) {
				result := trackerStatusToJira(tt.trackerStatus)
				assert.Equal(t, tt.jiraStatus, result)
			})
		}
	})

	t.Run("jiraIssueTypeToTracker", func(t *testing.T) {
		tests := []struct {
			jiraType    string
			trackerType tracker.IssueType
		}{
			{"Story", tracker.IssueTypeStory},
			{"story", tracker.IssueTypeStory},
			{"Feature", tracker.IssueTypeStory},
			{"feature request", tracker.IssueTypeStory},
			{"Bug", tracker.IssueTypeBug},
			{"bug fix", tracker.IssueTypeBug},
			{"Task", tracker.IssueTypeTask},
			{"task item", tracker.IssueTypeTask},
			{"Sub-task", tracker.IssueTypeSubTask},
			{"Subtask", tracker.IssueTypeSubTask},
			{"sub task", tracker.IssueTypeTask}, // Only "sub-task" and "subtask" match, not "sub task"
			{"Epic", tracker.IssueTypeEpic},
			{"epic story", tracker.IssueTypeStory}, // "story" is checked before "epic"
			{"Unknown", tracker.IssueTypeTask},
			{"", tracker.IssueTypeTask},
		}

		for _, tt := range tests {
			t.Run(tt.jiraType, func(t *testing.T) {
				result := jiraIssueTypeToTracker(tt.jiraType)
				assert.Equal(t, tt.trackerType, result)
			})
		}
	})

	t.Run("trackerIssueTypeToJira", func(t *testing.T) {
		tests := []struct {
			trackerType tracker.IssueType
			jiraType    string
		}{
			{tracker.IssueTypeStory, "Story"},
			{tracker.IssueTypeBug, "Bug"},
			{tracker.IssueTypeTask, "Task"},
			{tracker.IssueTypeSubTask, "Sub-task"},
			{tracker.IssueTypeEpic, "Epic"},
			{tracker.IssueType("unknown"), "Task"},
		}

		for _, tt := range tests {
			t.Run(string(tt.trackerType), func(t *testing.T) {
				result := trackerIssueTypeToJira(tt.trackerType)
				assert.Equal(t, tt.jiraType, result)
			})
		}
	})
}

// TestIsValidIssueID tests the issue ID validation
func TestIsValidIssueID(t *testing.T) {
	server := testutil.NewMockJiraServer()
	defer server.Close()

	cfg := viper.New()
	client, err := NewWithHTTPClient(cfg, server.Client(), server.URL())
	require.NoError(t, err)

	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		// Valid patterns
		{"valid simple", "TEST-123", true},
		{"valid lowercase", "test-123", true},
		{"valid long project", "MYPROJECT-1", true},
		{"valid with numbers", "TEST2-456", true},
		{"valid with underscore", "TEST_PROJ-789", true},
		{"valid large number", "ABC-99999", true},
		{"valid single letter", "A-1", false}, // Requires 2+ chars before hyphen

		// Invalid patterns
		{"invalid no hyphen", "TEST123", false},
		{"invalid no number", "TEST-", false},
		{"invalid starts with number", "123-TEST", false},
		{"invalid empty", "", false},
		{"invalid only numbers", "123-456", false},
		{"invalid special chars", "TEST@-123", false},
		{"invalid spaces", "TEST -123", false},
		{"invalid double hyphen", "TEST--123", false},
		{"invalid just text", "TESTPROJECT", false},
		{"invalid just number", "12345", false},
		{"invalid lowercase start", "test-123", true}, // Actually valid - gets uppercased
		{"invalid with dot", "TEST.PROJ-123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.IsValidIssueID(tt.id)
			assert.Equal(t, tt.valid, result, "IsValidIssueID(%q) = %v, want %v", tt.id, result, tt.valid)
		})
	}
}
