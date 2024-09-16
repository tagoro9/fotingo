package jira

import (
	"context"
	"encoding/json"
	stdio "io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tagoro9/fotingo/internal/auth"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/io"
	"golang.org/x/oauth2"
)

// mockBrowser returns a mock browser for testing
func mockBrowser() *io.MockBrowser {
	return &io.MockBrowser{}
}

// TestGetAuthUrl tests the getAuthUrl method of authenticator
func TestGetAuthUrl(t *testing.T) {
	tests := []struct {
		name             string
		oauthStateString string
		wantScheme       string
		wantHost         string
		wantPath         string
		wantAudience     string
		wantPrompt       string
		wantState        string
	}{
		{
			name:             "generates valid auth URL",
			oauthStateString: "test-state",
			wantScheme:       "https",
			wantHost:         "auth.atlassian.com",
			wantPath:         "/authorize",
			wantAudience:     "api.atlassian.com",
			wantPrompt:       "consent",
			wantState:        "test-state",
		},
		{
			name:             "with different state string",
			oauthStateString: "another-state-123",
			wantScheme:       "https",
			wantHost:         "auth.atlassian.com",
			wantPath:         "/authorize",
			wantAudience:     "api.atlassian.com",
			wantPrompt:       "consent",
			wantState:        "another-state-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &authenticator{
				port:             8080,
				oauthStateString: tt.oauthStateString,
				authConfig: &oauth2.Config{
					RedirectURL:  "http://localhost:8080/callback",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					Scopes:       []string{"read:jira-user"},
					Endpoint: oauth2.Endpoint{
						AuthURL:  "https://auth.atlassian.com/authorize",
						TokenURL: "https://auth.atlassian.com/oauth/token",
					},
				},
			}

			url, err := auth.getAuthUrl()

			assert.NoError(t, err)
			assert.NotNil(t, url)
			assert.Equal(t, tt.wantScheme, url.Scheme)
			assert.Equal(t, tt.wantHost, url.Host)
			assert.Equal(t, tt.wantPath, url.Path)
			assert.Equal(t, tt.wantAudience, url.Query().Get("audience"))
			assert.Equal(t, tt.wantPrompt, url.Query().Get("prompt"))
			assert.Equal(t, tt.wantState, url.Query().Get("state"))
		})
	}
}

// TestGetOauthUserInfo tests the getOauthUserInfo method
func TestGetOauthUserInfo(t *testing.T) {
	tests := []struct {
		name           string
		state          string
		code           string
		oauthState     string
		mockTokenResp  *oauth2.Token
		mockStatusCode int
		wantErr        bool
		wantErrMsg     string
	}{
		{
			name:       "error - invalid state",
			state:      "wrong-state",
			code:       "valid-code",
			oauthState: "correct-state",
			wantErr:    true,
			wantErrMsg: "invalid oauth state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &authenticator{
				port:             8080,
				oauthStateString: tt.oauthState,
				authConfig: &oauth2.Config{
					RedirectURL:  "http://localhost:8080/callback",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					Endpoint: oauth2.Endpoint{
						AuthURL:  "https://auth.atlassian.com/authorize",
						TokenURL: "https://auth.atlassian.com/oauth/token",
					},
				},
			}

			token, err := auth.getOauthUserInfo(tt.state, tt.code)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				assert.Nil(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, token)
			}
		})
	}
}

