package auth

import (
	"errors"
	"testing"

	"github.com/cli/oauth"
	"github.com/cli/oauth/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticate_UsesInteractiveFlowRunner(t *testing.T) {
	t.Cleanup(func() {
		SetInteractiveFlowRunner(nil)
	})

	originalRun := runOAuthFlow
	t.Cleanup(func() {
		runOAuthFlow = originalRun
	})

	calledRunner := false
	SetInteractiveFlowRunner(func(run func() error) error {
		calledRunner = true
		return run()
	})

	runOAuthFlow = func(_ *oauth.Flow) (*api.AccessToken, error) {
		return &api.AccessToken{Token: "token-value"}, nil
	}
	host, err := oauth.NewGitHubHost("https://github.com")
	require.NoError(t, err)

	var stored string
	authenticator := Authenticator{
		getToken: func() string {
			return ""
		},
		storeToken: func(token string) error {
			stored = token
			return nil
		},
		flow: &oauth.Flow{
			Host: host,
		},
	}

	token, err := authenticator.Authenticate()
	require.NoError(t, err)
	require.NotNil(t, token)
	assert.Equal(t, "token-value", token.Token)
	assert.True(t, calledRunner)
	assert.NotEmpty(t, stored)
}

func TestAuthenticate_PropagatesRunnerError(t *testing.T) {
	t.Cleanup(func() {
		SetInteractiveFlowRunner(nil)
	})

	originalRun := runOAuthFlow
	t.Cleanup(func() {
		runOAuthFlow = originalRun
	})

	expectedErr := errors.New("handoff failed")
	SetInteractiveFlowRunner(func(_ func() error) error {
		return expectedErr
	})
	runOAuthFlow = func(_ *oauth.Flow) (*api.AccessToken, error) {
		return &api.AccessToken{Token: "ignored"}, nil
	}
	host, err := oauth.NewGitHubHost("https://github.com")
	require.NoError(t, err)

	authenticator := Authenticator{
		getToken: func() string {
			return ""
		},
		storeToken: func(_ string) error {
			return nil
		},
		flow: &oauth.Flow{
			Host: host,
		},
	}

	_, err = authenticator.Authenticate()
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}
