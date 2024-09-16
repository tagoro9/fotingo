package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Root command structure
// ---------------------------------------------------------------------------

func TestRootCommand_HasExpectedSubcommands(t *testing.T) {

	expectedNames := []string{
		"ai", "cache", "completion", "config", "inspect", "login", "open", "release", "review", "start", "version",
	}

	var commandNames []string
	for _, cmd := range Fotingo.Commands() {
		commandNames = append(commandNames, cmd.Name())
	}

	for _, expected := range expectedNames {
		assert.Contains(t, commandNames, expected, "root command should have subcommand %q", expected)
	}
}

func TestRootCommand_GlobalFlags(t *testing.T) {

	flags := Fotingo.PersistentFlags()

	tests := []struct {
		name     string
		flagName string
	}{
		{"json flag", "json"},
		{"yes flag", "yes"},
		{"quiet flag", "quiet"},
		{"verbose flag", "verbose"},
		{"debug flag", "debug"},
		{"no-color flag", "no-color"},
		{"branch flag", "branch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := flags.Lookup(tt.flagName)
			assert.NotNil(t, f, "global flag %q should exist", tt.flagName)
		})
	}
}

func TestRootCommand_SilenceSettings(t *testing.T) {

	assert.True(t, Fotingo.SilenceUsage, "SilenceUsage should be true")
	assert.True(t, Fotingo.SilenceErrors, "SilenceErrors should be true")
}

func TestRootCommand_UseAndDescription(t *testing.T) {

	assert.Equal(t, "fotingo", Fotingo.Use)
	assert.NotEmpty(t, Fotingo.Short)
	assert.NotEmpty(t, Fotingo.Long)
	assert.Contains(t, Fotingo.Long, "EXIT CODES")
	assert.Contains(t, Fotingo.Long, "GLOBAL FLAGS")
}

// ---------------------------------------------------------------------------
// Version command
// ---------------------------------------------------------------------------

func TestVersionVar(t *testing.T) {
	assert.Equal(t, "dev", Version)
	assert.Equal(t, "unknown", GitCommit)
	assert.Equal(t, "unknown", BuildTime)
	assert.Equal(t, "unknown/unknown", Platform)
}

// ---------------------------------------------------------------------------
// Open command structure
// ---------------------------------------------------------------------------

func TestOpenCommand_ValidArgs(t *testing.T) {

	assert.Equal(t, []string{"branch", "issue", "pr", "repo"}, openCmd.ValidArgs)
}

func TestOpenCommand_UseAndDescription(t *testing.T) {

	assert.Equal(t, "open [what]", openCmd.Use)
	assert.NotEmpty(t, openCmd.Short)
	assert.NotEmpty(t, openCmd.Long)
	assert.Contains(t, openCmd.Long, "branch")
	assert.Contains(t, openCmd.Long, "issue")
	assert.Contains(t, openCmd.Long, "pr")
	assert.Contains(t, openCmd.Long, "repo")
}

// ---------------------------------------------------------------------------
// Release command structure
// ---------------------------------------------------------------------------

func TestReleaseCommand_Flags(t *testing.T) {

	flags := releaseCmd.Flags()

	issuesFlag := flags.Lookup("issues")
	assert.NotNil(t, issuesFlag, "issues flag should exist")
	assert.Equal(t, "i", issuesFlag.Shorthand)

	simpleFlag := flags.Lookup("simple")
	assert.NotNil(t, simpleFlag, "simple flag should exist")
	assert.Equal(t, "s", simpleFlag.Shorthand)

	noVCSFlag := flags.Lookup("no-vcs-release")
	assert.NotNil(t, noVCSFlag, "no-vcs-release flag should exist")
	assert.Equal(t, "n", noVCSFlag.Shorthand)
}

func TestReleaseCommand_UseAndDescription(t *testing.T) {

	assert.Equal(t, "release <name>", releaseCmd.Use)
	assert.NotEmpty(t, releaseCmd.Short)
	assert.NotEmpty(t, releaseCmd.Long)
}

// ---------------------------------------------------------------------------
// Login command structure
// ---------------------------------------------------------------------------

func TestLoginCommand_UseAndDescription(t *testing.T) {

	assert.Equal(t, "login", loginCmd.Use)
	assert.NotEmpty(t, loginCmd.Short)
}

// ---------------------------------------------------------------------------
// Completion command structure
// ---------------------------------------------------------------------------

func TestCompletionCommand_DisablesFlagsInUseLine(t *testing.T) {
	assert.True(t, completionCmd.DisableFlagsInUseLine)
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

func TestUpdateStatus(t *testing.T) {

	msg := updateStatus("processing...")
	sMsg, ok := msg.(statusMsg)
	assert.True(t, ok, "updateStatus should return statusMsg")
	assert.Equal(t, statusMsg("processing..."), sMsg)
}

func TestFinishProcess(t *testing.T) {

	msg := finishProcess()
	dMsg, ok := msg.(doneMsg)
	assert.True(t, ok, "finishProcess should return doneMsg")
	assert.Equal(t, doneMsg(true), dMsg)
}