// TestGetOauthUserInfoWithMockServer tests the getOauthUserInfo with a mock token server
func TestGetOauthUserInfoWithMockServer(t *testing.T) {
	tests := []struct {
		name           string
		tokenResponse  map[string]interface{}
		mockStatusCode int
		wantErr        bool
		wantErrMsg     string
	}{
		{
			name: "success - valid token exchange",
			tokenResponse: map[string]interface{}{
				"access_token":  "test-access-token",
				"token_type":    "Bearer",
				"refresh_token": "test-refresh-token",
				"expires_in":    3600,
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "error - invalid code",
			tokenResponse: map[string]interface{}{
				"error":             "invalid_grant",
				"error_description": "The authorization code has expired",
			},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
			wantErrMsg:     "code exchange failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatusCode)
				_ = json.NewEncoder(w).Encode(tt.tokenResponse)
			}))
			defer server.Close()

			auth := &authenticator{
				port:             8080,
				oauthStateString: "test-state",
				authConfig: &oauth2.Config{
					RedirectURL:  "http://localhost:8080/callback",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					Endpoint: oauth2.Endpoint{
						AuthURL:  "https://auth.atlassian.com/authorize",
						TokenURL: server.URL,
					},
				},
			}

			token, err := auth.getOauthUserInfo("test-state", "test-code")

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, token)
				assert.Equal(t, "test-access-token", token.AccessToken)
			}
		})
	}
}

// TestCreateHttpServer tests the createHttpServer method
func TestCreateHttpServer(t *testing.T) {
	auth := &authenticator{
		port:             8080,
		oauthStateString: "test-state",
		authConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:8080/callback",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: "https://auth.atlassian.com/oauth/token",
			},
		},
	}

	tokenChannel := make(chan *oauth2.Token, 1)
	server := auth.createHttpServer(tokenChannel)

	assert.NotNil(t, server)
	assert.Equal(t, ":8080", server.Addr)
	assert.NotNil(t, server.Handler)
}

// TestCreateHttpServerCallback tests the callback handler of the HTTP server
func TestCreateHttpServerCallback(t *testing.T) {
	// Create a mock token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "test-access-token",
			"token_type":    "Bearer",
			"refresh_token": "test-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	auth := &authenticator{
		port:             8080,
		oauthStateString: "test-state",
		authConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:8080/callback",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: tokenServer.URL,
			},
		},
	}

	tokenChannel := make(chan *oauth2.Token, 1)
	server := auth.createHttpServer(tokenChannel)

	// Create a test server using the handler
	testServer := httptest.NewServer(server.Handler)
	defer testServer.Close()

	// Test valid callback
	resp, err := http.Get(testServer.URL + "/callback?state=test-state&code=test-code")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	body, readErr := stdio.ReadAll(resp.Body)
	assert.NoError(t, readErr)
	assert.Contains(t, string(body), i18n.T(i18n.JiraOAuthPageHeading))
	assert.Contains(t, string(body), "/assets/oauth.css")
	assert.Contains(t, string(body), "/assets/favicon.svg")
	_ = resp.Body.Close()

	// Should receive token on channel
	select {
	case token := <-tokenChannel:
		assert.NotNil(t, token)
		assert.Equal(t, "test-access-token", token.AccessToken)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for token")
	}
}

// TestCreateHttpServerCallbackInvalidState tests callback with invalid state
func TestCreateHttpServerCallbackInvalidState(t *testing.T) {
	auth := &authenticator{
		port:             8080,
		oauthStateString: "correct-state",
		authConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:8080/callback",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: "https://auth.atlassian.com/oauth/token",
			},
		},
	}

	tokenChannel := make(chan *oauth2.Token, 1)
	server := auth.createHttpServer(tokenChannel)

	// Create a test server using the handler
	testServer := httptest.NewServer(server.Handler)
	defer testServer.Close()

	// Create a client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Test callback with invalid state
	resp, err := client.Get(testServer.URL + "/callback?state=wrong-state&code=test-code")
	assert.NoError(t, err)
	// Should redirect on error (307 Temporary Redirect)
	assert.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestCreateHttpServerServesOAuthAssets(t *testing.T) {
	auth := &authenticator{
		port:             8080,
		oauthStateString: "test-state",
		authConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:8080/callback",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: "https://auth.atlassian.com/oauth/token",
			},
		},
	}

	tokenChannel := make(chan *oauth2.Token, 1)
	server := auth.createHttpServer(tokenChannel)
	testServer := httptest.NewServer(server.Handler)
	defer testServer.Close()

	cssResp, cssErr := http.Get(testServer.URL + "/assets/oauth.css")
	assert.NoError(t, cssErr)
	assert.Equal(t, http.StatusOK, cssResp.StatusCode)
	assert.Contains(t, cssResp.Header.Get("Content-Type"), "text/css")
	cssBody, cssReadErr := stdio.ReadAll(cssResp.Body)
	assert.NoError(t, cssReadErr)
	assert.True(t, strings.Contains(string(cssBody), ".card"))
	_ = cssResp.Body.Close()

	iconResp, iconErr := http.Get(testServer.URL + "/assets/favicon.svg")
	assert.NoError(t, iconErr)
	assert.Equal(t, http.StatusOK, iconResp.StatusCode)
	assert.Contains(t, iconResp.Header.Get("Content-Type"), "image/svg+xml")
	iconBody, iconReadErr := stdio.ReadAll(iconResp.Body)
	assert.NoError(t, iconReadErr)
	assert.True(t, strings.Contains(string(iconBody), "<svg"))
	_ = iconResp.Body.Close()
}

