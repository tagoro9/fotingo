package review

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeManagedSections_DefaultsToAll(t *testing.T) {
	t.Parallel()

	sections, err := NormalizeManagedSections(nil)
	require.NoError(t, err)
	assert.Equal(t, ManagedSections(), sections)
}

func TestNormalizeManagedSections_DedupesAndNormalizes(t *testing.T) {
	t.Parallel()

	sections, err := NormalizeManagedSections([]string{"Changes", "summary", "changes"})
	require.NoError(t, err)
	assert.Equal(t, []string{ManagedSectionChanges, ManagedSectionSummary}, sections)
}

func TestNormalizeManagedSections_RejectsUnknownSection(t *testing.T) {
	t.Parallel()

	_, err := NormalizeManagedSections([]string{"custom"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported review sync section")
}

func TestManagedSectionMarkers(t *testing.T) {
	t.Parallel()

	start, end := ManagedSectionMarkers(ManagedSectionChanges)
	assert.Equal(t, "<!-- fotingo:start changes -->", start)
	assert.Equal(t, "<!-- fotingo:end changes -->", end)
}

func TestExtractManagedSectionContent(t *testing.T) {
	t.Parallel()

	body := "**Summary**\n\n<!-- fotingo:start summary -->\nText\n<!-- fotingo:end summary -->"
	content, err := ExtractManagedSectionContent(body, ManagedSectionSummary)
	require.NoError(t, err)
	assert.Equal(t, "\nText\n", content)
}

func TestReplaceManagedSectionContent(t *testing.T) {
	t.Parallel()

	body := "prefix\n<!-- fotingo:start changes -->\nold\n<!-- fotingo:end changes -->\nsuffix"
	updated, err := ReplaceManagedSectionContent(body, ManagedSectionChanges, "\nnew\n")
	require.NoError(t, err)
	assert.Equal(t, "prefix\n<!-- fotingo:start changes -->\nnew\n<!-- fotingo:end changes -->\nsuffix", updated)
}

func TestReplaceManagedSectionContent_FailsWhenMarkersMissing(t *testing.T) {
	t.Parallel()

	_, err := ReplaceManagedSectionContent("no markers", ManagedSectionDescription, "replacement")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `missing fotingo markers for section "description"`)
}
