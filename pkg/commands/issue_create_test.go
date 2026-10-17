package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/jira"
	jiratestutil "github.com/tagoro9/fotingo/internal/jira/testutil"
	"github.com/tagoro9/fotingo/internal/tracker"
)

func TestIssueCreateOutsideRepository(t *testing.T) {
	for _, mode := range []string{"default", "parent", "text", "quiet"} {
		t.Run(mode, func(t *testing.T) {
			t.Chdir(t.TempDir())
			oldClient, oldGit, oldConfig, oldGlobal := newJiraClient, newGitClient, fotingoConfig, Global
			t.Cleanup(func() { newJiraClient, newGitClient, fotingoConfig, Global = oldClient, oldGit, oldConfig, oldGlobal })
			Global.JSON = mode != "text" && mode != "quiet"
			Global.Quiet = mode == "quiet"
			fotingoConfig = viper.New()
			fotingoConfig.Set("jira.root", "https://jira.example.com")
			fotingoConfig.Set("tracker.labels.inProgress", []string{"active"})
			server := jiratestutil.NewMockJiraServer()
			defer server.Close()
			server.AddProject(jiratestutil.DefaultProject())
			server.AddIssue(jiratestutil.NewIssue("TEST-100", "Parent", "To Do", "Story"))
			server.AddIssue(jiratestutil.NewIssue("TEST-200", "Epic", "To Do", "Epic"))
			newJiraClient = func(cfg *viper.Viper) (jira.Jira, error) {
				return jira.NewWithHTTPClient(cfg, server.Client(), server.URL())
			}
			newGitClient = func(*viper.Viper, *chan string) (git.Git, error) {
				t.Fatal("issue create must not initialize Git")
				return nil, nil
			}
			args := []string{"create", "-p", " test ", "-t", " Plan work ", "-d", "Detailed plan", "-l", "backend", "-l", "planned"}
			if mode == "parent" {
				args = append(args, "--parent", "TEST-100")
			}
			cmd := newIssueCommand()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetArgs(args)
			require.NoError(t, cmd.Execute())
			if Global.JSON {
				var result issueCreateResult
				require.NoError(t, json.Unmarshal(output.Bytes(), &result))
				assert.Equal(t, "Plan work", result.Summary)
				assert.Equal(t, "Backlog", result.Status)
				assert.NotEmpty(t, result.IssueKey)
				assert.Contains(t, result.URL, "/browse/"+result.IssueKey)
			} else if mode == "quiet" {
				assert.Empty(t, output.String())
			} else {
				assert.Contains(t, output.String(), "Created TEST-")
			}
			mutations := 0
			for _, request := range server.GetRequestLog() {
				if request.Method == "GET" {
					continue
				}
				mutations++
				assert.Equal(t, "POST", request.Method)
				assert.True(t, strings.HasSuffix(request.Path, "/issue"), "unexpected mutation: %s", request.Path)
			}
			assert.Equal(t, 1, mutations)
			created := server.GetIssue("TEST-3")
			require.NotNil(t, created)
			assert.Equal(t, "Plan work", created.Summary)
			assert.Equal(t, "Detailed plan", created.Description)
			assert.Equal(t, "Backlog", created.Status.Name)
			assert.Nil(t, created.Assignee)
			assert.ElementsMatch(t, []string{"backend", "planned"}, created.Labels)
			if mode == "parent" {
				require.NotNil(t, created.Parent)
				assert.Equal(t, "TEST-100", created.Parent.Key)
				assert.Equal(t, "Sub-task", created.IssueType.Name)
			} else {
				assert.Equal(t, "Task", created.IssueType.Name)
			}
		})
	}
}

func TestIssueCreateValidation(t *testing.T) {
	oldClient := newJiraClient
	t.Cleanup(func() { newJiraClient = oldClient })
	newJiraClient = func(*viper.Viper) (jira.Jira, error) {
		t.Fatal("invalid input must fail before Jira initialization")
		return nil, nil
	}
	for _, args := range [][]string{
		{}, {"-p", "TEST"}, {"-t", "Plan"}, {"-p", "TEST", "-t", " "},
		{"-p", "TEST", "-t", "Plan", "-k", "Unknown"},
		{"-p", "TEST", "-t", "Plan", "-k", "SubTask"},
		{"-p", "TEST", "-t", "Plan", "--parent", "TEST-1", "--epic", "TEST-2"},
		{"-p", "TEST", "-t", "Plan", "-l", "bad label"},
		{"-p", "TEST", "-t", "Plan", "unexpected"},
	} {
		cmd := newIssueCreateCommand()
		cmd.SilenceErrors, cmd.SilenceUsage = true, true
		cmd.SetArgs(args)
		require.Error(t, cmd.Execute())
	}
}

func TestIssueCreateClientError(t *testing.T) {
	oldClient, oldConfig := newJiraClient, fotingoConfig
	t.Cleanup(func() { newJiraClient, fotingoConfig = oldClient, oldConfig })
	fotingoConfig = viper.New()
	fotingoConfig.Set("jira.root", "https://jira.example.com")
	newJiraClient = func(*viper.Viper) (jira.Jira, error) { return nil, errors.New("authentication required") }
	cmd := newIssueCreateCommand()
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs([]string{"-p", "TEST", "-t", "Plan"})
	require.ErrorContains(t, cmd.Execute(), "authentication required")
}

func TestIssueCreateEpic(t *testing.T) {
	oldClient, oldConfig, oldGlobal := newJiraClient, fotingoConfig, Global
	t.Cleanup(func() { newJiraClient, fotingoConfig, Global = oldClient, oldConfig, oldGlobal })
	Global.JSON = true
	fotingoConfig = viper.New()
	fotingoConfig.Set("jira.root", "https://jira.example.com")
	client := &mockJira{
		trackerIssue: &tracker.Issue{Key: "TEST-200", Type: tracker.IssueTypeEpic},
		createIssue:  &tracker.Issue{Key: "TEST-123", Summary: "Plan"},
		jiraIssue:    &jira.Issue{Key: "TEST-123", Summary: "Plan", Status: "To Do"},
	}
	newJiraClient = func(*viper.Viper) (jira.Jira, error) { return client, nil }
	cmd := newIssueCreateCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"-p", "TEST", "-t", "Plan", "--epic", "TEST-200"})
	require.NoError(t, cmd.Execute())
	assert.Equal(t, "TEST-200", client.createIssueInput.EpicID)
	assert.Empty(t, client.setJiraIssueStatusIDs)
}
