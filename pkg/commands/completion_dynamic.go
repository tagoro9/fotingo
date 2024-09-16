package commands

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
	internalcompletion "github.com/tagoro9/fotingo/internal/commands/completion"
	internalreview "github.com/tagoro9/fotingo/internal/commands/review"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/jira"
	"github.com/tagoro9/fotingo/internal/tracker"
)

var newCompletionGitHubClient = func() (github.Github, error) {
	statusCh := make(chan string, 1)
	gitClient, err := newGitClient(fotingoConfig, &statusCh)
	if err != nil {
		return nil, err
	}
	return github.NewWithOptions(gitClient, fotingoConfig, false)
}

var newCompletionJiraClient = func() (jira.Jira, error) {
	return jira.NewWithOptions(fotingoConfig, false)
}

func completeReviewLabelsFlag(
	_ *cobra.Command,
	_ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	ghClient, err := newCompletionGitHubClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	labels, err := ghClient.GetLabels()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	options := make([]reviewMatchOption, 0, len(labels))
	for _, label := range labels {
		options = append(options, reviewMatchOption{
			Resolved: label.Name,
			Label:    label.Name,
			Detail:   label.Description,
			Fields:   []string{label.Name, label.Description, label.Color},
			Kind:     reviewMatchKindLabel,
		})
	}

	return completeReviewMatchOptions(toComplete, options), cobra.ShellCompDirectiveNoFileComp
}

func completeReviewReviewersFlag(
	_ *cobra.Command,
	_ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	ghClient, err := newCompletionGitHubClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	options, _, err := internalreview.BuildParticipantOptions(ghClient)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return completeReviewMatchOptions(toComplete, options), cobra.ShellCompDirectiveNoFileComp
}

func completeReviewAssigneesFlag(
	_ *cobra.Command,
	_ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	ghClient, err := newCompletionGitHubClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	options, _, err := internalreview.BuildParticipantOptions(ghClient)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	userOptions := make([]reviewMatchOption, 0, len(options))
	for _, option := range options {
		if option.Kind == reviewMatchKindTeam {
			continue
		}
		userOptions = append(userOptions, option)
	}

	return completeReviewMatchOptions(toComplete, userOptions), cobra.ShellCompDirectiveNoFileComp
}

func completeReviewMatchOptions(toComplete string, options []reviewMatchOption) []string {
	completionOptions := make([]internalcompletion.ReviewMatchOption, 0, len(options))
	for _, option := range options {
		completionOptions = append(completionOptions, internalcompletion.ReviewMatchOption{
			Resolved: option.Resolved,
			Label:    option.Label,
			Detail:   option.Detail,
			Fields:   option.Fields,
			Kind:     string(option.Kind),
		})
	}

	return internalcompletion.CompleteReviewMatchOptions(
		toComplete,
		completionOptions,
		func(token string, completionOptions []internalcompletion.ReviewMatchOption) []internalcompletion.ReviewMatchOption {
			reviewOptions := make([]reviewMatchOption, 0, len(completionOptions))
			for _, option := range completionOptions {
				reviewOptions = append(reviewOptions, reviewMatchOption{
					Resolved: option.Resolved,
					Label:    option.Label,
					Fields:   option.Fields,
					Detail:   option.Detail,
					Kind:     reviewMatchKind(option.Kind),
				})
			}

			matches := internalreview.FindTokenMatchesForCompletion(token, reviewOptions)
			result := make([]internalcompletion.ReviewMatchOption, 0, len(matches))
			for _, match := range matches {
				result = append(result, internalcompletion.ReviewMatchOption{
					Resolved: match.Resolved,
					Label:    match.Label,
					Detail:   match.Detail,
					Fields:   match.Fields,
					Kind:     string(match.Kind),
				})
			}
			return result
		},
	)
}

func completeStartProjectFlag(
	_ *cobra.Command,
	_ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	jiraClient, err := newCompletionJiraClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	issues, err := jiraClient.GetUserOpenIssues()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	projects := internalcompletion.CollectProjectsFromIssues(issues)
	return internalcompletion.FilterByContainsFold(projects, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeStartIssueTypeFlag(
	cmd *cobra.Command,
	_ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	project := strings.ToUpper(strings.TrimSpace(startCmdFlags.project))
	if project == "" && cmd != nil && cmd.Flags() != nil {
		if value, err := cmd.Flags().GetString("project"); err == nil {
			project = strings.ToUpper(strings.TrimSpace(value))
		}
	}

	if project == "" {
		defaultKinds := []string{
			string(tracker.IssueTypeTask),
			string(tracker.IssueTypeStory),
			string(tracker.IssueTypeBug),
			string(tracker.IssueTypeSubTask),
			string(tracker.IssueTypeEpic),
		}
		return internalcompletion.FilterByContainsFold(defaultKinds, toComplete), cobra.ShellCompDirectiveNoFileComp
	}

	jiraClient, err := newCompletionJiraClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	types, err := jiraClient.GetProjectIssueTypes(project)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	issueTypeNames := make([]string, 0, len(types))
	for _, issueType := range types {
		name := strings.TrimSpace(issueType.Name)
		if name == "" {
			continue
		}
		issueTypeNames = append(issueTypeNames, name)
	}

	issueTypeNames = internalreview.DedupeStringsPreserveOrder(issueTypeNames)
	sort.Strings(issueTypeNames)
	return internalcompletion.FilterByContainsFold(issueTypeNames, toComplete), cobra.ShellCompDirectiveNoFileComp
}
