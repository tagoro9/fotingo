package commands

import (
	"bytes"
	"testing"
)

func TestCompletionCommand(t *testing.T) {
	// Note: Completion commands write directly to os.Stdout via cmd.Root().GenBashCompletion()
	// which bypasses cmd.SetOut(). The actual output is verified in integration tests.
	// Here we just verify the command structure and that it doesn't error.

	tests := []struct {
		name    string
		shell   string
		wantErr bool
	}{
		{
			name:    "bash completion",
			shell:   "bash",
			wantErr: false,
		},
		{
			name:    "zsh completion",
			shell:   "zsh",
			wantErr: false,
		},
		{
			name:    "fish completion",
			shell:   "fish",
			wantErr: false,
		},
		{
			name:    "powershell completion",
			shell:   "powershell",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily capture the output since it goes to os.Stdout,
			// but we can verify the command executes without error
			resetCommandStateForExecute(Fotingo)
			buf := new(bytes.Buffer)
			Fotingo.SetOut(buf)
			Fotingo.SetErr(buf)
			Fotingo.SetArgs([]string{"completion", tt.shell})

			err := Fotingo.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompletionCommand_InvalidShell(t *testing.T) {
	cmd := Fotingo
	resetCommandStateForExecute(cmd)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"completion", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for invalid shell, got nil")
	}
}

func TestCompletionCommand_NoArgs(t *testing.T) {
	cmd := Fotingo
	resetCommandStateForExecute(cmd)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"completion"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for missing shell argument, got nil")
	}
}

func TestCompletionCommand_ValidArgs(t *testing.T) {
	t.Parallel()

	// Verify ValidArgs is properly configured
	validShells := []string{"bash", "zsh", "fish", "powershell"}

	if len(completionCmd.ValidArgs) != len(validShells) {
		t.Errorf("ValidArgs length = %d, want %d", len(completionCmd.ValidArgs), len(validShells))
	}

	for _, shell := range validShells {
		found := false
		for _, validArg := range completionCmd.ValidArgs {
			if validArg == shell {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Shell %q not in ValidArgs", shell)
		}
	}
}
