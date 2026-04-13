package commands

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	internalreview "github.com/tagoro9/fotingo/internal/commands/review"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
)

type reviewStacksMockGit struct {
	*mockGit
	worktrees   []git.WorktreeInfo
	clean       map[string]bool
	cleanErr    error
	rebaseErr   error
	pushErr     error
	rebases     []reviewStacksRebaseCall
	pushes      []reviewStacksPushCall
	worktreeErr error
}

type reviewStacksRebaseCall struct {
	path string
	base string
}

type reviewStacksPushCall struct {
	path           string
	branch         string
	forceWithLease bool
}

func (m *reviewStacksMockGit) ListWorktrees() ([]git.WorktreeInfo, error) {
	return m.worktrees, m.worktreeErr
}

func (m *reviewStacksMockGit) IsWorktreeClean(path string) (bool, error) {
	if m.cleanErr != nil {
		return false, m.cleanErr
	}
	return m.clean[path], nil
}

func (m *reviewStacksMockGit) RebaseWorktree(path string, baseRef string) error {
	m.rebases = append(m.rebases, reviewStacksRebaseCall{path: path, base: baseRef})
	return m.rebaseErr
}

func (m *reviewStacksMockGit) PushWorktreeBranch(path string, branch string, forceWithLease bool) error {
	m.pushes = append(m.pushes, reviewStacksPushCall{path: path, branch: branch, forceWithLease: forceWithLease})
	return m.pushErr
}

type reviewStacksMockGitHub struct {
	*mockGitHub
	headPRs       map[string]*github.PullRequest
	basePRs       map[string][]github.PullRequest
	stackMembers  map[string][]github.PullRequest
	bodyUpdates   []github.PullRequestBodyUpdate
	findErr       error
	baseErr       error
	stackErr      error
	bodyUpdateErr error
}

func (m *reviewStacksMockGitHub) FindOpenPullRequestByHeadBranch(branch string) (*github.PullRequest, bool, error) {
	if m.findErr != nil {
		return nil, false, m.findErr
	}
	pr, ok := m.headPRs[branch]
	return pr, ok, nil
}

func (m *reviewStacksMockGitHub) ListOpenPullRequestsByBaseBranch(branch string) ([]github.PullRequest, error) {
	if m.baseErr != nil {
		return nil, m.baseErr
	}
	return append([]github.PullRequest(nil), m.basePRs[branch]...), nil
}

func (m *reviewStacksMockGitHub) ListOpenPullRequestsByStackID(stackID string) ([]github.PullRequest, error) {
	if m.stackErr != nil {
		return nil, m.stackErr
	}
	return append([]github.PullRequest(nil), m.stackMembers[stackID]...), nil
}

func (m *reviewStacksMockGitHub) UpdatePullRequestBodies(updates []github.PullRequestBodyUpdate) ([]*github.PullRequest, error) {
	m.bodyUpdates = append([]github.PullRequestBodyUpdate(nil), updates...)
	if m.bodyUpdateErr != nil {
		return nil, m.bodyUpdateErr
	}

	updated := make([]*github.PullRequest, 0, len(updates))
	for _, update := range updates {
		for _, members := range m.stackMembers {
			for _, member := range members {
				if member.Number == update.Number {
					member.Body = update.Body
					updated = append(updated, &member)
					break
				}
			}
		}
	}
	return updated, nil
}

func TestDiscoverCurrentReviewStack_UsesExistingStackID(t *testing.T) {
	parent, child, _ := reviewStackPullRequests()
	ghClient := &reviewStacksMockGitHub{
		mockGitHub:   &mockGitHub{},
		headPRs:      map[string]*github.PullRequest{child.HeadRef: &child},
		stackMembers: map[string][]github.PullRequest{"owner/repo#12": {parent, child}},
	}
	gitClient := &reviewStacksMockGit{mockGit: &mockGit{currentBranch: child.HeadRef}}

	stack, err := discoverCurrentReviewStack(gitClient, ghClient)

	require.NoError(t, err)
	assert.Equal(t, "owner/repo#12", stack.StackID)
	require.Len(t, stack.Members, 2)
	assert.Equal(t, 12, stack.Members[0].Number)
	assert.Equal(t, 13, stack.Members[1].Number)
	assert.True(t, stack.Members[1].Current)
	assert.Equal(t, "🟢", stack.Members[1].Status)
}

