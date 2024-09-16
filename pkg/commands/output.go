// Package commands provides the CLI commands for fotingo.
//
// This file defines JSON output structures for all commands to support
// machine-readable output via the --json flag.
package commands

import (
	"encoding/json"
	"fmt"
	"os"

	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/i18n"
)

// JSON Output Schema Documentation
//
// All JSON output follows a consistent structure with these conventions:
//
// 1. Success responses include relevant data fields for the command
// 2. Error responses always include an "error" field with a message
// 3. Fields use camelCase naming
// 4. Optional fields are omitted when empty (omitempty)
// 5. Arrays are never null - empty arrays are represented as []
//
// Exit codes in JSON mode follow the same contract as non-JSON mode.
// The exit code can be determined from the error type or success state.

// StartOutput represents the JSON output of the start command.
//
// Example success output:
//
//	{
//	  "success": true,
//	  "issue": {
//	    "key": "PROJ-123",
//	    "summary": "Fix login bug",
//	    "status": "In Progress",
//	    "type": "Bug",
//	    "url": "https://jira.example.com/browse/PROJ-123"
//	  },
//	  "branch": {
//	    "name": "feature/PROJ-123-fix-login-bug",
//	    "created": true
//	  }
//	}
//
// Example error output:
//
//	{
//	  "success": false,
//	  "error": "failed to initialize Jira client: authentication required"
//	}
type StartOutput struct {
	Success bool             `json:"success"`
	Issue   *IssueInfo       `json:"issue,omitempty"`
	Branch  *StartBranchInfo `json:"branch,omitempty"`
	Error   string           `json:"error,omitempty"`
}

// StartBranchInfo contains branch information for the start command output.
type StartBranchInfo struct {
	Name    string `json:"name"`
	Created bool   `json:"created"`
}

// ReviewOutput represents the JSON output of the review command.
//
// Example success output:
//
//	{
//	  "success": true,
//	  "pullRequest": {
//	    "number": 42,
//	    "url": "https://github.com/owner/repo/pull/42",
//	    "title": "[PROJ-123] Fix login bug",
//	    "draft": false,
//	    "state": "open"
//	  },
//	  "issue": {
//	    "key": "PROJ-123",
//	    "summary": "Fix login bug",
//	    "status": "In Review",
//	    "type": "Bug",
//	    "url": "https://jira.example.com/browse/PROJ-123"
//	  },
//	  "labels": ["bug", "priority"],
//	  "reviewers": ["alice", "bob"],
//	  "teamReviewers": ["acme/platform"],
//	  "assignees": ["alice"]
//	}
//
// Example when PR already exists:
//
//	{
//	  "success": true,
//	  "pullRequest": {
//	    "number": 42,
//	    "url": "https://github.com/owner/repo/pull/42",
//	    "title": "[PROJ-123] Fix login bug",
//	    "draft": false,
//	    "state": "open"
//	  },
//	  "existed": true
//	}
type ReviewOutput struct {
	Success       bool             `json:"success"`
	PullRequest   *PullRequestInfo `json:"pullRequest,omitempty"`
	Issue         *IssueInfo       `json:"issue,omitempty"`
	Labels        []string         `json:"labels,omitempty"`
	Reviewers     []string         `json:"reviewers,omitempty"`
	TeamReviewers []string         `json:"teamReviewers,omitempty"`
	Assignees     []string         `json:"assignees,omitempty"`
	Existed       bool             `json:"existed,omitempty"`
	Error         string           `json:"error,omitempty"`
}

// SearchOutput represents the JSON output of the search command.
type SearchOutput struct {
	Success bool               `json:"success"`
	Domain  string             `json:"domain"`
	Query   string             `json:"query"`
	Results []SearchResultInfo `json:"results"`
	Error   string             `json:"error,omitempty"`
}

