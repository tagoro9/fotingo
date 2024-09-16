package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestExitCodeError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *ExitCodeError
		expected string
	}{
		{
			name:     "message only",
			err:      NewExitCodeError(ExitConfig, "config not found"),
			expected: "config not found",
		},
		{
			name:     "message with wrapped error",
			err:      WrapWithExitCode(ExitGit, "failed to push", fmt.Errorf("remote rejected")),
			expected: "failed to push: remote rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestExitCodeError_Unwrap(t *testing.T) {
	t.Parallel()

	underlying := fmt.Errorf("underlying error")
	err := WrapWithExitCode(ExitJira, "jira failed", underlying)

	if !errors.Is(err, underlying) {
		t.Error("errors.Is should find underlying error")
	}
}

func TestExitCodeError_ExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *ExitCodeError
		want int
	}{
		{"success", NewExitCodeError(ExitSuccess, "ok"), ExitSuccess},
		{"general", NewExitCodeError(ExitGeneral, "error"), ExitGeneral},
		{"config", NewExitCodeError(ExitConfig, "bad config"), ExitConfig},
		{"auth", NewExitCodeError(ExitAuth, "unauthorized"), ExitAuth},
		{"git", NewExitCodeError(ExitGit, "git failed"), ExitGit},
		{"jira", NewExitCodeError(ExitJira, "jira failed"), ExitJira},
		{"github", NewExitCodeError(ExitGitHub, "github failed"), ExitGitHub},
		{"cancelled", NewExitCodeError(ExitUserCancelled, "cancelled"), ExitUserCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.err.ExitCode(); got != tt.want {
				t.Errorf("ExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil error", nil, ExitSuccess},
		{"exit code error", NewExitCodeError(ExitJira, "jira failed"), ExitJira},
		{"wrapped exit code error", fmt.Errorf("wrapper: %w", NewExitCodeError(ExitGit, "git failed")), ExitGit},
		{"regular error", fmt.Errorf("some error"), ExitGeneral},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := GetExitCode(tt.err); got != tt.want {
				t.Errorf("GetExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestErrorHelpers(t *testing.T) {
	t.Parallel()

	t.Run("ConfigError", func(t *testing.T) {
		t.Parallel()
		err := ConfigError("config missing")
		if err.ExitCode() != ExitConfig {
			t.Errorf("ConfigError should have ExitConfig code")
		}
	})

	t.Run("ConfigErrorf", func(t *testing.T) {
		t.Parallel()
		err := ConfigErrorf("config %s not found", "test.yaml")
		if err.ExitCode() != ExitConfig {
			t.Errorf("ConfigErrorf should have ExitConfig code")
		}
		if err.Error() != "config test.yaml not found" {
			t.Errorf("ConfigErrorf message mismatch: %s", err.Error())
		}
	})

	t.Run("AuthError", func(t *testing.T) {
		t.Parallel()
		err := AuthError("invalid token")
		if err.ExitCode() != ExitAuth {
			t.Errorf("AuthError should have ExitAuth code")
		}
	})

	t.Run("GitError", func(t *testing.T) {
		t.Parallel()
		err := GitError("not a git repository")
		if err.ExitCode() != ExitGit {
			t.Errorf("GitError should have ExitGit code")
		}
	})

	t.Run("WrapGitError", func(t *testing.T) {
		t.Parallel()
		underlying := fmt.Errorf("remote rejected")
		err := WrapGitError("push failed", underlying)
		if err.ExitCode() != ExitGit {
			t.Errorf("WrapGitError should have ExitGit code")
		}
		if !errors.Is(err, underlying) {
			t.Errorf("WrapGitError should wrap underlying error")
		}
	})

	t.Run("JiraError", func(t *testing.T) {
		t.Parallel()
		err := JiraError("issue not found")
		if err.ExitCode() != ExitJira {
			t.Errorf("JiraError should have ExitJira code")
		}
	})

	t.Run("WrapJiraError", func(t *testing.T) {
		t.Parallel()
		underlying := fmt.Errorf("404")
		err := WrapJiraError("issue not found", underlying)
		if err.ExitCode() != ExitJira {
			t.Errorf("WrapJiraError should have ExitJira code")
		}
	})

	t.Run("GitHubError", func(t *testing.T) {
		t.Parallel()
		err := GitHubError("rate limited")
		if err.ExitCode() != ExitGitHub {
			t.Errorf("GitHubError should have ExitGitHub code")
		}
	})

	t.Run("WrapGitHubError", func(t *testing.T) {
		t.Parallel()
		underlying := fmt.Errorf("403")
		err := WrapGitHubError("permission denied", underlying)
		if err.ExitCode() != ExitGitHub {
			t.Errorf("WrapGitHubError should have ExitGitHub code")
		}
	})
}

func TestExitCodeDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code int
		want string
	}{
		{ExitSuccess, "Success"},
		{ExitGeneral, "General error"},
		{ExitConfig, "Configuration error"},
		{ExitAuth, "Authentication error"},
		{ExitGit, "Git error"},
		{ExitJira, "Jira error"},
		{ExitGitHub, "GitHub error"},
		{ExitUserCancelled, "Operation cancelled"},
		{999, "Unknown error"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := ExitCodeDescription(tt.code); got != tt.want {
				t.Errorf("ExitCodeDescription(%d) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *ExitCodeError
		code int
	}{
		{"ErrConfig", ErrConfig, ExitConfig},
		{"ErrAuth", ErrAuth, ExitAuth},
		{"ErrGit", ErrGit, ExitGit},
		{"ErrJira", ErrJira, ExitJira},
		{"ErrGitHub", ErrGitHub, ExitGitHub},
		{"ErrUserCancelled", ErrUserCancelled, ExitUserCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err.ExitCode() != tt.code {
				t.Errorf("%s has wrong exit code: got %d, want %d", tt.name, tt.err.ExitCode(), tt.code)
			}
		})
	}
}
