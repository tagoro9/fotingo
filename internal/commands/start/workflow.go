package start

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

// WorkflowOptions contains flag values used by start workflow execution.
type WorkflowOptions struct {
	Title    string
	NoBranch bool
	Worktree bool
}

// WorkflowResult contains the structured result for non-interactive execution paths.
type WorkflowResult struct {
	Issue        *jira.Issue
	BranchName   string
	WorktreePath string
	Created      bool
	Err          error
}

// WorkflowEmitter defines the status logging operations used by this workflow.
type WorkflowEmitter interface {
	Info(emoji string, key i18n.Key, args ...any)
	Verbose(key i18n.Key, args ...any)
	Debug(key i18n.Key, args ...any)
	DebugRaw(message string)
}

// WorkflowDeps defines dependencies required by start workflow orchestration.
type WorkflowDeps struct {
	NormalizeFlags        func(*cobra.Command, string) error
	NewJiraClient         func(*viper.Viper) (jira.Jira, error)
	CreateNewIssue        func(WorkflowEmitter, jira.Jira) (*jira.Issue, error)
	SelectIssueWithPicker func([]tracker.Issue) (*tracker.Issue, error)
	RunWithSpinner        func(func(WorkflowEmitter) error) error
	ResolveIssueAssignee  func(WorkflowEmitter, jira.Jira, string)
	NewGitClient          func(*viper.Viper, *chan string) (git.Git, error)
	StashChanges          func(WorkflowEmitter, git.Git) error
}

// WorkflowRunner executes start workflow steps using configured dependencies.
type WorkflowRunner struct {
	Config   *viper.Viper
	Options  WorkflowOptions
	Localize func(i18n.Key, ...any) string
	Deps     WorkflowDeps
}

type interactiveContext struct {
	jiraClient     jira.Jira
	issues         []tracker.Issue
	issue          *jira.Issue
	issueID        string
	needsSelection bool
}

// RunInteractive executes the interactive start flow.
func (r WorkflowRunner) RunInteractive(cmd *cobra.Command, issueID string) error {
	issueID = normalizeWorkflowIssueID(issueID, r.Config)
	if err := r.validateDeps(); err != nil {
		return err
	}
	if err := r.Deps.NormalizeFlags(cmd, issueID); err != nil {
		return err
	}

	ctx, err := r.loadInteractiveContext(issueID)
	if err != nil {
		return err
	}

	if ctx.needsSelection {
		selectedIssue, err := r.Deps.SelectIssueWithPicker(ctx.issues)
		if err != nil {
			return err
		}
		ctx.issueID = selectedIssue.Key
	}

	return r.progressStartWorkflow(ctx.jiraClient, ctx.issue, ctx.issueID)
}

