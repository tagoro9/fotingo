package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfigEntry(t *testing.T) {
	fotingoConfig.Set("git.remote", "upstream")
	entry, err := getConfigEntry("git.remote")
	require.NoError(t, err)
	assert.Equal(t, "git.remote", entry.Key)
	assert.Equal(t, "upstream", entry.Value)

	_, err = getConfigEntry("jira.siteId")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Legacy Jira keys")
}

func TestCurrentTerminalController(t *testing.T) {
	controller := &mockTerminalController{}
	assert.Nil(t, currentTerminalController())

	err := withActiveTerminal(controller, func() error {
		assert.Same(t, controller, currentTerminalController())
		return nil
	})
	require.NoError(t, err)
	assert.Nil(t, currentTerminalController())
}
