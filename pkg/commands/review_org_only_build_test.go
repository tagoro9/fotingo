//go:build fotingo_org_only_participants

package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/github"
)

func TestSearchReviewMetadata_OrgOnlyBuildSkipsCollaboratorLookup(t *testing.T) {
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

	results, err := searchReviewMetadata(searchDomainReviewers, "ali", nil)
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.NotContains(t, gh.calls, "get_collaborators")
}

func TestResolveReviewReviewers_OrgOnlyBuildSkipsCollaboratorLookup(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	ghClient := &mockGitHub{
		orgMembers: []github.User{
			{Login: "bob", Name: "Bob Member"},
		},
		collaborators: []github.User{
			{Login: "alice", Name: "Alice Collaborator"},
		},
	}

	_, _, _, err := resolveReviewReviewers(ghClient, []string{"alice"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no reviewer matches found for "alice"`)
	assert.NotContains(t, ghClient.calls, "get_collaborators")
}

func TestSearchReviewMetadata_OrgOnlyBuildFallsBackForNonOrgRepositories(t *testing.T) {
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
}

func TestResolveReviewReviewers_OrgOnlyBuildFallsBackForNonOrgRepositories(t *testing.T) {
	restoreFlags := saveGlobalFlags()
	defer restoreFlags()

	supportsOrganizationMetadata := false
	ghClient := &mockGitHub{
		supportsOrganizationMetadata: &supportsOrganizationMetadata,
		collaborators: []github.User{
			{Login: "alice", Name: "Alice Collaborator"},
		},
	}

	users, teams, warnings, err := resolveReviewReviewers(ghClient, []string{"alice"})
	require.NoError(t, err)
	assert.Equal(t, []string{"alice"}, users)
	assert.Empty(t, teams)
	assert.Empty(t, warnings)
	assert.Contains(t, ghClient.calls, "get_collaborators")
}
