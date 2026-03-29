package review

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/template"
)

func TestRenderTemplate_FillsEmptyManagedMarkerRanges(t *testing.T) {
	t.Parallel()

	content := "" +
		"**Changes**\n\n" +
		"<!-- fotingo:start changes -->\n" +
		"<!-- fotingo:end changes -->\n"
	data := map[string]string{
		template.PlaceholderChanges: "* feat: refresh changes (+2/-1)",
	}

	rendered, usedLegacy, err := RenderTemplate(content, data)

	require.NoError(t, err)
	assert.False(t, usedLegacy)
	assert.Contains(t, rendered, "<!-- fotingo:start changes -->\n* feat: refresh changes (+2/-1)\n<!-- fotingo:end changes -->")
}

func TestRenderTemplate_KeepsLegacyManagedPlaceholdersWorking(t *testing.T) {
	t.Parallel()

	content := "Before\n{changes}\nAfter"
	data := map[string]string{
		template.PlaceholderChanges: "* feat: refresh changes (+2/-1)",
	}

	rendered, usedLegacy, err := RenderTemplate(content, data)

	require.NoError(t, err)
	assert.True(t, usedLegacy)
	assert.Equal(t, "Before\n* feat: refresh changes (+2/-1)\nAfter", rendered)
}

func TestRenderTemplate_DoesNotOverwriteMixedMarkerLegacyContent(t *testing.T) {
	t.Parallel()

	content := "" +
		"<!-- fotingo:start changes -->\n" +
		"## Delta\n\n" +
		"{changes}\n" +
		"<!-- fotingo:end changes -->\n"
	data := map[string]string{
		template.PlaceholderChanges: "* feat: preserve markers (+3/-1)",
	}

	rendered, usedLegacy, err := RenderTemplate(content, data)

	require.NoError(t, err)
	assert.True(t, usedLegacy)
	assert.Contains(t, rendered, "## Delta")
	assert.Contains(t, rendered, "* feat: preserve markers (+3/-1)")
}
