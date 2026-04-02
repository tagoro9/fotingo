package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/i18n"
	"gopkg.in/yaml.v3"
)

const (
	// CanonicalConfigFileName is the primary user config filename.
	CanonicalConfigFileName = "config.yaml"
)

// trackedDefaultValues defines keys whose default values are implicit:
// they should not be persisted unless explicitly changed from default.
var trackedDefaultValues = map[string]any{
	"git.remote":           "origin",
	"git.worktree.enabled": false,
	"locale":               i18n.DefaultLocale,
	"telemetry.enabled":    true,
}

// FileSystem represents the file system operations needed by the config package
type FileSystem interface {
	UserHomeDir() (string, error)
	MkdirAll(path string, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
}

// DefaultFileSystem implements FileSystem using the real os package
type DefaultFileSystem struct{}

func (d *DefaultFileSystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

func (d *DefaultFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (d *DefaultFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

type ConfigurableService interface {
	GetConfig() *viper.Viper
	SaveConfig(key string, value any) error
}

type ViperConfigurableService struct {
	Config *viper.Viper
	Prefix string
}

func (v *ViperConfigurableService) GetConfig() *viper.Viper {
	return v.Config.Sub(v.Prefix)
}

func (v *ViperConfigurableService) GetConfigString(key string) string {
	return v.Config.GetString(fmt.Sprintf("%s.%s", v.Prefix, key))
}

func (v *ViperConfigurableService) SaveConfig(key string, value any) error {
	if v == nil || v.Config == nil {
		return errors.New("config cannot be nil")
	}

	fullKey := fmt.Sprintf("%s.%s", v.Prefix, key)
	return PersistConfigValue(v.Config, fullKey, value)
}

func createConfigDir(fs FileSystem) (string, error) {
	homeDir, err := fs.UserHomeDir()
	if err != nil {
		return "", err
	}
	configFolder := fmt.Sprintf("%s/.config/fotingo", homeDir)
	if err := fs.MkdirAll(configFolder, 0700); err != nil {
		return "", err
	}
	return configFolder, nil
}

func NewDefaultConfig() *viper.Viper {
	config := viper.New()
	config.SetConfigName(strings.TrimSuffix(CanonicalConfigFileName, filepath.Ext(CanonicalConfigFileName)))
	config.SetEnvPrefix("FOTINGO")
	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	config.AutomaticEnv()
	config.SetDefault("git.remote", "origin")
	config.SetDefault("git.branchTemplate", "{{.Issue.ShortName}}/{{.Issue.Info}}_{{.Issue.SanitizedSummary}}")
	config.SetDefault("git.worktree.enabled", false)
	config.SetDefault("github.cache.labelsTTL", "30m")
	config.SetDefault("github.cache.collaboratorsTTL", "720h")
	config.SetDefault("github.cache.orgMembersTTL", "720h")
	config.SetDefault("github.cache.teamsTTL", "720h")
	config.SetDefault("github.cache.userProfilesTTL", "720h")
	config.SetDefault("locale", i18n.DefaultLocale)
	config.SetDefault("telemetry.enabled", true)
	return config
}

func NewConfig() *viper.Viper {
	return newConfigWithFileSystem(&DefaultFileSystem{})
}

func newConfigWithFileSystem(fs FileSystem) *viper.Viper {
	configPath, err := createConfigDir(fs)
	if err != nil {
		panic(fmt.Sprintf(i18n.T(i18n.RootErrConfigDir), err))
	}
	config := NewDefaultConfig()

	canonicalPath := filepath.Join(configPath, CanonicalConfigFileName)
	config.SetConfigFile(canonicalPath)
	if !fileExists(fs, canonicalPath) {
		if err := writeBootstrapConfig(config); err != nil {
			panic(fmt.Sprintf(i18n.T(i18n.RootErrConfigFile), err))
		}
	}

	if err := config.ReadInConfig(); err != nil {
		panic(fmt.Sprintf(i18n.T(i18n.RootErrConfigFile), err))
	}

	return config
}

// PersistConfigValue writes a single full config key (e.g. "jira.root") without serializing runtime defaults.
func PersistConfigValue(config *viper.Viper, key string, value any) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}

	config.Set(key, value)

	targetPath, err := resolveConfigWritePath(config)
	if err != nil {
		return err
	}

	settings, err := readConfigSettings(targetPath)
	if err != nil {
		return err
	}

	if shouldPersistConfigValue(config, key, value) {
		setNestedSetting(settings, key, value)
	} else {
		removeNestedSetting(settings, key)
	}
	return writeConfigSettings(targetPath, settings)
}

func writeBootstrapConfig(config *viper.Viper) error {
	targetPath, err := resolveConfigWritePath(config)
	if err != nil {
		return err
	}

	settings := map[string]any{}
	for key := range trackedDefaultValues {
		value := config.Get(key)
		if shouldPersistConfigValue(config, key, value) {
			setNestedSetting(settings, key, value)
		}
	}

	return writeConfigSettings(targetPath, settings)
}

func resolveConfigWritePath(config *viper.Viper) (string, error) {
	configFile := strings.TrimSpace(config.ConfigFileUsed())
	if configFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configFile = filepath.Join(homeDir, ".config", "fotingo", CanonicalConfigFileName)
	}

	if err := os.MkdirAll(filepath.Dir(configFile), 0700); err != nil {
		return "", err
	}

	config.SetConfigFile(configFile)
	return configFile, nil
}

func readConfigSettings(path string) (map[string]any, error) {
	settings := map[string]any{}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settings, nil
		}
		return nil, err
	}

	if len(raw) == 0 {
		return settings, nil
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if parsed == nil {
		return settings, nil
	}

	return parsed, nil
}

func writeConfigSettings(path string, settings map[string]any) error {
	serialized, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	return os.WriteFile(path, serialized, 0644)
}

func setNestedSetting(settings map[string]any, dottedKey string, value any) {
	parts := strings.Split(strings.TrimSpace(dottedKey), ".")
	if len(parts) == 0 {
		return
	}

	current := settings
	for index, part := range parts {
		if index == len(parts)-1 {
			current[part] = value
			return
		}

		next, ok := current[part]
		if !ok {
			child := map[string]any{}
			current[part] = child
			current = child
			continue
		}

		child, ok := next.(map[string]any)
		if !ok {
			child = map[string]any{}
			current[part] = child
		}
		current = child
	}
}

func removeNestedSetting(settings map[string]any, dottedKey string) {
	parts := strings.Split(strings.TrimSpace(dottedKey), ".")
	if len(parts) == 0 {
		return
	}

	current := settings
	parents := make([]map[string]any, 0, len(parts))
	parents = append(parents, current)
	for index, part := range parts {
		if index == len(parts)-1 {
			delete(current, part)
			break
		}

		next, ok := current[part]
		if !ok {
			return
		}

		child, ok := next.(map[string]any)
		if !ok {
			return
		}

		current = child
		parents = append(parents, current)
	}

	for index := len(parts) - 2; index >= 0; index-- {
		parent := parents[index]
		childKey := parts[index]
		child, ok := parent[childKey].(map[string]any)
		if !ok || len(child) > 0 {
			break
		}
		delete(parent, childKey)
	}
}

func shouldPersistConfigValue(config *viper.Viper, key string, value any) bool {
	defaultValue, ok := trackedDefaultValues[key]
	if !ok {
		return true
	}

	return fmt.Sprintf("%v", value) != fmt.Sprintf("%v", defaultValue)
}

func fileExists(fs FileSystem, path string) bool {
	if _, err := fs.Stat(path); err != nil {
		return false
	}
	return true
}
