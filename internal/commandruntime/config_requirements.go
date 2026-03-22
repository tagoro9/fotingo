package commandruntime

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/tagoro9/fotingo/internal/ui"
)

// ConfigRequirement describes one required config value and optional prompt metadata.
type ConfigRequirement struct {
	Key         string
	EnvVar      string
	Prompt      string
	Placeholder string
	Validate    func(string) (string, error)
}

// ConfigRequirementOptions provides callbacks required to resolve missing config values.
type ConfigRequirementOptions struct {
	GetValue  func(key string) string
	CanPrompt func() bool
	Prompt    func(requirement ConfigRequirement) (string, error)
	Persist   func(key string, value string) error
}

// EnsureConfigRequirements resolves and persists missing config values using the supplied options.
func EnsureConfigRequirements(options ConfigRequirementOptions, requirements ...ConfigRequirement) error {
	missing := make([]ConfigRequirement, 0, len(requirements))
	for _, requirement := range requirements {
		if strings.TrimSpace(options.GetValue(requirement.Key)) == "" {
			missing = append(missing, requirement)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	if !options.CanPrompt() {
		keys := make([]string, 0, len(missing))
		envHints := make([]string, 0, len(missing))
		for _, requirement := range missing {
			keys = append(keys, requirement.Key)
			if requirement.EnvVar != "" {
				envHints = append(envHints, requirement.EnvVar)
			}
		}

		return fmt.Errorf(
			"missing required config: %s. Set %s or run `fotingo login`",
			strings.Join(keys, ", "),
			strings.Join(envHints, ", "),
		)
	}

	for _, requirement := range missing {
		value, err := options.Prompt(requirement)
		if err != nil {
			return fmt.Errorf("missing required config %s: %w", requirement.Key, err)
		}

		if err := options.Persist(requirement.Key, value); err != nil {
			return fmt.Errorf("failed to persist config %s: %w", requirement.Key, err)
		}
	}

	return nil
}

// NormalizeHTTPSRootURL validates and normalizes root URLs requiring the https scheme.
func NormalizeHTTPSRootURL(raw string, label string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", label)
	}

	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid %s: %w", label, err)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid %s: host is required", label)
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid %s: scheme must be https", label)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("invalid %s: path is not allowed", label)
	}

	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return strings.TrimRight(parsed.String(), "/"), nil
}

// IsInputTerminal reports whether stdin points to an interactive terminal.
func IsInputTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// IsOutputTerminal reports whether stdout points to an interactive terminal.
func IsOutputTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// CanPromptForMissingConfig reports whether interactive config prompts are allowed.
func CanPromptForMissingConfig(jsonOutput bool, assumeYes bool, isInputTerminal func() bool) bool {
	if jsonOutput || assumeYes {
		return false
	}
	if isInputTerminal == nil {
		return false
	}
	return isInputTerminal()
}

// PromptForConfigRequirement asks the user for one missing config value.
func PromptForConfigRequirement(requirement ConfigRequirement) (string, error) {
	input := ui.NewInputProgram(
		ui.WithPrompt(requirement.Prompt),
		ui.WithPlaceholder(requirement.Placeholder),
		ui.WithValidation(func(value string) error {
			if requirement.Validate == nil {
				return nil
			}
			_, err := requirement.Validate(value)
			return err
		}),
	)

	value, cancelled, err := input.RunWithCancel()
	if err != nil {
		return "", err
	}
	if cancelled || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("input cancelled")
	}

	if requirement.Validate == nil {
		return strings.TrimSpace(value), nil
	}

	normalized, err := requirement.Validate(value)
	if err != nil {
		return "", err
	}
	return normalized, nil
}
