package ai

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseScope(t *testing.T) {
	t.Parallel()

	scope, err := ParseScope("")
	require.NoError(t, err)
	assert.Equal(t, ScopeProject, scope)

	scope, err = ParseScope("user")
	require.NoError(t, err)
	assert.Equal(t, ScopeUser, scope)

	_, err = ParseScope("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported scope")
}

func TestParseProviders(t *testing.T) {
	t.Parallel()

	providers, err := ParseProviders(nil, true)
	require.NoError(t, err)
	assert.Equal(t, []Provider{ProviderCodex, ProviderCursor, ProviderClaudeCode}, providers)

	providers, err = ParseProviders([]string{"cursor", "codex", "cursor"}, false)
	require.NoError(t, err)
	assert.Equal(t, []Provider{ProviderCodex, ProviderCursor}, providers)

	_, err = ParseProviders([]string{"unknown"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestPlanInstallTargets_ProjectScope(t *testing.T) {
	t.Parallel()

	targets, err := PlanInstallTargets(
		[]Provider{ProviderCodex, ProviderCursor},
		ScopeProject,
		"/repo",
		"/home/me",
		"",
	)
	require.NoError(t, err)
	require.Len(t, targets, 2)
	assert.Equal(t, "/repo/.codex/skills/fotingo/SKILL.md", filepath.ToSlash(targets[0].SkillPath))
	assert.Equal(t, "/repo/.cursor/skills/fotingo/SKILL.md", filepath.ToSlash(targets[1].SkillPath))
}

func TestPlanInstallTargets_UserScope_CodexHomeOverride(t *testing.T) {
	t.Parallel()

	targets, err := PlanInstallTargets(
		[]Provider{ProviderCodex, ProviderClaudeCode},
		ScopeUser,
		"/repo",
		"/home/me",
		"/custom/codex",
	)
	require.NoError(t, err)
	require.Len(t, targets, 2)
	assert.Equal(t, "/custom/codex/skills/fotingo/SKILL.md", filepath.ToSlash(targets[0].SkillPath))
	assert.Equal(t, "/home/me/.claude/skills/fotingo/SKILL.md", filepath.ToSlash(targets[1].SkillPath))
}

func TestFindProjectRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))

	assert.Equal(t, root, FindProjectRoot(nested))
}