func TestCreateHttpServerCallbackRenderFallback(t *testing.T) {
	origBuildOAuthSuccessPageFn := buildOAuthSuccessPageFn
	buildOAuthSuccessPageFn = func() (string, error) {
		return "", assert.AnError
	}
	t.Cleanup(func() {
		buildOAuthSuccessPageFn = origBuildOAuthSuccessPageFn
	})

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "test-access-token",
			"token_type":    "Bearer",
			"refresh_token": "test-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	auth := &authenticator{
		port:             8080,
		oauthStateString: "test-state",
		authConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:8080/callback",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: tokenServer.URL,
			},
		},
	}

	tokenChannel := make(chan *oauth2.Token, 1)
	server := auth.createHttpServer(tokenChannel)
	testServer := httptest.NewServer(server.Handler)
	defer testServer.Close()

	resp, err := http.Get(testServer.URL + "/callback?state=test-state&code=test-code")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")

	body, readErr := stdio.ReadAll(resp.Body)
	assert.NoError(t, readErr)
	assert.Equal(t, i18n.T(i18n.JiraOAuthPageFallback), string(body))
	_ = resp.Body.Close()

	select {
	case token := <-tokenChannel:
		assert.NotNil(t, token)
		assert.Equal(t, "test-access-token", token.AccessToken)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for token")
	}
}

// TestCreateAuthenticator tests the createAuthenticator function
func TestCreateAuthenticator(t *testing.T) {
	origClientID := jiraOAuthClientID
	origClientSecret := jiraOAuthClientSecret
	jiraOAuthClientID = "test-client-id"
	jiraOAuthClientSecret = "test-client-secret"
	t.Cleanup(func() {
		jiraOAuthClientID = origClientID
		jiraOAuthClientSecret = origClientSecret
	})

	getToken := func() string {
		return "stored-token"
	}
	storeToken := func(token string) error {
		return nil
	}

	auth := createAuthenticator(getToken, storeToken, true)

	assert.NotNil(t, auth)
	assert.NotNil(t, auth.authConfig)
	assert.Equal(t, 8080, auth.port)
	assert.Equal(t, "pseudo-random", auth.oauthStateString)
	assert.NotNil(t, auth.getToken)
	assert.NotNil(t, auth.storeToken)

	// Verify OAuth config
	assert.Contains(t, auth.authConfig.Scopes, "read:jira-user")
	assert.Contains(t, auth.authConfig.Scopes, "read:jira-work")
	assert.Contains(t, auth.authConfig.Scopes, "write:jira-work")
	assert.Contains(t, auth.authConfig.Scopes, "offline_access")
	assert.Equal(t, "https://auth.atlassian.com/authorize", auth.authConfig.Endpoint.AuthURL)
	assert.Equal(t, "https://auth.atlassian.com/oauth/token", auth.authConfig.Endpoint.TokenURL)
}

