package start

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/tracker"
)

type mockIssueLinkClient struct {
	validIDs      map[string]bool
	issues        map[string]*tracker.Issue
	searchResults map[string][]tracker.Issue
	searchErr     error
	searchCalls   int
}

func (m *mockIssueLinkClient) IsValidIssueID(id string) bool {
	return m.validIDs[id]
}

func (m *mockIssueLinkClient) GetIssue(id string) (*tracker.Issue, error) {
	if issue, ok := m.issues[id]; ok {
		return issue, nil
	}
	return nil, errors.New("issue not found")
}

func (m *mockIssueLinkClient) SearchIssues(projectKey string, query string, issueTypes []tracker.IssueType, limit int) ([]tracker.Issue, error) {
	m.searchCalls++
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchResults[query], nil
}

func TestResolveIssueLink_QueryRequired(t *testing.T) {
	t.Parallel()

	client := &mockIssueLinkClient{}
	_, err := ResolveIssueLink(client, ResolveIssueLinkOptions{
		RawQuery: "   ",
		Errors: ResolveIssueLinkErrors{
			QueryRequired: func() error { return errors.New("query required") },
		},
	})
	require.Error(t, err)
	assert.Equal(t, "query required", err.Error())
}

func TestResolveIssueLink_ValidIssueID(t *testing.T) {
	t.Parallel()

	client := &mockIssueLinkClient{
		validIDs: map[string]bool{"TEST-1": true},
		issues: map[string]*tracker.Issue{
			"TEST-1": {Key: "TEST-1", Type: tracker.IssueTypeStory},
		},
	}

	resolved, err := ResolveIssueLink(client, ResolveIssueLinkOptions{
		RawQuery: "test-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "TEST-1", resolved)
	assert.Equal(t, 0, client.searchCalls)
}

func TestResolveIssueLink_MatchingJiraBrowseURL(t *testing.T) {
	t.Parallel()

	client := &mockIssueLinkClient{
		validIDs: map[string]bool{"FOTINGO-17": true},
		issues: map[string]*tracker.Issue{
			"FOTINGO-17": {Key: "FOTINGO-17", Type: tracker.IssueTypeStory},
		},
	}

	resolved, err := ResolveIssueLink(client, ResolveIssueLinkOptions{
		JiraRoot: "https://tagoro9.atlassian.net",
		RawQuery: "https://tagoro9.atlassian.net/browse/FOTINGO-17?focusedCommentId=1",
	})
	require.NoError(t, err)
	assert.Equal(t, "FOTINGO-17", resolved)
	assert.Equal(t, 0, client.searchCalls)
}

func TestNormalizeIssueInput_MatchingJiraBrowseURL(t *testing.T) {
	t.Parallel()

	normalized := NormalizeIssueInput(
		"https://tagoro9.atlassian.net/browse/FOTINGO-17?focusedCommentId=1",
		"https://tagoro9.atlassian.net",
	)

	assert.Equal(t, "FOTINGO-17", normalized)
}

func TestResolveIssueLink_NonMatchingJiraBrowseURLFallsBackToSearch(t *testing.T) {
	t.Parallel()

	rawURL := "https://other.atlassian.net/browse/FOTINGO-17"
	client := &mockIssueLinkClient{
		validIDs: map[string]bool{"FOTINGO-17": true},
		issues: map[string]*tracker.Issue{
			"FOTINGO-17": {Key: "FOTINGO-17", Type: tracker.IssueTypeStory},
		},
		searchResults: map[string][]tracker.Issue{
			rawURL: {{Key: "FOTINGO-99"}},
		},
	}

	resolved, err := ResolveIssueLink(client, ResolveIssueLinkOptions{
		JiraRoot: "https://tagoro9.atlassian.net",
		RawQuery: rawURL,
	})
	require.NoError(t, err)
	assert.Equal(t, "FOTINGO-99", resolved)
	assert.Equal(t, 1, client.searchCalls)
}

func TestResolveIssueLink_SearchSingleCandidate(t *testing.T) {
	t.Parallel()

	client := &mockIssueLinkClient{
		searchResults: map[string][]tracker.Issue{
			"auth": {{Key: "TEST-2"}},
		},
	}

	resolved, err := ResolveIssueLink(client, ResolveIssueLinkOptions{
		ProjectKey: "TEST",
		RawQuery:   "auth",
	})
	require.NoError(t, err)
	assert.Equal(t, "TEST-2", resolved)
}

func TestResolveIssueLink_AmbiguousNonInteractive(t *testing.T) {
	t.Parallel()

	client := &mockIssueLinkClient{
		searchResults: map[string][]tracker.Issue{
			"auth": {
				{Key: "TEST-2"},
				{Key: "TEST-3"},
			},
		},
	}

	_, err := ResolveIssueLink(client, ResolveIssueLinkOptions{
		ProjectKey:  "TEST",
		RawQuery:    "auth",
		Interactive: false,
		Errors: ResolveIssueLinkErrors{
			LinkAmbiguous: func(query string) error { return fmt.Errorf("ambiguous %s", query) },
		},
	})
	require.Error(t, err)
	assert.Equal(t, "ambiguous auth", err.Error())
}

func TestResolveIssueLink_InteractivePick(t *testing.T) {
	t.Parallel()

	client := &mockIssueLinkClient{
		searchResults: map[string][]tracker.Issue{
			"auth": {
				{Key: "TEST-2"},
				{Key: "TEST-3"},
			},
		},
	}

	resolved, err := ResolveIssueLink(client, ResolveIssueLinkOptions{
		ProjectKey:  "TEST",
		RawQuery:    "auth",
		Interactive: true,
		SelectIssueLink: func(candidates []tracker.Issue, title string) (*tracker.Issue, error) {
			require.Len(t, candidates, 2)
			return &candidates[1], nil
		},
		PromptRefineLink: func(currentQuery string) (string, bool, error) {
			return "", false, nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "TEST-3", resolved)
}

func TestResolveIssueLink_InteractiveCancelled(t *testing.T) {
	t.Parallel()

	client := &mockIssueLinkClient{
		searchResults: map[string][]tracker.Issue{
			"auth": {
				{Key: "TEST-2"},
				{Key: "TEST-3"},
			},
		},
	}

	_, err := ResolveIssueLink(client, ResolveIssueLinkOptions{
		ProjectKey:  "TEST",
		RawQuery:    "auth",
		Interactive: true,
		SelectIssueLink: func(candidates []tracker.Issue, title string) (*tracker.Issue, error) {
			return nil, nil
		},
		PromptRefineLink: func(currentQuery string) (string, bool, error) {
			return "", true, nil
		},
		Errors: ResolveIssueLinkErrors{
			Cancelled: func() error { return errors.New("cancelled") },
		},
	})
	require.Error(t, err)
	assert.Equal(t, "cancelled", err.Error())
}
