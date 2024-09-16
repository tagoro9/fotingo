package commandruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetBuildInfo(t *testing.T) {
	original := GetBuildInfo()
	t.Cleanup(func() { SetBuildInfo(original) })

	SetBuildInfo(BuildInfo{
		Version:   "v1.2.3",
		GitCommit: "abc123",
		BuildTime: "2026-03-01T00:00:00Z",
		Platform:  "darwin/arm64",
	})

	assert.Equal(t, BuildInfo{
		Version:   "v1.2.3",
		GitCommit: "abc123",
		BuildTime: "2026-03-01T00:00:00Z",
		Platform:  "darwin/arm64",
	}, GetBuildInfo())
}
