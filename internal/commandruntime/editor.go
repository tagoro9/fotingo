package commandruntime

// OpenEditorWithTerminalHandoff runs an editor call while safely releasing/restoring terminal ownership.
func OpenEditorWithTerminalHandoff(
	initialContent string,
	runWithTerminalHandoff func(func() error) error,
	openEditor func(string) (string, error),
) (string, error) {
	if runWithTerminalHandoff == nil {
		return openEditor(initialContent)
	}

	var editedContent string
	err := runWithTerminalHandoff(func() error {
		edited, runErr := openEditor(initialContent)
		if runErr != nil {
			return runErr
		}
		editedContent = edited
		return nil
	})
	if err != nil {
		return "", err
	}

	return editedContent, nil
}
