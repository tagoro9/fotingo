package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/jira"
)

type issueActionsStub struct {
	key    string
	body   string
	labels []string
}

func (s *issueActionsStub) CommentOnIssue(key, body string) (*jira.IssueCommentResult, error) {
	s.key, s.body = key, body
	return &jira.IssueCommentResult{IssueKey: key, CommentID: "42", URL: "https://jira.example/browse/TEST-123?focusedCommentId=42"}, nil
}
func (s *issueActionsStub) LabelIssue(key string, labels []string) (*jira.IssueLabelResult, error) {
	s.key, s.labels = key, labels
	return &jira.IssueLabelResult{IssueKey: key, Labels: append([]string{"existing"}, labels...)}, nil
}

func TestIssueCommandsOutsideRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	oldFactory, oldGlobal := newIssueActions, Global
	t.Cleanup(func() { newIssueActions, Global = oldFactory, oldGlobal })
	Global.JSON = true
	stub := &issueActionsStub{}
	newIssueActions = func() (jira.IssueActions, error) { return stub, nil }
	for _, test := range []struct {
		name        string
		args        []string
		stdin, body string
	}{
		{"argument", []string{"comment", "-i", "TEST-123", "{{.Literal}}\n\nQuestion?"}, "", "{{.Literal}}\n\nQuestion?"},
		{"stdin", []string{"comment", "-i", "TEST-123", "--body-file", "-"}, "hello\nworld\n", "hello\nworld\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := newIssueCommand()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetIn(bytes.NewBufferString(test.stdin))
			cmd.SetArgs(test.args)
			require.NoError(t, cmd.Execute())
			require.Equal(t, test.body, stub.body)
			require.JSONEq(t, `{"issueKey":"TEST-123","commentId":"42","url":"https://jira.example/browse/TEST-123?focusedCommentId=42"}`, output.String())
		})
	}
	path := filepath.Join(t.TempDir(), "note.txt")
	require.NoError(t, os.WriteFile(path, []byte("from file\n\nquestion"), 0600))
	cmd := newIssueCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"comment", "-i", "TEST-123", "--body-file", path})
	require.NoError(t, cmd.Execute())
	require.Equal(t, "from file\n\nquestion", stub.body)
	cmd = newIssueCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"label", "-i", "TEST-123", "--add", "ai", "--add", "backend"})
	require.NoError(t, cmd.Execute())
	require.Equal(t, []string{"ai", "backend"}, stub.labels)
	require.JSONEq(t, `{"issueKey":"TEST-123","labels":["existing","ai","backend"]}`, output.String())
}

func TestIssueCommandsValidateBeforeClient(t *testing.T) {
	oldFactory := newIssueActions
	t.Cleanup(func() { newIssueActions = oldFactory })
	newIssueActions = func() (jira.IssueActions, error) { t.Fatal("must validate before initializing Jira"); return nil, nil }
	for _, args := range [][]string{
		{"comment", "-i", "../TEST-123", "hello"},
		{"comment", "-i", "TEST-123"},
		{"comment", "-i", "TEST-123", " "},
		{"comment", "-i", "TEST-123", "hello", "--body-file", "-"},
		{"comment", "-i", "TEST-123", "--body-file", "missing-file"},
		{"label", "-i", "test-123", "--add", "ai"},
		{"label", "-i", "TEST-123"},
		{"label", "-i", "TEST-123", "--add", "bad label"},
	} {
		cmd := newIssueCommand()
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cmd.SetArgs(args)
		require.Error(t, cmd.Execute())
	}
}

func TestIssueCommandJSONValidationError(t *testing.T) {
	t.Chdir(t.TempDir())
	restore := saveGlobalFlags()
	defer restore()
	Fotingo.SetArgs([]string{"issue", "comment", "-i", "invalid/path", "hello", "--json"})
	t.Cleanup(func() { Fotingo.SetArgs(nil) })
	output := captureStdout(t, func() { require.NotZero(t, ExecuteWithExitCode()) })
	var result struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(output), &result))
	require.Contains(t, result.Error, "invalid issue key")
}
