package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	ftconfig "github.com/tagoro9/fotingo/internal/config"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/ui"
)

const (
	configGetOperationID = "config-get"
	configSetOperationID = "config-set"
)

var (
	isInteractiveTerminalFn = isInteractiveTerminal
	runConfigBrowserFn      = runConfigBrowser
)

func init() {
	Fotingo.AddCommand(configCmd)

	configCmd.AddCommand(configViewCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}

var configCmd = &cobra.Command{
	Use:   i18n.T(i18n.ConfigUse),
	Short: i18n.T(i18n.ConfigShort),
	Long:  i18n.T(i18n.ConfigLong),
}

var configViewCmd = &cobra.Command{
	Use:               i18n.T(i18n.ConfigViewUse),
	Short:             i18n.T(i18n.ConfigViewShort),
	Long:              i18n.T(i18n.ConfigViewLong),
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeConfigKeys,
	RunE: func(cmd *cobra.Command, args []string) error {
		filter := ""
		if len(args) > 0 {
			filter = strings.TrimSpace(args[0])
		}

		entries, err := filterConfigEntries(filter)
		if err != nil {
			return err
		}
		entries = filterNonEmptyConfigEntries(entries)
		if len(entries) == 0 {
			return fterrors.ConfigError("no non-empty config keys found")
		}

		if ShouldOutputJSON() {
			OutputJSON(entries)
			return nil
		}

		if !isInteractiveTerminalFn() {
			return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
				for _, entry := range entries {
					out.InfoRaw(commandruntime.LogEmojiConfigure, fmt.Sprintf("%s (%s) %s", entry.Key, entry.Type, entry.Value))
				}
				return nil
			})
		}

		return runWithSharedShell(func(_ commandruntime.LocalizedEmitter) error {
			return runConfigBrowserFn(entries)
		})
	},
}

var configGetCmd = &cobra.Command{
	Use:               i18n.T(i18n.ConfigGetUse),
	Short:             i18n.T(i18n.ConfigGetShort),
	Long:              i18n.T(i18n.ConfigGetLong),
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeConfigKeys,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			interactive := isInteractiveTerminalFn()
			if ShouldOutputJSON() || !interactive {
				OutputJSON(listConfigEntries())
				return nil
			}
			return configViewCmd.RunE(cmd, args)
		}

		entry, err := getConfigEntry(args[0])
		if err != nil {
			return err
		}

		if ShouldOutputJSON() {
			OutputJSON(entry)
			return nil
		}

		return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
			out.Success(configGetOperationID, commandruntime.LogEmojiConfigure, i18n.ConfigStatusSelectedKey, entry.Key, entry.Type)
			out.Info(commandruntime.LogEmojiInfo, i18n.ConfigStatusCurrentValue, entry.Value)
			return nil
		})
	},
}

var configSetCmd = &cobra.Command{
	Use:               i18n.T(i18n.ConfigSetUse),
	Short:             i18n.T(i18n.ConfigSetShort),
	Long:              i18n.T(i18n.ConfigSetLong),
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeConfigSetArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.TrimSpace(args[0])
		value := args[1]
		entry, err := setConfigEntry(key, value)
		if err != nil {
			return err
		}

		if ShouldOutputJSON() {
			OutputJSON(entry)
			return nil
		}

		return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
			out.Success(configSetOperationID, commandruntime.LogEmojiConfigure, i18n.ConfigStatusUpdated, entry.Key, entry.Value)
			return nil
		})
	},
}

func runConfigBrowser(entries []configEntry) error {
	items := make([]ui.PickerItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, ui.PickerItem{
			ID:     entry.Key,
			Label:  entry.Key,
			Detail: fmt.Sprintf("(%s) %s", entry.Type, entry.Value),
			Icon:   string(commandruntime.LogEmojiConfigure),
			Value:  entry,
		})
	}

	browser := ui.NewBrowserProgram(localizer.T(i18n.ConfigViewShort), items, func(item ui.PickerItem) string {
		entry, ok := item.Value.(configEntry)
		if !ok {
			return ""
		}

		return strings.Join([]string{
			fmt.Sprintf("Key: %s", entry.Key),
			fmt.Sprintf("Type: %s", entry.Type),
			fmt.Sprintf("Sensitive: %t", entry.Sensitive),
			"",
			entry.Value,
			"",
			entry.Description,
		}, "\n")
	})

	return browser.Run()
}

