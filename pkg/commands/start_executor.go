package commands

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalstart "github.com/tagoro9/fotingo/internal/commands/start"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

type startExecutorDeps struct {
	normalizeFlags        func(*cobra.Command, string) error
	newJiraClient         func(*viper.Viper) (jira.Jira, error)
	createNewIssue        func(commandruntime.LocalizedEmitter, jira.Jira) (*jira.Issue, error)
	selectIssueWithPicker func([]tracker.Issue) (*tracker.Issue, error)
	runWithSpinner        func(func(commandruntime.LocalizedEmitter) error) error
	resolveIssueAssignee  func(commandruntime.LocalizedEmitter, jira.Jira, string)
	newGitClient          func(*viper.Viper, *chan string) (git.Git, error)
	stashChanges          func(commandruntime.LocalizedEmitter, git.Git) error
}

func defaultStartExecutorDeps() startExecutorDeps {
	return startExecutorDeps{
		normalizeFlags:        normalizeStartCreateFlags,
		newJiraClient:         newJiraClient,
		createNewIssue:        createNewIssue,
		selectIssueWithPicker: selectIssueWithPicker,
		runWithSpinner:        runWithSpinner,
		resolveIssueAssignee:  resolveStartIssueAssignee,
		newGitClient:          newGitClient,
		stashChanges:          stashChanges,
	}
}

type startExecutor struct {
	deps startExecutorDeps
}

func newStartExecutor() startExecutor {
	return newStartExecutorWithDeps(startExecutorDeps{})
}

func newStartExecutorWithDeps(deps startExecutorDeps) startExecutor {
	defaults := defaultStartExecutorDeps()

	if deps.normalizeFlags == nil {
		deps.normalizeFlags = defaults.normalizeFlags
	}
	if deps.newJiraClient == nil {
		deps.newJiraClient = defaults.newJiraClient
	}
	if deps.createNewIssue == nil {
		deps.createNewIssue = defaults.createNewIssue
	}
	if deps.selectIssueWithPicker == nil {
		deps.selectIssueWithPicker = defaults.selectIssueWithPicker
	}
	if deps.runWithSpinner == nil {
		deps.runWithSpinner = defaults.runWithSpinner
	}
	if deps.resolveIssueAssignee == nil {
		deps.resolveIssueAssignee = defaults.resolveIssueAssignee
	}
	if deps.newGitClient == nil {
		deps.newGitClient = defaults.newGitClient
	}
	if deps.stashChanges == nil {
		deps.stashChanges = defaults.stashChanges
	}

	return startExecutor{deps: deps}
}

func (e startExecutor) runInteractive(cmd *cobra.Command, issueID string) error {
	runner := e.newWorkflowRunner()
	return runner.RunInteractive(cmd, issueID)
}

func (e startExecutor) runWithResult(cmd *cobra.Command, statusCh *chan string, issueID string) startResult {
	runner := e.newWorkflowRunner()
	workflowResult := runner.RunWithResult(cmd, statusCh, issueID, startWorkflowEmitter{out: commandruntime.NewLocalizedEmitter(*statusCh, shouldEmitCommandLevel, localizer.T)})
	return startResult{
		issue:      workflowResult.Issue,
		branchName: workflowResult.BranchName,
		created:    workflowResult.Created,
		err:        workflowResult.Err,
	}
}

func (e startExecutor) newWorkflowRunner() internalstart.WorkflowRunner {
	return internalstart.WorkflowRunner{
		Config:   fotingoConfig,
		Localize: localizer.T,
		Options: internalstart.WorkflowOptions{
			Title:    startCmdFlags.title,
			NoBranch: startCmdFlags.noBranch,
		},
		Deps: internalstart.WorkflowDeps{
			NormalizeFlags: e.deps.normalizeFlags,
			NewJiraClient:  e.deps.newJiraClient,
			CreateNewIssue: func(out internalstart.WorkflowEmitter, jiraClient jira.Jira) (*jira.Issue, error) {
				emitter, err := startWorkflowStatusEmitter(out)
				if err != nil {
					return nil, err
				}
				return e.deps.createNewIssue(emitter, jiraClient)
			},
			SelectIssueWithPicker: e.deps.selectIssueWithPicker,
			RunWithSpinner: func(work func(internalstart.WorkflowEmitter) error) error {
				return e.deps.runWithSpinner(func(out commandruntime.LocalizedEmitter) error {
					return work(startWorkflowEmitter{out: out})
				})
			},
			ResolveIssueAssignee: func(out internalstart.WorkflowEmitter, jiraClient jira.Jira, issueID string) {
				emitter, err := startWorkflowStatusEmitter(out)
				if err != nil {
					return
				}
				e.deps.resolveIssueAssignee(emitter, jiraClient, issueID)
			},
			NewGitClient: e.deps.newGitClient,
			StashChanges: func(out internalstart.WorkflowEmitter, gitClient git.Git) error {
				emitter, err := startWorkflowStatusEmitter(out)
				if err != nil {
					return err
				}
				return e.deps.stashChanges(emitter, gitClient)
			},
		},
	}
}

type startWorkflowEmitter struct {
	out commandruntime.LocalizedEmitter
}

func (e startWorkflowEmitter) Info(emoji string, key i18n.Key, args ...any) {
	e.out.Info(startWorkflowEmoji(emoji), key, args...)
}

func (e startWorkflowEmitter) Verbose(key i18n.Key, args ...any) {
	e.out.Verbose(key, args...)
}

func (e startWorkflowEmitter) Debug(key i18n.Key, args ...any) {
	e.out.Debug(key, args...)
}

func (e startWorkflowEmitter) DebugRaw(message string) {
	e.out.DebugRaw(message)
}

func startWorkflowStatusEmitter(out internalstart.WorkflowEmitter) (commandruntime.LocalizedEmitter, error) {
	emitter, ok := out.(startWorkflowEmitter)
	if !ok {
		return commandruntime.LocalizedEmitter{}, errors.New("unexpected start workflow emitter")
	}
	return emitter.out, nil
}

func startWorkflowEmoji(emoji string) commandruntime.LogEmoji {
	switch emoji {
	case "progress":
		return commandruntime.LogEmojiProgress
	case "issue":
		return commandruntime.LogEmojiIssue
	case "jira":
		return commandruntime.LogEmojiJira
	case "warning":
		return commandruntime.LogEmojiWarning
	case "package":
		return commandruntime.LogEmojiPackage
	case "branch":
		return commandruntime.LogEmojiBranch
	case "check":
		return commandruntime.LogEmojiCheck
	case "success":
		return commandruntime.LogEmojiSuccess
	default:
		return commandruntime.LogEmojiInfo
	}
}
