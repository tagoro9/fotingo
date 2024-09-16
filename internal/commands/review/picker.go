package review

import (
	"fmt"

	"github.com/tagoro9/fotingo/internal/ui"
)

// RunPickerFunc runs a picker with a title and list of items.
type RunPickerFunc func(title string, items []ui.PickerItem) (*ui.PickerItem, error)

// PickMatchWithPicker maps review match options to picker items and returns a selected value.
func PickMatchWithPicker(
	kind string,
	token string,
	matches []MatchOption,
	runPicker RunPickerFunc,
) (string, error) {
	if runPicker == nil {
		return "", fmt.Errorf("missing picker runner for %s", kind)
	}

	items := make([]ui.PickerItem, len(matches))
	for i, match := range matches {
		items[i] = ui.PickerItem{
			ID:     match.Resolved,
			Label:  match.Label,
			Detail: match.Detail,
			Value:  i,
		}
	}

	title := fmt.Sprintf("Select %s for %q", kind, token)
	selected, err := runPicker(title, items)
	if err != nil {
		return "", err
	}
	if selected == nil {
		return "", fmt.Errorf("selection cancelled for %s %q", kind, token)
	}

	return selected.ID, nil
}
