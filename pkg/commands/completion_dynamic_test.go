package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

func TestCompleteReviewLabelsFlag_UsesFuzzyMatching(t *testing.T) {
	origFactory := newCompletionGitHubClient
	defer func() { newCompletionGitHubClient = origFactory }()

	newCompletionGitHubClient = func() (github.Github, error) {
		return &mockGitHub{
			labels: []github.Label{
				{Name: "bug", Description: "Bug fixes"},
				{Name: "documentation", Description: "Documentation updates"},
			},
		}, nil
	}

	completions, directive := completeReviewLabelsFlag(nil, nil, "doc")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Contains(t, completions, "documentation")
	assert.NotContains(t, completions, "bug")
}

func TestCompleteReviewReviewersFlag_IncludesUsersAndTeams(t *testing.T) {
	origFactory := newCompletionGitHubClient
	defer func() { newCompletionGitHubClient = origFactory }()

	newCompletionGitHubClient = func() (github.Github, error) {
		return &mockGitHub{
			collaborators: []github.User{
				{Login: "alice", Name: "Alice Dev"},
			},
			orgMembers: []github.User{
				{Login: "bob", Name: "Bob Member"},
			},
			teams: []github.Team{
				{Organization: "acme", Slug: "platform", Name: "Platform Team"},
			},
		}, nil
	}

	userCompletions, _ := completeReviewReviewersFlag(nil, nil, "ali")
	assert.Contains(t, completionValues(userCompletions), "alice")

	teamCompletions, _ := completeReviewReviewersFlag(nil, nil, "plat")
	assert.Contains(t, completionValues(teamCompletions), "acme/platform")
}

func TestCompleteReviewReviewersFlag_UsesOrgMembersBeforeCollaborators(t *testing.T) {
	origFactory := newCompletionGitHubClient
	defer func() { newCompletionGitHubClient = origFactory }()

	gh := &mockGitHub{
		orgMembers: []github.User{
			{Login: "alice", Name: "Alice Member"},
		},
		collaborators: []github.User{
			{Login: "alice-collab", Name: "Alice Collaborator"},
		},
	}
	newCompletionGitHubClient = func() (github.Github, error) {
		return gh, nil
	}

	completions, _ := completeReviewReviewersFlag(nil, nil, "ali")
	assert.Contains(t, completionValues(completions), "alice")
	assert.Contains(t, gh.calls, "get_collaborators")
}

func TestCompleteReviewReviewersFlag_FetchesCollaboratorsAfterOrgMemberMiss(t *testing.T) {
	origFactory := newCompletionGitHubClient
	defer func() { newCompletionGitHubClient = origFactory }()

	gh := &mockGitHub{
		orgMembers: []github.User{
			{Login: "bob", Name: "Bob Member"},
		},
		collaborators: []github.User{
			{Login: "alice", Name: "Alice Collaborator"},
		},
	}
	newCompletionGitHubClient = func() (github.Github, error) {
		return gh, nil
	}

	completions, _ := completeReviewReviewersFlag(nil, nil, "ali")
	assert.Contains(t, completionValues(completions), "alice")
	assert.Contains(t, gh.calls, "get_collaborators")
}

func TestCompleteReviewReviewersFlag_UsesTeamMatchesBeforeCollaborators(t *testing.T) {
	origFactory := newCompletionGitHubClient
	defer func() { newCompletionGitHubClient = origFactory }()

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
	newCompletionGitHubClient = func() (github.Github, error) {
		return gh, nil
	}

	completions, _ := completeReviewReviewersFlag(nil, nil, "plat")
	assert.Contains(t, completionValues(completions), "acme/platform")
	assert.Contains(t, gh.calls, "get_collaborators")
}

func TestCompleteReviewAssigneesFlag_ExcludesTeams(t *testing.T) {
	origFactory := newCompletionGitHubClient
	defer func() { newCompletionGitHubClient = origFactory }()

	newCompletionGitHubClient = func() (github.Github, error) {
		return &mockGitHub{
			collaborators: []github.User{
				{Login: "alice", Name: "Alice Dev"},
			},
			teams: []github.Team{
				{Organization: "acme", Slug: "platform", Name: "Platform Team"},
			},
		}, nil
	}

	completions, _ := completeReviewAssigneesFlag(nil, nil, "")
	assert.Contains(t, completionValues(completions), "alice")
	assert.NotContains(t, completionValues(completions), "acme/platform")
}

