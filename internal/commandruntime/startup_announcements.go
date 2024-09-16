package commandruntime

import "sync"

const (
	// AnnouncementKindUpdateBanner marks a startup announcement for version updates.
	AnnouncementKindUpdateBanner = "update_banner"
)

// StartupAnnouncement is a user-facing startup message emitted before command execution.
type StartupAnnouncement struct {
	Emoji   LogEmoji
	Message string
	Kind    string
	Meta    map[string]string
}

// StartupAnnouncementProvider generates startup announcements.
type StartupAnnouncementProvider interface {
	Announcements() ([]StartupAnnouncement, error)
}

// StartupAnnouncementManager stores and emits pending startup announcements.
type StartupAnnouncementManager struct {
	pendingMu sync.Mutex
	pending   []StartupAnnouncement
}

// NewStartupAnnouncementManager creates a manager for startup announcement state.
func NewStartupAnnouncementManager() *StartupAnnouncementManager {
	return &StartupAnnouncementManager{}
}

// Prepare collects provider announcements when enabled and stores them as pending.
func (m *StartupAnnouncementManager) Prepare(
	enabled bool,
	providers []StartupAnnouncementProvider,
) error {
	m.SetPending(nil)
	if !enabled {
		return nil
	}

	announcements := make([]StartupAnnouncement, 0)
	for _, provider := range providers {
		provided, err := provider.Announcements()
		if err != nil {
			continue
		}
		announcements = append(announcements, provided...)
	}

	m.SetPending(announcements)
	return nil
}

// SetPending replaces pending announcements atomically.
func (m *StartupAnnouncementManager) SetPending(announcements []StartupAnnouncement) {
	m.pendingMu.Lock()
	defer m.pendingMu.Unlock()
	m.pending = append([]StartupAnnouncement(nil), announcements...)
}

// ConsumePending drains and returns pending announcements.
func (m *StartupAnnouncementManager) ConsumePending() []StartupAnnouncement {
	m.pendingMu.Lock()
	defer m.pendingMu.Unlock()

	drained := append([]StartupAnnouncement(nil), m.pending...)
	m.pending = nil
	return drained
}

// ShouldSkipStartupCommand reports whether startup announcements should be skipped for a command.
func ShouldSkipStartupCommand(commandName string, isShellCompletion bool, helpRequested bool) bool {
	if isShellCompletion {
		return true
	}
	if helpRequested || commandName == "help" {
		return true
	}

	switch commandName {
	case "version", "completion", "inspect", "start", "review":
		return true
	default:
		return false
	}
}
