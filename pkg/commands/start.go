package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	internalstart "github.com/tagoro9/fotingo/internal/commands/start"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
	"github.com/tagoro9/fotingo/internal/ui"
)

// startFlags holds the flags for the start command
type startFlags struct {
	title       string
	description string
	project     string
	kind        string
	parent      string
	epic        string
	labels      []string
	noBranch    bool
	interactive bool
}

var startCmdFlags = startFlags{}

var startIsInteractiveTerminalFn = func() bool {
	stdinInfo, stdinErr := os.Stdin.Stat()
	if stdinErr != nil {
		return false
	}
	stdoutInfo, stdoutErr := os.Stdout.Stat()
	if stdoutErr != nil {
		return false
	}

	return (stdinInfo.Mode()&os.ModeCharDevice) != 0 && (stdoutInfo.Mode()&os.ModeCharDevice) != 0
}

var startPromptInputFn = func(prompt string, placeholder string, required bool, initialValue string) (string, bool, error) {
	opts := []ui.InputOption{
		ui.WithPrompt(prompt),
		ui.WithPlaceholder(placeholder),
		ui.WithInitialValue(initialValue),
	}
	if required {
		opts = append(opts, ui.WithValidation(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New(localizer.T(i18n.StartErrPromptRequired))
			}
			return nil
		}))
	}

	program := ui.NewInputProgram(opts...)
	return program.RunWithCancel()
}

var startPromptMultilineInputFn = func(prompt string, placeholder string, required bool, initialValue string) (string, bool, error) {
	opts := []ui.InputOption{
		ui.WithPrompt(prompt),
		ui.WithPlaceholder(placeholder),
		ui.WithInitialValue(initialValue),
		ui.WithMultiline(6),
	}
	if required {
		opts = append(opts, ui.WithValidation(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New(localizer.T(i18n.StartErrPromptRequired))
			}
			return nil
		}))
	}

	program := ui.NewInputProgram(opts...)
	return program.RunWithCancel()
}

var startSelectKindFn = func(kinds []string, currentKind string) (string, bool, error) {
	items := make([]ui.PickerItem, len(kinds))
	for idx, kind := range kinds {
		items[idx] = ui.PickerItem{
			ID:    kind,
			Label: kind,
			Value: kind,
		}
	}

	picker := ui.NewPickerProgram(
		ui.WithPickerTitle(localizer.T(i18n.StartPickerIssueTypeTitle)),
		ui.WithPickerItems(items),
		ui.WithPickerSearch(true),
	)

	selected, err := picker.Run()
	if err != nil {
		return "", false, err
	}
	if selected == nil {
		return "", true, nil
	}

	value, _ := selected.Value.(string)
	if strings.TrimSpace(value) == "" {
		value = selected.ID
	}
	if strings.TrimSpace(value) == "" {
		value = currentKind
	}
	return value, false, nil
}

var startConfirmFn = func(prompt string, defaultYes bool) (bool, error) {
	return ui.Confirm(prompt, defaultYes)
}

var startProjectIssueTypeNamesFn = getInteractiveProjectIssueTypeNames
var startSelectIssueLinkFn = selectIssueLinkWithPicker

func init() {
	Fotingo.AddCommand(startCmd)

	startCmd.Flags().StringVarP(&startCmdFlags.title, "title", "t", "", localizer.T(i18n.StartFlagTitle))
	startCmd.Flags().StringVarP(&startCmdFlags.description, "description", "d", "", localizer.T(i18n.StartFlagDescription))
	startCmd.Flags().StringVarP(&startCmdFlags.project, "project", "p", "", localizer.T(i18n.StartFlagProject))
	startCmd.Flags().StringVarP(&startCmdFlags.kind, "kind", "k", "Task", localizer.T(i18n.StartFlagKind))
	startCmd.Flags().StringVarP(&startCmdFlags.parent, "parent", "a", "", localizer.T(i18n.StartFlagParent))
	startCmd.Flags().StringVarP(&startCmdFlags.epic, "epic", "e", "", localizer.T(i18n.StartFlagEpic))
	startCmd.Flags().StringSliceVarP(&startCmdFlags.labels, "labels", "l", []string{}, localizer.T(i18n.StartFlagLabels))
	startCmd.Flags().BoolVarP(&startCmdFlags.noBranch, "no-branch", "n", false, localizer.T(i18n.StartFlagNoBranch))
	startCmd.Flags().BoolVarP(&startCmdFlags.interactive, "interactive", "i", false, localizer.T(i18n.StartFlagInteractive))
	_ = startCmd.RegisterFlagCompletionFunc("project", completeStartProjectFlag)
	_ = startCmd.RegisterFlagCompletionFunc("kind", completeStartIssueTypeFlag)
}

