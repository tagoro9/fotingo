package commands

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	hub "github.com/google/go-github/v84/github"
	giturl "github.com/kubescape/go-git-url"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

func TestRunLoginWithStatus_Success(t *testing.T) {
	setDefaultOutputFlags(t)

	origNewGitHubAuthClient := newGitHubAuthClient
	origNewJiraAuthClient := newJiraAuthClient
	origWarnIgnoredOAuth := shouldWarnIgnoredJiraOAuthTokenFn
	origConfig := fotingoConfig
	defer func() {
		newGitHubAuthClient = origNewGitHubAuthClient
		newJiraAuthClient = origNewJiraAuthClient
		shouldWarnIgnoredJiraOAuthTokenFn = origWarnIgnoredOAuth
		fotingoConfig = origConfig
	}()
	fotingoConfig = viper.New()
	shouldWarnIgnoredJiraOAuthTokenFn = func(*viper.Viper) bool { return false }

	newGitHubAuthClient = func(cfg *viper.Viper) (github.Github, error) {
		return &mockGitHub{
			currentUser: &hub.User{Login: hub.Ptr("octocat")},
		}, nil
	}
	newJiraAuthClient = func(cfg *viper.Viper) (jira.Jira, error) {
		return &mockJira{
			currentUser: &tracker.User{Name: "jira-user"},
		}, nil
	}

	statusCh := make(chan string, 10)
	err := runLoginWithStatus(&statusCh)
	require.NoError(t, err)

	close(statusCh)
	var messages []string
	for msg := range statusCh {
		messages = append(messages, msg)
	}

	assert.Len(t, messages, 2)
	assert.Contains(t, strings.Join(messages, "\n"), "octocat")
	assert.Contains(t, strings.Join(messages, "\n"), "jira-user")
}

func TestRunLoginWithStatus_GitHubInitFailure(t *testing.T) {
	setDefaultOutputFlags(t)

	origNewGitHubAuthClient := newGitHubAuthClient
	defer func() { newGitHubAuthClient = origNewGitHubAuthClient }()

	newGitHubAuthClient = func(cfg *viper.Viper) (github.Github, error) {
		return nil, errors.New("github init failed")
	}

	statusCh := make(chan string, 10)
	err := runLoginWithStatus(&statusCh)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize GitHub client")
}

func TestRunLoginWithStatus_DoesNotRequireGitRepository(t *testing.T) {
	setDefaultOutputFlags(t)

	origNewGitClient := newGitClient
	origNewGitHubAuthClient := newGitHubAuthClient
	origNewJiraAuthClient := newJiraAuthClient
	defer func() {
		newGitClient = origNewGitClient
		newGitHubAuthClient = origNewGitHubAuthClient
		newJiraAuthClient = origNewJiraAuthClient
	}()

	newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return nil, errors.New("not a git repository")
	}
	newGitHubAuthClient = func(cfg *viper.Viper) (github.Github, error) {
		return &mockGitHub{currentUser: &hub.User{Login: hub.Ptr("octocat")}}, nil
	}
	newJiraAuthClient = func(cfg *viper.Viper) (jira.Jira, error) {
		return &mockJira{currentUser: &tracker.User{Name: "jira-user"}}, nil
	}

	statusCh := make(chan string, 10)
	err := runLoginWithStatus(&statusCh)
	require.NoError(t, err)
}

func TestRunLoginWithStatus_Jira401RetriesWithFreshCredentials(t *testing.T) {
	setDefaultOutputFlags(t)

	origNewGitHubAuthClient := newGitHubAuthClient
	origNewJiraAuthClient := newJiraAuthClient
	origWarnIgnoredOAuth := shouldWarnIgnoredJiraOAuthTokenFn
	origConfig := fotingoConfig
	defer func() {
		newGitHubAuthClient = origNewGitHubAuthClient
		newJiraAuthClient = origNewJiraAuthClient
		shouldWarnIgnoredJiraOAuthTokenFn = origWarnIgnoredOAuth
		fotingoConfig = origConfig
	}()

	cfg := viper.New()
	cfg.Set("jira.user.login", "bad@example.com")
	cfg.Set("jira.user.token", "bad-token")
	cfg.Set("jira.token", `{"access_token":"stale-oauth"}`)
	fotingoConfig = cfg

	newGitHubAuthClient = func(cfg *viper.Viper) (github.Github, error) {
		return &mockGitHub{currentUser: &hub.User{Login: hub.Ptr("octocat")}}, nil
	}

	attempt := 0
	newJiraAuthClient = func(cfg *viper.Viper) (jira.Jira, error) {
		attempt++
		if attempt == 1 {
			return &mockJira{currentUserErr: errors.New("request failed. Status code: 401")}, nil
		}
		return &mockJira{currentUser: &tracker.User{Name: "jira-user"}}, nil
	}

	statusCh := make(chan string, 20)
	err := runLoginWithStatus(&statusCh)
	require.NoError(t, err)
	assert.Equal(t, 2, attempt)
	assert.Equal(t, "", fotingoConfig.GetString("jira.user.login"))
	assert.Equal(t, "", fotingoConfig.GetString("jira.user.token"))
	assert.Equal(t, "", fotingoConfig.GetString("jira.token"))
}

