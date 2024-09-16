package commands

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalcache "github.com/tagoro9/fotingo/internal/commands/cache"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/ui"
)

const (
	cacheClearOperationID = "cache-clear"
)

var (
	cacheClearAll     bool
	sensitiveText     = regexp.MustCompile(`(?i)(token|secret|password|authorization|api[_-]?key)`)
	runCacheBrowserFn = runCacheBrowser
	cacheSelectIDsFn  = ui.SelectIDs
	newCacheBrowserFn = func(
		title string,
		items []ui.PickerItem,
		detailRenderer func(ui.PickerItem) string,
	) cacheBrowser {
		return ui.NewBrowserProgram(title, items, detailRenderer)
	}
	runCacheMultiSelectFn = func(entries []cache.Entry) ([]string, error) {
		return pickCacheEntriesForDelete(entries)
	}
)

type cacheBrowser interface {
	Run() error
}

var newUtilityCacheStore = func() (cache.Store, error) {
	customPath := strings.TrimSpace(fotingoConfig.GetString("cache.path"))
	if customPath == "" {
		store, err := cache.NewDefault()
		if err != nil {
			return nil, err
		}
		return store, nil
	}

	store, err := cache.New(cache.WithPath(customPath), cache.WithLogger(nil))
	if err != nil {
		return nil, err
	}
	return store, nil
}

func init() {
	Fotingo.AddCommand(cacheCmd)

	cacheCmd.AddCommand(cacheViewCmd)
	cacheCmd.AddCommand(cacheClearCmd)

	cacheClearCmd.Flags().BoolVar(&cacheClearAll, "all", false, localizer.T(i18n.CacheFlagAll))
}

var cacheCmd = &cobra.Command{
	Use:   i18n.T(i18n.CacheUse),
	Short: i18n.T(i18n.CacheShort),
	Long:  i18n.T(i18n.CacheLong),
}

var cacheViewCmd = &cobra.Command{
	Use:   i18n.T(i18n.CacheViewUse),
	Short: i18n.T(i18n.CacheViewShort),
	Long:  i18n.T(i18n.CacheViewLong),
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filter := ""
		if len(args) > 0 {
			filter = strings.TrimSpace(args[0])
		}

		entries, err := listCacheEntries(filter)
		if err != nil {
			return err
		}

		if ShouldOutputJSON() {
			OutputJSON(entries)
			return nil
		}

		if !isInteractiveTerminalFn() {
			return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
				for _, entry := range entries {
					out.InfoRaw(commandruntime.LogEmojiConfigure, fmt.Sprintf("%s (%s)", entry.DisplayKey, entry.Metadata))
				}
				return nil
			})
		}

		return runWithSharedShell(func(_ commandruntime.LocalizedEmitter) error {
			return runCacheBrowserFn(entries)
		})
	},
}

var cacheClearCmd = &cobra.Command{
	Use:   i18n.T(i18n.CacheClearUse),
	Short: i18n.T(i18n.CacheClearShort),
	Long:  i18n.T(i18n.CacheClearLong),
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := newUtilityCacheStore()
		if err != nil {
			return fterrors.ConfigErrorf("failed to initialize cache store: %v", err)
		}
		defer func() { _ = store.Close() }()

		entries, err := store.List("")
		if err != nil {
			return fterrors.ConfigErrorf("failed to list cache entries: %v", err)
		}

		if len(entries) == 0 {
			return fterrors.ConfigError(localizer.T(i18n.CacheErrNoEntriesAvailable))
		}

		toDelete, err := resolveCacheEntriesToDelete(entries, args)
		if err != nil {
			return err
		}

		if cacheClearAll {
			if err := store.Clear(); err != nil {
				return fterrors.ConfigErrorf("failed to clear cache: %v", err)
			}
		} else {
			for _, key := range toDelete {
				if err := store.Delete(key); err != nil {
					return fterrors.ConfigErrorf("failed to delete cache key %s: %v", key, err)
				}
			}
		}

		if ShouldOutputJSON() {
			OutputJSON(map[string]any{
				"cleared": toDelete,
				"count":   len(toDelete),
			})
			return nil
		}

		return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
			out.Success(cacheClearOperationID, commandruntime.LogEmojiConfigure, i18n.CacheStatusClearedEntries, len(toDelete))
			return nil
		})
	},
}

type cacheEntryView = internalcache.EntryView

func listCacheEntries(filter string) ([]cacheEntryView, error) {
	store, err := newUtilityCacheStore()
	if err != nil {
		return nil, fterrors.ConfigErrorf("failed to initialize cache store: %v", err)
	}
	defer func() { _ = store.Close() }()

	entries, err := store.List("")
	if err != nil {
		return nil, fterrors.ConfigErrorf("failed to list cache entries: %v", err)
	}

	filtered := internalcache.FilterEntries(entries, filter)

	if len(filtered) == 0 {
		return nil, fterrors.ConfigErrorf(localizer.T(i18n.CacheErrNoMatchingEntries), filter)
	}

	return internalcache.BuildEntryViews(filtered, sensitiveText), nil
}

func runCacheBrowser(entries []cacheEntryView) error {
	items := internalcache.BuildBrowserItems(entries, string(commandruntime.LogEmojiPackage))

	browser := newCacheBrowserFn(localizer.T(i18n.CacheViewShort), items, func(item ui.PickerItem) string {
		entry, ok := item.Value.(cacheEntryView)
		if !ok {
			return ""
		}
		return internalcache.DetailText(entry)
	})

	return browser.Run()
}

func resolveCacheEntriesToDelete(entries []cache.Entry, args []string) ([]string, error) {
	if cacheClearAll {
		keys := make([]string, 0, len(entries))
		for _, entry := range entries {
			keys = append(keys, entry.Key)
		}
		return keys, nil
	}

	if len(args) > 0 {
		pattern := strings.TrimSpace(args[0])
		matched := make([]string, 0, len(entries))
		for _, entry := range entries {
			if internalcache.MatchEntry(entry.Key, pattern) {
				matched = append(matched, entry.Key)
			}
		}
		if len(matched) == 0 {
			return nil, fterrors.ConfigErrorf(localizer.T(i18n.CacheErrNoMatchingEntries), pattern)
		}
		return matched, nil
	}

	if !isInteractiveTerminalFn() {
		return nil, fterrors.ConfigError("provide a key/pattern or use --all")
	}
	return runCacheMultiSelectFn(entries)
}

func pickCacheEntriesForDelete(entries []cache.Entry) ([]string, error) {
	keys, err := internalcache.PickEntriesForDelete(
		localizer.T(i18n.CacheClearShort),
		entries,
		string(commandruntime.LogEmojiPackage),
		1,
		cacheSelectIDsFn,
	)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		// Keep legacy cancel semantics: an empty submit is treated as user cancellation.
		return nil, fterrors.ErrUserCancelled
	}
	return keys, nil
}
