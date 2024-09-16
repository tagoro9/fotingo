package release

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

// WorkflowFlags carries release command options required by the workflow.
type WorkflowFlags struct {
	Issues       []string
	Simple       bool
	NoVCSRelease bool
}

// WorkflowEmitter defines logging operations used by the release workflow.
type WorkflowEmitter interface {
	Info(emoji string, key i18n.Key, args ...any)
	Verbose(key i18n.Key, args ...any)
}

// WorkflowDeps defines external dependencies required to execute release workflow.
type WorkflowDeps struct {
	NewGitClient           func(*viper.Viper, *chan string) (git.Git, error)
	NewJiraClient          func(*viper.Viper) (jira.Jira, error)
	NewGitHubClient        func(git.Git, *viper.Viper) (github.Github, error)
	FetchIssueDetails      func(jira.Jira, []string) ([]*tracker.Issue, error)
	BuildReleaseNotes      func(string, []*tracker.Issue, *tracker.Release, jira.Jira) string
	DefaultTargetCommitish func() string
}

// WorkflowRunner executes release creation using injected configuration and dependencies.
type WorkflowRunner struct {
	Config   *viper.Viper
	Localize func(i18n.Key, ...any) string
	Flags    WorkflowFlags
	Deps     WorkflowDeps
}

// Run executes the release workflow end-to-end.
func (r WorkflowRunner) Run(statusCh *chan string, out WorkflowEmitter, releaseName string) error {
	if err := r.validateDeps(); err != nil {
		return err
	}

	t := r.localize
	if out == nil {
		out = noopEmitter{}
	}

	out.Verbose(i18n.ReleaseStatusInitGit)
	gitClient, err := r.Deps.NewGitClient(r.Config, statusCh)
	if err != nil {
		return fmt.Errorf(t(i18n.ReleaseErrInitGit), err)
	}
	out.Verbose(i18n.ReleaseStatusGitInit)

	out.Verbose(i18n.ReleaseStatusFetchCommits)
	commits, err := gitClient.GetCommitsSinceDefaultBranch()
	if err != nil {
		return fmt.Errorf(t(i18n.ReleaseErrGetCommits), err)
	}
	out.Verbose(i18n.ReleaseStatusFoundCommits, len(commits))

	out.Verbose(i18n.ReleaseStatusExtractIssues)
	issueIDs := gitClient.GetIssuesFromCommits(commits)
	for _, issue := range r.Flags.Issues {
		normalizedID := strings.ToUpper(strings.TrimSpace(issue))
		if normalizedID != "" && !contains(issueIDs, normalizedID) {
			issueIDs = append(issueIDs, normalizedID)
		}
	}

	if len(issueIDs) == 0 {
		out.Info("warning", i18n.ReleaseStatusNoIssues)
	} else {
		out.Info("issue", i18n.ReleaseStatusFoundIssues, len(issueIDs), strings.Join(issueIDs, ", "))
	}

	var jiraRelease *tracker.Release
	var jiraClient jira.Jira
	var issues []*tracker.Issue

	if !r.Flags.Simple && len(issueIDs) > 0 {
		out.Verbose(i18n.ReleaseStatusInitJira)
		jiraClient, err = r.Deps.NewJiraClient(r.Config)
		if err != nil {
			return fmt.Errorf(t(i18n.ReleaseErrInitJira), err)
		}
		out.Verbose(i18n.ReleaseStatusJiraInit)

		out.Verbose(i18n.ReleaseStatusFetchDetails)
		issues, err = r.Deps.FetchIssueDetails(jiraClient, issueIDs)
		if err != nil {
			out.Info("warning", i18n.ReleaseStatusFetchWarn, err)
		}

		out.Info("release", i18n.ReleaseStatusCreateJiraRel, releaseName)
		jiraRelease, err = jiraClient.CreateRelease(tracker.CreateReleaseInput{
			Name:        releaseName,
			Description: t(i18n.ReleaseIssueDesc, releaseName),
			IssueIDs:    issueIDs,
		})
		if err != nil {
			return fmt.Errorf(t(i18n.ReleaseErrCreateJiraRel), err)
		}
		out.Info("bookmark", i18n.ReleaseStatusJiraRelDone, jiraRelease.URL)

		out.Verbose(i18n.ReleaseStatusSetFix, len(issueIDs))
		if err := jiraClient.SetFixVersion(issueIDs, jiraRelease); err != nil {
			out.Info("warning", i18n.ReleaseStatusSetFixWarn, err)
		} else {
			out.Verbose(i18n.ReleaseStatusSetFixDone)
		}
	}

	if !r.Flags.NoVCSRelease {
		out.Verbose(i18n.ReleaseStatusInitGitHub)
		ghClient, err := r.Deps.NewGitHubClient(gitClient, r.Config)
		if err != nil {
			return fmt.Errorf(t(i18n.ReleaseErrInitGitHub), err)
		}
		out.Verbose(i18n.ReleaseStatusGitHubInit)

		releaseNotes := r.Deps.BuildReleaseNotes(releaseName, issues, jiraRelease, jiraClient)

		out.Info("release", i18n.ReleaseStatusCreateGHRel, releaseName)
		release, err := ghClient.CreateRelease(github.CreateReleaseOptions{
			TagName:         releaseName,
			Name:            releaseName,
			Body:            releaseNotes,
			TargetCommitish: r.defaultTargetCommitish(),
			Draft:           false,
			Prerelease:      false,
		})
		if err != nil {
			return fmt.Errorf(t(i18n.ReleaseErrCreateGHRel), err)
		}
		out.Info("rocket", i18n.ReleaseStatusGHRelDone, release.HTMLURL)
	}

	out.Info("success", i18n.ReleaseStatusSuccess, releaseName)
	return nil
}

func (r WorkflowRunner) localize(key i18n.Key, args ...any) string {
	if r.Localize != nil {
		return r.Localize(key, args...)
	}
	return i18n.T(key, args...)
}

func (r WorkflowRunner) defaultTargetCommitish() string {
	if r.Deps.DefaultTargetCommitish == nil {
		return ""
	}
	return r.Deps.DefaultTargetCommitish()
}

func (r WorkflowRunner) validateDeps() error {
	if r.Deps.NewGitClient == nil {
		return fmt.Errorf("release workflow dependency NewGitClient is required")
	}
	if r.Deps.NewJiraClient == nil {
		return fmt.Errorf("release workflow dependency NewJiraClient is required")
	}
	if r.Deps.NewGitHubClient == nil {
		return fmt.Errorf("release workflow dependency NewGitHubClient is required")
	}
	if r.Deps.FetchIssueDetails == nil {
		return fmt.Errorf("release workflow dependency FetchIssueDetails is required")
	}
	if r.Deps.BuildReleaseNotes == nil {
		return fmt.Errorf("release workflow dependency BuildReleaseNotes is required")
	}
	return nil
}

type noopEmitter struct{}

func (noopEmitter) Info(string, i18n.Key, ...any) {}
func (noopEmitter) Verbose(i18n.Key, ...any)      {}

func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
