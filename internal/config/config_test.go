package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// MockFileSystem implements FileSystem for testing
type MockFileSystem struct {
	homeDir     string
	createdDirs map[string]struct{}
	statError   error
}

func NewMockFileSystem(homeDir string) *MockFileSystem {
	return &MockFileSystem{
		homeDir:     homeDir,
		createdDirs: make(map[string]struct{}),
	}
}

func (m *MockFileSystem) UserHomeDir() (string, error) {
	if m.homeDir == "" {
		return "", os.ErrNotExist
	}
	return m.homeDir, nil
}

func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	if m.createdDirs == nil {
		m.createdDirs = make(map[string]struct{})
	}
	m.createdDirs[path] = struct{}{}
	return os.MkdirAll(path, perm)
}

func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if m.statError != nil {
		return nil, m.statError
	}
	return os.Stat(name)
}

// ConfigTestSuite is the test suite for config package
type ConfigTestSuite struct {
	suite.Suite
	mockFS  *MockFileSystem
	tmpDir  string
	oldHome string
}

func (suite *ConfigTestSuite) SetupTest() {
	var err error
	suite.tmpDir, err = os.MkdirTemp("", "fotingo-test-*")
	assert.NoError(suite.T(), err)

	suite.oldHome = os.Getenv("HOME")
	assert.NoError(suite.T(), os.Setenv("HOME", suite.tmpDir))

	suite.mockFS = NewMockFileSystem(suite.tmpDir)
}

func (suite *ConfigTestSuite) TearDownTest() {
	assert.NoError(suite.T(), os.Setenv("HOME", suite.oldHome))
	assert.NoError(suite.T(), os.RemoveAll(suite.tmpDir))
}

func (suite *ConfigTestSuite) TestNewDefaultConfig() {
	config := NewDefaultConfig()

	assert.NotNil(suite.T(), config)
	assert.Equal(suite.T(), "origin", config.GetString("git.remote"))
	assert.Equal(suite.T(), "{{.Issue.ShortName}}/{{.Issue.Info}}_{{.Issue.SanitizedSummary}}", config.GetString("git.branchTemplate"))
	assert.Equal(suite.T(), "30m", config.GetString("github.cache.labelsTTL"))
	assert.Equal(suite.T(), "720h", config.GetString("github.cache.collaboratorsTTL"))
	assert.Equal(suite.T(), "720h", config.GetString("github.cache.orgMembersTTL"))
	assert.Equal(suite.T(), "720h", config.GetString("github.cache.teamsTTL"))
	assert.True(suite.T(), config.GetBool("telemetry.enabled"))
}

func (suite *ConfigTestSuite) TestNewDefaultConfig_UsesEnvironmentVariables() {
	suite.T().Setenv("FOTINGO_JIRA_ROOT", "https://env.atlassian.net")
	config := NewDefaultConfig()

	assert.Equal(suite.T(), "https://env.atlassian.net", config.GetString("jira.root"))
}

func (suite *ConfigTestSuite) TestViperConfigurableService() {
	configDir := filepath.Join(suite.tmpDir, ".config/fotingo")
	err := suite.mockFS.MkdirAll(configDir, 0700)
	assert.NoError(suite.T(), err)

	config := viper.New()
	configPath := filepath.Join(configDir, "test.yaml")
	config.SetConfigFile(configPath)
	config.SetConfigType("yaml")

	// Initialize the config with some data
	config.Set("test.key", "value")
	err = config.WriteConfig()
	assert.NoError(suite.T(), err)

	service := &ViperConfigurableService{
		Config: config,
		Prefix: "test",
	}

	subConfig := service.GetConfig()
	assert.NotNil(suite.T(), subConfig)
	assert.Equal(suite.T(), "value", subConfig.GetString("key"))

	err = service.SaveConfig("newKey", "newValue")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "newValue", config.GetString("test.newKey"))

	configFile, err := os.ReadFile(configPath)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), string(configFile), "test:\n    key: value\n    newKey: newValue")
}

func (suite *ConfigTestSuite) TestCreateConfigDir() {
	configDir, err := createConfigDir(suite.mockFS)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), filepath.Join(suite.tmpDir, ".config/fotingo"), configDir)
	assert.Contains(suite.T(), suite.mockFS.createdDirs, filepath.Join(suite.tmpDir, ".config/fotingo"))

	errorFS := &MockFileSystem{homeDir: ""}
	_, err = createConfigDir(errorFS)
	assert.Error(suite.T(), err)
}

func (suite *ConfigTestSuite) TestNewConfig() {
	config := newConfigWithFileSystem(suite.mockFS)
	assert.NotNil(suite.T(), config)

	assert.Contains(suite.T(), suite.mockFS.createdDirs, filepath.Join(suite.tmpDir, ".config/fotingo"))

	assert.Equal(suite.T(), "origin", config.GetString("git.remote"))
	assert.Equal(suite.T(), "{{.Issue.ShortName}}/{{.Issue.Info}}_{{.Issue.SanitizedSummary}}", config.GetString("git.branchTemplate"))
	assert.Equal(suite.T(), "30m", config.GetString("github.cache.labelsTTL"))
	assert.Equal(suite.T(), "720h", config.GetString("github.cache.collaboratorsTTL"))
	assert.Equal(suite.T(), "720h", config.GetString("github.cache.orgMembersTTL"))
	assert.Equal(suite.T(), "720h", config.GetString("github.cache.teamsTTL"))
	assert.True(suite.T(), config.GetBool("telemetry.enabled"))

	// Verify the config file exists
	configFilePath := filepath.Join(suite.tmpDir, ".config/fotingo/config.yaml")
	_, err := os.Stat(configFilePath)
	assert.NoError(suite.T(), err)

	contents, err := os.ReadFile(configFilePath)
	assert.NoError(suite.T(), err)
	assert.NotContains(suite.T(), string(contents), "remote: origin")
	assert.NotContains(suite.T(), string(contents), "locale: en")
	assert.NotContains(suite.T(), string(contents), "branchtemplate")
}

func (suite *ConfigTestSuite) TestPersistConfigValue_DropsTrackedDefaults() {
	configPath := filepath.Join(suite.tmpDir, ".config/fotingo/config.yaml")
	config := NewDefaultConfig()
	config.SetConfigFile(configPath)

	err := writeBootstrapConfig(config)
	assert.NoError(suite.T(), err)

	err = PersistConfigValue(config, "git.remote", "upstream")
	assert.NoError(suite.T(), err)

	contents, err := os.ReadFile(configPath)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), string(contents), "remote: upstream")

	err = PersistConfigValue(config, "git.remote", "origin")
	assert.NoError(suite.T(), err)
	contents, err = os.ReadFile(configPath)
	assert.NoError(suite.T(), err)
	assert.NotContains(suite.T(), string(contents), "remote:")

	err = PersistConfigValue(config, "locale", "en")
	assert.NoError(suite.T(), err)
	contents, err = os.ReadFile(configPath)
	assert.NoError(suite.T(), err)
	assert.NotContains(suite.T(), string(contents), "locale:")
}

func (suite *ConfigTestSuite) TestNewConfigWithFileSystemError() {
	errorFS := &MockFileSystem{homeDir: ""}
	assert.Panics(suite.T(), func() {
		newConfigWithFileSystem(errorFS)
	})
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}
