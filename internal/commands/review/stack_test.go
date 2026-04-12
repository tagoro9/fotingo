package review

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/github"
)

func TestRenderStackedPRSection_RendersTableWithEmojiStatus(t *testing.T) {
	t.Parallel()

	content := RenderStackedPRSection(StackRenderOptions{
		StackID: "owner/repo#12",
		Items: []StackPullRequest{
			{
				Number:  12,
				Title:   "Parent change",
				HTMLURL: "https://github.example/pull/12",
				JiraKey: "ABC-1",
				JiraURL: "https://jira.example/browse/ABC-1",
				State:   "open",
			},
			{
				Number:  13,
				Title:   "Child change",
				HTMLURL: "https://github.example/pull/13",
				JiraKey: "ABC-2",
				JiraURL: "https://jira.example/browse/ABC-2",
				State:   "open",
				Current: true,
			},
			{
				Number: 14,
				State:  "closed",
			},
			{
				Number: 15,
				Draft:  true,
			},
		},
	})

	assert.Contains(t, content, `<!-- fotingo:stack id="owner/repo#12" version="1" -->`)
	assert.Contains(t, content, "**Stacked PRs**")
	assert.Contains(t, content, "| Order | Jira | Pull request | Status |")
	assert.Contains(t, content, "| 1 | [ABC-1](https://jira.example/browse/ABC-1) | [#12 Parent change](https://github.example/pull/12) | 🟢 |")
	assert.Contains(t, content, "| 2 | [ABC-2](https://jira.example/browse/ABC-2) | [#13 Child change](https://github.example/pull/13) | 🟢 👀 |")
	assert.Contains(t, content, "| 3 | - | #14 | 🔴 |")
	assert.Contains(t, content, "| 4 | - | #15 | 📝 |")
	assert.NotContains(t, content, "open")
	assert.NotContains(t, content, "closed")
}

func TestStackStatusEmoji(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item StackPullRequest
		want string
	}{
		{name: "open", item: StackPullRequest{State: "open"}, want: "🟢"},
		{name: "draft", item: StackPullRequest{State: "open", Draft: true}, want: "📝"},
		{name: "closed", item: StackPullRequest{State: "closed"}, want: "🔴"},
		{name: "merged", item: StackPullRequest{State: "merged"}, want: "🟣"},
		{name: "current", item: StackPullRequest{State: "open", Current: true}, want: "🟢 👀"},
		{name: "unknown", item: StackPullRequest{}, want: "⚪"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, StackStatusEmoji(tt.item))
		})
	}
}

func TestExtractStackID(t *testing.T) {
	t.Parallel()

	body := `<!-- fotingo:stack id="owner/repo#12" version="1" -->`

	assert.Equal(t, "owner/repo#12", ExtractStackID(body))
}

func TestStackIDForRootPR(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "owner/repo#12", StackIDForRootPR(12, "https://github.com/owner/repo/pull/12"))
	assert.Equal(t, "pr-12", StackIDForRootPR(12, ""))
}

func TestDeriveStackJiraKey(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "ABC-123", DeriveStackJiraKey("feature/abc-123-test"))
	assert.Equal(t, "ABC-124", DeriveStackJiraKey("", "ABC-124 title"))
	assert.Empty(t, DeriveStackJiraKey("feature/no-ticket"))
}

func TestOrderStackPullRequests(t *testing.T) {
	t.Parallel()

	members := []github.PullRequest{
		{Number: 13, HeadRef: "child", BaseRef: "parent"},
		{Number: 12, HeadRef: "parent", BaseRef: "main"},
		{Number: 14, HeadRef: "leaf", BaseRef: "child"},
	}

	ordered, err := OrderStackPullRequests(members)

	require.NoError(t, err)
	assert.Equal(t, []int{12, 13, 14}, []int{ordered[0].Number, ordered[1].Number, ordered[2].Number})
}

func TestOrderStackPullRequests_RejectsBranchingStack(t *testing.T) {
	t.Parallel()

	_, err := OrderStackPullRequests([]github.PullRequest{
		{Number: 12, HeadRef: "parent", BaseRef: "main"},
		{Number: 13, HeadRef: "child-a", BaseRef: "parent"},
		{Number: 14, HeadRef: "child-b", BaseRef: "parent"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "branching stacks are not supported")
}

func TestStackedPRSectionMarkers(t *testing.T) {
	t.Parallel()

	start, end := StackedPRSectionMarkers()
	assert.Equal(t, "<!-- fotingo:start stacked-prs -->", start)
	assert.Equal(t, "<!-- fotingo:end stacked-prs -->", end)
}

func TestReplaceStackedPRSectionContent(t *testing.T) {
	t.Parallel()

	body := "prefix\n<!-- fotingo:start stacked-prs -->\nold\n<!-- fotingo:end stacked-prs -->\nsuffix"
	updated, err := ReplaceStackedPRSectionContent(body, "\nnew\n")

	require.NoError(t, err)
	assert.Equal(t, "prefix\n<!-- fotingo:start stacked-prs -->\nnew\n<!-- fotingo:end stacked-prs -->\nsuffix", updated)
}

func TestReplaceStackedPRSectionContent_FailsWhenMarkersMissing(t *testing.T) {
	t.Parallel()

	_, err := ReplaceStackedPRSectionContent("no markers", "replacement")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `missing fotingo markers for section "stacked-prs"`)
}
