package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecuteWithExitCode_HelpSuccess(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	origArgsFn := completionArgsFn
	completionArgsFn = func() []string { return []string{"fotingo"} }
	defer func() { completionArgsFn = origArgsFn }()

	setInvocationShellCompletion(false)
	Global.JSON = false
	Global.Quiet = false

	buf := new(bytes.Buffer)
	Fotingo.SetOut(buf)
	Fotingo.SetErr(buf)
	Fotingo.SetArgs([]string{"--help"})

	code := ExecuteWithExitCode()
	assert.Equal(t, 0, code)
}

func TestExecuteWithExitCode_InvalidSubcommand(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	origArgsFn := completionArgsFn
	completionArgsFn = func() []string { return []string{"fotingo"} }
	defer func() { completionArgsFn = origArgsFn }()

	setInvocationShellCompletion(false)
	Global.JSON = false
	Global.Quiet = false

	buf := new(bytes.Buffer)
	Fotingo.SetOut(buf)
	Fotingo.SetErr(buf)
	Fotingo.SetArgs([]string{"nonexistent-command"})

	code := ExecuteWithExitCode()
	assert.NotEqual(t, 0, code)
}

func TestExecute_HelpSuccess(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	origArgsFn := completionArgsFn
	completionArgsFn = func() []string { return []string{"fotingo"} }
	defer func() { completionArgsFn = origArgsFn }()

	setInvocationShellCompletion(false)
	Global.JSON = false
	Global.Quiet = false

	buf := new(bytes.Buffer)
	Fotingo.SetOut(buf)
	Fotingo.SetErr(buf)
	Fotingo.SetArgs([]string{"--help"})

	output := captureStdout(t, func() {
		Execute()
	})
	assert.Contains(t, output, "🎉 Done in ")
}
