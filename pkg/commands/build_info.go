package commands

import "github.com/tagoro9/fotingo/internal/commandruntime"

// Build metadata injected at release time via ldflags.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
	Platform  = "unknown/unknown"
)

func init() {
	commandruntime.SetBuildInfo(commandruntime.BuildInfo{
		Version:   Version,
		GitCommit: GitCommit,
		BuildTime: BuildTime,
		Platform:  Platform,
	})
}
