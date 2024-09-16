package start

import (
	"fmt"

	"github.com/tagoro9/fotingo/internal/tracker"
	"github.com/tagoro9/fotingo/internal/ui"
)

// RunPickerFunc runs a picker with a title and item list.
type RunPickerFunc func(title string, items []ui.PickerItem) (*ui.PickerItem, error)

// SelectIssueWithPicker presents issues and returns the selected issue.
func SelectIssueWithPicker(
	issues []tracker.Issue,
	title string,
	runPicker RunPickerFunc,
	statusIndicator func(tracker.IssueStatus) string,
	issueTypeIcon func(tracker.IssueType) string,
) (*tracker.Issue, error) {
	items := make([]ui.PickerItem, len(issues))
	for i, issue := range issues {
		items[i] = ui.PickerItem{
			ID:     issue.Key,
			Label:  fmt.Sprintf("%s %s", issue.Key, issue.Summary),
			Detail: statusIndicator(issue.Status),
			Icon:   issueTypeIcon(issue.Type),
			Value:  i,
		}
	}

	selected, err := runPicker(title, items)
	if err != nil {
		return nil, err
	}
	if selected == nil {
		return nil, nil
	}

	idx, _ := selected.Value.(int)
	if idx < 0 || idx >= len(issues) {
		return nil, nil
	}
	return &issues[idx], nil
}

// SelectIssueLinkWithPicker presents issue-link candidates and returns the selected issue.
func SelectIssueLinkWithPicker(
	issues []tracker.Issue,
	title string,
	runPicker RunPickerFunc,
) (*tracker.Issue, error) {
	items := make([]ui.PickerItem, len(issues))
	for i, issue := range issues {
		items[i] = ui.PickerItem{
			ID:    issue.Key,
			Label: fmt.Sprintf("%s %s", issue.Key, issue.Summary),
			Value: i,
		}
	}

	selected, err := runPicker(title, items)
	if err != nil {
		return nil, err
	}
	if selected == nil {
		return nil, nil
	}

	idx, _ := selected.Value.(int)
	if idx < 0 || idx >= len(issues) {
		return nil, nil
	}
	return &issues[idx], nil
}