func completeConfigKeys(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	keys := filterByPrefix(configKeyNames(), toComplete)
	return keys, cobra.ShellCompDirectiveNoFileComp
}

func completeConfigSetArgs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return filterByPrefix(configKeyNames(), toComplete), cobra.ShellCompDirectiveNoFileComp
}

func filterByPrefix(values []string, prefix string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

type configEntry struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Sensitive   bool   `json:"sensitive"`
}

func listConfigEntries() []configEntry {
	specs := visibleConfigKeySpecs()
	entries := make([]configEntry, 0, len(specs))

	for _, spec := range specs {
		entries = append(entries, configEntry{
			Key:         spec.Key,
			Type:        string(spec.ValueType),
			Value:       configValueAsString(spec, getConfigValue(spec)),
			Description: spec.Description,
			Sensitive:   spec.Sensitive,
		})
	}

	return entries
}

func getConfigEntry(rawKey string) (configEntry, error) {
	spec, ok := lookupConfigKeySpec(rawKey)
	if !ok {
		return configEntry{}, unknownConfigKeyError(rawKey)
	}

	value := getConfigValue(spec)
	return configEntry{
		Key:         spec.Key,
		Type:        string(spec.ValueType),
		Value:       configValueAsString(spec, value),
		Description: spec.Description,
		Sensitive:   spec.Sensitive,
	}, nil
}

func filterConfigEntries(filter string) ([]configEntry, error) {
	entries := listConfigEntries()
	filtered := make([]configEntry, 0, len(entries))
	for _, entry := range entries {
		if matchesFilter(entry.Key, filter) {
			filtered = append(filtered, entry)
		}
	}
	if len(filtered) == 0 {
		return nil, fterrors.ConfigErrorf(localizer.T(i18n.ConfigErrNoMatchingKeys), filter)
	}
	return filtered, nil
}

func filterNonEmptyConfigEntries(entries []configEntry) []configEntry {
	filtered := make([]configEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Value) == "" {
			continue
		}
		filtered = append(filtered, entry)
	}

	return filtered
}

func matchesFilter(key string, filter string) bool {
	trimmed := strings.TrimSpace(filter)
	if trimmed == "" {
		return true
	}
	return strings.Contains(strings.ToLower(key), strings.ToLower(trimmed))
}

func getConfigValue(spec configKeySpec) any {
	switch spec.ValueType {
	case configValueTypeBool:
		return fotingoConfig.GetBool(spec.Key)
	default:
		if value := fotingoConfig.Get(spec.Key); value != nil {
			return value
		}
		return ""
	}
}

func setConfigEntry(rawKey string, rawValue string) (configEntry, error) {
	spec, ok := lookupConfigKeySpec(rawKey)
	if !ok {
		return configEntry{}, unknownConfigKeyError(rawKey)
	}

	parsed, err := parseConfigKeyValue(spec, rawValue)
	if err != nil {
		return configEntry{}, fterrors.ConfigError(err.Error())
	}

	fotingoConfig.Set(spec.Key, parsed)
	if err := ftconfig.PersistConfigValue(fotingoConfig, spec.Key, parsed); err != nil {
		return configEntry{}, fterrors.ConfigErrorf("failed to persist config %s: %v", spec.Key, err)
	}

	return configEntry{
		Key:         spec.Key,
		Type:        string(spec.ValueType),
		Value:       configValueAsString(spec, parsed),
		Description: spec.Description,
		Sensitive:   spec.Sensitive,
	}, nil
}

func isInteractiveTerminal() bool {
	stdinInfo, stdinErr := os.Stdin.Stat()
	if stdinErr != nil {
		return false
	}

	stdoutInfo, stdoutErr := os.Stdout.Stat()
	if stdoutErr != nil {
		return false
	}

	return (stdinInfo.Mode()&os.ModeCharDevice) != 0 && (stdoutInfo.Mode()&os.ModeCharDevice) != 0
}

func unknownConfigKeyError(rawKey string) error {
	trimmed := strings.TrimSpace(rawKey)
	switch trimmed {
	case "jira.siteId", "jira.siteRoot":
		return fterrors.ConfigErrorf(
			"unknown key %q. Legacy Jira keys are no longer supported; use jira.root",
			trimmed,
		)
	default:
		return fterrors.ConfigErrorf(localizer.T(i18n.ConfigErrUnknownKey), rawKey)
	}
}
