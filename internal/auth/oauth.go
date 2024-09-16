package auth

import (
	"encoding/json"
	"sync"

	"github.com/cli/oauth"
	"github.com/cli/oauth/api"
	"time"
)

type OauthService interface {
	// Authenticate starts the oauth authentication flow and returns the access token
	Authenticate() (token *AccessToken, err error)
}

type Authenticator struct {
	getToken   func() string
	storeToken func(string) error
	flow       *oauth.Flow
}

type AccessToken struct {
	api.AccessToken
	Expiry time.Time
}

var (
	interactiveFlowRunnerMu sync.RWMutex
	interactiveFlowRunner   = func(run func() error) error { return run() }
	runOAuthFlow            = func(flow *oauth.Flow) (*api.AccessToken, error) {
		if flow.Host.DeviceCodeURL != "" {
			return flow.DetectFlow()
		}
		return flow.WebAppFlow()
	}
)

// SetInteractiveFlowRunner sets a wrapper used to execute interactive OAuth flows.
// Passing nil resets to direct execution.
func SetInteractiveFlowRunner(runner func(run func() error) error) {
	interactiveFlowRunnerMu.Lock()
	defer interactiveFlowRunnerMu.Unlock()

	if runner == nil {
		interactiveFlowRunner = func(run func() error) error { return run() }
		return
	}
	interactiveFlowRunner = runner
}

func executeInteractiveOAuthFlow(run func() error) error {
	interactiveFlowRunnerMu.RLock()
	runner := interactiveFlowRunner
	interactiveFlowRunnerMu.RUnlock()
	return runner(run)
}

func (a Authenticator) Authenticate() (*AccessToken, error) {
	storedToken := a.getToken()
	var unmarshalledToken AccessToken
	if err := json.Unmarshal([]byte(storedToken), &unmarshalledToken); err == nil {
		return &unmarshalledToken, nil
	}

	var (
		token *api.AccessToken
		err   error
	)
	err = executeInteractiveOAuthFlow(func() error {
		token, err = runOAuthFlow(a.flow)
		return err
	})
	if err != nil {
		return nil, err
	}
	serializedToken, err := json.Marshal(token)
	if err != nil {
		return nil, err
	}
	err = a.storeToken(string(serializedToken))
	if err != nil {
		return nil, err
	}
	return &AccessToken{AccessToken: *token}, nil
}

func NewAuthenticator(clientID string, clientSecret string, scopes []string, host *oauth.Host, getToken func() string, storeToken func(string) error) *Authenticator {
	return &Authenticator{
		getToken:   getToken,
		storeToken: storeToken,
		flow: &oauth.Flow{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       scopes,
			Host:         host,
		},
	}
}