var startCmd = &cobra.Command{
	Use:   i18n.T(i18n.StartUse),
	Short: i18n.T(i18n.StartShort),
	Long:  i18n.T(i18n.StartLong),
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureJiraRootConfigured(); err != nil {
			return err
		}

		// Determine issue ID from args or selection
		var issueId string
		if len(args) > 0 {
			issueId = args[0]
		}

		// JSON mode: run without TUI and output JSON result
		if ShouldOutputJSON() {
			// Create a no-op status channel for JSON mode
			statusCh := make(chan string, 100)
			go func() {
				for range statusCh {
				}
			}()

			result := runStartWithResult(cmd, &statusCh, issueId)
			close(statusCh)
			return outputStartJSON(result)
		}

		if err := normalizeStartCreateFlags(cmd, issueId); err != nil {
			return err
		}

		if err := collectInteractiveCreateFlags(cmd); err != nil {
			return err
		}

		return runStartInteractive(cmd, issueId)
	},
}

func normalizeStartCreateFlags(cmd *cobra.Command, issueID string) error {
	startCmdFlags.title = strings.TrimSpace(startCmdFlags.title)
	startCmdFlags.description = strings.TrimSpace(startCmdFlags.description)
	startCmdFlags.project = strings.TrimSpace(startCmdFlags.project)
	startCmdFlags.kind = strings.TrimSpace(startCmdFlags.kind)
	startCmdFlags.parent = strings.TrimSpace(startCmdFlags.parent)
	startCmdFlags.epic = strings.TrimSpace(startCmdFlags.epic)

	if cmd.Flags().Changed("kind") && startCmdFlags.kind != "" {
		if _, err := parseIssueKind(startCmdFlags.kind); err != nil {
			return err
		}
	}

	createFlagsPresent := startCmdFlags.title != "" ||
		startCmdFlags.description != "" ||
		startCmdFlags.project != "" ||
		startCmdFlags.parent != "" ||
		startCmdFlags.epic != "" ||
		len(startCmdFlags.labels) > 0
	if issueID != "" && (startCmdFlags.interactive || createFlagsPresent) {
		return errors.New(localizer.T(i18n.StartErrIssueCreateConflict))
	}

	if startCmdFlags.interactive && ShouldOutputJSON() {
		return errors.New(localizer.T(i18n.StartErrInteractiveJSON))
	}

	return nil
}

func collectInteractiveCreateFlags(cmd *cobra.Command) error {
	if !startCmdFlags.interactive {
		return nil
	}

	if !startIsInteractiveTerminalFn() {
		return errors.New(localizer.T(i18n.StartErrInteractiveTTY))
	}

	if startCmdFlags.title == "" {
		title, cancelled, err := startPromptInputFn(
			localizer.T(i18n.StartPromptTitle),
			localizer.T(i18n.StartPromptTitlePlaceholder),
			true,
			"",
		)
		if err != nil {
			return err
		}
		if cancelled {
			return errors.New(localizer.T(i18n.StartErrCancelled))
		}
		startCmdFlags.title = strings.TrimSpace(title)
	}

	if startCmdFlags.description == "" {
		description, cancelled, err := startPromptMultilineInputFn(
			localizer.T(i18n.StartPromptDescription),
			localizer.T(i18n.StartPromptDescriptionPlaceholder),
			false,
			"",
		)
		if err != nil {
			return err
		}
		if cancelled {
			return errors.New(localizer.T(i18n.StartErrCancelled))
		}
		startCmdFlags.description = strings.TrimSpace(description)
	}

	if startCmdFlags.project == "" {
		project, cancelled, err := startPromptInputFn(
			localizer.T(i18n.StartPromptProject),
			localizer.T(i18n.StartPromptProjectPlaceholder),
			true,
			"",
		)
		if err != nil {
			return err
		}
		if cancelled {
			return errors.New(localizer.T(i18n.StartErrCancelled))
		}
		startCmdFlags.project = strings.ToUpper(strings.TrimSpace(project))
	}

	if !cmd.Flags().Changed("kind") {
		issueTypeNames, err := startProjectIssueTypeNamesFn(startCmdFlags.project)
		if err != nil {
			return err
		}

		kind, cancelled, err := startSelectKindFn(issueTypeNames, startCmdFlags.kind)
		if err != nil {
			return err
		}
		if cancelled {
			return errors.New(localizer.T(i18n.StartErrCancelled))
		}
		startCmdFlags.kind = strings.TrimSpace(kind)
	}

	jiraClient, err := newJiraClient(fotingoConfig)
	if err != nil {
		return fterrors.WrapJiraError(localizer.T(i18n.StartWrapInitJira), err)
	}

	issueType, err := parseIssueKind(startCmdFlags.kind)
	if err != nil {
		return err
	}

	if issueType == tracker.IssueTypeSubTask {
		if startCmdFlags.parent == "" {
			parentQuery, cancelled, promptErr := startPromptInputFn(
				localizer.T(i18n.StartPromptParent),
				localizer.T(i18n.StartPromptParentPlaceholder),
				true,
				"",
			)
			if promptErr != nil {
				return promptErr
			}
			if cancelled {
				return errors.New(localizer.T(i18n.StartErrCancelled))
			}
			startCmdFlags.parent = strings.TrimSpace(parentQuery)
		}

		resolvedParent, err := resolveIssueLink(
			jiraClient,
			startCmdFlags.project,
			startCmdFlags.parent,
			[]tracker.IssueType{},
			true,
			localizer.T(i18n.StartPickerParentTitle),
		)
		if err != nil {
			return fmt.Errorf(localizer.T(i18n.StartErrResolveParent), err)
		}
		startCmdFlags.parent = resolvedParent
		startCmdFlags.epic = ""
	} else {
		if startCmdFlags.epic == "" {
			epicQuery, cancelled, promptErr := startPromptInputFn(
				localizer.T(i18n.StartPromptEpic),
				localizer.T(i18n.StartPromptEpicPlaceholder),
				false,
				"",
			)
			if promptErr != nil {
				return promptErr
			}
			if cancelled {
				return errors.New(localizer.T(i18n.StartErrCancelled))
			}
			startCmdFlags.epic = strings.TrimSpace(epicQuery)
		}

		if strings.TrimSpace(startCmdFlags.epic) != "" {
			resolvedEpic, err := resolveIssueLink(
				jiraClient,
				startCmdFlags.project,
				startCmdFlags.epic,
				[]tracker.IssueType{tracker.IssueTypeEpic},
				true,
				localizer.T(i18n.StartPickerEpicTitle),
			)
			if err != nil {
				return fmt.Errorf(localizer.T(i18n.StartErrResolveEpic), err)
			}
			startCmdFlags.epic = resolvedEpic
		}
	}

	if !cmd.Flags().Changed("labels") {
		labelsRaw, cancelled, err := startPromptInputFn(
			localizer.T(i18n.StartPromptLabels),
			localizer.T(i18n.StartPromptLabelsPlaceholder),
			false,
			"",
		)
		if err != nil {
			return err
		}
		if cancelled {
			return errors.New(localizer.T(i18n.StartErrCancelled))
		}
		startCmdFlags.labels = internalstart.ParseInteractiveLabels(labelsRaw)
	}

	if !cmd.Flags().Changed("no-branch") {
		noBranch, err := startConfirmFn(localizer.T(i18n.StartPromptNoBranch), false)
		if err != nil {
			return err
		}
		startCmdFlags.noBranch = noBranch
	}

	return nil
}

