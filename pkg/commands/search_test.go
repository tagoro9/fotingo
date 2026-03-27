package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
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

	results, err := searchReviewMetadata(searchDomainAssignees, "plat", nil)
	require.NoError(t, err)
	assert.Empty(t, results)

	results, err = searchReviewMetadata(searchDomainAssignees, "ali", nil)
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

	results, err := searchReviewMetadata(searchDomainReviewers, "plat", nil)
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

	results, err := searchReviewMetadata(searchDomainLabels, "bug", nil)
	require.NoError(t, err)
	require.Len(t, results, 5)
	assert.Equal(t, "bug-1", results[0].Resolved)
	assert.Equal(t, "bug-5", results[4].Resolved)
}

func TestSearchReviewMetadata_PrefersOrgScopedMatchesBeforeCollaborators(t *testing.T) {
	origFactory := newSearchGitHubClient
	defer func() { newSearchGitHubClient = origFactory }()

	gh := &mockGitHub{
		orgMembers: []github.User{
			{Login: "alice", Name: "Alice Member"},
		},
		collaborators: []github.User{
			{Login: "alice-collab", Name: "Alice Collaborator"},
		},
	}
	newSearchGitHubClient = func() (github.Github, error) {
		return gh, nil
	}

	results, err := searchReviewMetadata(searchDomainReviewers, "ali", nil)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "alice", results[0].Resolved)
	assert.Equal(t, "alice-collab", results[1].Resolved)
	assert.Contains(t, gh.calls, "get_collaborators")
}

func TestSearchReviewMetadata_FallsBackToCollaboratorsAfterOrgMiss(t *testing.T) {
	origFactory := newSearchGitHubClient
	defer func() { newSearchGitHubClient = origFactory }()

	gh := &mockGitHub{
		orgMembers: []github.User{
			{Login: "bob", Name: "Bob Member"},
		},
		collaborators: []github.User{
			{Login: "alice", Name: "Alice Collaborator"},
		},
	}
	newSearchGitHubClient = func() (github.Github, error) {
		return gh, nil
	}

	results, err := searchReviewMetadata(searchDomainAssignees, "ali", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "alice", results[0].Resolved)
	assert.Contains(t, gh.calls, "get_org_members")
	assert.Contains(t, gh.calls, "get_collaborators")
}

func TestSearchReviewMetadata_FallsBackToCollaboratorsForNonOrgRepositories(t *testing.T) {
	origFactory := newSearchGitHubClient
	defer func() { newSearchGitHubClient = origFactory }()

	supportsOrganizationMetadata := false
	gh := &mockGitHub{
		supportsOrganizationMetadata: &supportsOrganizationMetadata,
		collaborators: []github.User{
			{Login: "alice", Name: "Alice Collaborator"},
		},
	}
	newSearchGitHubClient = func() (github.Github, error) {
		return gh, nil
	}

	results, err := searchReviewMetadata(searchDomainReviewers, "ali", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "alice", results[0].Resolved)
	assert.Contains(t, gh.calls, "get_collaborators")
	assert.NotContains(t, gh.calls, "get_org_members")
	assert.NotContains(t, gh.calls, "get_teams")
}

func TestSearchReviewMetadata_NonOrgCollaboratorFailureReturnsCollaboratorError(t *testing.T) {
	origFactory := newSearchGitHubClient
	defer func() { newSearchGitHubClient = origFactory }()

	supportsOrganizationMetadata := false
	gh := &mockGitHub{
		supportsOrganizationMetadata: &supportsOrganizationMetadata,
		collaboratorsErr:             errors.New("collaborators unavailable"),
	}
	newSearchGitHubClient = func() (github.Github, error) {
		return gh, nil
	}

	results, err := searchReviewMetadata(searchDomainReviewers, "ali", nil)
	require.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "failed to load repository collaborators")
	assert.NotContains(t, err.Error(), "organization members and teams")
}

func TestSearchReviewMetadata_UsesTeamMatchesBeforeCollaborators(t *testing.T) {
	origFactory := newSearchGitHubClient
	defer func() { newSearchGitHubClient = origFactory }()

	gh := &mockGitHub{
		orgMembers: []github.User{
			{Login: "bob", Name: "Bob Member"},
		},
		teams: []github.Team{
			{Organization: "acme", Slug: "platform", Name: "Platform Team"},
		},
		collaborators: []github.User{
			{Login: "platform-dev", Name: "Platform Developer"},
		},
	}
	newSearchGitHubClient = func() (github.Github, error) {
		return gh, nil
	}

	results, err := searchReviewMetadata(searchDomainReviewers, "plat", nil)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "acme/platform", results[0].Resolved)
	assert.Equal(t, reviewMatchKindTeam, results[0].Kind)
	assert.Equal(t, "platform-dev", results[1].Resolved)
	assert.Contains(t, gh.calls, "get_collaborators")
}

func TestSearchReviewMetadata_ProgressCallbackReceivesMetadataMessages(t *testing.T) {
	origFactory := newSearchGitHubClient
	defer func() { newSearchGitHubClient = origFactory }()

	gh := &mockGitHub{
		orgMembers: []github.User{
			{Login: "alice", Name: "Alice Member"},
		},
	}
	newSearchGitHubClient = func() (github.Github, error) {
		return gh, nil
	}

	var progress []string
	_, err := searchReviewMetadata(searchDomainReviewers, "ali", func(message string) {
		progress = append(progress, message)
	})
	require.NoError(t, err)
	require.NotEmpty(t, progress)
	assert.Contains(t, progress[0], "Initializing GitHub metadata client")
	assert.Contains(t, strings.Join(progress, "\n"), "Loaded 1 GitHub organization members for testowner from cache")
	assert.Contains(t, strings.Join(progress, "\n"), "Loaded 1 reviewers candidates")
	assert.Contains(t, strings.Join(progress, "\n"), "Ranked 1 reviewers matches")
}

func TestBuildSearchProgressLogger_UsesDebugOutput(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()

	Global.Debug = true

	var output bytes.Buffer
	logger := buildSearchProgressLogger(&output, nil)
	require.NotNil(t, logger)

	logger("Loaded 1 GitHub organization members for testowner from cache in 1ms")

	assert.Contains(t, output.String(), "Loaded 1 GitHub organization members for testowner from cache in 1ms")
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
		err := runSearchMetadataCommand(io.Discard, searchDomainReviewers, []string{"ali"}, nil)
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
	err := runSearchMetadataCommand(&output, searchDomainLabels, []string{"bug"}, nil)
	require.NoError(t, err)
	assert.Contains(t, output.String(), `No labels matches found for "bug".`)
}

func TestRunSearchMetadataCommand_TextOutputUsesSingleLineResults(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()

	origFactory := newSearchGitHubClient
	defer func() { newSearchGitHubClient = origFactory }()

	newSearchGitHubClient = func() (github.Github, error) {
		return &mockGitHub{
			collaborators: []github.User{
				{Login: "Toolo", Name: "Esau Suarez"},
			},
		}, nil
	}

	var output bytes.Buffer
	err := runSearchMetadataCommand(&output, searchDomainReviewers, []string{"Esau"}, nil)
	require.NoError(t, err)
	assert.Contains(t, output.String(), `Top reviewers matches for "Esau":`)
	assert.Contains(t, output.String(), "1. Toolo (user)  resolved: Toolo  detail: Esau Suarez")
}

func TestRunSearchMetadataCommand_RequiresQuery(t *testing.T) {
	err := runSearchMetadataCommand(io.Discard, searchDomainLabels, []string{"   "}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "search query is required")
}
