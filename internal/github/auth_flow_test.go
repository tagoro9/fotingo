package github

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/auth"
	"github.com/tagoro9/fotingo/internal/config"
)

func TestResolveToken_UsesStoredRawToken(t *testing.T) {
	cfg := viper.New()
	cfg.Set("github.token", "ghp_raw_token")

	client := &github{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "github"},
		allowPrompt:              false,
	}

	token, err := client.resolveToken()
	require.NoError(t, err)
	assert.Equal(t, "ghp_raw_token", token)
}

func TestResolveToken_UsesStoredOAuthToken(t *testing.T) {
	cfg := viper.New()
	oauthToken := auth.AccessToken{}
	oauthToken.Token = "oauth_access_token"
	serialized, err := json.Marshal(oauthToken)
	require.NoError(t, err)

	cfg.Set("github.token", string(serialized))

	client := &github{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "github"},
		allowPrompt:              false,
	}

	token, resolveErr := client.resolveToken()
	require.NoError(t, resolveErr)
	assert.Equal(t, "oauth_access_token", token)
}

func TestResolveToken_ReturnsAuthRequiredWhenPromptDisabled(t *testing.T) {
	cfg := viper.New()
	client := &github{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "github"},
		allowPrompt:              false,
	}

	_, err := client.resolveToken()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAuthRequired))
}

func TestResolveToken_PromptTokenPersistsConfig(t *testing.T) {
	cfg := viper.New()
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	cfg.SetConfigFile(configFile)
	cfg.SetConfigType("yaml")
	require.NoError(t, cfg.WriteConfigAs(configFile))

	client := &github{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "github"},
		allowPrompt:              true,
		promptAuth: func() (string, error) {
			return githubAuthMethodToken, nil
		},
		promptToken: func() (string, error) {
			return "ghp_prompted_token", nil
		},
	}

	token, err := client.resolveToken()
	require.NoError(t, err)
	assert.Equal(t, "ghp_prompted_token", token)
	assert.Equal(t, "ghp_prompted_token", cfg.GetString("github.token"))
}

func TestNewMetadataCacheStore_ReturnsNilStoreOnInitError(t *testing.T) {
	cacheFilePath := filepath.Join(t.TempDir(), "cache-file")
	require.NoError(t, os.WriteFile(cacheFilePath, []byte("not-a-directory"), 0644))

	cfg := viper.New()
	cfg.Set("cache.path", cacheFilePath)

	store, err := newMetadataCacheStore(cfg)
	require.Error(t, err)
	assert.Nil(t, store)
}

func TestValidateOAuthClientID(t *testing.T) {
	origClientID := oauthClientID
	t.Cleanup(func() {
		oauthClientID = origClientID
	})

	oauthClientID = ""
	err := validateOAuthClientID()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOAuthClientIDMissing)

	oauthClientID = "client-id"
	require.NoError(t, validateOAuthClientID())
}

func TestNewAuthOnlyWithOptions_UsesConfiguredTokenWithoutGitRemote(t *testing.T) {
	cfg := viper.New()
	cfg.Set("github.token", "ghp_auth_only_token")

	client, err := NewAuthOnlyWithOptions(cfg, false)
	require.NoError(t, err)
	require.NotNil(t, client)

	internalClient, ok := client.(*github)
	require.True(t, ok)
	assert.Empty(t, internalClient.owner)
	assert.Empty(t, internalClient.repo)
}
