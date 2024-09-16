package cache

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"

	storecache "github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/ui"
)

// MatchEntry reports whether a cache key matches the provided filter expression.
func MatchEntry(key string, pattern string) bool {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return true
	}

	if wildcard := strings.ContainsAny(trimmed, "*?[]"); wildcard {
		matched, err := path.Match(trimmed, key)
		if err == nil && matched {
			return true
		}
	}

	loweredPattern := strings.ToLower(trimmed)
	loweredKey := strings.ToLower(key)
	return loweredKey == loweredPattern || strings.Contains(loweredKey, loweredPattern)
}

// RedactAndFormatValue renders a cache value for display while protecting sensitive content.
func RedactAndFormatValue(sensitive *regexp.Regexp, key string, raw string, maxLen int) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if sensitive.MatchString(key) || sensitive.MatchString(trimmed) {
		return "<redacted>"
	}

	trimmed = DecodeURLEncodedText(trimmed)

	if maxLen > 0 && len(trimmed) > maxLen {
		return trimmed[:maxLen] + "..."
	}

	return trimmed
}

// DecodeURLEncodedText best-effort decodes URL-escaped content.
func DecodeURLEncodedText(value string) string {
	if !strings.Contains(value, "%") {
		return value
	}

	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return value
	}

	return decoded
}

// MetadataLine formats the expiration metadata line for cache entry detail views.
func MetadataLine(expiresAt string) string {
	if strings.TrimSpace(expiresAt) == "" {
		return "Expires: none"
	}

	return fmt.Sprintf("Expires: %s", expiresAt)
}

// BuildMultiSelectItems converts cache entries into UI multi-select items.
func BuildMultiSelectItems(entries []storecache.Entry, icon string) []ui.MultiSelectItem {
	items := make([]ui.MultiSelectItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, ui.MultiSelectItem{
			ID:     entry.Key,
			Label:  entry.Key,
			Detail: fmt.Sprintf("%d bytes", len(entry.Value)),
			Icon:   icon,
		})
	}

	slices.SortFunc(items, func(a, b ui.MultiSelectItem) int {
		return strings.Compare(a.ID, b.ID)
	})

	return items
}

// PickEntriesForDelete requests a multi-selection and returns selected cache keys.
func PickEntriesForDelete(
	title string,
	entries []storecache.Entry,
	icon string,
	minimum int,
	selectIDs func(string, []ui.MultiSelectItem, int) ([]string, error),
) ([]string, error) {
	items := BuildMultiSelectItems(entries, icon)
	return selectIDs(title, items, minimum)
}
