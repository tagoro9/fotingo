package cache

import (
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	storecache "github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/ui"
)

func TestMatchEntry(t *testing.T) {
	assert.True(t, MatchEntry("github:labels", ""))
	assert.True(t, MatchEntry("github:labels", "github:*"))
	assert.True(t, MatchEntry("github:labels", "labels"))
	assert.False(t, MatchEntry("github:labels", "jira:"))
}

func TestRedactAndFormatValue(t *testing.T) {
	sensitive := regexp.MustCompile(`(?i)(token|secret)`)
	assert.Equal(t, "<redacted>", RedactAndFormatValue(sensitive, "github:token", `{"token":"x"}`, 0))
	assert.Equal(t, "hello world", RedactAndFormatValue(sensitive, "key", "hello%20world", 0))
	assert.Equal(t, "hello...", RedactAndFormatValue(sensitive, "key", "hello world", 5))
}

func TestDecodeURLEncodedText(t *testing.T) {
	assert.Equal(t, "plain", DecodeURLEncodedText("plain"))
	assert.Equal(t, "a b", DecodeURLEncodedText("a%20b"))
}

func TestMetadataLine(t *testing.T) {
	assert.Equal(t, "Expires: none", MetadataLine(""))
	assert.Equal(t, "Expires: 2026-03-01", MetadataLine("2026-03-01"))
}

func TestPickEntriesForDelete(t *testing.T) {
	entries := []storecache.Entry{{Key: "key:two", Value: []byte("22")}, {Key: "key:one", Value: []byte("1")}}
	selected, err := PickEntriesForDelete(
		"Clear",
		entries,
		"📦",
		1,
		func(_ string, items []ui.MultiSelectItem, _ int) ([]string, error) {
			assert.Equal(t, "key:one", items[0].ID)
			assert.Equal(t, "key:two", items[1].ID)
			return []string{"key:two"}, nil
		},
	)
	assert.NoError(t, err)
	assert.Equal(t, []string{"key:two"}, selected)
}

func TestFilterEntriesAndBuildViews(t *testing.T) {
	sensitive := regexp.MustCompile(`(?i)(token|secret)`)
	exp := time.Now().UTC().Add(24 * time.Hour)
	entries := []storecache.Entry{
		{Key: "github:token", Value: []byte("top-secret"), SizeBytes: 10},
		{Key: "github:labels", Value: []byte("hello%20world"), SizeBytes: 11, ExpiresAt: &exp},
		{Key: "jira:site", Value: []byte("https://example"), SizeBytes: 15},
	}

	filtered := FilterEntries(entries, "github:")
	assert.Len(t, filtered, 2)
	assert.Equal(t, "github:labels", filtered[0].Key)

	views := BuildEntryViews(filtered, sensitive)
	assert.Len(t, views, 2)
	assert.Equal(t, "<redacted>", views[1].Value)
	assert.Contains(t, views[0].Metadata, "expires")
}

func TestBuildBrowserItemsAndDetailText(t *testing.T) {
	items := BuildBrowserItems([]EntryView{{
		Key:        "github:labels",
		DisplayKey: "github:labels",
		Metadata:   "10 bytes",
		SizeBytes:  10,
		Detail:     "value",
	}}, "📦")
	assert.Len(t, items, 1)
	assert.Equal(t, "github:labels", items[0].ID)
	assert.Contains(t, DetailText(items[0].Value.(EntryView)), "Key: github:labels")
}