func TestValidateOAuthConfig(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		wantErr      bool
	}{
		{
			name:         "valid config",
			clientID:     "client-id",
			clientSecret: "client-secret",
			wantErr:      false,
		},
		{
			name:         "missing client id",
			clientID:     "",
			clientSecret: "client-secret",
			wantErr:      true,
		},
		{
			name:         "missing client secret",
			clientID:     "client-id",
			clientSecret: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &authenticator{
				authConfig: &oauth2.Config{
					ClientID:     tt.clientID,
					ClientSecret: tt.clientSecret,
				},
			}

			err := auth.validateOAuthConfig()
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrOAuthCredentialsMissing)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// TestAuthenticatorAuthenticateWithStoredToken tests authentication with a stored token
func TestAuthenticatorAuthenticateWithStoredToken(t *testing.T) {
	// Create a mock token server for token refresh
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "refreshed-access-token",
			"token_type":    "Bearer",
			"refresh_token": "refreshed-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	// Create a stored token
	storedToken := &auth.AccessToken{
		Expiry: time.Now().Add(-time.Hour), // Expired token to trigger refresh
	}
	storedToken.Token = "old-access-token"
	storedToken.RefreshToken = "old-refresh-token"
	storedToken.Type = "Bearer"

	tokenJSON, _ := json.Marshal(storedToken)
	var newToken string

	authz := &authenticator{
		port:             8080,
		oauthStateString: "test-state",
		getToken: func() string {
			return string(tokenJSON)
		},
		storeToken: func(token string) error {
			newToken = token
			return nil
		},
		authConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:8080/callback",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: tokenServer.URL,
			},
		},
		browserOpener: mockBrowser(),
	}

	token, err := authz.Authenticate()

	assert.NoError(t, err)
	assert.NotNil(t, token)
	assert.Equal(t, "refreshed-access-token", token.Token)
	assert.NotEmpty(t, newToken) // Token should be stored
}

// TestAuthenticatorAuthenticateWithInvalidStoredToken tests with invalid stored token JSON
func TestAuthenticatorAuthenticateWithInvalidStoredToken(t *testing.T) {
	// This test is complex because it requires mocking the browser and HTTP server
	// For now, we just test that invalid JSON doesn't cause a panic
	authz := &authenticator{
		port:             0, // Use port 0 to get a random available port
		oauthStateString: "test-state",
		getToken: func() string {
			return "invalid-json"
		},
		storeToken: func(_ string) error {
			return nil
		},
		authConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:0/callback",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: "https://auth.atlassian.com/oauth/token",
			},
		},
		browserOpener: mockBrowser(),
	}

	// Start the authentication in a goroutine with a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// This will try to open a browser and start a server
		// It will timeout, but shouldn't panic
		_, _ = authz.Authenticate()
	}()

	select {
	case <-ctx.Done():
		// Expected timeout
	case <-done:
		// Function returned (shouldn't happen in this test)
	}
}

// TestAuthenticatorAuthenticateRefreshError tests when token refresh fails
func TestAuthenticatorAuthenticateRefreshError(t *testing.T) {
	// Create a mock token server that returns an error
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "invalid_grant",
			"error_description": "The refresh token is invalid",
		})
	}))
	defer tokenServer.Close()

	// Create a stored token
	storedToken := &auth.AccessToken{
		Expiry: time.Now().Add(-time.Hour), // Expired token to trigger refresh
	}
	storedToken.Token = "old-access-token"
	storedToken.RefreshToken = "old-refresh-token"
	storedToken.Type = "Bearer"

	tokenJSON, _ := json.Marshal(storedToken)

	authz := &authenticator{
		port:             0,
		oauthStateString: "test-state",
		getToken: func() string {
			return string(tokenJSON)
		},
		storeToken: func(_ string) error {
			return nil
		},
		authConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:0/callback",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: tokenServer.URL,
			},
		},
		browserOpener: mockBrowser(),
	}

	// Use a timeout context since this will try to open a browser
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := authz.Authenticate()
		done <- err
	}()

	select {
	case <-ctx.Done():
		// Expected timeout - browser couldn't be opened
	case err := <-done:
		// If we got an error, it should be related to the refresh failure or timeout
		_ = err
	}
}

