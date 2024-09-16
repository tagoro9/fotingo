package commands

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
)

// TestErrorOutput_JSON tests that error output correctly maps error types.
// This is not covered by golden tests because error types come from the errors package.
func TestErrorOutput_JSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantCode int
		wantType string
	}{
		{
			name:     "git error",
			err:      fterrors.GitError("not a repository"),
			wantCode: fterrors.ExitGit,
			wantType: "Git error",
		},
		{
			name:     "jira error",
			err:      fterrors.JiraError("issue not found"),
			wantCode: fterrors.ExitJira,
			wantType: "Jira error",
		},
		{
			name:     "github error",
			err:      fterrors.GitHubError("rate limited"),
			wantCode: fterrors.ExitGitHub,
			wantType: "GitHub error",
		},
		{
			name:     "config error",
			err:      fterrors.ConfigError("missing config"),
			wantCode: fterrors.ExitConfig,
			wantType: "Configuration error",
		},
		{
			name:     "auth error",
			err:      fterrors.AuthError("invalid token"),
			wantCode: fterrors.ExitAuth,
			wantType: "Authentication error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code := fterrors.GetExitCode(tt.err)
			output := ErrorOutput{
				Error: tt.err.Error(),
				Code:  code,
				Type:  fterrors.ExitCodeDescription(code),
			}

			data, err := json.Marshal(output)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			var decoded ErrorOutput
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if decoded.Code != tt.wantCode {
				t.Errorf("Code = %d, want %d", decoded.Code, tt.wantCode)
			}
			if decoded.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", decoded.Type, tt.wantType)
			}
		})
	}
}

func TestShouldOutputJSON(t *testing.T) {
	originalJSON := Global.JSON
	defer func() { Global.JSON = originalJSON }()

	Global.JSON = false
	if ShouldOutputJSON() {
		t.Error("ShouldOutputJSON should return false when Global.JSON is false")
	}

	Global.JSON = true
	if !ShouldOutputJSON() {
		t.Error("ShouldOutputJSON should return true when Global.JSON is true")
	}
}

func TestShouldSuppressOutput(t *testing.T) {
	originalJSON := Global.JSON
	originalQuiet := Global.Quiet
	originalVerbose := Global.Verbose
	originalDebug := Global.Debug
	defer func() {
		Global.JSON = originalJSON
		Global.Quiet = originalQuiet
		Global.Verbose = originalVerbose
		Global.Debug = originalDebug
	}()

	tests := []struct {
		name   string
		json   bool
		quiet  bool
		expect bool
	}{
		{"neither", false, false, false},
		{"json only", true, false, true},
		{"quiet only", false, true, true},
		{"both", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Global.JSON = tt.json
			Global.Quiet = tt.quiet
			if got := ShouldSuppressOutput(); got != tt.expect {
				t.Errorf("ShouldSuppressOutput() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestShouldOutputVerboseAndDebug(t *testing.T) {
	originalJSON := Global.JSON
	originalQuiet := Global.Quiet
	originalVerbose := Global.Verbose
	originalDebug := Global.Debug
	defer func() {
		Global.JSON = originalJSON
		Global.Quiet = originalQuiet
		Global.Verbose = originalVerbose
		Global.Debug = originalDebug
	}()

	// default
	Global.JSON = false
	Global.Quiet = false
	Global.Verbose = false
	Global.Debug = false
	if ShouldOutputVerbose() {
		t.Error("ShouldOutputVerbose should be false by default")
	}
	if ShouldOutputDebug() {
		t.Error("ShouldOutputDebug should be false by default")
	}

	// verbose only
	Global.Verbose = true
	if !ShouldOutputVerbose() {
		t.Error("ShouldOutputVerbose should be true when --verbose is set")
	}
	if ShouldOutputDebug() {
		t.Error("ShouldOutputDebug should be false when only --verbose is set")
	}

	// debug implies verbose
	Global.Verbose = false
	Global.Debug = true
	if !ShouldOutputVerbose() {
		t.Error("ShouldOutputVerbose should be true when --debug is set")
	}
	if !ShouldOutputDebug() {
		t.Error("ShouldOutputDebug should be true when --debug is set")
	}

	// quiet suppresses both
	Global.Quiet = true
	if ShouldOutputVerbose() {
		t.Error("ShouldOutputVerbose should be false when --quiet is set")
	}
	if ShouldOutputDebug() {
		t.Error("ShouldOutputDebug should be false when --quiet is set")
	}

	// json suppresses both
	Global.Quiet = false
	Global.JSON = true
	if ShouldOutputVerbose() {
		t.Error("ShouldOutputVerbose should be false when --json is set")
	}
	if ShouldOutputDebug() {
		t.Error("ShouldOutputDebug should be false when --json is set")
	}
}

func TestIsShellCompletionRequest(t *testing.T) {
	origArgsFn := completionArgsFn
	defer func() { completionArgsFn = origArgsFn }()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "normal command",
			args: []string{"fotingo", "open", "pr"},
			want: false,
		},
		{
			name: "completion command",
			args: []string{"fotingo", "completion", "zsh"},
			want: true,
		},
		{
			name: "hidden complete command",
			args: []string{"fotingo", "__complete", "review", "--"},
			want: true,
		},
		{
			name: "hidden complete no desc command",
			args: []string{"fotingo", "__completeNoDesc", "review", "--"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completionArgsFn = func() []string { return tt.args }
			if got := IsShellCompletionRequest(); got != tt.want {
				t.Errorf("IsShellCompletionRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsShellCompletionCommand(t *testing.T) {
	origArgsFn := completionArgsFn
	defer func() { completionArgsFn = origArgsFn }()

	completionArgsFn = func() []string { return []string{"fotingo", "open", "pr"} }
	openCmd := &cobra.Command{Use: "open"}
	if isShellCompletionCommand(openCmd) {
		t.Error("isShellCompletionCommand(open) = true, want false")
	}

	hiddenCmd := &cobra.Command{Use: "__complete"}
	if !isShellCompletionCommand(hiddenCmd) {
		t.Error("isShellCompletionCommand(__complete) = false, want true")
	}

	completionArgsFn = func() []string { return []string{"fotingo", "__complete", "open", ""} }
	if !isShellCompletionCommand(openCmd) {
		t.Error("isShellCompletionCommand(open with __complete args) = false, want true")
	}
}