// RunWithResult executes the non-interactive start flow used by JSON mode.
func (r WorkflowRunner) RunWithResult(cmd *cobra.Command, statusCh *chan string, issueID string, out WorkflowEmitter) WorkflowResult {
	result := WorkflowResult{}
	issueID = normalizeWorkflowIssueID(issueID, r.Config)
	if err := r.validateDeps(); err != nil {
		result.Err = err
		return result
	}
	if out == nil {
		out = noopWorkflowEmitter{}
	}

	if err := r.Deps.NormalizeFlags(cmd, issueID); err != nil {
		result.Err = err
		return result
	}

	t := r.localize
	totalStart := time.Now()

	out.Verbose(i18n.StartStatusInitJira)
	initJiraStart := time.Now()
	jiraClient, err := r.Deps.NewJiraClient(r.Config)
	logStartPhaseTiming(out, "init_jira", initJiraStart)
	if err != nil {
		result.Err = fterrors.WrapJiraError(t(i18n.StartWrapInitJira), err)
		return result
	}
	out.Debug(i18n.StartStatusJiraInit)

	var issue *jira.Issue
	if r.Options.Title != "" {
		issue, err = r.Deps.CreateNewIssue(out, jiraClient)
		if err != nil {
			result.Err = fterrors.WrapJiraError(t(i18n.StartWrapCreateIssue), err)
			return result
		}
		issueID = issue.Key
	} else if issueID == "" {
		result.Err = fterrors.NewExitCodeError(fterrors.ExitGeneral, t(i18n.StartErrIssueRequired))
		return result
	}

	out.Verbose(i18n.StartStatusSetProgress, issueID)
	setStatusStart := time.Now()
	issue, err = jiraClient.SetJiraIssueStatus(issueID, jira.StatusInProgress)
	logStartPhaseTiming(out, "set_issue_in_progress", setStatusStart)
	if err != nil {
		result.Err = fterrors.WrapJiraError(t(i18n.StartWrapSetStatus), err)
		return result
	}
	result.Issue = issue
	out.Info("jira", i18n.StartStatusIssueSet, issueID)

	if r.Options.NoBranch {
		resolveAssigneeStart := time.Now()
		r.Deps.ResolveIssueAssignee(out, jiraClient, issueID)
		logStartPhaseTiming(out, "resolve_assignee", resolveAssigneeStart)
		logStartPhaseTiming(out, "total", totalStart)
		out.Info("success", i18n.StartStatusNoBranchDone, issueID)
		return result
	}

	assigneeDone := make(chan struct{})
	resolveAssigneeStart := time.Now()
	go func() {
		defer close(assigneeDone)
		r.Deps.ResolveIssueAssignee(out, jiraClient, issueID)
	}()
	defer func() {
		<-assigneeDone
		logStartPhaseTiming(out, "resolve_assignee", resolveAssigneeStart)
	}()

	out.Verbose(i18n.StartStatusInitGit)
	initGitStart := time.Now()
	gitClient, err := r.Deps.NewGitClient(r.Config, statusCh)
	logStartPhaseTiming(out, "init_git", initGitStart)
	if err != nil {
		result.Err = fterrors.WrapGitError(t(i18n.StartWrapInitGit), err)
		return result
	}
	out.Debug(i18n.StartStatusGitInit)

	out.Verbose(i18n.StartStatusCheckChanges)
	checkChangesStart := time.Now()
	hasChanges, err := gitClient.HasUncommittedChanges()
	logStartPhaseTiming(out, "check_uncommitted_changes", checkChangesStart)
	if err != nil {
		result.Err = fterrors.WrapGitError(t(i18n.StartWrapCheckChanges), err)
		return result
	}

	if hasChanges {
		stashMessage := t(i18n.StartStashMessage)
		out.Info("package", i18n.StartStatusStashing)
		if err := gitClient.StashChanges(stashMessage); err != nil {
			result.Err = fterrors.WrapGitError(t(i18n.StartWrapStashChanges), err)
			return result
		}
		out.Info("package", i18n.StartStatusStashDone)
	} else {
		out.Debug(i18n.StartStatusClean)
	}

	branchName, worktreePath, err := r.createIssueBranch(out, gitClient, issue)
	if err != nil {
		result.Err = err
		return result
	}
	result.BranchName = branchName
	result.WorktreePath = worktreePath
	result.Created = true

	logStartPhaseTiming(out, "total", totalStart)
	out.Info("success", i18n.StartStatusSuccess, issueID)
	return result
}

func (r WorkflowRunner) loadInteractiveContext(issueID string) (interactiveContext, error) {
	ctx := interactiveContext{
		issueID:        issueID,
		needsSelection: issueID == "" && r.Options.Title == "",
	}

	initialKey := i18n.StartInitialFetch
	initialArgs := []any{}
	if issueID != "" {
		initialKey = i18n.StartInitialIssue
		initialArgs = []any{issueID}
	} else if r.Options.Title != "" {
		initialKey = i18n.StartInitialCreate
		initialArgs = []any{r.Options.Title}
	}

	err := r.Deps.RunWithSpinner(func(out WorkflowEmitter) error {
		out.Info("progress", initialKey, initialArgs...)
		var err error
		out.Verbose(i18n.StartStatusInitJira)
		initJiraStart := time.Now()
		ctx.jiraClient, err = r.Deps.NewJiraClient(r.Config)
		logStartPhaseTiming(out, "init_jira", initJiraStart)
		if err != nil {
			return fterrors.WrapJiraError(r.localize(i18n.StartWrapInitJira), err)
		}
		out.Debug(i18n.StartStatusJiraInit)

		if r.Options.Title != "" {
			createIssueStart := time.Now()
			ctx.issue, err = r.Deps.CreateNewIssue(out, ctx.jiraClient)
			logStartPhaseTiming(out, "create_issue", createIssueStart)
			if err != nil {
				return fterrors.WrapJiraError(r.localize(i18n.StartWrapCreateIssue), err)
			}
			ctx.issueID = ctx.issue.Key
			return nil
		}

		if !ctx.needsSelection {
			return nil
		}

		out.Verbose(i18n.StartStatusFetchIssues)
		fetchIssuesStart := time.Now()
		ctx.issues, err = ctx.jiraClient.GetUserOpenIssues()
		logStartPhaseTiming(out, "fetch_open_issues", fetchIssuesStart)
		if err != nil {
			return fmt.Errorf(r.localize(i18n.StartErrFetchOpenIssues), err)
		}
		if len(ctx.issues) == 0 {
			return errors.New(r.localize(i18n.StartErrNoOpenIssues))
		}
		out.Info("issue", i18n.StartStatusFoundIssues, len(ctx.issues))
		return nil
	})
	if err != nil {
		return interactiveContext{}, err
	}

	return ctx, nil
}