func TestRunLoginWithStatus_WarnsWhenStoredOAuthTokenIgnored(t *testing.T) {
	setDefaultOutputFlags(t)

	origNewGitHubAuthClient := newGitHubAuthClient
	origNewJiraAuthClient := newJiraAuthClient
	origConfig := fotingoConfig
	defer func() {
		newGitHubAuthClient = origNewGitHubAuthClient
		newJiraAuthClient = origNewJiraAuthClient
		fotingoConfig = origConfig
	}()

	cfg := viper.New()
	cfg.Set("jira.token", `{"access_token":"stored-oauth-token"}`)
	fotingoConfig = cfg

	newGitHubAuthClient = func(cfg *viper.Viper) (github.Github, error) {
		return &mockGitHub{currentUser: &hub.User{Login: hub.Ptr("octocat")}}, nil
	}
	newJiraAuthClient = func(cfg *viper.Viper) (jira.Jira, error) {
		return &mockJira{currentUser: &tracker.User{Name: "jira-user"}}, nil
	}
	shouldWarnIgnoredJiraOAuthTokenFn = func(*viper.Viper) bool { return true }

	statusCh := make(chan string, 20)
	err := runLoginWithStatus(&statusCh)
	require.NoError(t, err)
	close(statusCh)

	var messages []string
	for msg := range statusCh {
		messages = append(messages, msg)
	}
	joined := strings.Join(messages, "\n")
	assert.Contains(t, joined, "Stored Jira OAuth token is ignored")
}

func TestRunOpenWithStatus_RepoOpensBrowserWithSharedStatus(t *testing.T) {
	setDefaultOutputFlags(t)

	origNewOpenGitClient := newOpenGitClient
	origOpenBrowserFn := openBrowserFn
	defer func() {
		newOpenGitClient = origNewOpenGitClient
		openBrowserFn = origOpenBrowserFn
	}()

	remoteURL, err := giturl.NewGitURL("https://github.com/testowner/testrepo.git")
	require.NoError(t, err)

	newOpenGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return &mockGit{remoteURL: remoteURL}, nil
	}

	var openedURL string
	openBrowserFn = func(url string) error {
		openedURL = url
		return nil
	}

	statusCh := make(chan string, 10)
	url, err := runOpenWithStatus(&statusCh, "repo", true)
	require.NoError(t, err)
	assert.Equal(t, openedURL, url)
	assert.Contains(t, openedURL, "github.com/testowner/testrepo")

	close(statusCh)
	var messages []string
	for msg := range statusCh {
		messages = append(messages, msg)
	}
	assert.NotEmpty(t, messages)
	assert.Contains(t, strings.Join(messages, "\n"), "Opening browser")
}

func TestRunOpenWithStatus_BrowserOpenFailure(t *testing.T) {
	setDefaultOutputFlags(t)

	origNewOpenGitClient := newOpenGitClient
	origOpenBrowserFn := openBrowserFn
	defer func() {
		newOpenGitClient = origNewOpenGitClient
		openBrowserFn = origOpenBrowserFn
	}()

	remoteURL, err := giturl.NewGitURL("https://github.com/testowner/testrepo.git")
	require.NoError(t, err)

	newOpenGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
		return &mockGit{remoteURL: remoteURL}, nil
	}
	openBrowserFn = func(url string) error { return errors.New("browser unavailable") }

	statusCh := make(chan string, 10)
	_, err = runOpenWithStatus(&statusCh, "repo", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open browser")
}

func TestSharedShellGuardrail_UserFacingCommands(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	commandsDir := filepath.Dir(testFile)
	requiredSharedShell := []string{
		"cache.go",
		"config.go",
		"login.go",
		"open.go",
		"release.go",
		"review.go",
	}

	for _, file := range requiredSharedShell {
		content, err := os.ReadFile(filepath.Join(commandsDir, file))
		require.NoError(t, err)
		assert.Containsf(
			t,
			string(content),
			"runWithSharedShell(",
			"%s must use runWithSharedShell to keep command runtime parity",
			file,
		)
	}
}
