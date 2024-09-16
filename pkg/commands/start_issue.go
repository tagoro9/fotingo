package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalstart "github.com/tagoro9/fotingo/internal/commands/start"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

func resolveStartIssueAssignee(out commandruntime.LocalizedEmitter, jiraClient jira.Jira, issueID string) {
	out.Verbose(i18n.StartStatusResolveAssignee)

	issue, err := jiraClient.GetIssue(issueID)
	if err != nil {
		out.Info(commandruntime.LogEmojiWarning, i18n.StartStatusAssigneeLookupWarn, err)
		return
	}

	currentUser, err := jiraClient.GetCurrentUser()
	if err != nil {
		out.Info(commandruntime.LogEmojiWarning, i18n.StartStatusAssigneeLookupWarn, err)
		return
	}
	if currentUser == nil || strings.TrimSpace(currentUser.ID) == "" {
		out.Info(commandruntime.LogEmojiWarning, i18n.StartStatusAssigneeLookupWarn, errors.New("current user is unavailable"))
		return
	}

	if issue.Assignee == nil {
		out.Info(commandruntime.LogEmojiJira, i18n.StartStatusAssignSelf, currentUser.Name)
		if _, err := jiraClient.AssignIssue(issueID, currentUser.ID); err != nil {
			out.Info(commandruntime.LogEmojiWarning, i18n.StartStatusAssignSelfWarn, err)
			return
		}
		out.Info(commandruntime.LogEmojiCheck, i18n.StartStatusAssignSelfDone, currentUser.Name)
		return
	}

	if issue.Assignee.ID == currentUser.ID {
		out.Debug(i18n.StartStatusAssigneeSelf)
		return
	}

	displayName := strings.TrimSpace(issue.Assignee.Name)
	if displayName == "" {
		displayName = strings.TrimSpace(issue.Assignee.ID)
	}
	out.Info(commandruntime.LogEmojiWarning, i18n.StartStatusAssigneeOtherWarn, displayName)
}

// createNewIssue creates a new Jira issue based on the provided flags.
func createNewIssue(out commandruntime.LocalizedEmitter, jiraClient jira.Jira) (*jira.Issue, error) {
	if startCmdFlags.project == "" {
		return nil, errors.New(localizer.T(i18n.StartErrProjectRequired))
	}

	issueType, err := parseIssueKind(startCmdFlags.kind)
	if err != nil {
		return nil, err
	}

	if startCmdFlags.parent != "" {
		issueType = tracker.IssueTypeSubTask
	}

	resolvedParent := strings.TrimSpace(startCmdFlags.parent)
	resolvedEpic := strings.TrimSpace(startCmdFlags.epic)
	isInteractiveResolution := startCmdFlags.interactive

	if issueType == tracker.IssueTypeSubTask {
		if resolvedParent == "" {
			return nil, errors.New(localizer.T(i18n.StartErrParentRequired))
		}

		parent, err := resolveIssueLink(
			jiraClient,
			startCmdFlags.project,
			resolvedParent,
			[]tracker.IssueType{},
			isInteractiveResolution,
			localizer.T(i18n.StartPickerParentTitle),
		)
		if err != nil {
			return nil, fmt.Errorf(localizer.T(i18n.StartErrResolveParent), err)
		}
		resolvedParent = parent
		resolvedEpic = ""
	} else if resolvedEpic != "" {
		epic, err := resolveIssueLink(
			jiraClient,
			startCmdFlags.project,
			resolvedEpic,
			[]tracker.IssueType{tracker.IssueTypeEpic},
			isInteractiveResolution,
			localizer.T(i18n.StartPickerEpicTitle),
		)
		if err != nil {
			return nil, fmt.Errorf(localizer.T(i18n.StartErrResolveEpic), err)
		}
		resolvedEpic = epic
	}

	out.Debug(i18n.StartStatusCreateIssue, startCmdFlags.kind, startCmdFlags.project)

	createInput := tracker.CreateIssueInput{
		Title:       startCmdFlags.title,
		Description: startCmdFlags.description,
		Project:     strings.ToUpper(startCmdFlags.project),
		Type:        issueType,
		TypeName:    strings.TrimSpace(startCmdFlags.kind),
		ParentID:    resolvedParent,
		EpicID:      resolvedEpic,
		Labels:      startCmdFlags.labels,
	}

	trackerIssue, err := jiraClient.CreateIssue(createInput)
	if err != nil {
		return nil, fmt.Errorf(localizer.T(i18n.StartErrCreateIssue), err)
	}

	out.Info(commandruntime.LogEmojiIssue, i18n.StartStatusIssueCreated, trackerIssue.Key, trackerIssue.Summary)

	issue, err := jiraClient.GetJiraIssue(trackerIssue.Key)
	if err != nil {
		return nil, fmt.Errorf(localizer.T(i18n.StartErrGetCreatedIssue), err)
	}

	return issue, nil
}

// parseIssueKind converts the kind flag value to a tracker.IssueType.
func parseIssueKind(kind string) (tracker.IssueType, error) {
	issueType, ok := internalstart.ParseIssueKind(kind)
	if !ok {
		return "", fmt.Errorf(localizer.T(i18n.StartErrInvalidType), kind)
	}
	return issueType, nil
}

// getIssueTypeIcon returns an icon for the issue type.
func getIssueTypeIcon(issueType tracker.IssueType) string {
	switch issueType {
	case tracker.IssueTypeStory:
		return localizer.T(i18n.StartIssueTypeStory)
	case tracker.IssueTypeBug:
		return localizer.T(i18n.StartIssueTypeBug)
	case tracker.IssueTypeTask:
		return localizer.T(i18n.StartIssueTypeTask)
	case tracker.IssueTypeSubTask:
		return localizer.T(i18n.StartIssueTypeSubTask)
	case tracker.IssueTypeEpic:
		return localizer.T(i18n.StartIssueTypeEpic)
	default:
		return ""
	}
}

// getStatusIndicator returns a visual indicator for the issue status.
func getStatusIndicator(status tracker.IssueStatus) string {
	switch status {
	case tracker.IssueStatusBacklog:
		return localizer.T(i18n.StartStatusBacklog)
	case tracker.IssueStatusToDo:
		return localizer.T(i18n.StartStatusToDo)
	case tracker.IssueStatusInProgress:
		return localizer.T(i18n.StartStatusInProgress)
	case tracker.IssueStatusInReview:
		return localizer.T(i18n.StartStatusInReview)
	case tracker.IssueStatusDone:
		return localizer.T(i18n.StartStatusDone)
	default:
		return ""
	}
}