func (r WorkflowRunner) progressStartWorkflow(jiraClient jira.Jira, issue *jira.Issue, issueID string) error {
	return r.Deps.RunWithSpinner(func(out WorkflowEmitter) error {
		totalStart := time.Now()
		out.Info("progress", i18n.StartStatusSetProgress, issueID)

		var err error
		setStatusStart := time.Now()
		issue, err = jiraClient.SetJiraIssueStatus(issueID, jira.StatusInProgress)
		logStartPhaseTiming(out, "set_issue_in_progress", setStatusStart)
		if err != nil {
			return fterrors.WrapJiraError(r.localize(i18n.StartWrapSetStatus), err)
		}
		out.Info("jira", i18n.StartStatusIssueSet, issueID)

		if r.Options.NoBranch {
			resolveAssigneeStart := time.Now()
			r.Deps.ResolveIssueAssignee(out, jiraClient, issueID)
			logStartPhaseTiming(out, "resolve_assignee", resolveAssigneeStart)
			logStartPhaseTiming(out, "total", totalStart)
			out.Info("success", i18n.StartStatusNoBranchDone, issueID)
			return nil
		}

		assigneeDone := make(chan struct{})
		resolveAssigneeStart := time.Now()
		go func() {
			defer close(assigneeDone)
			r.Deps.ResolveIssueAssignee(out, jiraClient, issueID)
		}()
		defer func() {
			<-assigneeDone
			logStartPhaseTiming(out, "resolve_assignee", resolveAssigneeStart)
		}()

		out.Verbose(i18n.StartStatusInitGit)
		chForGit := make(chan string)
		gitDebugForwardDone := make(chan struct{})
		go func() {
			defer close(gitDebugForwardDone)
			for msg := range chForGit {
				out.DebugRaw(msg)
			}
		}()
		var closeGitDebugBridge sync.Once
		closeGitDebug := func() {
			closeGitDebugBridge.Do(func() {
				close(chForGit)
				<-gitDebugForwardDone
			})
		}
		defer closeGitDebug()
		initGitStart := time.Now()
		gitClient, err := r.Deps.NewGitClient(r.Config, &chForGit)
		logStartPhaseTiming(out, "init_git", initGitStart)
		if err != nil {
			return fterrors.WrapGitError(r.localize(i18n.StartWrapInitGit), err)
		}
		out.Debug(i18n.StartStatusGitInit)

		out.Verbose(i18n.StartStatusCheckChanges)
		checkChangesStart := time.Now()
		hasChanges, err := gitClient.HasUncommittedChanges()
		logStartPhaseTiming(out, "check_uncommitted_changes", checkChangesStart)
		if err != nil {
			return fterrors.WrapGitError(r.localize(i18n.StartWrapCheckChanges), err)
		}

		if hasChanges {
			out.Info("warning", i18n.StartStatusChangesFound)
			if err := r.Deps.StashChanges(out, gitClient); err != nil {
				return err
			}
		} else {
			out.Debug(i18n.StartStatusClean)
		}

		_, _, err = r.createIssueBranch(out, gitClient, issue)
		if err != nil {
			return err
		}
		logStartPhaseTiming(out, "total", totalStart)
		out.Info("success", i18n.StartStatusSuccess, issueID)
		return nil
	})
}

