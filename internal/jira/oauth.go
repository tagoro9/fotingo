package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	url2 "net/url"
	"strings"

	"github.com/cli/oauth/api"
	"github.com/tagoro9/fotingo/internal/auth"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/io"
	"golang.org/x/oauth2"
)

var jiraOAuthClientID = ""

var jiraOAuthClientSecret = ""

type authenticator struct {
	port             int
	authConfig       *oauth2.Config
	oauthStateString string
	allowInteractive bool
	getToken         func() string
	storeToken       func(string) error
	browserOpener    io.BrowserOpener
}

func (auth *authenticator) validateOAuthConfig() error {
	if strings.TrimSpace(auth.authConfig.ClientID) == "" {
		return ErrOAuthCredentialsMissing
	}
	if strings.TrimSpace(auth.authConfig.ClientSecret) == "" {
		return ErrOAuthCredentialsMissing
	}
	return nil
}

// getAuthUrl builds the URL users need to visit
// to authenticate with Jira
func (auth *authenticator) getAuthUrl() (*url2.URL, error) {
	url, err := url2.Parse(auth.authConfig.AuthCodeURL(auth.oauthStateString))
	if err != nil {
		return nil, err
	}
	query := url.Query()
	query.Set("audience", "api.atlassian.com")
	query.Set("prompt", "consent")
	url.RawQuery = query.Encode()
	return url, nil
}

// getOauthUserInfo uses the oauth data from the callback to get
// an access token from the authorization server
func (auth *authenticator) getOauthUserInfo(state string, code string) (*oauth2.Token, error) {
	if state != auth.oauthStateString {
		return nil, fmt.Errorf("invalid oauth state")
	}
	token, err := auth.authConfig.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	return token, nil
}

// createHttpServer returns an HTTP server that can listen on the oauth callbacks
// and send back the token via the passed channel
func (auth *authenticator) createHttpServer(tokenChannel chan *oauth2.Token) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/assets/oauth.css", serveOAuthCSS)
	mux.HandleFunc("/assets/favicon.svg", serveOAuthFavicon)
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		content, err := auth.getOauthUserInfo(r.FormValue("state"), r.FormValue("code"))
		if err != nil {
			fmt.Println(err.Error())
			// TODO Handle errors better
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		pageContent, pageErr := buildOAuthSuccessPageFn()
		if pageErr != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = fmt.Fprint(w, i18n.T(i18n.JiraOAuthPageFallback))
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(pageContent))
		}

		tokenChannel <- content
	})
	return &http.Server{Addr: fmt.Sprintf(":%d", auth.port), Handler: mux}
}

func (authz *authenticator) Authenticate() (token *auth.AccessToken, err error) {
	storedToken := authz.getToken()
	var unmarshalledToken auth.AccessToken
	if err := json.Unmarshal([]byte(storedToken), &unmarshalledToken); err == nil {
		oauth2Token := &oauth2.Token{AccessToken: unmarshalledToken.Token, RefreshToken: unmarshalledToken.RefreshToken, TokenType: unmarshalledToken.Type, Expiry: unmarshalledToken.Expiry}
		// Refresh the token here in case the stored one is expired
		tokenSource := authz.authConfig.TokenSource(context.TODO(), oauth2Token)
		refreshedToken, err := tokenSource.Token()
		if err != nil {
			var oauthError *oauth2.RetrieveError
			// If the token is no longer valid reauthenticate
			if !errors.As(err, &oauthError) || oauthError.ErrorCode != "unauthorized_client" {
				return nil, err
			}
			if !authz.allowInteractive {
				return nil, ErrAuthRequired
			}
		} else {
			refreshedAccessToken := &auth.AccessToken{
				AccessToken: api.AccessToken{
					Token:        refreshedToken.AccessToken,
					Type:         refreshedToken.TokenType,
					RefreshToken: refreshedToken.RefreshToken,
				},
				Expiry: refreshedToken.Expiry,
			}
			serializedToken, err := json.Marshal(refreshedAccessToken)
			if err != nil {
				return nil, err
			}
			err = authz.storeToken(string(serializedToken))
			if err != nil {
				return nil, err
			}
			return refreshedAccessToken, nil
		}
	}

	if !authz.allowInteractive {
		return nil, ErrAuthRequired
	}

	url, err := authz.getAuthUrl()
	if err != nil {
		return nil, err
	}
	err = authz.browserOpener.Open(url.String())
	if err != nil {
		fmt.Printf("Visit this URL to authenticate with Jira %s\n", url)
	}

	tokenChannel := make(chan *oauth2.Token)
	httpServer := authz.createHttpServer(tokenChannel)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// TODO These errors should be handled gracefully
			log.Fatal(err)
		}
	}()
	oauthToken := <-tokenChannel
	token = &auth.AccessToken{
		AccessToken: api.AccessToken{
			Token:        oauthToken.AccessToken,
			Type:         oauthToken.TokenType,
			RefreshToken: oauthToken.RefreshToken,
		},
		Expiry: oauthToken.Expiry,
	}
	err = httpServer.Shutdown(context.Background())
	if err != nil {
		return nil, err
	}
	serializedToken, err := json.Marshal(token)
	if err != nil {
		return nil, err
	}
	err = authz.storeToken(string(serializedToken))
	if err != nil {
		return nil, err
	}
	return token, nil
}

func createAuthenticator(getToken func() string, storeToken func(string) error, allowInteractive bool) *authenticator {
	// TODO Don't use a port like this, but copy from flow
	port := 8080
	config := oauth2.Config{
		// TODO This port needs to be in sync
		RedirectURL:  fmt.Sprintf("http://localhost:%d/callback", port),
		ClientID:     jiraOAuthClientID,
		ClientSecret: jiraOAuthClientSecret,
		Scopes: []string{
			"read:jira-user",
			"read:jira-work",
			"write:jira-work",
			"manage:jira-project",
			// This scope is needed to get a refresh token
			"offline_access",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://auth.atlassian.com/authorize",
			TokenURL: "https://auth.atlassian.com/oauth/token"},
	}
	return &authenticator{
		getToken:         getToken,
		storeToken:       storeToken,
		allowInteractive: allowInteractive,
		authConfig:       &config,
		oauthStateString: "pseudo-random",
		port:             port,
		browserOpener:    io.NewDefaultBrowser(),
	}
}
