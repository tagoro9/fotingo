package ai

import (
	"fmt"
	"os"
	"path/filepath"
)

// InstallStatus represents the file-write state for a setup target.
type InstallStatus string

const (
	InstallStatusPlanned InstallStatus = "planned"
	InstallStatusCreated InstallStatus = "created"
	InstallStatusUpdated InstallStatus = "updated"
	InstallStatusSkipped InstallStatus = "skipped"
)

// InstallOptions controls write behavior.
type InstallOptions struct {
	DryRun bool
	Force  bool
}

// InstallResult captures outcome for one provider target.
type InstallResult struct {
	Target InstallTarget
	Status InstallStatus
	Reason string
}

// RenderSkillFunc renders provider-specific markdown content.
type RenderSkillFunc func(Provider) (string, error)

// ApplyInstallPlan applies the planned install targets using the provided renderer.
func ApplyInstallPlan(
	targets []InstallTarget,
	opts InstallOptions,
	render RenderSkillFunc,
) ([]InstallResult, error) {
	results := make([]InstallResult, 0, len(targets))
	for _, target := range targets {
		content, err := render(target.Provider)
		if err != nil {
			return nil, fmt.Errorf("failed to render skill for %s: %w", target.Provider, err)
		}

		result := InstallResult{Target: target}
		if opts.DryRun {
			result.Status = InstallStatusPlanned
			results = append(results, result)
			continue
		}

		exists, err := fileExists(target.SkillPath)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect %s: %w", target.SkillPath, err)
		}

		if exists && !opts.Force {
			result.Status = InstallStatusSkipped
			result.Reason = "file already exists (use --force to overwrite)"
			results = append(results, result)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target.SkillPath), 0o755); err != nil {
			return nil, fmt.Errorf("failed to create %s: %w", filepath.Dir(target.SkillPath), err)
		}
		if err := os.WriteFile(target.SkillPath, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", target.SkillPath, err)
		}

		if exists {
			result.Status = InstallStatusUpdated
		} else {
			result.Status = InstallStatusCreated
		}
		results = append(results, result)
	}

	return results, nil
}

// CountInstallResults returns counts for each install status.
func CountInstallResults(results []InstallResult) (planned int, created int, updated int, skipped int) {
	for _, result := range results {
		switch result.Status {
		case InstallStatusPlanned:
			planned++
		case InstallStatusCreated:
			created++
		case InstallStatusUpdated:
			updated++
		case InstallStatusSkipped:
			skipped++
		}
	}
	return planned, created, updated, skipped
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
