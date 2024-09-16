package commandruntime

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureConfigRequirementsNoMissing(t *testing.T) {
	err := EnsureConfigRequirements(ConfigRequirementOptions{
		GetValue:  func(key string) string { return "set" },
		CanPrompt: func() bool { return false },
		Prompt: func(requirement ConfigRequirement) (string, error) {
			return "", nil
		},
		Persist: func(key string, value string) error { return nil },
	}, ConfigRequirement{Key: "jira.root"})

	require.NoError(t, err)
}

func TestEnsureConfigRequirementsMissingWithoutPrompt(t *testing.T) {
	err := EnsureConfigRequirements(ConfigRequirementOptions{
		GetValue:  func(key string) string { return "" },
		CanPrompt: func() bool { return false },
		Prompt: func(requirement ConfigRequirement) (string, error) {
			return "", nil
		},
		Persist: func(key string, value string) error { return nil },
	},
		ConfigRequirement{Key: "jira.root", EnvVar: "FOTINGO_JIRA_ROOT"},
		ConfigRequirement{Key: "jira.user", EnvVar: "FOTINGO_JIRA_USER"},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required config")
	assert.Contains(t, err.Error(), "jira.root")
	assert.Contains(t, err.Error(), "FOTINGO_JIRA_ROOT")
}

func TestEnsureConfigRequirementsPromptAndPersist(t *testing.T) {
	written := map[string]string{}
	err := EnsureConfigRequirements(ConfigRequirementOptions{
		GetValue: func(key string) string {
			if key == "jira.user" {
				return "already-set"
			}
			return ""
		},
		CanPrompt: func() bool { return true },
		Prompt: func(requirement ConfigRequirement) (string, error) {
			return "https://example.atlassian.net", nil
		},
		Persist: func(key string, value string) error {
			written[key] = value
			return nil
		},
	},
		ConfigRequirement{Key: "jira.root"},
		ConfigRequirement{Key: "jira.user"},
	)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"jira.root": "https://example.atlassian.net",
	}, written)
}

func TestEnsureConfigRequirementsReturnsPromptError(t *testing.T) {
	err := EnsureConfigRequirements(ConfigRequirementOptions{
		GetValue:  func(key string) string { return "" },
		CanPrompt: func() bool { return true },
		Prompt: func(requirement ConfigRequirement) (string, error) {
			return "", errors.New("cancelled")
		},
		Persist: func(key string, value string) error { return nil },
	}, ConfigRequirement{Key: "jira.root"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required config jira.root")
}

func TestEnsureConfigRequirementsReturnsPersistError(t *testing.T) {
	err := EnsureConfigRequirements(ConfigRequirementOptions{
		GetValue:  func(key string) string { return "" },
		CanPrompt: func() bool { return true },
		Prompt: func(requirement ConfigRequirement) (string, error) {
			return "value", nil
		},
		Persist: func(key string, value string) error { return errors.New("write failed") },
	}, ConfigRequirement{Key: "jira.root"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to persist config jira.root")
}

func TestNormalizeHTTPSRootURL(t *testing.T) {
	normalized, err := NormalizeHTTPSRootURL("mycompany.atlassian.net", "jira site URL")
	require.NoError(t, err)
	assert.Equal(t, "https://mycompany.atlassian.net", normalized)

	_, err = NormalizeHTTPSRootURL("http://mycompany.atlassian.net", "jira site URL")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scheme must be https")

	_, err = NormalizeHTTPSRootURL("https://mycompany.atlassian.net/path", "jira site URL")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path is not allowed")
}
