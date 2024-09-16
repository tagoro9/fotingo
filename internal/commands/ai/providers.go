package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Provider identifies an AI provider supported by fotingo setup.
type Provider string

const (
	ProviderCodex      Provider = "codex"
	ProviderCursor     Provider = "cursor"
	ProviderClaudeCode Provider = "claude-code"
)

// Scope determines where generated skills are installed.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeUser    Scope = "user"
)

const (
	defaultSkillPackageName = "fotingo"
	codexHomeEnvKey         = "CODEX_HOME"
)

// ProviderSpec defines setup metadata for one provider.
type ProviderSpec struct {
	ID               Provider
	Label            string
	ProjectConfigDir string
	UserConfigDir    string
	UserRootEnvKey   string
}

// InstallTarget defines one planned filesystem target.
type InstallTarget struct {
	Provider     Provider
	Scope        Scope
	ProviderRoot string
	SkillDir     string
	SkillPath    string
}

var providerSpecs = []ProviderSpec{
	{
		ID:               ProviderCodex,
		Label:            "Codex",
		ProjectConfigDir: ".codex",
		UserConfigDir:    ".codex",
		UserRootEnvKey:   codexHomeEnvKey,
	},
	{
		ID:               ProviderCursor,
		Label:            "Cursor",
		ProjectConfigDir: ".cursor",
		UserConfigDir:    ".cursor",
	},
	{
		ID:               ProviderClaudeCode,
		Label:            "Claude Code",
		ProjectConfigDir: ".claude",
		UserConfigDir:    ".claude",
	},
}

// SupportedProviders returns providers in stable display order.
func SupportedProviders() []Provider {
	providers := make([]Provider, 0, len(providerSpecs))
	for _, spec := range providerSpecs {
		providers = append(providers, spec.ID)
	}
	return providers
}

// SupportedProviderValues returns supported provider string values.
func SupportedProviderValues() []string {
	values := make([]string, 0, len(providerSpecs))
	for _, spec := range providerSpecs {
		values = append(values, string(spec.ID))
	}
	return values
}

// ProviderLabel returns the display label for a provider.
func ProviderLabel(provider Provider) string {
	spec, ok := providerSpec(provider)
	if !ok {
		return string(provider)
	}
	return spec.Label
}

// ScopeValues returns supported scope values.
func ScopeValues() []string {
	return []string{string(ScopeProject), string(ScopeUser)}
}

// ParseScope validates and normalizes scope input.
func ParseScope(raw string) (Scope, error) {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	if normalized == "" {
		return ScopeProject, nil
	}

	switch Scope(normalized) {
	case ScopeProject, ScopeUser:
		return Scope(normalized), nil
	default:
		return "", fmt.Errorf(
			"unsupported scope %q (valid: %s)",
			raw,
			strings.Join(ScopeValues(), ", "),
		)
	}
}

// ParseProviders validates and normalizes provider inputs.
func ParseProviders(requested []string, all bool) ([]Provider, error) {
	if all {
		return SupportedProviders(), nil
	}

	if len(requested) == 0 {
		return nil, nil
	}

	seen := make(map[Provider]bool)
	invalid := make([]string, 0)
	for _, item := range requested {
		raw := strings.TrimSpace(strings.ToLower(item))
		if raw == "" {
			continue
		}

		provider, ok := parseProvider(raw)
		if !ok {
			invalid = append(invalid, raw)
			continue
		}
		seen[provider] = true
	}

	if len(invalid) > 0 {
		sort.Strings(invalid)
		invalid = dedupeStrings(invalid)
		return nil, fmt.Errorf(
			"unsupported provider(s): %s (valid: %s)",
			strings.Join(invalid, ", "),
			strings.Join(SupportedProviderValues(), ", "),
		)
	}

	resolved := make([]Provider, 0, len(seen))
	for _, provider := range SupportedProviders() {
		if seen[provider] {
			resolved = append(resolved, provider)
		}
	}
	return resolved, nil
}

// PlanInstallTargets resolves output paths for each provider and scope.
func PlanInstallTargets(
	providers []Provider,
	scope Scope,
	projectRoot string,
	userHome string,
	codexHome string,
) ([]InstallTarget, error) {
	targets := make([]InstallTarget, 0, len(providers))
	for _, provider := range providers {
		spec, ok := providerSpec(provider)
		if !ok {
			return nil, fmt.Errorf("unsupported provider %q", provider)
		}

		providerRoot, err := resolveProviderRoot(spec, scope, projectRoot, userHome, codexHome)
		if err != nil {
			return nil, err
		}

		skillDir := filepath.Join(providerRoot, "skills", defaultSkillPackageName)
		targets = append(targets, InstallTarget{
			Provider:     provider,
			Scope:        scope,
			ProviderRoot: providerRoot,
			SkillDir:     skillDir,
			SkillPath:    filepath.Join(skillDir, "SKILL.md"),
		})
	}
	return targets, nil
}

// FindProjectRoot returns the nearest ancestor containing .git, or cwd if none.
func FindProjectRoot(cwd string) string {
	current := strings.TrimSpace(cwd)
	if current == "" {
		return ""
	}

	for {
		gitPath := filepath.Join(current, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return current
		}

		parent := filepath.Dir(current)
		if parent == current {
			return cwd
		}
		current = parent
	}
}

func providerSpec(provider Provider) (ProviderSpec, bool) {
	for _, spec := range providerSpecs {
		if spec.ID == provider {
			return spec, true
		}
	}
	return ProviderSpec{}, false
}

func parseProvider(raw string) (Provider, bool) {
	provider := Provider(raw)
	_, ok := providerSpec(provider)
	return provider, ok
}

func resolveProviderRoot(
	spec ProviderSpec,
	scope Scope,
	projectRoot string,
	userHome string,
	codexHome string,
) (string, error) {
	switch scope {
	case ScopeProject:
		if strings.TrimSpace(projectRoot) == "" {
			return "", fmt.Errorf("project scope requires a valid project root")
		}
		return filepath.Clean(filepath.Join(projectRoot, spec.ProjectConfigDir)), nil
	case ScopeUser:
		if spec.UserRootEnvKey == codexHomeEnvKey && strings.TrimSpace(codexHome) != "" {
			return filepath.Clean(codexHome), nil
		}
		if strings.TrimSpace(userHome) == "" {
			return "", fmt.Errorf("user scope requires a valid user home directory")
		}
		return filepath.Clean(filepath.Join(userHome, spec.UserConfigDir)), nil
	default:
		return "", fmt.Errorf("unsupported scope %q", scope)
	}
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool)
	deduped := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		deduped = append(deduped, value)
	}
	return deduped
}
