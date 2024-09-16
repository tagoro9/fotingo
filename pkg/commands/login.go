package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
)

var shouldWarnIgnoredJiraOAuthTokenFn = func(cfg *viper.Viper) bool {
	return jira.ShouldWarnIgnoredStoredOAuthToken(cfg)
}

func init() {
	Fotingo.AddCommand(loginCmd)
}

var loginCmd = &cobra.Command{
	Use:   i18n.T(i18n.LoginUse),
	Short: i18n.T(i18n.LoginShort),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
			out.Success("login-context", commandruntime.LogEmojiAuth, i18n.LoginShort)
			statusCh, done := out.BridgeChannel()
			defer done()
			return runLoginWithStatus(statusCh)
		})
	},
}

func runLoginWithStatus(statusCh *chan string) error {
	out := commandruntime.NewLocalizedEmitter(*statusCh, shouldEmitCommandLevel, localizer.T)

	hub, err := newGitHubAuthClient(fotingoConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize GitHub client: %w", err)
	}
	if shouldWarnIgnoredJiraOAuthTokenFn(fotingoConfig) {
		out.InfoRaw(commandruntime.LogEmojiWarning, "Stored Jira OAuth token is ignored because this binary has no Jira OAuth client credentials. Falling back to API token auth.")
	}

	jiraClient, err := newJiraAuthClient(fotingoConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize Jira client: %w", err)
	}

	hubUser, err := hub.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current GitHub user: %w", err)
	}
	out.Info(commandruntime.LogEmojiGitHub, i18n.LoginGitHubLoggedIn, *hubUser.Login)

	user, err := jiraClient.GetCurrentUser()
	if err != nil {
		if jira.IsUnauthorizedError(err) {
			out.InfoRaw(commandruntime.LogEmojiWarning, "Jira credentials were rejected (401). Re-authenticating with API token.")
			clearJiraAPICredentialsInMemory(fotingoConfig)

			jiraClient, err = newJiraAuthClient(fotingoConfig)
			if err != nil {
				return fmt.Errorf("failed to re-authenticate Jira client after 401: %w", err)
			}

			user, err = jiraClient.GetCurrentUser()
			if err != nil {
				return fmt.Errorf("failed to get current Jira user after re-authentication: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get current Jira user: %w", err)
		}
	}
	out.Info(commandruntime.LogEmojiJira, i18n.LoginJiraLoggedIn, user.Name)

	return nil
}

func clearJiraAPICredentialsInMemory(cfg configSetter) {
	if cfg == nil {
		return
	}
	cfg.Set("jira.user.login", "")
	cfg.Set("jira.user.token", "")
	cfg.Set("jira.token", "")
}

type configSetter interface {
	Set(string, interface{})
}
