package jira

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	jiraClient "github.com/andygrunwald/go-jira"
)

var standaloneIssueKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)

// ValidateIssueKey validates an explicit issue key without consulting Git or Jira.
func ValidateIssueKey(key string) error {
	if !standaloneIssueKeyPattern.MatchString(key) {
		return fmt.Errorf("invalid issue key %q: expected an uppercase project key followed by - and digits", key)
	}
	return nil
}

// IssueActions provides issue mutations independent of repository context.
type IssueActions interface {
	CommentOnIssue(key, body string) (*IssueCommentResult, error)
	LabelIssue(key string, labels []string) (*IssueLabelResult, error)
}

// IssueCommentResult identifies the newly created Jira comment.
type IssueCommentResult struct {
	IssueKey  string `json:"issueKey"`
	CommentID string `json:"commentId"`
	URL       string `json:"url"`
}

// IssueLabelResult contains all labels present after an additive update.
type IssueLabelResult struct {
	IssueKey string   `json:"issueKey"`
	Labels   []string `json:"labels"`
}

type adfNode struct {
	Type    string    `json:"type"`
	Version int       `json:"version,omitempty"`
	Text    string    `json:"text,omitempty"`
	Content []adfNode `json:"content,omitempty"`
}

func plainTextDocument(body string) adfNode {
	body = strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	doc := adfNode{Type: "doc", Version: 1}
	for _, paragraph := range strings.Split(body, "\n\n") {
		node := adfNode{Type: "paragraph"}
		for idx, line := range strings.Split(paragraph, "\n") {
			if idx > 0 {
				node.Content = append(node.Content, adfNode{Type: "hardBreak"})
			}
			if line != "" {
				node.Content = append(node.Content, adfNode{Type: "text", Text: line})
			}
		}
		doc.Content = append(doc.Content, node)
	}
	return doc
}

func issueActionError(key string, response *jiraClient.Response, err error) error {
	if response != nil {
		switch response.StatusCode {
		case http.StatusForbidden:
			return fmt.Errorf("permission denied for issue %s: %w", key, err)
		case http.StatusNotFound:
			return fmt.Errorf("issue %s was not found or is not visible to this account: %w", key, err)
		}
	}
	return fmt.Errorf("Jira operation failed for issue %s: %w", key, err)
}

// CommentOnIssue posts literal plain text as an ADF document through Jira API v3.
func (j *jira) CommentOnIssue(key, body string) (*IssueCommentResult, error) {
	if err := ValidateIssueKey(key); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("comment body must not be empty")
	}
	request, err := j.client.NewRequest(http.MethodPost, "rest/api/3/issue/"+key+"/comment", struct {
		Body adfNode `json:"body"`
	}{plainTextDocument(body)})
	if err != nil {
		return nil, err
	}
	var comment struct {
		ID string `json:"id"`
	}
	response, err := j.client.Do(request, &comment)
	if err != nil {
		return nil, issueActionError(key, response, err)
	}
	if comment.ID == "" {
		return nil, fmt.Errorf("Jira returned no comment ID for issue %s", key)
	}
	commentURL := j.GetIssueURL(key)
	if commentURL != "" {
		commentURL += "?focusedCommentId=" + url.QueryEscape(comment.ID)
	}
	return &IssueCommentResult{IssueKey: key, CommentID: comment.ID, URL: commentURL}, nil
}

// LabelIssue atomically adds labels and reads the complete resulting label list.
func (j *jira) LabelIssue(key string, labels []string) (*IssueLabelResult, error) {
	if err := ValidateIssueKey(key); err != nil {
		return nil, err
	}
	updates := make([]map[string]string, 0, len(labels))
	seen := make(map[string]bool)
	for _, label := range labels {
		if strings.TrimSpace(label) == "" || strings.ContainsAny(label, " \t\r\n") {
			return nil, fmt.Errorf("labels must be non-empty and contain no whitespace")
		}
		if !seen[label] {
			updates = append(updates, map[string]string{"add": label})
			seen[label] = true
		}
	}
	if len(updates) == 0 {
		return nil, fmt.Errorf("at least one --add label is required")
	}
	request, err := j.client.NewRequest(http.MethodPut, "rest/api/3/issue/"+key, map[string]interface{}{"update": map[string]interface{}{"labels": updates}})
	if err != nil {
		return nil, err
	}
	response, err := j.client.Do(request, nil)
	if err != nil {
		return nil, issueActionError(key, response, err)
	}
	request, err = j.client.NewRequest(http.MethodGet, "rest/api/3/issue/"+key+"?fields=labels", nil)
	if err != nil {
		return nil, err
	}
	var issue struct {
		Fields struct {
			Labels []string `json:"labels"`
		} `json:"fields"`
	}
	response, err = j.client.Do(request, &issue)
	if err != nil {
		return nil, issueActionError(key, response, err)
	}
	if issue.Fields.Labels == nil {
		issue.Fields.Labels = []string{}
	}
	return &IssueLabelResult{IssueKey: key, Labels: issue.Fields.Labels}, nil
}
