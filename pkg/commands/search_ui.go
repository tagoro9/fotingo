package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/ui"
)

type searchProgressMsg string

type searchCompletedMsg struct {
	results []reviewMatchOption
	err     error
}

var runInteractiveSearchMetadataCommandFn = runInteractiveSearchMetadataCommand
var shouldUseInteractiveSearchUIFn = shouldUseInteractiveSearchUI

// searchUIModel owns the interactive search terminal lifecycle so progress and
// final results are rendered by one Bubble Tea program.
type searchUIModel struct {
	domain   searchDomain
	query    string
	spinner  spinner.Model
	styles   ui.Styles
	progress []string
	results  []reviewMatchOption
	err      error
	done     bool
}

// newSearchUIModel creates a Bubble Tea search model with spinner styling
// aligned to the rest of the CLI UI components.
func newSearchUIModel(domain searchDomain, query string) searchUIModel {
	s := spinner.New()
	s.Spinner = spinner.Dot

	styles := ui.DefaultStyles()
	s.Style = styles.Spinner

	return searchUIModel{
		domain:   domain,
		query:    query,
		spinner:  s,
		styles:   styles,
		progress: []string{},
		results:  []reviewMatchOption{},
	}
}

func (m searchUIModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m searchUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.done = true
			m.err = fmt.Errorf("interrupted")
			return m, tea.Quit
		}
	case spinner.TickMsg:
		if m.done {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case searchProgressMsg:
		m.appendProgress(strings.TrimSpace(string(msg)))
		return m, nil
	case searchCompletedMsg:
		m.results = append([]reviewMatchOption{}, msg.results...)
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

func (m searchUIModel) View() tea.View {
	if m.done {
		if m.err != nil {
			return tea.NewView("")
		}
		return tea.NewView(m.renderFinalResults())
	}

	var sections []string
	sections = append(sections, m.styles.Title.Render(fmt.Sprintf("Searching %s for %q", m.domain, m.query)))

	statusLine := m.spinner.View() + " " + m.styles.Bold.Render(fmt.Sprintf("Searching %s", m.domain))
	sections = append(sections, statusLine)

	for _, message := range m.progress {
		if strings.TrimSpace(message) == "" {
			continue
		}
		sections = append(sections, m.styles.Muted.Render("  "+message))
	}

	return tea.NewView(strings.Join(sections, "\n") + "\n")
}

func (m *searchUIModel) appendProgress(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	if len(m.progress) > 0 && m.progress[len(m.progress)-1] == message {
		return
	}
	m.progress = append(m.progress, message)
}

func (m searchUIModel) renderFinalResults() string {
	lines := renderSearchResultLines(m.domain, m.query, m.results)
	if len(lines) == 0 {
		return ""
	}

	rendered := make([]string, 0, len(lines))
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case index == 0:
			rendered = append(rendered, m.styles.Title.Render(trimmed))
		case strings.HasPrefix(trimmed, "No "):
			rendered = append(rendered, m.styles.Muted.Render(trimmed))
		default:
			rendered = append(rendered, m.styles.Normal.Render(trimmed))
		}
	}

	return strings.Join(rendered, "\n") + "\n"
}

func shouldUseInteractiveSearchUI() bool {
	return !ShouldSuppressOutput() &&
		!ShouldOutputJSON() &&
		isInputTerminalFn() &&
		commandruntime.IsInputTerminal() &&
		commandruntime.IsOutputTerminal()
}

// runInteractiveSearchMetadataCommand executes `fotingo search` inside a
// command-local Bubble Tea program so progress and final results share one
// renderer and do not race with direct stdout writes.
func runInteractiveSearchMetadataCommand(
	_ io.Writer,
	domain searchDomain,
	args []string,
) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return fmt.Errorf("search query is required")
	}

	model := newSearchUIModel(domain, query)
	// Let Bubble Tea own the real terminal directly, matching the rest of the
	// command UIs. Overriding the output writer here can leak terminal teardown
	// control sequences instead of restoring terminal modes cleanly.
	//
	// Search is unusually short-lived compared with the other interactive
	// commands. Bubble Tea v2 may probe terminal mode 2026 (synchronized output)
	// on startup, and some terminals can emit the delayed response after a very
	// fast program exits. That leaves raw control bytes in the shell prompt. We
	// sanitize the Bubble Tea environment for this command so the short-lived
	// search UI skips that capability probe.
	program := tea.NewProgram(
		model,
		tea.WithEnvironment(searchUITeaEnvironment(os.Environ())),
	)

	go func() {
		results, err := searchReviewMetadata(domain, query, func(message string) {
			program.Send(searchProgressMsg(message))
		})
		program.Send(searchCompletedMsg{
			results: results,
			err:     err,
		})
	}()

	var finalModel searchUIModel
	err := withActiveTerminal(program, func() error {
		result, runErr := program.Run()
		if typed, ok := result.(searchUIModel); ok {
			finalModel = typed
		}
		return runErr
	})
	if err != nil {
		return fmt.Errorf(localizer.T(i18n.RootErrRunningUI), err)
	}

	return finalModel.err
}

// searchUITeaEnvironment disables Bubble Tea's synchronized-output capability
// probe for the short-lived search UI. Longer-lived TUIs usually stay alive
// long enough to consume the terminal's mode-2026 response, but search often
// completes fast enough that the response can arrive after the Tea program has
// already exited. When that happens the shell renders the raw control bytes.
//
// This is a command-local workaround until Bubble Tea exposes a narrower way
// to disable the capability query or the startup/teardown race is fixed
// upstream.
func searchUITeaEnvironment(base []string) []string {
	env := make([]string, 0, len(base)+2)
	replacedTerm := false
	replacedSSH := false
	for _, entry := range base {
		switch {
		case strings.HasPrefix(entry, "TERM="):
			env = append(env, "TERM=xterm-256color")
			replacedTerm = true
		case strings.HasPrefix(entry, "SSH_TTY="):
			env = append(env, "SSH_TTY=fotingo-search-ui")
			replacedSSH = true
		default:
			env = append(env, entry)
		}
	}
	if !replacedTerm {
		env = append(env, "TERM=xterm-256color")
	}
	if !replacedSSH {
		env = append(env, "SSH_TTY=fotingo-search-ui")
	}
	return env
}
