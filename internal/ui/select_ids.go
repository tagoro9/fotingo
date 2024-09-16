package ui

// MultiSelectItem represents a selection candidate used by shared multi-select helpers.
type MultiSelectItem struct {
	ID     string
	Label  string
	Detail string
	Icon   string
}

var runMultiSelectIDsProgram = func(opts ...MultiSelectOption) ([]PickerItem, error) {
	program := NewMultiSelectProgram(opts...)
	return program.Run()
}

// SelectIDs renders a shared multi-select prompt and returns selected item IDs.
func SelectIDs(title string, items []MultiSelectItem, minimum int) ([]string, error) {
	pickerItems := make([]PickerItem, 0, len(items))
	for _, item := range items {
		pickerItems = append(pickerItems, PickerItem{
			ID:     item.ID,
			Label:  item.Label,
			Detail: item.Detail,
			Icon:   item.Icon,
			Value:  item.ID,
		})
	}

	selected, err := runMultiSelectIDsProgram(
		WithMultiSelectTitle(title),
		WithMultiSelectItems(pickerItems),
		WithMultiSelectSearch(true),
		WithMultiSelectMinimum(minimum),
	)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(selected))
	for _, item := range selected {
		ids = append(ids, item.ID)
	}
	return ids, nil
}
