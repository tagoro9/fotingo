package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalreview "github.com/tagoro9/fotingo/internal/commands/review"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
)

type reviewStacksFlags struct {
	push bool
}

type reviewStacksGithub interface {
	FindOpenPullRequestByHeadBranch(string) (*github.PullRequest, bool, error)
	ListOpenPullRequestsByBaseBranch(string) ([]github.PullRequest, error)
	ListOpenPullRequestsByStackID(string) ([]github.PullRequest, error)
	UpdatePullRequestBodies([]github.PullRequestBodyUpdate) ([]*github.PullRequest, error)
}

type reviewStacksGit interface {
	git.Git
	ListWorktrees() ([]git.WorktreeInfo, error)
	IsWorktreeClean(string) (bool, error)
	RebaseWorktree(string, string) error
	PushWorktreeBranch(string, string, bool) error
}

type reviewStackContext struct {
	StackID       string               `json:"stackId"`
	CurrentBranch string               `json:"currentBranch"`
	CurrentPR     int                  `json:"currentPullRequest"`
	Members       []reviewStackMember  `json:"members"`
	rawMembers    []github.PullRequest `json:"-"`
}

type reviewStackMember struct {
	Number  int    `json:"number"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	JiraKey string `json:"jiraKey,omitempty"`
	HeadRef string `json:"headRef"`
	BaseRef string `json:"baseRef"`
	Status  string `json:"status"`
	Current bool   `json:"current,omitempty"`
}

type reviewStacksOutput struct {
	Success bool                `json:"success"`
	Stack   *reviewStackContext `json:"stack,omitempty"`
	Error   string              `json:"error,omitempty"`
}

var reviewStacksCmdFlags = reviewStacksFlags{}

func init() {
	reviewStacksRebaseCmd.Flags().BoolVar(&reviewStacksCmdFlags.push, "push", false, "Push rebased stack branches with force-with-lease")

	reviewStacksCmd.AddCommand(reviewStacksSyncCmd)
	reviewStacksCmd.AddCommand(reviewStacksRebaseCmd)
	reviewCmd.AddCommand(reviewStacksCmd)
}

var reviewStacksCmd = &cobra.Command{
	Use:   "stacks",
	Short: "Inspect and manage stacked pull requests",
	Long:  "Inspect and manage the stacked pull request chain for the current branch.",
	RunE: func(cmd *cobra.Command, args []string) error {
		stack, err := loadCurrentReviewStack()
		if ShouldOutputJSON() {
			outputReviewStackJSON(stack, err)
		}
		if err != nil {
			return err
		}
		if !ShouldOutputJSON() {
			printReviewStack(stack)
		}
		return nil
	},
}

var reviewStacksSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Refresh stacked PR sections across the current stack",
	Long:  "Refresh the deterministic stacked PR sections for every open pull request in the current stack.",
	RunE: func(cmd *cobra.Command, args []string) error {
		stack, err := syncCurrentReviewStack()
		if ShouldOutputJSON() {
			outputReviewStackJSON(stack, err)
		}
		if err != nil {
			return err
		}
		if !ShouldOutputJSON() {
			return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
				for _, member := range stack.Members {
					out.InfoRaw(commandruntime.LogEmojiCheck, fmt.Sprintf("Updated pull request #%d: %s", member.Number, member.URL))
				}
				return nil
			})
		}
		return nil
	},
}

var reviewStacksRebaseCmd = &cobra.Command{
	Use:   "rebase",
	Short: "Rebase local stack branches in order",
	Long:  "Rebase local stack branches in order, using each branch's existing local worktree when present.",
	RunE: func(cmd *cobra.Command, args []string) error {
		stack, err := rebaseCurrentReviewStack(reviewStacksCmdFlags.push)
		if ShouldOutputJSON() {
			outputReviewStackJSON(stack, err)
		}
		if err != nil {
			return err
		}
		if !ShouldOutputJSON() {
			return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
				out.InfoRaw(commandruntime.LogEmojiCheck, "Stack rebase complete")
				if !reviewStacksCmdFlags.push {
					out.InfoRaw(commandruntime.LogEmojiInfo, "Rebased branches were not pushed. Rerun with --push to force-push with lease.")
				}
				return nil
			})
		}
		return nil
	},
}

func outputReviewStackJSON(stack *reviewStackContext, err error) {
	output := reviewStacksOutput{Success: err == nil, Stack: stack}
	if err != nil {
		output.Error = err.Error()
	}
	OutputJSON(output)
}

func loadCurrentReviewStack() (*reviewStackContext, error) {
	gitClient, ghClient, err := newReviewStacksClients()
	if err != nil {
		return nil, err
	}
	return discoverCurrentReviewStack(gitClient, ghClient)
}

func syncCurrentReviewStack() (*reviewStackContext, error) {
	gitClient, ghClient, err := newReviewStacksClients()
	if err != nil {
		return nil, err
	}
	stack, err := discoverCurrentReviewStack(gitClient, ghClient)
	if err != nil {
		return nil, err
	}
	updates := make([]github.PullRequestBodyUpdate, 0, len(stack.rawMembers))
	for _, member := range stack.rawMembers {
		body, err := internalreview.ReplaceStackedPRSectionContent(
			member.Body,
			internalreview.RenderStackedPRSection(internalreview.StackRenderOptions{
				StackID: stack.StackID,
				Items:   stackItemsForMembers(stack.rawMembers, member.Number),
			}),
		)
		if err != nil {
			return nil, fmt.Errorf("pull request #%d: %w", member.Number, err)
		}
		updates = append(updates, github.PullRequestBodyUpdate{Number: member.Number, Body: body})
	}
	if _, err := ghClient.UpdatePullRequestBodies(updates); err != nil {
		return nil, err
	}
	return stack, nil
}

func rebaseCurrentReviewStack(push bool) (*reviewStackContext, error) {
	gitClient, ghClient, err := newReviewStacksClients()
	if err != nil {
		return nil, err
	}
	stack, err := discoverCurrentReviewStack(gitClient, ghClient)
	if err != nil {
		return nil, err
	}
	if len(stack.rawMembers) < 2 {
		return stack, fmt.Errorf("stack must contain at least two pull requests to rebase")
	}

	worktreeByBranch, err := loadStackWorktrees(gitClient)
	if err != nil {
		return stack, err
	}
	for _, member := range stack.rawMembers[1:] {
		worktree, ok := worktreeByBranch[member.HeadRef]
		if !ok {
			return stack, fmt.Errorf("no local worktree found for stack branch %s", member.HeadRef)
		}
		clean, err := gitClient.IsWorktreeClean(worktree.Path)
		if err != nil {
			return stack, err
		}
		if !clean {
			return stack, fmt.Errorf("worktree %s for branch %s has uncommitted changes", worktree.Path, member.HeadRef)
		}
	}
	for index := 1; index < len(stack.rawMembers); index++ {
		member := stack.rawMembers[index]
		base := stack.rawMembers[index-1]
		worktree := worktreeByBranch[member.HeadRef]
		if err := gitClient.RebaseWorktree(worktree.Path, base.HeadRef); err != nil {
			return stack, fmt.Errorf("failed to rebase branch %s in worktree %s: %w", member.HeadRef, worktree.Path, err)
		}
		if push {
			if err := gitClient.PushWorktreeBranch(worktree.Path, member.HeadRef, true); err != nil {
				return stack, fmt.Errorf("failed to push branch %s from worktree %s: %w", member.HeadRef, worktree.Path, err)
			}
		}
	}
	return stack, nil
}

func newReviewStacksClients() (reviewStacksGit, reviewStacksGithub, error) {
	statusCh := make(chan string, 1)
	gitClient, err := newGitClient(fotingoConfig, &statusCh)
	if err != nil {
		return nil, nil, err
	}
	stackGit, ok := gitClient.(reviewStacksGit)
	if !ok {
		return nil, nil, fmt.Errorf("git client does not support stacked PR worktree operations")
	}
	ghClient, err := newGitHubClient(gitClient, fotingoConfig)
	if err != nil {
		return nil, nil, err
	}
	stackGitHub, ok := ghClient.(reviewStacksGithub)
	if !ok {
		return nil, nil, fmt.Errorf("github client does not support stacked PR operations")
	}
	return stackGit, stackGitHub, nil
}

func discoverCurrentReviewStack(gitClient git.Git, ghClient reviewStacksGithub) (*reviewStackContext, error) {
	currentBranch, err := gitClient.GetCurrentBranch()
	if err != nil {
		return nil, err
	}
	currentPR, exists, err := ghClient.FindOpenPullRequestByHeadBranch(currentBranch)
	if err != nil {
		return nil, err
	}
	if !exists || currentPR == nil {
		return nil, fmt.Errorf("no pull request found for branch %s", currentBranch)
	}

	stackID := internalreview.ExtractStackID(currentPR.Body)
	members := []github.PullRequest{*currentPR}
	if stackID != "" {
		stackMembers, err := ghClient.ListOpenPullRequestsByStackID(stackID)
		if err != nil {
			return nil, err
		}
		members = stackMembers
		members = appendReviewStackMember(members, *currentPR)
	} else {
		children, err := ghClient.ListOpenPullRequestsByBaseBranch(currentPR.HeadRef)
		if err != nil {
			return nil, err
		}
		if len(children) == 0 {
			return nil, fmt.Errorf("no stacked pull requests found for branch %s", currentBranch)
		}
		stackID = internalreview.StackIDForRootPR(currentPR.Number, currentPR.HTMLURL)
		for _, child := range children {
			members = appendReviewStackMember(members, child)
		}
	}
	ordered, err := internalreview.OrderStackPullRequests(members)
	if err != nil {
		return nil, err
	}
	return buildReviewStackContext(stackID, currentBranch, currentPR.Number, ordered), nil
}

func buildReviewStackContext(stackID string, currentBranch string, currentPRNumber int, members []github.PullRequest) *reviewStackContext {
	stack := &reviewStackContext{
		StackID:       stackID,
		CurrentBranch: currentBranch,
		CurrentPR:     currentPRNumber,
		Members:       make([]reviewStackMember, 0, len(members)),
		rawMembers:    members,
	}
	for _, member := range members {
		jiraKey := internalreview.DeriveStackJiraKey(member.HeadRef, member.Title, member.Body)
		stack.Members = append(stack.Members, reviewStackMember{
			Number:  member.Number,
			URL:     member.HTMLURL,
			Title:   member.Title,
			JiraKey: jiraKey,
			HeadRef: member.HeadRef,
			BaseRef: member.BaseRef,
			Status:  internalreview.StackStatusEmoji(internalreview.StackPullRequest{State: member.State, Draft: member.Draft, Merged: member.Merged}),
			Current: member.Number == currentPRNumber,
		})
	}
	return stack
}

func printReviewStack(stack *reviewStackContext) {
	_, _ = fmt.Fprintln(os.Stdout, "Stacked PRs")
	_, _ = fmt.Fprintln(os.Stdout, renderReviewStackTable(stack))
}

func renderReviewStackTable(stack *reviewStackContext) string {
	width := reviewStackTableWidth()
	columns := reviewStackTableColumns(width)
	rows := make([]table.Row, 0, len(stack.Members))
	for index, member := range stack.Members {
		rows = append(rows, table.Row{
			internalreview.StackOrderLabel(index+1, member.Current),
			formatReviewStackJira(member),
			formatReviewStackPR(member),
			member.HeadRef,
			member.BaseRef,
		})
	}

	styles := table.DefaultStyles()
	styles.Header = lipgloss.NewStyle()
	styles.Cell = lipgloss.NewStyle()
	styles.Selected = styles.Cell

	model := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(len(rows)+1),
		table.WithWidth(width),
		table.WithStyles(styles),
	)

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Render(model.View())
}

func reviewStackTableColumns(width int) []table.Column {
	const (
		orderWidth = 6
		jiraWidth  = 10
		prWidth    = 24
		minRef     = 16
	)

	innerWidth := max(width, orderWidth+jiraWidth+prWidth+minRef*2)
	refWidth := max((innerWidth-orderWidth-jiraWidth-prWidth)/2, minRef)
	return []table.Column{
		{Title: "Order", Width: orderWidth},
		{Title: "Jira", Width: jiraWidth},
		{Title: "PR", Width: prWidth},
		{Title: "Branch", Width: refWidth},
		{Title: "Base", Width: refWidth},
	}
}

func reviewStackTableWidth() int {
	const (
		defaultWidth = 96
		minWidth     = 72
		tableChrome  = 4
	)
	if raw := strings.TrimSpace(os.Getenv("COLUMNS")); raw != "" {
		width, err := strconv.Atoi(raw)
		if err == nil && width > 0 {
			return max(width-tableChrome, minWidth)
		}
	}
	return defaultWidth
}

func formatReviewStackJira(member reviewStackMember) string {
	if strings.TrimSpace(member.JiraKey) == "" {
		return "-"
	}
	return member.JiraKey
}

func formatReviewStackPR(member reviewStackMember) string {
	label := fmt.Sprintf("#%d", member.Number)
	title := strings.TrimSpace(member.Title)
	if title != "" {
		label = fmt.Sprintf("%s %s", label, title)
	}
	url := strings.TrimSpace(member.URL)
	if url == "" {
		return label
	}
	return lipgloss.NewStyle().Hyperlink(url).Render(label)
}

func stackItemsForMembers(members []github.PullRequest, currentNumber int) []internalreview.StackPullRequest {
	items := make([]internalreview.StackPullRequest, 0, len(members))
	for _, member := range members {
		jiraKey := internalreview.DeriveStackJiraKey(member.HeadRef, member.Title, member.Body)
		items = append(items, internalreview.StackPullRequest{
			Number:  member.Number,
			Title:   member.Title,
			HTMLURL: member.HTMLURL,
			JiraKey: jiraKey,
			State:   member.State,
			Draft:   member.Draft,
			Merged:  member.Merged,
			Current: member.Number == currentNumber,
		})
	}
	return items
}

func appendReviewStackMember(members []github.PullRequest, pr github.PullRequest) []github.PullRequest {
	for _, member := range members {
		if member.Number == pr.Number {
			return members
		}
	}
	return append(members, pr)
}

func loadStackWorktrees(gitClient reviewStacksGit) (map[string]git.WorktreeInfo, error) {
	worktrees, err := gitClient.ListWorktrees()
	if err != nil {
		return nil, err
	}
	byBranch := make(map[string]git.WorktreeInfo, len(worktrees))
	for _, worktree := range worktrees {
		branch := strings.TrimSpace(worktree.Branch)
		if branch != "" {
			byBranch[branch] = worktree
		}
	}
	return byBranch, nil
}
