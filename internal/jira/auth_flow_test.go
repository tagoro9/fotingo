package jira

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/auth"
	"github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/config"
	"golang.org/x/oauth2"
)

func TestConfiguredAPICredentials_PrefersUserToken(t *testing.T) {
	cfg := viper.New()
	cfg.Set("jira.user.login", "user@example.com")
	cfg.Set("jira.user.token", "api-token")

	client := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "jira"},
	}

	login, token, ok := client.configuredAPICredentials()
	require.True(t, ok)
	assert.Equal(t, "user@example.com", login)
	assert.Equal(t, "api-token", token)
}

func TestConfiguredAPICredentials_RequiresUserToken(t *testing.T) {
	cfg := viper.New()
	cfg.Set("jira.user.login", "user@example.com")
	cfg.Set("jira.token", "legacy-api-token")

	client := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "jira"},
	}

	_, _, ok := client.configuredAPICredentials()
	assert.False(t, ok)
}

func TestConfiguredAPICredentials_RequiresUserLogin(t *testing.T) {
	cfg := viper.New()
	cfg.Set("jira.user.token", "api-token")

	client := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "jira"},
	}

	_, _, ok := client.configuredAPICredentials()
	assert.False(t, ok)
}

func TestAuthenticate_ReturnsAuthRequiredWhenPromptDisabled(t *testing.T) {
	cfg := viper.New()
	cfg.Set("jira.root", "https://acme.atlassian.net")

	client := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "jira"},
		allowPrompt:              false,
	}

	_, err := client.Authenticate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAuthRequired))
}

func TestAuthenticate_OAuthTokenWithoutCredentialsFallsBackToAuthRequiredWhenNonInteractive(t *testing.T) {
	cfg := viper.New()
	cfg.Set("jira.root", "https://acme.atlassian.net")

	oauthToken := auth.AccessToken{}
	oauthToken.Token = "oauth-token"
	serialized, err := marshalOAuthToken(oauthToken)
	require.NoError(t, err)
	cfg.Set("jira.token", serialized)

	client := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "jira"},
		allowPrompt:              false,
	}

	_, err = client.Authenticate()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthRequired)
	assert.NotErrorIs(t, err, ErrOAuthCredentialsMissing)
}

func TestAuthenticate_OAuthTokenWithoutCredentialsPromptsForAPIToken(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()

	cfg := viper.New()
	cfg.Set("jira.root", server.URL)
	cfg.SetConfigFile(filepath.Join(t.TempDir(), "config.yaml"))

	oauthToken := auth.AccessToken{}
	oauthToken.Token = "oauth-token"
	serialized, err := marshalOAuthToken(oauthToken)
	require.NoError(t, err)
	cfg.Set("jira.token", serialized)

	promptCalled := false
	client := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "jira"},
		allowPrompt:              true,
		promptAPICreds: func() (string, string, error) {
			promptCalled = true
			return "user@example.com", "api-token", nil
		},
	}

	origIsInputTerminalFn := isInputTerminalFn
	isInputTerminalFn = func() bool { return true }
	t.Cleanup(func() { isInputTerminalFn = origIsInputTerminalFn })

	_, err = client.Authenticate()
	require.NoError(t, err)
	assert.True(t, promptCalled)
	assert.Equal(t, "user@example.com", cfg.GetString("jira.user.login"))
	assert.Equal(t, "api-token", cfg.GetString("jira.user.token"))
	require.NotNil(t, client.client)
}

func TestAuthenticate_UsesAPITokenCredentials(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()

	cfg := viper.New()
	cfg.Set("jira.root", server.URL)
	cfg.Set("jira.user.login", "user@example.com")
	cfg.Set("jira.user.token", "api-token")

	client := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "jira"},
		allowPrompt:              false,
	}

	_, err := client.Authenticate()
	require.NoError(t, err)
	require.NotNil(t, client.client)
}

func TestAuthenticate_OAuthCapablePromptSupportsTokenMethod(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()

	origClientID := jiraOAuthClientID
	origClientSecret := jiraOAuthClientSecret
	jiraOAuthClientID = "jira-client-id"
	jiraOAuthClientSecret = "jira-client-secret"
	t.Cleanup(func() {
		jiraOAuthClientID = origClientID
		jiraOAuthClientSecret = origClientSecret
	})

	cfg := viper.New()
	cfg.Set("jira.root", server.URL)
	cfg.Set("jira.token", `{"access_token":"old-oauth-token"}`)
	cfg.SetConfigFile(filepath.Join(t.TempDir(), "config.yaml"))

	methodPromptCalled := false
	credsPromptCalled := false
	client := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "jira"},
		allowPrompt:              true,
		authenticator: &authenticator{
			authConfig: &oauth2.Config{
				ClientID:     jiraOAuthClientID,
				ClientSecret: jiraOAuthClientSecret,
			},
			allowInteractive: false,
			getToken:         func() string { return "" },
			storeToken:       func(string) error { return nil },
		},
		promptAuthMethod: func() (string, error) {
			methodPromptCalled = true
			return jiraAuthMethodToken, nil
		},
		promptAPICreds: func() (string, string, error) {
			credsPromptCalled = true
			return "user@example.com", "api-token", nil
		},
	}

	origIsInputTerminalFn := isInputTerminalFn
	isInputTerminalFn = func() bool { return true }
	t.Cleanup(func() { isInputTerminalFn = origIsInputTerminalFn })

	_, err := client.Authenticate()
	require.NoError(t, err)
	assert.True(t, methodPromptCalled)
	assert.True(t, credsPromptCalled)
	assert.Equal(t, "user@example.com", cfg.GetString("jira.user.login"))
	assert.Equal(t, "api-token", cfg.GetString("jira.user.token"))
	assert.Equal(t, "", cfg.GetString("jira.token"))
}

