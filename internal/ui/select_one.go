package ui

var runSelectOneProgram = func(opts ...PickerOption) (*PickerItem, error) {
	return NewPickerProgram(opts...).Run()
}

// SelectOne renders a searchable single-select picker and returns the selected item.
func SelectOne(title string, items []PickerItem) (*PickerItem, error) {
	return runSelectOneProgram(
		WithPickerTitle(title),
		WithPickerItems(items),
		WithPickerSearch(true),
	)
}