func TestDiscoverCurrentReviewStack_DiscoversChildWhenRootHasNoStackID(t *testing.T) {
	parent, child, _ := reviewStackPullRequests()
	parent.Body = emptyStackBody()
	ghClient := &reviewStacksMockGitHub{
		mockGitHub: &mockGitHub{},
		headPRs:    map[string]*github.PullRequest{parent.HeadRef: &parent},
		basePRs:    map[string][]github.PullRequest{parent.HeadRef: {child}},
	}
	gitClient := &reviewStacksMockGit{mockGit: &mockGit{currentBranch: parent.HeadRef}}

	stack, err := discoverCurrentReviewStack(gitClient, ghClient)

	require.NoError(t, err)
	assert.Equal(t, "owner/repo#12", stack.StackID)
	require.Len(t, stack.Members, 2)
	assert.True(t, stack.Members[0].Current)
}

func TestDiscoverCurrentReviewStack_RejectsStandalonePullRequest(t *testing.T) {
	parent, _, _ := reviewStackPullRequests()
	parent.Body = emptyStackBody()
	ghClient := &reviewStacksMockGitHub{
		mockGitHub: &mockGitHub{},
		headPRs:    map[string]*github.PullRequest{parent.HeadRef: &parent},
		basePRs:    map[string][]github.PullRequest{},
	}
	gitClient := &reviewStacksMockGit{mockGit: &mockGit{currentBranch: parent.HeadRef}}

	stack, err := discoverCurrentReviewStack(gitClient, ghClient)

	require.Error(t, err)
	assert.Nil(t, stack)
	assert.Contains(t, err.Error(), "no stacked pull requests found")
}

func TestSyncCurrentReviewStack_UpdatesEveryStackMember(t *testing.T) {
	restoreClients := stubReviewStacksClients(t)
	origRoot := fotingoConfig.GetString("jira.root")
	fotingoConfig.Set("jira.root", "https://jira.example.com")
	t.Cleanup(func() { fotingoConfig.Set("jira.root", origRoot) })

	parent, child, _ := reviewStackPullRequests()
	gitClient := &reviewStacksMockGit{mockGit: &mockGit{currentBranch: child.HeadRef}}
	ghClient := &reviewStacksMockGitHub{
		mockGitHub:   &mockGitHub{},
		headPRs:      map[string]*github.PullRequest{child.HeadRef: &child},
		stackMembers: map[string][]github.PullRequest{"owner/repo#12": {parent, child}},
	}
	restoreClients(gitClient, ghClient)

	stack, err := syncCurrentReviewStack()

	require.NoError(t, err)
	assert.Equal(t, "owner/repo#12", stack.StackID)
	require.Len(t, ghClient.bodyUpdates, 2)
	assert.Equal(t, 12, ghClient.bodyUpdates[0].Number)
	assert.Equal(t, 13, ghClient.bodyUpdates[1].Number)
	assert.Contains(t, ghClient.bodyUpdates[0].Body, "**Stacked PRs**")
	assert.Contains(t, ghClient.bodyUpdates[0].Body, "| 1 👉 | [ABC-1](https://jira.example.com/browse/ABC-1) | [#12")
	assert.Contains(t, ghClient.bodyUpdates[1].Body, "| 2 👉 | [ABC-2](https://jira.example.com/browse/ABC-2) | [#13")
	assert.NotContains(t, ghClient.bodyUpdates[1].Body, "| Status |")
}

func TestSyncCurrentReviewStack_FailsBeforePartialUpdateWhenMarkerMissing(t *testing.T) {
	restoreClients := stubReviewStacksClients(t)
	parent, child, _ := reviewStackPullRequests()
	parent.Body = "no stack markers"
	gitClient := &reviewStacksMockGit{mockGit: &mockGit{currentBranch: child.HeadRef}}
	ghClient := &reviewStacksMockGitHub{
		mockGitHub:   &mockGitHub{},
		headPRs:      map[string]*github.PullRequest{child.HeadRef: &child},
		stackMembers: map[string][]github.PullRequest{"owner/repo#12": {parent, child}},
	}
	restoreClients(gitClient, ghClient)

	stack, err := syncCurrentReviewStack()

	require.Error(t, err)
	assert.Nil(t, stack)
	assert.Empty(t, ghClient.bodyUpdates)
	assert.Contains(t, err.Error(), "pull request #12")
}

