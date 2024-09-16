package commandruntime

import "sync"

// BuildInfo holds release metadata injected at build time.
type BuildInfo struct {
	Version   string
	GitCommit string
	BuildTime string
	Platform  string
}

var (
	buildInfoMu sync.RWMutex
	buildInfo   = BuildInfo{
		Version:   "dev",
		GitCommit: "unknown",
		BuildTime: "unknown",
		Platform:  "unknown/unknown",
	}
)

// SetBuildInfo updates the process-wide build metadata snapshot.
func SetBuildInfo(info BuildInfo) {
	buildInfoMu.Lock()
	defer buildInfoMu.Unlock()
	buildInfo = info
}

// GetBuildInfo returns the process-wide build metadata snapshot.
func GetBuildInfo() BuildInfo {
	buildInfoMu.RLock()
	defer buildInfoMu.RUnlock()
	return buildInfo
}
