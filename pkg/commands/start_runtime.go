package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalstart "github.com/tagoro9/fotingo/internal/commands/start"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/tracker"
	"github.com/tagoro9/fotingo/internal/ui"
)

var startSelectOneFn = ui.SelectOne

// runStartInteractive runs the start command with phased TUI programs
// to avoid conflicts between spinner rendering and interactive prompts.
func runStartInteractive(cmd *cobra.Command, issueId string) error {
	return newStartExecutor().runInteractive(cmd, issueId)
}

// stashChanges stashes uncommitted changes.
func stashChanges(out commandruntime.LocalizedEmitter, gitClient git.Git) error {
	stashMessage := localizer.T(i18n.StartStashMessage)
	out.Info(commandruntime.LogEmojiPackage, i18n.StartStatusStashing)

	if err := gitClient.StashChanges(stashMessage); err != nil {
		return fmt.Errorf(localizer.T(i18n.StartErrStashChanges), err)
	}

	out.Info(commandruntime.LogEmojiPackage, i18n.StartStatusStashDone)
	return nil
}

// selectIssueWithPicker shows an interactive picker for issue selection.
func selectIssueWithPicker(issues []tracker.Issue) (*tracker.Issue, error) {
	selected, err := internalstart.SelectIssueWithPicker(
		issues,
		localizer.T(i18n.StartPickerTitle),
		startSelectOneFn,
		getStatusIndicator,
		getIssueTypeIcon,
	)
	if err != nil {
		return nil, fmt.Errorf(localizer.T(i18n.StartErrPickerRun), err)
	}
	if selected == nil {
		return nil, errors.New(localizer.T(i18n.StartErrSelectionCancel))
	}
	return selected, nil
}

func selectIssueLinkWithPicker(issues []tracker.Issue, title string) (*tracker.Issue, error) {
	selected, err := internalstart.SelectIssueLinkWithPicker(
		issues,
		title,
		startSelectOneFn,
	)
	if err != nil {
		return nil, fmt.Errorf(localizer.T(i18n.StartErrPickerRun), err)
	}
	return selected, nil
}
