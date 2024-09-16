package commandruntime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/version"
)

const versionAnnouncementStateKey = "version:last-announcement"

type versionAnnouncementState struct {
	LastShownAt time.Time `json:"lastShownAt"`
}

// BuildVersionCheckerFunc builds a version checker and optional cache store.
type BuildVersionCheckerFunc func() (*version.Checker, cache.Store, func(), error)

// VersionAnnouncementProviderConfig configures the version update startup announcement provider.
type VersionAnnouncementProviderConfig struct {
	CurrentVersion func() string
	BuildChecker   BuildVersionCheckerFunc
}

type versionAnnouncementProvider struct {
	currentVersion func() string
	buildChecker   BuildVersionCheckerFunc
}

// NewVersionAnnouncementProvider creates a startup announcement provider that emits update notices.
func NewVersionAnnouncementProvider(config VersionAnnouncementProviderConfig) StartupAnnouncementProvider {
	return &versionAnnouncementProvider{
		currentVersion: config.CurrentVersion,
		buildChecker:   config.BuildChecker,
	}
}

func (p *versionAnnouncementProvider) Announcements() ([]StartupAnnouncement, error) {
	if p.currentVersion == nil {
		return nil, nil
	}

	currentVersion := strings.TrimSpace(p.currentVersion())
	if strings.EqualFold(currentVersion, "dev") || currentVersion == "" {
		return nil, nil
	}
	if p.buildChecker == nil {
		return nil, nil
	}

	checker, store, cleanup, err := p.buildChecker()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := checker.Check(ctx)
	if err != nil && !result.UpdateIsAvailable {
		return nil, err
	}
	if !result.UpdateIsAvailable {
		return nil, nil
	}

	if !shouldEmitVersionAnnouncement(store) {
		return nil, nil
	}
	_ = markVersionAnnouncementShown(store)

	message := fmt.Sprintf(
		"New fotingo version available: %s (current %s). Run `go install github.com/tagoro9/fotingo@latest` to upgrade.",
		result.LatestVersion,
		result.CurrentVersion,
	)
	return []StartupAnnouncement{{
		Emoji:   LogEmojiRocket,
		Message: message,
		Kind:    AnnouncementKindUpdateBanner,
		Meta: map[string]string{
			"current_version": result.CurrentVersion,
			"latest_version":  result.LatestVersion,
			"trigger":         "startup",
		},
	}}, nil
}

func shouldEmitVersionAnnouncement(store cache.Store) bool {
	if store == nil {
		return true
	}

	var state versionAnnouncementState
	hit, err := store.Get(versionAnnouncementStateKey, &state)
	if err != nil || !hit {
		return true
	}
	if state.LastShownAt.IsZero() {
		return true
	}

	return time.Since(state.LastShownAt) >= 24*time.Hour
}

func markVersionAnnouncementShown(store cache.Store) error {
	if store == nil {
		return nil
	}
	return store.SetWithTTL(versionAnnouncementStateKey, versionAnnouncementState{
		LastShownAt: time.Now().UTC(),
	}, 0)
}