// SearchResultInfo contains one resolved review metadata search result.
type SearchResultInfo struct {
	Resolved string `json:"resolved"`
	Label    string `json:"label"`
	Detail   string `json:"detail,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

// PullRequestInfo contains information about a GitHub pull request.
type PullRequestInfo struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Draft  bool   `json:"draft"`
	State  string `json:"state"`
}

// OpenOutput represents the JSON output of the open command.
//
// Example success output:
//
//	{
//	  "success": true,
//	  "target": "pr",
//	  "url": "https://github.com/owner/repo/pull/42",
//	  "opened": true
//	}
type OpenOutput struct {
	Success bool   `json:"success"`
	Target  string `json:"target"`
	URL     string `json:"url,omitempty"`
	Opened  bool   `json:"opened,omitempty"`
	Error   string `json:"error,omitempty"`
}

// AISetupOutput represents JSON output for `fotingo ai setup`.
type AISetupOutput struct {
	Success   bool            `json:"success"`
	Scope     string          `json:"scope,omitempty"`
	Providers []string        `json:"providers,omitempty"`
	DryRun    bool            `json:"dryRun,omitempty"`
	Force     bool            `json:"force,omitempty"`
	Results   []AISetupResult `json:"results,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// AISetupResult captures one provider install outcome.
type AISetupResult struct {
	Provider string `json:"provider"`
	Scope    string `json:"scope"`
	Root     string `json:"root"`
	Path     string `json:"path"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
}

// ErrorOutput represents a JSON error response.
//
// Example:
//
//	{
//	  "error": "failed to create branch",
//	  "code": 4,
//	  "type": "Git error"
//	}
type ErrorOutput struct {
	Error string `json:"error"`
	Code  int    `json:"code,omitempty"`
	Type  string `json:"type,omitempty"`
}

// JSONOutput provides utilities for outputting JSON responses.
type JSONOutput struct {
	writer *json.Encoder
}

// NewJSONOutput creates a new JSONOutput that writes to stdout.
func NewJSONOutput() *JSONOutput {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return &JSONOutput{writer: encoder}
}

// Write outputs data as formatted JSON to stdout.
func (j *JSONOutput) Write(data interface{}) error {
	return j.writer.Encode(data)
}

// WriteError outputs an error as JSON.
func (j *JSONOutput) WriteError(err error) error {
	code := fterrors.GetExitCode(err)
	output := ErrorOutput{
		Error: err.Error(),
		Code:  code,
		Type:  fterrors.ExitCodeDescription(code),
	}
	return j.writer.Encode(output)
}

// WriteSuccess outputs a generic success response.
func (j *JSONOutput) WriteSuccess(data interface{}) error {
	return j.writer.Encode(data)
}

// OutputJSON outputs the data as formatted JSON to stdout.
// This is a convenience function that creates a new JSONOutput and writes.
func OutputJSON(data interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, localizer.T(i18n.RootErrEncodingJSON), err)
	}
}

// OutputJSONError outputs an error as JSON to stdout.
func OutputJSONError(err error) {
	code := fterrors.GetExitCode(err)
	output := ErrorOutput{
		Error: err.Error(),
		Code:  code,
		Type:  fterrors.ExitCodeDescription(code),
	}
	OutputJSON(output)
}

// ShouldOutputJSON returns true if the --json flag is set.
// Commands should check this before outputting human-readable text.
func ShouldOutputJSON() bool {
	return Global.JSON
}

// ShouldSuppressOutput returns true if output should be suppressed.
// This is true when --quiet is set or when --json is set (to avoid
// mixing human-readable output with JSON).
func ShouldSuppressOutput() bool {
	return Global.Quiet || Global.JSON
}

// ShouldOutputVerbose returns true when step-level progress should be shown.
func ShouldOutputVerbose() bool {
	if ShouldSuppressOutput() {
		return false
	}
	return Global.Verbose || Global.Debug
}

// ShouldOutputDebug returns true when diagnostic output should be shown.
func ShouldOutputDebug() bool {
	if ShouldSuppressOutput() {
		return false
	}
	return Global.Debug
}