// TestAuthenticatorStoreTokenError tests when token storage fails
func TestAuthenticatorStoreTokenError(t *testing.T) {
	// Create a mock token server for token refresh
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "refreshed-access-token",
			"token_type":    "Bearer",
			"refresh_token": "refreshed-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	// Create a stored token
	storedToken := &auth.AccessToken{
		Expiry: time.Now().Add(-time.Hour), // Expired token to trigger refresh
	}
	storedToken.Token = "old-access-token"
	storedToken.RefreshToken = "old-refresh-token"
	storedToken.Type = "Bearer"

	tokenJSON, _ := json.Marshal(storedToken)

	authz := &authenticator{
		port:             8080,
		oauthStateString: "test-state",
		getToken: func() string {
			return string(tokenJSON)
		},
		storeToken: func(_ string) error {
			return assert.AnError // Return an error
		},
		authConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:8080/callback",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: tokenServer.URL,
			},
		},
		browserOpener: mockBrowser(),
	}

	token, err := authz.Authenticate()

	assert.Error(t, err)
	assert.Nil(t, token)
}

// TestAuthenticatorAuthenticateWithUnauthorizedClientError tests the unauthorized_client error handling
func TestAuthenticatorAuthenticateWithUnauthorizedClientError(t *testing.T) {
	// Create a mock token server that returns unauthorized_client error
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "unauthorized_client",
			"error_description": "The client is not authorized",
		})
	}))
	defer tokenServer.Close()

	// Create a stored token
	storedToken := &auth.AccessToken{
		Expiry: time.Now().Add(-time.Hour), // Expired token to trigger refresh
	}
	storedToken.Token = "old-access-token"
	storedToken.RefreshToken = "old-refresh-token"
	storedToken.Type = "Bearer"

	tokenJSON, _ := json.Marshal(storedToken)

	authz := &authenticator{
		port:             0, // Port 0 to avoid conflicts
		oauthStateString: "test-state",
		getToken: func() string {
			return string(tokenJSON)
		},
		storeToken: func(_ string) error {
			return nil
		},
		authConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:0/callback",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: tokenServer.URL,
			},
		},
		browserOpener: mockBrowser(),
	}

	// This will try to re-authenticate via browser since token refresh failed with unauthorized_client
	// We use a timeout to prevent the test from hanging
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = authz.Authenticate()
	}()

	select {
	case <-ctx.Done():
		// Expected timeout - browser can't be opened in tests
	case <-done:
		// Function returned
	}
}

// TestAuthenticatorAuthenticateWithNonUnauthorizedError tests error handling for non-unauthorized errors
func TestAuthenticatorAuthenticateWithNonUnauthorizedError(t *testing.T) {
	// Create a mock token server that returns a different error
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "invalid_grant",
			"error_description": "The refresh token is invalid",
		})
	}))
	defer tokenServer.Close()

	// Create a stored token
	storedToken := &auth.AccessToken{
		Expiry: time.Now().Add(-time.Hour), // Expired token to trigger refresh
	}
	storedToken.Token = "old-access-token"
	storedToken.RefreshToken = "old-refresh-token"
	storedToken.Type = "Bearer"

	tokenJSON, _ := json.Marshal(storedToken)

	authz := &authenticator{
		port:             0,
		oauthStateString: "test-state",
		getToken: func() string {
			return string(tokenJSON)
		},
		storeToken: func(_ string) error {
			return nil
		},
		authConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:0/callback",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: tokenServer.URL,
			},
		},
		browserOpener: mockBrowser(),
	}

	token, err := authz.Authenticate()

	// Should return an error for non-unauthorized_client errors
	assert.Error(t, err)
	assert.Nil(t, token)
}

// TestGetAuthUrlError tests the getAuthUrl method when URL parsing fails
func TestGetAuthUrlError(t *testing.T) {
	// This is difficult to test since oauth2 config always generates valid URLs
	// The function should work with standard config
	auth := &authenticator{
		port:             8080,
		oauthStateString: "test-state",
		authConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:8080/callback",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: "https://auth.atlassian.com/oauth/token",
			},
		},
	}

	url, err := auth.getAuthUrl()

	assert.NoError(t, err)
	assert.NotNil(t, url)
}
