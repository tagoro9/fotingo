package commands

import (
	"errors"

	"github.com/pkg/browser"
	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/ui"
)

// newJiraClient is a factory function for creating Jira clients.
// It can be overridden in tests to inject mock Jira servers.
var newJiraClient = func(cfg *viper.Viper) (jira.Jira, error) {
	client, err := jira.NewWithOptions(cfg, !Global.Yes && !Global.JSON)
	if err != nil {
		if Global.JSON && errors.Is(err, jira.ErrAuthRequired) {
			return nil, fterrors.AuthError("authentication required; run `fotingo login` interactively")
		}
		return nil, err
	}
	return client, nil
}

var newJiraAuthClient = func(cfg *viper.Viper) (jira.Jira, error) {
	client, err := jira.NewWithOptions(cfg, true)
	if err != nil {
		if Global.JSON && errors.Is(err, jira.ErrAuthRequired) {
			return nil, fterrors.AuthError("authentication required; run `fotingo login` interactively")
		}
		return nil, err
	}
	return client, nil
}

// newGitHubClient is a factory function for creating GitHub clients.
// It can be overridden in tests to inject mock GitHub servers.
var newGitHubClient = func(gitClient git.Git, cfg *viper.Viper) (github.Github, error) {
	client, err := github.NewWithOptions(gitClient, cfg, !Global.Yes && !Global.JSON)
	if err != nil {
		if Global.JSON && errors.Is(err, github.ErrAuthRequired) {
			return nil, fterrors.AuthError("authentication required; run `fotingo login` interactively")
		}
		return nil, err
	}
	return client, nil
}

var newGitHubAuthClient = func(cfg *viper.Viper) (github.Github, error) {
	client, err := github.NewAuthOnlyWithOptions(cfg, !Global.Yes && !Global.JSON)
	if err != nil {
		if Global.JSON && errors.Is(err, github.ErrAuthRequired) {
			return nil, fterrors.AuthError("authentication required; run `fotingo login` interactively")
		}
		return nil, err
	}
	return client, nil
}

// newGitClient is a factory function for creating Git clients.
// It can be overridden in tests to inject mock credential providers.
var newGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
	return git.New(cfg, messages)
}

// newOpenGitClient is a dedicated git factory for the open command.
// It defaults to direct git.New behavior so open can resolve repository URLs
// from the user's configured remote even when other tests override newGitClient.
var newOpenGitClient = func(cfg *viper.Viper, messages *chan string) (git.Git, error) {
	return git.New(cfg, messages)
}

// openBrowserFn is a function for opening URLs in the browser.
// It can be overridden in tests to capture URLs instead of opening a browser.
var openBrowserFn = func(url string) error {
	return browser.OpenURL(url)
}

// openEditorFn opens an external editor and returns edited content.
// It can be overridden in tests to avoid launching real editors.
var openEditorFn = func(initialContent string) (string, error) {
	return openEditorWithRuntime(initialContent)
}

var openEditorProcessFn = func(initialContent string) (string, error) {
	return ui.OpenEditor(initialContent)
}

func openEditorWithRuntime(initialContent string) (string, error) {
	return commandruntime.OpenEditorWithTerminalHandoff(
		initialContent,
		runInteractiveProcessWithTerminalHandoff,
		openEditorProcessFn,
	)
}
