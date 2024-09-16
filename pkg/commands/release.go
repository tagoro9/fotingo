package commands

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalrelease "github.com/tagoro9/fotingo/internal/commands/release"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

// releaseFlags holds the flags for the release command
type releaseFlags struct {
	issues       []string
	simple       bool
	noVCSRelease bool
}

var releaseCmdFlags = releaseFlags{}

// defaultReleaseTemplate is the default template used for creating release notes
var defaultReleaseTemplate = i18n.T(i18n.ReleaseDefaultTemplate)

func init() {
	Fotingo.AddCommand(releaseCmd)

	releaseCmd.Flags().StringSliceVarP(&releaseCmdFlags.issues, "issues", "i", []string{}, localizer.T(i18n.ReleaseFlagIssues))
	releaseCmd.Flags().BoolVarP(&releaseCmdFlags.simple, "simple", "s", false, localizer.T(i18n.ReleaseFlagSimple))
	releaseCmd.Flags().BoolVarP(&releaseCmdFlags.noVCSRelease, "no-vcs-release", "n", false, localizer.T(i18n.ReleaseFlagNoVCS))
}

var releaseCmd = &cobra.Command{
	Use:   i18n.T(i18n.ReleaseUse),
	Short: i18n.T(i18n.ReleaseShort),
	Long:  i18n.T(i18n.ReleaseLong),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		releaseName := args[0]
		if !releaseCmdFlags.simple {
			if err := ensureJiraRootConfigured(); err != nil {
				return err
			}
		}

		return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
			out.Info(commandruntime.LogEmojiRelease, i18n.ReleaseInitialCreating, releaseName)
			statusCh, done := out.BridgeChannel()
			defer done()
			return runReleaseWithStatus(statusCh, releaseName)
		})
	},
}

// runRelease executes the main release logic
func runRelease(statusCh *chan string, processDone chan<- bool, releaseName string) error {
	defer func() { processDone <- true }()
	return runReleaseWithStatus(statusCh, releaseName)
}

func runReleaseWithStatus(statusCh *chan string, releaseName string) error {
	out := commandruntime.NewLocalizedEmitter(*statusCh, shouldEmitCommandLevel, localizer.T)
	runner := internalrelease.WorkflowRunner{
		Config:   fotingoConfig,
		Localize: localizer.T,
		Flags: internalrelease.WorkflowFlags{
			Issues:       append([]string(nil), releaseCmdFlags.issues...),
			Simple:       releaseCmdFlags.simple,
			NoVCSRelease: releaseCmdFlags.noVCSRelease,
		},
		Deps: internalrelease.WorkflowDeps{
			NewGitClient:      newGitClient,
			NewJiraClient:     newJiraClient,
			NewGitHubClient:   newGitHubClient,
			FetchIssueDetails: fetchIssueDetails,
			BuildReleaseNotes: buildReleaseNotes,
			DefaultTargetCommitish: func() string {
				return localizer.T(i18n.ReviewPlaceholderEmpty)
			},
		},
	}
	return runner.Run(statusCh, releaseWorkflowEmitter{out: out}, releaseName)
}

type releaseWorkflowEmitter struct {
	out commandruntime.LocalizedEmitter
}

func (e releaseWorkflowEmitter) Info(emoji string, key i18n.Key, args ...any) {
	e.out.Info(releaseWorkflowEmoji(emoji), key, args...)
}

func (e releaseWorkflowEmitter) Verbose(key i18n.Key, args ...any) {
	e.out.Verbose(key, args...)
}

func releaseWorkflowEmoji(emoji string) commandruntime.LogEmoji {
	switch strings.TrimSpace(strings.ToLower(emoji)) {
	case "warning":
		return commandruntime.LogEmojiWarning
	case "issue":
		return commandruntime.LogEmojiIssue
	case "release":
		return commandruntime.LogEmojiRelease
	case "bookmark":
		return commandruntime.LogEmojiBookmark
	case "rocket":
		return commandruntime.LogEmojiRocket
	case "success":
		return commandruntime.LogEmojiSuccess
	default:
		return commandruntime.LogEmojiInfo
	}
}

// fetchIssueDetails fetches full issue details from Jira for all issue IDs
func fetchIssueDetails(jiraClient jira.Jira, issueIDs []string) ([]*tracker.Issue, error) {
	return internalrelease.FetchIssueDetails(jiraClient, issueIDs)
}

// buildReleaseNotes constructs the release notes using the template
func buildReleaseNotes(releaseName string, issues []*tracker.Issue, jiraRelease *tracker.Release, jiraClient jira.Jira) string {
	// Get template from config or use default
	releaseTemplate := fotingoConfig.GetString("github.releaseTemplate")
	if releaseTemplate == "" {
		releaseTemplate = defaultReleaseTemplate
	}

	return internalrelease.BuildReleaseNotes(
		releaseName,
		issues,
		jiraRelease,
		jiraClient,
		releaseTemplate,
		releaseNotesText(),
	)
}

// formatIssuesByCategory groups issues by type and formats them for release notes
func formatIssuesByCategory(issues []*tracker.Issue, jiraClient jira.Jira) string {
	return internalrelease.FormatIssuesByCategory(issues, jiraClient, releaseNotesText())
}

func releaseNotesText() internalrelease.NotesText {
	return internalrelease.NotesText{
		NoIssues:              localizer.T(i18n.ReleaseNoIssues),
		HeadingCategoryFormat: localizer.T(i18n.ReleaseHeadingCategory),
		IssueBulletFormat:     localizer.T(i18n.ReleaseIssueBullet),
		CategoryBugFixes:      localizer.T(i18n.ReleaseCategoryBugFixes),
		CategoryFeatures:      localizer.T(i18n.ReleaseCategoryFeatures),
		CategoryTasks:         localizer.T(i18n.ReleaseCategoryTasks),
		CategorySubtasks:      localizer.T(i18n.ReleaseCategorySubtasks),
		CategoryEpics:         localizer.T(i18n.ReleaseCategoryEpics),
	}
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