func TestRebaseCurrentReviewStack_UsesBranchWorktreesInOrder(t *testing.T) {
	restoreClients := stubReviewStacksClients(t)
	parent, child, leaf := reviewStackPullRequests()
	gitClient := &reviewStacksMockGit{
		mockGit: &mockGit{currentBranch: child.HeadRef},
		worktrees: []git.WorktreeInfo{
			{Path: "/workspace/parent", Branch: parent.HeadRef},
			{Path: "/workspace/child", Branch: child.HeadRef},
			{Path: "/workspace/leaf", Branch: leaf.HeadRef},
		},
		clean: map[string]bool{
			"/workspace/child": true,
			"/workspace/leaf":  true,
		},
	}
	ghClient := &reviewStacksMockGitHub{
		mockGitHub:   &mockGitHub{},
		headPRs:      map[string]*github.PullRequest{child.HeadRef: &child},
		stackMembers: map[string][]github.PullRequest{"owner/repo#12": {leaf, parent, child}},
	}
	restoreClients(gitClient, ghClient)

	stack, err := rebaseCurrentReviewStack(true)

	require.NoError(t, err)
	assert.Len(t, stack.Members, 3)
	assert.Equal(t, []reviewStacksRebaseCall{
		{path: "/workspace/child", base: parent.HeadRef},
		{path: "/workspace/leaf", base: child.HeadRef},
	}, gitClient.rebases)
	assert.Equal(t, []reviewStacksPushCall{
		{path: "/workspace/child", branch: child.HeadRef, forceWithLease: true},
		{path: "/workspace/leaf", branch: leaf.HeadRef, forceWithLease: true},
	}, gitClient.pushes)
}

func TestRebaseCurrentReviewStack_FailsBeforeRebaseWhenWorktreeDirty(t *testing.T) {
	restoreClients := stubReviewStacksClients(t)
	parent, child, _ := reviewStackPullRequests()
	gitClient := &reviewStacksMockGit{
		mockGit: &mockGit{currentBranch: child.HeadRef},
		worktrees: []git.WorktreeInfo{
			{Path: "/workspace/parent", Branch: parent.HeadRef},
			{Path: "/workspace/child", Branch: child.HeadRef},
		},
		clean: map[string]bool{"/workspace/child": false},
	}
	ghClient := &reviewStacksMockGitHub{
		mockGitHub:   &mockGitHub{},
		headPRs:      map[string]*github.PullRequest{child.HeadRef: &child},
		stackMembers: map[string][]github.PullRequest{"owner/repo#12": {parent, child}},
	}
	restoreClients(gitClient, ghClient)

	stack, err := rebaseCurrentReviewStack(false)

	require.Error(t, err)
	require.NotNil(t, stack)
	assert.Contains(t, err.Error(), "/workspace/child")
	assert.Empty(t, gitClient.rebases)
}

func TestRebaseCurrentReviewStack_ReportsRebaseConflictWorktree(t *testing.T) {
	restoreClients := stubReviewStacksClients(t)
	parent, child, _ := reviewStackPullRequests()
	gitClient := &reviewStacksMockGit{
		mockGit: &mockGit{currentBranch: child.HeadRef},
		worktrees: []git.WorktreeInfo{
			{Path: "/workspace/parent", Branch: parent.HeadRef},
			{Path: "/workspace/child", Branch: child.HeadRef},
		},
		clean:     map[string]bool{"/workspace/child": true},
		rebaseErr: errors.New("conflict in file.go"),
	}
	ghClient := &reviewStacksMockGitHub{
		mockGitHub:   &mockGitHub{},
		headPRs:      map[string]*github.PullRequest{child.HeadRef: &child},
		stackMembers: map[string][]github.PullRequest{"owner/repo#12": {parent, child}},
	}
	restoreClients(gitClient, ghClient)

	stack, err := rebaseCurrentReviewStack(false)

	require.Error(t, err)
	require.NotNil(t, stack)
	assert.Contains(t, err.Error(), "conflict in file.go")
	assert.Contains(t, err.Error(), "/workspace/child")
	assert.Equal(t, []reviewStacksRebaseCall{{path: "/workspace/child", base: parent.HeadRef}}, gitClient.rebases)
	assert.Empty(t, gitClient.pushes)
}