func logStartPhaseTiming(out WorkflowEmitter, phase string, start time.Time) {
	if out == nil {
		return
	}
	out.DebugRaw(fmt.Sprintf("start timing phase=%s duration=%s", phase, commandruntime.HumanizeDuration(time.Since(start))))
}

func normalizeWorkflowIssueID(issueID string, cfg *viper.Viper) string {
	jiraRoot := ""
	if cfg != nil {
		jiraRoot = cfg.GetString("jira.root")
	}

	return NormalizeIssueInput(issueID, jiraRoot)
}

func (r WorkflowRunner) createIssueBranch(out WorkflowEmitter, gitClient git.Git, issue *jira.Issue) (string, string, error) {
	out.Info("branch", i18n.StartStatusCreateBranch, issue.Key)
	fetchDefaultBranchStart := time.Now()
	if err := gitClient.FetchDefaultBranch(); err != nil {
		logStartPhaseTiming(out, "fetch_default_branch", fetchDefaultBranchStart)
		return "", "", fterrors.WrapGitError(r.localize(i18n.StartWrapCreateBranch), err)
	}
	logStartPhaseTiming(out, "fetch_default_branch", fetchDefaultBranchStart)

	if r.Options.Worktree {
		createIssueWorktreeStart := time.Now()
		branchName, worktreePath, err := gitClient.CreateIssueWorktreeBranch(issue)
		logStartPhaseTiming(out, "create_issue_worktree_branch", createIssueWorktreeStart)
		if err != nil {
			return "", "", fterrors.WrapGitError(r.localize(i18n.StartWrapCreateBranch), err)
		}
		out.Info("check", i18n.StartStatusBranchDone, branchName)
		out.Info("branch", i18n.StartStatusWorktreeDone, worktreePath)
		out.Info("success", i18n.StartStatusWorktreeReady, branchName, worktreePath)
		return branchName, worktreePath, nil
	}

	createIssueBranchStart := time.Now()
	branchName, err := gitClient.CreateIssueBranch(issue)
	logStartPhaseTiming(out, "create_issue_branch", createIssueBranchStart)
	if err != nil {
		return "", "", fterrors.WrapGitError(r.localize(i18n.StartWrapCreateBranch), err)
	}
	out.Info("check", i18n.StartStatusBranchDone, branchName)
	return branchName, "", nil
}

func (r WorkflowRunner) validateDeps() error {
	if r.Deps.NormalizeFlags == nil {
		return fmt.Errorf("start workflow dependency NormalizeFlags is required")
	}
	if r.Deps.NewJiraClient == nil {
		return fmt.Errorf("start workflow dependency NewJiraClient is required")
	}
	if r.Deps.CreateNewIssue == nil {
		return fmt.Errorf("start workflow dependency CreateNewIssue is required")
	}
	if r.Deps.SelectIssueWithPicker == nil {
		return fmt.Errorf("start workflow dependency SelectIssueWithPicker is required")
	}
	if r.Deps.RunWithSpinner == nil {
		return fmt.Errorf("start workflow dependency RunWithSpinner is required")
	}
	if r.Deps.ResolveIssueAssignee == nil {
		return fmt.Errorf("start workflow dependency ResolveIssueAssignee is required")
	}
	if r.Deps.NewGitClient == nil {
		return fmt.Errorf("start workflow dependency NewGitClient is required")
	}
	if r.Deps.StashChanges == nil {
		return fmt.Errorf("start workflow dependency StashChanges is required")
	}
	return nil
}

func (r WorkflowRunner) localize(key i18n.Key, args ...any) string {
	if r.Localize != nil {
		return r.Localize(key, args...)
	}
	return i18n.T(key, args...)
}

type noopWorkflowEmitter struct{}

func (noopWorkflowEmitter) Info(string, i18n.Key, ...any) {}
func (noopWorkflowEmitter) Verbose(i18n.Key, ...any)      {}
func (noopWorkflowEmitter) Debug(i18n.Key, ...any)        {}
func (noopWorkflowEmitter) DebugRaw(string)               {}