func TestCompleteReviewReviewersFlag_FuzzyReturnsMultipleMatches(t *testing.T) {
	origFactory := newCompletionGitHubClient
	defer func() { newCompletionGitHubClient = origFactory }()

	newCompletionGitHubClient = func() (github.Github, error) {
		return &mockGitHub{
			collaborators: []github.User{
				{Login: "yprk", Name: "YoungJun Park"},
				{Login: "youngdev", Name: "Young Developer"},
			},
		}, nil
	}

	completions, _ := completeReviewReviewersFlag(nil, nil, "young")
	values := completionValues(completions)
	assert.Contains(t, values, "yprk")
	assert.Contains(t, values, "youngdev")
}

func TestCompleteReviewReviewersFlag_IncludesDisplayNameDescription(t *testing.T) {
	origFactory := newCompletionGitHubClient
	defer func() { newCompletionGitHubClient = origFactory }()

	newCompletionGitHubClient = func() (github.Github, error) {
		return &mockGitHub{
			collaborators: []github.User{
				{Login: "yprk", Name: "YoungJun Park"},
			},
		}, nil
	}

	completions, _ := completeReviewReviewersFlag(nil, nil, "young")
	assert.Contains(t, completions, "yprk\tYoungJun Park")
}

func TestCompleteStartProjectFlag_CollectsProjectKeys(t *testing.T) {
	origFactory := newCompletionJiraClient
	defer func() { newCompletionJiraClient = origFactory }()

	newCompletionJiraClient = func() (jira.Jira, error) {
		return &mockJira{
			userOpenIssues: []tracker.Issue{
				{Key: "FOTINGO-1"},
				{Key: "DEVEX-22"},
			},
		}, nil
	}

	completions, directive := completeStartProjectFlag(nil, nil, "dev")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Equal(t, []string{"DEVEX"}, completions)
}

func TestCompleteStartIssueTypeFlag_UsesProjectIssueTypes(t *testing.T) {
	origFactory := newCompletionJiraClient
	defer func() { newCompletionJiraClient = origFactory }()

	newCompletionJiraClient = func() (jira.Jira, error) {
		return &mockJira{
			projectIssueTypes: []tracker.ProjectIssueType{
				{Name: "Story"},
				{Name: "Bug"},
				{Name: "Subtask"},
			},
		}, nil
	}

	cmd := &cobra.Command{Use: "start"}
	cmd.Flags().String("project", "", "")
	require.NoError(t, cmd.Flags().Set("project", "FOTINGO"))

	completions, directive := completeStartIssueTypeFlag(cmd, nil, "sto")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Equal(t, []string{"Story"}, completions)
}

func TestCompleteReviewSyncSectionFlag_ReturnsSupportedSections(t *testing.T) {
	completions, directive := completeReviewSyncSectionFlag(nil, nil, "desc")

	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Contains(t, completions, "description\tPR description section")
	assert.NotContains(t, completionValues(completions), "summary")
}

func TestCompleteReviewSyncSectionFlag_ExcludesAlreadySelectedSections(t *testing.T) {
	cmd := &cobra.Command{Use: "sync"}
	cmd.Flags().StringSlice("section", nil, "")
	require.NoError(t, cmd.Flags().Set("section", "summary"))

	completions, directive := completeReviewSyncSectionFlag(cmd, nil, "")

	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.NotContains(t, completionValues(completions), "summary")
	assert.Contains(t, completionValues(completions), "description")
	assert.Contains(t, completionValues(completions), "fixed-issues")
	assert.Contains(t, completionValues(completions), "changes")
}

func completionValues(candidates []string) []string {
	values := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		value, _, _ := strings.Cut(candidate, "\t")
		values = append(values, value)
	}
	return values
}
