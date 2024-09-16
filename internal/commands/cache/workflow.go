package cache

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	storecache "github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/ui"
)

// EntryView is the view model used by cache command renderers and JSON output.
type EntryView struct {
	Key        string `json:"key"`
	DisplayKey string `json:"-"`
	Value      string `json:"value"`
	Preview    string `json:"-"`
	Detail     string `json:"-"`
	Metadata   string `json:"metadata"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
	SizeBytes  int    `json:"sizeBytes"`
}

// FilterEntries returns entries matching the filter pattern.
func FilterEntries(entries []storecache.Entry, filter string) []storecache.Entry {
	filtered := make([]storecache.Entry, 0, len(entries))
	for _, entry := range entries {
		if MatchEntry(entry.Key, filter) {
			filtered = append(filtered, entry)
		}
	}

	slices.SortFunc(filtered, func(a, b storecache.Entry) int {
		return strings.Compare(a.Key, b.Key)
	})
	return filtered
}

// BuildEntryViews converts cache entries to display and JSON views.
func BuildEntryViews(
	entries []storecache.Entry,
	sensitive *regexp.Regexp,
) []EntryView {
	views := make([]EntryView, 0, len(entries))
	for _, entry := range entries {
		expiresAt := ""
		if entry.ExpiresAt != nil {
			expiresAt = entry.ExpiresAt.Format("2006-01-02 15:04:05 MST")
		}

		metadata := fmt.Sprintf("%d bytes", entry.SizeBytes)
		if expiresAt != "" {
			metadata = fmt.Sprintf("%s, expires %s", metadata, expiresAt)
		}

		views = append(views, EntryView{
			Key:        entry.Key,
			DisplayKey: DecodeURLEncodedText(entry.Key),
			Value:      RedactAndFormatValue(sensitive, entry.Key, string(entry.Value), 0),
			Preview:    RedactAndFormatValue(sensitive, entry.Key, string(entry.Value), 240),
			Detail:     RedactAndFormatValue(sensitive, entry.Key, string(entry.Value), 0),
			Metadata:   metadata,
			ExpiresAt:  expiresAt,
			SizeBytes:  entry.SizeBytes,
		})
	}

	return views
}

// BuildBrowserItems converts entry views into picker items.
func BuildBrowserItems(entries []EntryView, icon string) []ui.PickerItem {
	items := make([]ui.PickerItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, ui.PickerItem{
			ID:     entry.Key,
			Label:  entry.DisplayKey,
			Detail: entry.Metadata,
			Icon:   icon,
			Value:  entry,
		})
	}
	return items
}

// DetailText renders browser detail text for an entry view.
func DetailText(entry EntryView) string {
	return strings.Join([]string{
		fmt.Sprintf("Key: %s", entry.DisplayKey),
		fmt.Sprintf("Size: %d bytes", entry.SizeBytes),
		MetadataLine(entry.ExpiresAt),
		"",
		entry.Detail,
	}, "\n")
}
