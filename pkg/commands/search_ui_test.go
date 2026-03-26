package commands

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	internaltestutil "github.com/tagoro9/fotingo/internal/testutil"
)

func TestSearchUIModelView_ShowsProgress(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	model := newSearchUIModel(searchDomainReviewers, "Esau")
	updated, _ := model.Update(searchProgressMsg("Fetching GitHub organization members for acme"))

	view := internaltestutil.ViewString(updated.(searchUIModel).View())

	assert.Contains(t, view, `Searching reviewers for "Esau"`)
	assert.Contains(t, view, "Searching reviewers")
	assert.Contains(t, view, "Fetching GitHub organization members for acme")
}

func TestSearchUIModelView_RendersFinalResultsWithoutControlSequences(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	model := newSearchUIModel(searchDomainReviewers, "Esau")
	updated, _ := model.Update(searchCompletedMsg{
		results: []reviewMatchOption{
			{
				Resolved: "Toolo",
				Label:    "Toolo",
				Detail:   "Esau Suarez",
				Kind:     reviewMatchKindUser,
			},
		},
	})

	finalModel := updated.(searchUIModel)
	view := finalModel.View()
	rendered := internaltestutil.ViewString(view)

	assert.Contains(t, rendered, `Top reviewers matches for "Esau":`)
	assert.Contains(t, rendered, "1. Toolo (user)")
	assert.Contains(t, rendered, "resolved: Toolo")
	assert.Contains(t, rendered, "detail: Esau Suarez")
	assert.NotContains(t, rendered, "────────────────")
	assert.NotContains(t, view.Content, "\x1b[?")
	assert.NotContains(t, view.Content, "\x1b[2J")
}

func TestSearchUIModelView_RendersEmptyState(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	model := newSearchUIModel(searchDomainLabels, "missing")
	updated, _ := model.Update(searchCompletedMsg{results: []reviewMatchOption{}})

	rendered := internaltestutil.ViewString(updated.(searchUIModel).View())

	assert.Contains(t, rendered, `No labels matches found for "missing".`)
}

func TestSearchMetadataCommand_UsesInteractiveUIWhenAvailable(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()

	origShould := shouldUseInteractiveSearchUIFn
	origRun := runInteractiveSearchMetadataCommandFn
	defer func() {
		shouldUseInteractiveSearchUIFn = origShould
		runInteractiveSearchMetadataCommandFn = origRun
	}()

	shouldUseInteractiveSearchUIFn = func() bool { return true }

	called := false
	runInteractiveSearchMetadataCommandFn = func(writer io.Writer, domain searchDomain, args []string) error {
		called = true
		assert.Equal(t, searchDomainReviewers, domain)
		assert.Equal(t, []string{"Esau"}, args)
		return nil
	}

	cmd := newSearchMetadataCommand(
		searchDomainReviewers,
		"Search review reviewers non-interactively",
		"Search review reviewers using the same fuzzy matching logic as `fotingo review`.",
		`fotingo search reviewers ali`,
	)
	cmd.SetOut(io.Discard)

	err := cmd.RunE(cmd, []string{"Esau"})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestRenderSearchResultLines_PreservesReadableTextLayout(t *testing.T) {
	lines := renderSearchResultLines(searchDomainReviewers, "Esau", []reviewMatchOption{
		{
			Resolved: "Toolo",
			Label:    "Toolo",
			Detail:   "Esau Suarez",
			Kind:     reviewMatchKindUser,
		},
	})

	rendered := strings.Join(lines, "\n")

	assert.Contains(t, rendered, `Top reviewers matches for "Esau":`)
	assert.Contains(t, rendered, "1. Toolo (user)  resolved: Toolo  detail: Esau Suarez")
}

func TestSearchUITeaEnvironment_DisablesSynchronizedOutputProbe(t *testing.T) {
	env := searchUITeaEnvironment([]string{
		"TERM=ghostty",
		"PATH=/usr/bin",
	})

	assert.Contains(t, env, "TERM=xterm-256color")
	assert.Contains(t, env, "SSH_TTY=fotingo-search-ui")
	assert.Contains(t, env, "PATH=/usr/bin")
}
