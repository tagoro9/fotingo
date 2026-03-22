package commandruntime

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	fio "github.com/tagoro9/fotingo/internal/io"
	"github.com/tagoro9/fotingo/internal/ui"
)

// StatusModelOptions configures the shared status model.
type StatusModelOptions struct {
	SuppressOutput func() bool
}

// StatusModel is the shared Bubble Tea model used by command flows.
type StatusModel struct {
	spinner        spinner.Model
	styles         ui.Styles
	messages       []fio.Message
	done           bool
	suppressOutput func() bool
}

// NewStatusModel creates a status model with no preloaded messages.
func NewStatusModel(options StatusModelOptions) StatusModel {
	s := spinner.New()
	s.Spinner = spinner.Dot

	styles := ui.DefaultStyles()
	s.Style = styles.Spinner

	return StatusModel{
		spinner:        s,
		styles:         styles,
		messages:       []fio.Message{},
		suppressOutput: options.SuppressOutput,
	}
}

// Init initializes the status model.
func (m StatusModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages and updates the status model.
func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case StatusMsg:
		m.applyStatusMessage(string(msg))
		return m, nil
	case DoneMsg:
		m.demoteActiveMessages()
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

// View renders the status model.
func (m StatusModel) View() tea.View {
	if m.suppressOutput != nil && m.suppressOutput() {
		return tea.NewView("")
	}

	var sb strings.Builder

	for i, msg := range m.messages {
		active := m.isMessageActive(i)
		normalized := FormatMessageWithEmoji(msg)
		display := normalized
		if active {
			display = ActiveDisplayMessage(msg, normalized)
		}

		rendered := m.renderMessage(i, display, normalized, msg, active)
		if active {
			sb.WriteString(m.spinner.View() + " " + rendered + "\n")
		} else {
			sb.WriteString(rendered + "\n")
		}
	}

	return tea.NewView(sb.String())
}

func (m StatusModel) renderMessage(
	index int,
	displayMessage string,
	severityMessage string,
	message fio.Message,
	active bool,
) string {
	trimmed := strings.TrimSpace(displayMessage)
	severity := strings.TrimSpace(severityMessage)

	switch {
	case message.Type == fio.MessageTypeError || IsErrorMessage(severity):
		if active {
			return m.styles.Error.Bold(true).Render(trimmed)
		}
		return m.styles.Error.Render(trimmed)
	case IsWarningMessage(severity):
		if active {
			return m.styles.Warning.Bold(true).Render(trimmed)
		}
		return m.styles.Warning.Render(trimmed)
	case active:
		return m.styles.Bold.Render(trimmed)
	case index == 0:
		return m.styles.Muted.Render(trimmed)
	default:
		return m.styles.Normal.Render(trimmed)
	}
}

func (m StatusModel) isMessageActive(index int) bool {
	if m.done || index < 0 || index >= len(m.messages) {
		return false
	}

	message := m.messages[index]
	if message.Type == fio.MessageTypeStatus {
		return true
	}

	if message.Type == "" {
		return index == len(m.messages)-1
	}

	return false
}

func (m *StatusModel) applyStatusMessage(raw string) {
	if event, ok := DecodeStatusEvent(raw); ok {
		m.applyStatusEvent(event)
		return
	}

	m.demoteActiveMessages()
	m.messages = append(m.messages, fio.Message{
		Message: strings.TrimSpace(raw),
		Type:    fio.MessageTypeStatus,
	})
}

func (m *StatusModel) applyStatusEvent(event StatusEvent) {
	operationID := OperationIDOrDefault(event.OperationID)
	trimmedMessage := strings.TrimSpace(event.Message)

	switch event.Kind {
	case StatusEventKindAppend:
		m.demoteActiveMessages()
		m.messages = append(m.messages, fio.Message{
			Message: trimmedMessage,
			Emoji:   EventEmoji(event),
			Type:    fio.MessageTypeStatus,
		})
	case StatusEventKindStart, StatusEventKindUpdate:
		m.demoteActiveMessages()
		index := m.findMessageByOperationID(operationID)
		if index < 0 {
			m.messages = append(m.messages, fio.Message{
				Detail:  operationID,
				Message: trimmedMessage,
				Emoji:   EventEmoji(event),
				Type:    fio.MessageTypeStatus,
			})
			return
		}

		m.messages[index].Message = trimmedMessage
		m.messages[index].Emoji = EventEmoji(event)
		m.messages[index].Type = fio.MessageTypeStatus
	case StatusEventKindSuccess:
		m.finalizeOperationMessage(operationID, trimmedMessage, EventEmoji(event), fio.MessageTypeInfo)
	case StatusEventKindError:
		m.finalizeOperationMessage(operationID, trimmedMessage, EventEmoji(event), fio.MessageTypeError)
	default:
		m.demoteActiveMessages()
		m.messages = append(m.messages, fio.Message{
			Message: trimmedMessage,
			Emoji:   EventEmoji(event),
			Type:    fio.MessageTypeStatus,
		})
	}
}

func (m *StatusModel) finalizeOperationMessage(
	operationID string,
	message string,
	emoji string,
	messageType fio.MessageType,
) {
	index := m.findMessageByOperationID(operationID)
	if index < 0 {
		m.demoteActiveMessages()
		m.messages = append(m.messages, fio.Message{
			Detail:  operationID,
			Message: message,
			Emoji:   emoji,
			Type:    messageType,
		})
		return
	}

	m.messages[index].Message = message
	m.messages[index].Emoji = emoji
	m.messages[index].Type = messageType
}

func (m *StatusModel) findMessageByOperationID(operationID string) int {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if strings.TrimSpace(m.messages[i].Detail) == operationID {
			return i
		}
	}

	return -1
}

func (m *StatusModel) demoteActiveMessages() {
	for i := range m.messages {
		if m.messages[i].Type == fio.MessageTypeStatus {
			m.messages[i].Type = fio.MessageTypeInfo
		}
	}
}