func TestPrintReviewStack_MarksCurrentPullRequest(t *testing.T) {
	origRoot := fotingoConfig.GetString("jira.root")
	fotingoConfig.Set("jira.root", "https://jira.example.com")
	t.Cleanup(func() { fotingoConfig.Set("jira.root", origRoot) })

	stack := &reviewStackContext{
		Members: []reviewStackMember{
			{Number: 12, URL: "https://github.com/owner/repo/pull/12", Title: "[ABC-1] Parent", JiraKey: "ABC-1", JiraURL: reviewStackJiraURL("ABC-1"), HeadRef: "feature/ABC-1-parent", BaseRef: "main", Status: "🟢"},
			{Number: 13, URL: "https://github.com/owner/repo/pull/13", Title: "[ABC-2] Child", JiraKey: "ABC-2", JiraURL: reviewStackJiraURL("ABC-2"), HeadRef: "feature/ABC-2-child", BaseRef: "feature/ABC-1-parent", Status: "🟢", Current: true},
		},
	}

	output := captureStdout(t, func() { printReviewStack(stack) })

	assert.Contains(t, output, "Stacked PRs")
	assert.Contains(t, output, "╭")
	assert.Contains(t, output, "Order")
	assert.Contains(t, output, "Jira")
	assert.Contains(t, output, "PR")
	assert.Contains(t, output, "2 👉")
	assert.Contains(t, output, "ABC-2")
	assert.Contains(t, output, "\x1b]8;;https://jira.example.com/browse/ABC-2\aABC-2\x1b]8;;\a")
	assert.Contains(t, output, "\x1b]8;;https://github.com/owner/repo/pull/13\a#13 [ABC-2] Child\x1b]8;;\a")
	assert.Contains(t, output, "feature/ABC-2-child")
	assert.NotContains(t, output, "|")
	assert.NotContains(t, output, "[#13]")
	assert.NotContains(t, output, "#13 https://github.com/owner/repo/pull/13")
	assert.NotContains(t, output, "Status")
}

func TestReviewStacksCommand_RegistersSubcommandsAndCompletion(t *testing.T) {
	assert.NotNil(t, findTestCommand(reviewCmd.Commands(), "stacks"))
	assert.NotNil(t, findTestCommand(reviewStacksCmd.Commands(), "sync"))
	assert.NotNil(t, findTestCommand(reviewStacksCmd.Commands(), "rebase"))
	assert.NotNil(t, reviewStacksRebaseCmd.Flags().Lookup("push"))

	resetCommandStateForExecute(Fotingo)
	buf := new(strings.Builder)
	Fotingo.SetOut(buf)
	Fotingo.SetErr(buf)
	Fotingo.SetArgs([]string{"__complete", "review", "st"})

	err := Fotingo.Execute()

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "stacks")
}

func findTestCommand(commands []*cobra.Command, name string) *cobra.Command {
	for _, cmd := range commands {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

func stubReviewStacksClients(t *testing.T) func(*reviewStacksMockGit, *reviewStacksMockGitHub) {
	t.Helper()

	origNewGitClient := newGitClient
	origNewGitHubClient := newGitHubClient
	t.Cleanup(func() {
		newGitClient = origNewGitClient
		newGitHubClient = origNewGitHubClient
	})

	return func(gitClient *reviewStacksMockGit, ghClient *reviewStacksMockGitHub) {
		newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
			return gitClient, nil
		}
		newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
			return ghClient, nil
		}
	}
}

func reviewStackPullRequests() (github.PullRequest, github.PullRequest, github.PullRequest) {
	parent := github.PullRequest{
		Number:  12,
		Title:   "[ABC-1] Parent",
		Body:    stackBody("owner/repo#12"),
		HTMLURL: "https://github.com/owner/repo/pull/12",
		HeadRef: "feature/ABC-1-parent",
		BaseRef: "main",
		State:   "open",
	}
	child := github.PullRequest{
		Number:  13,
		Title:   "[ABC-2] Child",
		Body:    stackBody("owner/repo#12"),
		HTMLURL: "https://github.com/owner/repo/pull/13",
		HeadRef: "feature/ABC-2-child",
		BaseRef: parent.HeadRef,
		State:   "open",
	}
	leaf := github.PullRequest{
		Number:  14,
		Title:   "[ABC-3] Leaf",
		Body:    stackBody("owner/repo#12"),
		HTMLURL: "https://github.com/owner/repo/pull/14",
		HeadRef: "feature/ABC-3-leaf",
		BaseRef: child.HeadRef,
		State:   "open",
	}
	return parent, child, leaf
}

func stackBody(stackID string) string {
	return emptyStackBodyWithContent(internalreview.RenderStackedPRSection(internalreview.StackRenderOptions{
		StackID: stackID,
		Items: []internalreview.StackPullRequest{
			{Number: 12, Title: "[ABC-1] Parent", HTMLURL: "https://github.com/owner/repo/pull/12", JiraKey: "ABC-1", State: "open"},
		},
	}))
}

func emptyStackBody() string {
	return emptyStackBodyWithContent("\n")
}

func emptyStackBodyWithContent(content string) string {
	start, end := internalreview.StackedPRSectionMarkers()
	return strings.Join([]string{"Intro", start, content, end, "Footer"}, "\n")
}
