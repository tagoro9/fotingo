// Package errors provides typed errors and exit codes for the fotingo CLI.
//
// Exit codes follow a documented contract for consistent behavior across
// all commands, making it easier for scripts and AI agents to handle errors.
package errors

import (
	"errors"
	"fmt"
)

// Exit codes for the fotingo CLI.
//
// These codes provide a stable contract for scripts and AI agents:
//
//	0 - Success: Command completed successfully
//	1 - General: Unclassified or unexpected error
//	2 - Config: Configuration file missing, invalid, or malformed
//	3 - Auth: Authentication failed (invalid token, expired credentials)
//	4 - Git: Git operation failed (not a repo, branch issues, push failed)
//	5 - Jira: Jira API error (issue not found, permission denied)
//	6 - GitHub: GitHub API error (PR creation failed, rate limited)
const (
	ExitSuccess       = 0
	ExitGeneral       = 1
	ExitConfig        = 2
	ExitAuth          = 3
	ExitGit           = 4
	ExitJira          = 5
	ExitGitHub        = 6
	ExitUserCancelled = 130 // Standard shell convention for SIGINT
)

// ExitCodeError is an error that carries a specific exit code.
// Commands can wrap errors with this type to indicate the appropriate
// exit code for the CLI to return.
type ExitCodeError struct {
	Code    int
	Message string
	Err     error
}

// Error implements the error interface.
func (e *ExitCodeError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *ExitCodeError) Unwrap() error {
	return e.Err
}

// ExitCode returns the exit code for this error.
func (e *ExitCodeError) ExitCode() int {
	return e.Code
}

// NewExitCodeError creates a new ExitCodeError with the given code and message.
func NewExitCodeError(code int, message string) *ExitCodeError {
	return &ExitCodeError{
		Code:    code,
		Message: message,
	}
}

// WrapWithExitCode wraps an error with an exit code.
func WrapWithExitCode(code int, message string, err error) *ExitCodeError {
	return &ExitCodeError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Sentinel errors for common error types.
// These can be used with errors.Is() for error checking.
var (
	ErrConfig        = NewExitCodeError(ExitConfig, "configuration error")
	ErrAuth          = NewExitCodeError(ExitAuth, "authentication error")
	ErrGit           = NewExitCodeError(ExitGit, "git error")
	ErrJira          = NewExitCodeError(ExitJira, "jira error")
	ErrGitHub        = NewExitCodeError(ExitGitHub, "github error")
	ErrUserCancelled = NewExitCodeError(ExitUserCancelled, "operation cancelled")
)

// ConfigError creates a configuration error with the given message.
func ConfigError(message string) *ExitCodeError {
	return NewExitCodeError(ExitConfig, message)
}

// ConfigErrorf creates a configuration error with a formatted message.
func ConfigErrorf(format string, args ...interface{}) *ExitCodeError {
	return NewExitCodeError(ExitConfig, fmt.Sprintf(format, args...))
}

// AuthError creates an authentication error with the given message.
func AuthError(message string) *ExitCodeError {
	return NewExitCodeError(ExitAuth, message)
}

// AuthErrorf creates an authentication error with a formatted message.
func AuthErrorf(format string, args ...interface{}) *ExitCodeError {
	return NewExitCodeError(ExitAuth, fmt.Sprintf(format, args...))
}

// GitError creates a git error with the given message.
func GitError(message string) *ExitCodeError {
	return NewExitCodeError(ExitGit, message)
}

// GitErrorf creates a git error with a formatted message.
func GitErrorf(format string, args ...interface{}) *ExitCodeError {
	return NewExitCodeError(ExitGit, fmt.Sprintf(format, args...))
}

// WrapGitError wraps an existing error as a git error.
func WrapGitError(message string, err error) *ExitCodeError {
	return WrapWithExitCode(ExitGit, message, err)
}

// JiraError creates a Jira error with the given message.
func JiraError(message string) *ExitCodeError {
	return NewExitCodeError(ExitJira, message)
}

// JiraErrorf creates a Jira error with a formatted message.
func JiraErrorf(format string, args ...interface{}) *ExitCodeError {
	return NewExitCodeError(ExitJira, fmt.Sprintf(format, args...))
}

// WrapJiraError wraps an existing error as a Jira error.
func WrapJiraError(message string, err error) *ExitCodeError {
	return WrapWithExitCode(ExitJira, message, err)
}

// GitHubError creates a GitHub error with the given message.
func GitHubError(message string) *ExitCodeError {
	return NewExitCodeError(ExitGitHub, message)
}

// GitHubErrorf creates a GitHub error with a formatted message.
func GitHubErrorf(format string, args ...interface{}) *ExitCodeError {
	return NewExitCodeError(ExitGitHub, fmt.Sprintf(format, args...))
}

// WrapGitHubError wraps an existing error as a GitHub error.
func WrapGitHubError(message string, err error) *ExitCodeError {
	return WrapWithExitCode(ExitGitHub, message, err)
}

// GetExitCode extracts the exit code from an error.
// If the error is an ExitCodeError, it returns its code.
// Otherwise, it returns ExitGeneral (1).
func GetExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return ExitGeneral
}

// ExitCodeDescription returns a human-readable description for an exit code.
func ExitCodeDescription(code int) string {
	switch code {
	case ExitSuccess:
		return "Success"
	case ExitGeneral:
		return "General error"
	case ExitConfig:
		return "Configuration error"
	case ExitAuth:
		return "Authentication error"
	case ExitGit:
		return "Git error"
	case ExitJira:
		return "Jira error"
	case ExitGitHub:
		return "GitHub error"
	case ExitUserCancelled:
		return "Operation cancelled"
	default:
		return "Unknown error"
	}
}
