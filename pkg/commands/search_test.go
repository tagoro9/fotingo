package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/github"
)

func TestSearchCommandUse(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "search", searchCmd.Use)
	assert.NotEmpty(t, searchCmd.Short)
	require.Len(t, searchCmd.Commands(), 3)
}

func TestSearchReviewMetadata_AssigneesExcludeTeams(t *testing.T) {
	origFactory := newSearchGitHubClient
	defer func() { newSearchGitHubClient = origFactory }()

	newSearchGitHubClient = func() (github.Github, error) {
		return &mockGitHub{
			collaborators: []github.User{
				{Login: "alice", Name: "Alice Dev"},
			},
			teams: []github.Team{
				{Organization: "acme", Slug: "platform", Name: "Platform Team"},
			},
		}, nil
	}

	results, err := searchReviewMetadata(searchDomainAssignees, "plat")
	require.NoError(t, err)
	assert.Empty(t, results)

	results, err = searchReviewMetadata(searchDomainAssignees, "ali")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "alice", results[0].Resolved)
	assert.Equal(t, reviewMatchKindUser, results[0].Kind)
}

func TestSearchReviewMetadata_ReviewersIncludeTeams(t *testing.T) {
	origFactory := newSearchGitHubClient
	defer func() { newSearchGitHubClient = origFactory }()

	newSearchGitHubClient = func() (github.Github, error) {
		return &mockGitHub{
			collaborators: []github.User{
				{Login: "alice", Name: "Alice Dev"},
			},
			teams: []github.Team{
				{Organization: "acme", Slug: "platform", Name: "Platform Team"},
			},
		}, nil
	}

	results, err := searchReviewMetadata(searchDomainReviewers, "plat")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "acme/platform", results[0].Resolved)
	assert.Equal(t, reviewMatchKindTeam, results[0].Kind)
}

func TestSearchReviewMetadata_LimitsResultsToTopFive(t *testing.T) {
	origFactory := newSearchGitHubClient
	defer func() { newSearchGitHubClient = origFactory }()

	newSearchGitHubClient = func() (github.Github, error) {
		return &mockGitHub{
			labels: []github.Label{
				{Name: "bug-1"},
				{Name: "bug-2"},
				{Name: "bug-3"},
				{Name: "bug-4"},
				{Name: "bug-5"},
				{Name: "bug-6"},
			},
		}, nil
	}

	results, err := searchReviewMetadata(searchDomainLabels, "bug")
	require.NoError(t, err)
	require.Len(t, results, 5)
	assert.Equal(t, "bug-1", results[0].Resolved)
	assert.Equal(t, "bug-5", results[4].Resolved)
}

func TestRunSearchMetadataCommand_JSONOutput(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()
	Global.JSON = true

	origFactory := newSearchGitHubClient
	defer func() { newSearchGitHubClient = origFactory }()

	newSearchGitHubClient = func() (github.Github, error) {
		return &mockGitHub{
			collaborators: []github.User{
				{Login: "alice", Name: "Alice Dev"},
			},
			teams: []github.Team{
				{Organization: "acme", Slug: "platform", Name: "Platform Team"},
			},
		}, nil
	}

	output := captureStdout(t, func() {
		err := runSearchMetadataCommand(io.Discard, searchDomainReviewers, []string{"ali"})
		require.NoError(t, err)
	})

	var decoded SearchOutput
	require.NoError(t, json.Unmarshal([]byte(output), &decoded))
	assert.True(t, decoded.Success)
	assert.Equal(t, "reviewers", decoded.Domain)
	assert.Equal(t, "ali", decoded.Query)
	require.Len(t, decoded.Results, 1)
	assert.Equal(t, "alice", decoded.Results[0].Resolved)
	assert.Equal(t, "Alice Dev", decoded.Results[0].Detail)
	assert.Equal(t, "user", decoded.Results[0].Kind)
}

func TestRunSearchMetadataCommand_TextOutputNoMatches(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()

	origFactory := newSearchGitHubClient
	defer func() { newSearchGitHubClient = origFactory }()

	newSearchGitHubClient = func() (github.Github, error) {
		return &mockGitHub{
			labels: []github.Label{
				{Name: "documentation"},
			},
		}, nil
	}

	var output bytes.Buffer
	err := runSearchMetadataCommand(&output, searchDomainLabels, []string{"bug"})
	require.NoError(t, err)
	assert.Contains(t, output.String(), `No labels matches found for "bug".`)
}

func TestRunSearchMetadataCommand_RequiresQuery(t *testing.T) {
	err := runSearchMetadataCommand(io.Discard, searchDomainLabels, []string{"   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "search query is required")
}
