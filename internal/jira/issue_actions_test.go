package jira

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestIssueActionsLiteralComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/rest/api/3/issue/TEST-123/comment", r.URL.Path)
		var payload struct {
			Body adfNode `json:"body"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, adfNode{Type: "doc", Version: 1, Content: []adfNode{
			{Type: "paragraph", Content: []adfNode{{Type: "text", Text: "{{.Literal}}"}, {Type: "hardBreak"}, {Type: "text", Text: "second line"}}},
			{Type: "paragraph", Content: []adfNode{{Type: "text", Text: "question?"}}},
		}}, payload.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"10234"}`))
	}))
	defer server.Close()
	client, err := NewWithHTTPClient(viper.New(), server.Client(), server.URL)
	require.NoError(t, err)
	result, err := client.(IssueActions).CommentOnIssue("TEST-123", "{{.Literal}}\r\nsecond line\r\n\r\nquestion?")
	require.NoError(t, err)
	require.Equal(t, "10234", result.CommentID)
	require.Equal(t, server.URL+"/browse/TEST-123?focusedCommentId=10234", result.URL)
}

func TestIssueActionsLabelsAndErrors(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method == http.MethodPut {
			require.Equal(t, "/rest/api/3/issue/TEST-123", r.URL.Path)
			var body map[string]map[string][]map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, []map[string]string{{"add": "ai"}, {"add": "backend"}}, body["update"]["labels"])
			w.WriteHeader(http.StatusNoContent)
			return
		}
		require.Equal(t, "labels", r.URL.Query().Get("fields"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"fields":{"labels":["existing","ai","backend"]}}`))
	}))
	defer server.Close()
	client, err := NewWithHTTPClient(viper.New(), server.Client(), server.URL)
	require.NoError(t, err)
	actions := client.(IssueActions)
	result, err := actions.LabelIssue("TEST-123", []string{"ai", "ai", "backend"})
	require.NoError(t, err)
	require.Equal(t, []string{"existing", "ai", "backend"}, result.Labels)
	require.Equal(t, 2, requests)
	for _, key := range []string{"test-123", "A-1", "TEST_X-1", "TEST-1/path", "TEST-1\n"} {
		_, err = actions.CommentOnIssue(key, "hello")
		require.Error(t, err)
		_, err = actions.LabelIssue(key, []string{"ai"})
		require.Error(t, err)
	}
	require.Equal(t, 2, requests)
}

func TestIssueActionsHTTPErrorMessages(t *testing.T) {
	for _, test := range []struct {
		status  int
		message string
	}{{403, "permission denied"}, {404, "was not found"}} {
		t.Run(test.message, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(`{"errorMessages":["rejected"]}`))
			}))
			defer server.Close()
			client, err := NewWithHTTPClient(viper.New(), server.Client(), server.URL)
			require.NoError(t, err)
			_, err = client.(IssueActions).CommentOnIssue("TEST-123", "hello")
			require.ErrorContains(t, err, test.message)
			_, err = client.(IssueActions).LabelIssue("TEST-123", []string{"ai"})
			require.ErrorContains(t, err, test.message)
		})
	}
}