func getInteractiveProjectIssueTypeNames(projectKey string) ([]string, error) {
	jiraClient, err := newJiraClient(fotingoConfig)
	if err != nil {
		return nil, fterrors.WrapJiraError(localizer.T(i18n.StartWrapInitJira), err)
	}

	issueTypes, err := jiraClient.GetProjectIssueTypes(projectKey)
	if err != nil {
		return nil, fmt.Errorf(localizer.T(i18n.StartErrFetchIssueTypes), projectKey, err)
	}

	names := internalstart.ProjectIssueTypeNames(issueTypes)

	if len(names) == 0 {
		return nil, fmt.Errorf(localizer.T(i18n.StartErrFetchIssueTypes), projectKey, errors.New("no issue types returned"))
	}

	return names, nil
}

func resolveIssueLink(
	jiraClient jira.Jira,
	projectKey string,
	raw string,
	issueTypes []tracker.IssueType,
	interactive bool,
	pickerTitle string,
) (string, error) {
	return internalstart.ResolveIssueLink(jiraClient, internalstart.ResolveIssueLinkOptions{
		ProjectKey:   projectKey,
		JiraRoot:     fotingoConfig.GetString("jira.root"),
		RawQuery:     raw,
		AllowedTypes: issueTypes,
		Interactive:  interactive,
		PickerTitle:  pickerTitle,
		SelectIssueLink: func(candidates []tracker.Issue, title string) (*tracker.Issue, error) {
			return startSelectIssueLinkFn(candidates, title)
		},
		PromptRefineLink: func(currentQuery string) (string, bool, error) {
			return startPromptInputFn(
				localizer.T(i18n.StartPromptRefineSearch),
				localizer.T(i18n.StartPromptRefineSearchPlaceholder),
				true,
				currentQuery,
			)
		},
		Errors: internalstart.ResolveIssueLinkErrors{
			QueryRequired: func() error {
				return errors.New(localizer.T(i18n.StartErrLinkQueryRequired))
			},
			SearchIssues: func(err error) error {
				return fmt.Errorf(localizer.T(i18n.StartErrSearchIssues), err)
			},
			LinkNotFound: func(query string) error {
				return fmt.Errorf(localizer.T(i18n.StartErrLinkNotFound), query)
			},
			LinkAmbiguous: func(query string) error {
				return fmt.Errorf(localizer.T(i18n.StartErrLinkAmbiguous), query)
			},
			Cancelled: func() error {
				return errors.New(localizer.T(i18n.StartErrCancelled))
			},
		},
	})
}