func TestResolveOAuthSiteID_UsesCacheHit(t *testing.T) {
	cacheStore, err := cache.New(cache.WithPath(filepath.Join(t.TempDir(), "cache.db")), cache.WithLogger(nil))
	require.NoError(t, err)
	defer func() { _ = cacheStore.Close() }()

	client := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: viper.New(), Prefix: "jira"},
		metadataCache:            cacheStore,
	}

	root := "https://acme.atlassian.net"
	client.cacheOAuthSiteID(root, "cached-site-id")

	siteID, err := client.resolveOAuthSiteID(root, &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network call should not happen on cache hit")
		}),
	})
	require.NoError(t, err)
	assert.Equal(t, "cached-site-id", siteID)
}

func TestResolveOAuthSiteID_UsesLegacyConfigFallbackAndCaches(t *testing.T) {
	cfg := viper.New()
	cfg.Set("jira.siteId", "legacy-site-id")
	cfg.Set("jira.siteRoot", "https://acme.atlassian.net")

	cacheStore, err := cache.New(cache.WithPath(filepath.Join(t.TempDir(), "cache.db")), cache.WithLogger(nil))
	require.NoError(t, err)
	defer func() { _ = cacheStore.Close() }()

	client := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: cfg, Prefix: "jira"},
		metadataCache:            cacheStore,
	}

	root := "https://acme.atlassian.net"
	siteID, err := client.resolveOAuthSiteID(root, &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network call should not happen on legacy fallback")
		}),
	})
	require.NoError(t, err)
	assert.Equal(t, "legacy-site-id", siteID)

	cached, ok := client.cachedOAuthSiteID(root)
	require.True(t, ok)
	assert.Equal(t, "legacy-site-id", cached)
}

func TestResolveOAuthSiteID_CacheMissFetchesAndCaches(t *testing.T) {
	cacheStore, err := cache.New(cache.WithPath(filepath.Join(t.TempDir(), "cache.db")), cache.WithLogger(nil))
	require.NoError(t, err)
	defer func() { _ = cacheStore.Close() }()

	client := &jira{
		ViperConfigurableService: &config.ViperConfigurableService{Config: viper.New(), Prefix: "jira"},
		metadataCache:            cacheStore,
	}

	root := "https://acme.atlassian.net"
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.atlassian.com/oauth/token/accessible-resources" {
				return nil, fmt.Errorf("unexpected URL: %s", req.URL.String())
			}

			body := `[{"id":"site-123","url":"https://acme.atlassian.net","name":"Acme"}]`
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	siteID, err := client.resolveOAuthSiteID(root, httpClient)
	require.NoError(t, err)
	assert.Equal(t, "site-123", siteID)

	cached, ok := client.cachedOAuthSiteID(root)
	require.True(t, ok)
	assert.Equal(t, "site-123", cached)
}

func TestNewIssueTypesCacheStore_ReturnsNilStoreOnInitError(t *testing.T) {
	cacheFilePath := filepath.Join(t.TempDir(), "cache-file")
	require.NoError(t, os.WriteFile(cacheFilePath, []byte("not-a-directory"), 0644))

	cfg := viper.New()
	cfg.Set("cache.path", cacheFilePath)

	store, err := newIssueTypesCacheStore(cfg)
	require.Error(t, err)
	assert.Nil(t, store)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func marshalOAuthToken(token auth.AccessToken) (string, error) {
	data, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func TestIsUnauthorizedError(t *testing.T) {
	t.Run("status code", func(t *testing.T) {
		err := errors.New("request failed. Please analyze the request body for more details. Status code: 401")
		assert.True(t, IsUnauthorizedError(err))
	})

	t.Run("message fallback", func(t *testing.T) {
		err := errors.New("Unauthorized")
		assert.True(t, IsUnauthorizedError(err))
	})

	t.Run("non unauthorized", func(t *testing.T) {
		err := errors.New("request failed. Status code: 500")
		assert.False(t, IsUnauthorizedError(err))
	})
}

func TestShouldWarnIgnoredStoredOAuthToken(t *testing.T) {
	oauthToken := auth.AccessToken{}
	oauthToken.Token = "oauth-token"
	serialized, err := marshalOAuthToken(oauthToken)
	require.NoError(t, err)

	cfg := viper.New()
	cfg.Set("jira.token", serialized)

	origClientID := jiraOAuthClientID
	origClientSecret := jiraOAuthClientSecret
	t.Cleanup(func() {
		jiraOAuthClientID = origClientID
		jiraOAuthClientSecret = origClientSecret
	})

	jiraOAuthClientID = ""
	jiraOAuthClientSecret = ""
	assert.True(t, ShouldWarnIgnoredStoredOAuthToken(cfg))

	jiraOAuthClientID = "jira-client-id"
	jiraOAuthClientSecret = "jira-client-secret"
	assert.False(t, ShouldWarnIgnoredStoredOAuthToken(cfg))
}
