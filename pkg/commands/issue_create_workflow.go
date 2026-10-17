package commands

import (
	"errors"
	"fmt"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
	"strings"
)

type issueCreateOptions struct {
	title, description, project, kind, parent, epic string
	labels                                          []string
	interactive                                     bool
}

// createIssue shares ticket creation and link resolution between start and issue create.
func createIssue(out commandruntime.LocalizedEmitter, jiraClient jira.Jira, options issueCreateOptions) (*jira.Issue, error) {
	if options.project == "" {
		return nil, errors.New(localizer.T(i18n.StartErrProjectRequired))
	}

	issueType, err := parseIssueKind(options.kind)
	if err != nil {
		return nil, err
	}

	typeName := strings.TrimSpace(options.kind)
	if options.parent != "" {
		if issueType != tracker.IssueTypeSubTask {
			typeName = ""
		}
		issueType = tracker.IssueTypeSubTask
	}

	resolvedParent := strings.TrimSpace(options.parent)
	resolvedEpic := strings.TrimSpace(options.epic)
	isInteractiveResolution := options.interactive

	if issueType == tracker.IssueTypeSubTask {
		if resolvedParent == "" {
			return nil, errors.New(localizer.T(i18n.StartErrParentRequired))
		}

		parent, err := resolveIssueLink(
			jiraClient,
			options.project,
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
			options.project,
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

	out.Debug(i18n.StartStatusCreateIssue, options.kind, options.project)

	createInput := tracker.CreateIssueInput{
		Title:       options.title,
		Description: options.description,
		Project:     strings.ToUpper(options.project),
		Type:        issueType,
		TypeName:    typeName,
		ParentID:    resolvedParent,
		EpicID:      resolvedEpic,
		Labels:      options.labels,
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
