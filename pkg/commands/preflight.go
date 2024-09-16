package commands

import (
	"strings"

	"github.com/tagoro9/fotingo/internal/commandruntime"
	ftconfig "github.com/tagoro9/fotingo/internal/config"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
)

type configRequirement = commandruntime.ConfigRequirement

var (
	promptForConfigValue = defaultPromptForConfigValue
	isInputTerminalFn    = commandruntime.IsInputTerminal
	writeConfigFn        = func(key string, value string) error {
		return ftconfig.PersistConfigValue(fotingoConfig, key, value)
	}
)

func ensureJiraRootConfigured() error {
	return ensureConfigRequirements(configRequirement{
		Key:         "jira.root",
		EnvVar:      "FOTINGO_JIRA_ROOT",
		Prompt:      "Jira site URL",
		Placeholder: "https://yourcompany.atlassian.net",
		Validate:    normalizeJiraRootConfigValue,
	})
}

func ensureConfigRequirements(requirements ...configRequirement) error {
	err := commandruntime.EnsureConfigRequirements(commandruntime.ConfigRequirementOptions{
		GetValue: func(key string) string {
			return strings.TrimSpace(fotingoConfig.GetString(key))
		},
		CanPrompt: canPromptForMissingConfig,
		Prompt:    promptForConfigValue,
		Persist:   writeConfigFn,
	}, requirements...)
	if err != nil {
		return fterrors.ConfigErrorf("%v", err)
	}
	return nil
}

func canPromptForMissingConfig() bool {
	return commandruntime.CanPromptForMissingConfig(Global.JSON, Global.Yes, isInputTerminalFn)
}

func defaultPromptForConfigValue(requirement configRequirement) (string, error) {
	return commandruntime.PromptForConfigRequirement(requirement)
}

func normalizeJiraRootConfigValue(raw string) (string, error) {
	return commandruntime.NormalizeHTTPSRootURL(raw, "jira site URL")
}
