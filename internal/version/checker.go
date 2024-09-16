package version

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/github"
)

const (
	defaultCadence        = 24 * time.Hour
	defaultCheckStateKey  = "version:latest-check"
	defaultRequestTimeout = 3 * time.Second
	defaultReleaseOwner   = "tagoro9"
	defaultReleaseRepo    = "fotingo"
)

// CheckState captures the latest known version and when it was checked.
type CheckState struct {
	CheckedAt     time.Time `json:"checkedAt"`
	LatestVersion string    `json:"latestVersion"`
}

// CheckResult describes the outcome of a version check operation.
type CheckResult struct {
	CurrentVersion    string
	LatestVersion     string
	CheckedAt         time.Time
	UsedCached        bool
	UpdateIsAvailable bool
}

// Checker performs bounded latest-version checks using cache storage.
type Checker struct {
	currentVersion string
	store          cache.Store
	stateKey       string
	cadence        time.Duration
	now            func() time.Time
	fetchLatest    func(context.Context) (string, error)
}

// CheckerOption configures checker behavior.
type CheckerOption func(*Checker)

// WithLatestVersionFetcher overrides the latest-version fetch behavior.
func WithLatestVersionFetcher(fetchLatest func(context.Context) (string, error)) CheckerOption {
	return func(c *Checker) {
		c.fetchLatest = fetchLatest
	}
}

// NewChecker constructs a version checker.
func NewChecker(currentVersion string, store cache.Store, opts ...CheckerOption) *Checker {
	checker := &Checker{
		currentVersion: strings.TrimSpace(currentVersion),
		store:          store,
		stateKey:       defaultCheckStateKey,
		cadence:        defaultCadence,
		now:            time.Now,
	}

	checker.fetchLatest = checker.defaultFetchLatest
	for _, opt := range opts {
		opt(checker)
	}

	return checker
}

// Check resolves latest-version information with daily cache cadence.
func (c *Checker) Check(ctx context.Context) (CheckResult, error) {
	if c == nil {
		return CheckResult{}, nil
	}

	result := CheckResult{CurrentVersion: c.currentVersion}
	if c.fetchLatest == nil || !isSemanticVersion(c.currentVersion) {
		return result, nil
	}

	state, hasState, err := c.loadState()
	if err != nil {
		return result, err
	}

	now := c.now()
	if hasState && now.Sub(state.CheckedAt) < c.cadence && strings.TrimSpace(state.LatestVersion) != "" {
		result.LatestVersion = state.LatestVersion
		result.CheckedAt = state.CheckedAt
		result.UsedCached = true
		result.UpdateIsAvailable = IsVersionNewer(result.LatestVersion, c.currentVersion)
		return result, nil
	}

	latest, err := c.fetchLatest(ctx)
	if err != nil {
		if hasState && strings.TrimSpace(state.LatestVersion) != "" {
			result.LatestVersion = state.LatestVersion
			result.CheckedAt = state.CheckedAt
			result.UsedCached = true
			result.UpdateIsAvailable = IsVersionNewer(result.LatestVersion, c.currentVersion)
		}
		return result, err
	}

	normalized := normalizeVersionString(latest)
	if normalized == "" {
		return result, fmt.Errorf("latest version is empty")
	}

	newState := CheckState{
		CheckedAt:     now.UTC(),
		LatestVersion: normalized,
	}
	if saveErr := c.saveState(newState); saveErr != nil {
		return result, saveErr
	}

	result.LatestVersion = normalized
	result.CheckedAt = newState.CheckedAt
	result.UpdateIsAvailable = IsVersionNewer(result.LatestVersion, c.currentVersion)
	return result, nil
}

func (c *Checker) loadState() (CheckState, bool, error) {
	if c.store == nil {
		return CheckState{}, false, nil
	}

	var state CheckState
	hit, err := c.store.Get(c.stateKey, &state)
	if err != nil {
		return CheckState{}, false, err
	}

	if !hit {
		return CheckState{}, false, nil
	}

	return state, true, nil
}

func (c *Checker) saveState(state CheckState) error {
	if c.store == nil {
		return nil
	}
	if err := c.store.SetWithTTL(c.stateKey, state, 0); err != nil {
		return err
	}
	return nil
}

func (c *Checker) defaultFetchLatest(ctx context.Context) (string, error) {
	client := &http.Client{Timeout: defaultRequestTimeout}
	return github.FetchLatestReleaseTag(ctx, client, defaultReleaseOwner, defaultReleaseRepo)
}

var semverPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$`)

func isSemanticVersion(version string) bool {
	return semverPattern.MatchString(strings.TrimSpace(version))
}

func normalizeVersionString(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "v") {
		trimmed = "v" + trimmed
	}
	if !isSemanticVersion(trimmed) {
		return ""
	}
	return trimmed
}

// IsVersionNewer reports whether latest is newer than current.
func IsVersionNewer(latest string, current string) bool {
	latestParts, ok := parseSemanticVersion(latest)
	if !ok {
		return false
	}

	currentParts, ok := parseSemanticVersion(current)
	if !ok {
		return false
	}

	for i := 0; i < len(latestParts); i++ {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}

	return false
}

func parseSemanticVersion(version string) ([3]int, bool) {
	match := semverPattern.FindStringSubmatch(strings.TrimSpace(version))
	if len(match) != 4 {
		return [3]int{}, false
	}

	var parts [3]int
	for index := 1; index <= 3; index++ {
		value, err := strconv.Atoi(match[index])
		if err != nil {
			return [3]int{}, false
		}
		parts[index-1] = value
	}

	return parts, true
}
