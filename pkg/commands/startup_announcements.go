package commands

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	"github.com/tagoro9/fotingo/internal/telemetry"
	"github.com/tagoro9/fotingo/internal/version"
)

type startupAnnouncement = commandruntime.StartupAnnouncement

type startupAnnouncementProvider = commandruntime.StartupAnnouncementProvider

var (
	startupAnnouncementProviders = []startupAnnouncementProvider{
		commandruntime.NewVersionAnnouncementProvider(commandruntime.VersionAnnouncementProviderConfig{
			CurrentVersion: func() string { return commandruntime.GetBuildInfo().Version },
			BuildChecker:   buildVersionChecker,
		}),
	}
	startupAnnouncementManager = commandruntime.NewStartupAnnouncementManager()
)

func prepareStartupAnnouncements(cmd *cobra.Command) error {
	enabled := !ShouldSuppressOutput() && !shouldSkipStartupAnnouncements(cmd)
	return startupAnnouncementManager.Prepare(enabled, startupAnnouncementProviders)
}

func shouldSkipStartupAnnouncements(cmd *cobra.Command) bool {
	if cmd == nil {
		return true
	}

	return commandruntime.ShouldSkipStartupCommand(
		cmd.Name(),
		isShellCompletionCommand(cmd),
		cmd.Flags().Changed("help"),
	)
}

func emitPendingStartupAnnouncements(out commandruntime.LocalizedEmitter) {
	for _, announcement := range consumePendingStartupAnnouncements() {
		out.InfoRaw(announcement.Emoji, announcement.Message)
		emitStartupAnnouncementTelemetry(announcement)
	}
}

func emitStartupAnnouncementTelemetry(announcement startupAnnouncement) {
	if announcement.Kind != commandruntime.AnnouncementKindUpdateBanner {
		return
	}

	meta := announcement.Meta
	if len(meta) == 0 {
		return
	}
	execution := currentOrNewTelemetryExecutionContext()

	telemetry.TrackUpdateBannerShown(telemetry.UpdateBannerEvent{
		CurrentVersion: strings.TrimSpace(meta["current_version"]),
		LatestVersion:  strings.TrimSpace(meta["latest_version"]),
		Trigger:        strings.TrimSpace(meta["trigger"]),
		Persona:        execution.Persona,
		InvocationMode: execution.InvocationMode,
	})
}

func setPendingStartupAnnouncements(announcements []startupAnnouncement) {
	startupAnnouncementManager.SetPending(announcements)
}

func consumePendingStartupAnnouncements() []startupAnnouncement {
	return startupAnnouncementManager.ConsumePending()
}

func buildVersionChecker() (*version.Checker, cache.Store, func(), error) {
	store, err := newUtilityCacheStore()
	if err != nil {
		return nil, nil, func() {}, err
	}

	checker := version.NewChecker(Version, store)

	cleanup := func() {
		_ = store.Close()
	}
	return checker, store, cleanup, nil
}
