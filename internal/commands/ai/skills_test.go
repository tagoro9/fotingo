package ai

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func TestRenderSkill(t *testing.T) {
	t.Parallel()

	providers := []Provider{ProviderCodex, ProviderCursor, ProviderClaudeCode}
	for _, provider := range providers {
		provider := provider
		t.Run(string(provider), func(t *testing.T) {
			t.Parallel()

			content, err := RenderSkill(provider)
			require.NoError(t, err)
			assert.Contains(t, content, "name: fotingo")
			assert.Contains(t, content, "description:")
			assert.Contains(t, content, "Fotingo Workflow Skill")
			assert.Contains(t, content, "fotingo inspect --json")
			assert.Contains(t, content, "fotingo review -y")
			assert.Contains(t, content, "fotingo start PROJ-123 -y")
		})
	}
}

func TestRenderSkill_UnknownProvider(t *testing.T) {
	t.Parallel()

	_, err := RenderSkill(Provider("other"))
	require.Error(t, err)
}

func TestSkillMetadataVersion_IsSemver(t *testing.T) {
	t.Parallel()

	assert.True(
		t,
		semverPattern.MatchString(skillMetadataVersion),
		"skillMetadataVersion must use semantic versioning (x.y.z)",
	)
}

func TestRenderSkill_Golden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		provider   Provider
		goldenFile string
	}{
		{
			name:       "codex",
			provider:   ProviderCodex,
			goldenFile: "skill_codex.golden",
		},
		{
			name:       "cursor",
			provider:   ProviderCursor,
			goldenFile: "skill_cursor.golden",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rendered, err := RenderSkill(tt.provider)
			require.NoError(t, err)

			goldenPath := filepath.Join("testdata", tt.goldenFile)
			expected, err := os.ReadFile(goldenPath)
			require.NoError(t, err)
			assert.Equal(t, string(expected), rendered)
		})
	}
}
