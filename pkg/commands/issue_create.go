package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/tracker"
)

type issueCreateResult struct {
	IssueKey string `json:"issueKey"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	URL      string `json:"url"`
}

func newIssueCreateCommand() *cobra.Command {
	options := issueCreateOptions{}
	command := &cobra.Command{
		Use: "create", Short: "Create a Jira issue without starting work or requiring Git", Args: cobra.NoArgs,
		Example: `  fotingo issue create -p PROJ -k Task -t "Plan follow-up" -y --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			options.title = strings.TrimSpace(options.title)
			options.description = strings.TrimSpace(options.description)
			options.project = strings.ToUpper(strings.TrimSpace(options.project))
			options.kind = strings.TrimSpace(options.kind)
			options.parent = strings.TrimSpace(options.parent)
			options.epic = strings.TrimSpace(options.epic)
			if options.title == "" || options.project == "" {
				return fmt.Errorf("--title and --project are required")
			}
			kind, err := parseIssueKind(options.kind)
			if err != nil {
				return err
			}
			if options.parent != "" && options.epic != "" {
				return fmt.Errorf("--parent and --epic cannot be used together")
			}
			if kind == tracker.IssueTypeSubTask && options.parent == "" {
				return fmt.Errorf("--parent is required for a sub-task")
			}
			for _, label := range options.labels {
				if strings.TrimSpace(label) == "" || strings.ContainsAny(label, " \t\r\n") {
					return fmt.Errorf("labels must be non-empty and contain no whitespace")
				}
			}
			if err := ensureJiraRootConfigured(); err != nil {
				return err
			}
			client, err := newJiraClient(fotingoConfig)
			if err != nil {
				return err
			}
			out := commandruntime.NewLocalizedEmitter(nil, shouldEmitCommandLevel, localizer.T)
			issue, err := createIssue(out, client, options)
			if err != nil {
				return err
			}
			url := client.GetIssueURL(issue.Key)
			result := issueCreateResult{IssueKey: issue.Key, Summary: issue.Summary, Status: issue.Status, Type: issue.Type, URL: url}
			if ShouldOutputJSON() {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			if !ShouldSuppressOutput() {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Created %s: %s\n%s\n", issue.Key, issue.Summary, url)
			}
			return err
		},
	}
	command.Flags().StringVarP(&options.title, "title", "t", "", localizer.T(i18n.StartFlagTitle))
	command.Flags().StringVarP(&options.description, "description", "d", "", localizer.T(i18n.StartFlagDescription))
	command.Flags().StringVarP(&options.project, "project", "p", "", localizer.T(i18n.StartFlagProject))
	command.Flags().StringVarP(&options.kind, "kind", "k", "Task", localizer.T(i18n.StartFlagKind))
	command.Flags().StringVarP(&options.parent, "parent", "a", "", localizer.T(i18n.StartFlagParent))
	command.Flags().StringVarP(&options.epic, "epic", "e", "", localizer.T(i18n.StartFlagEpic))
	command.Flags().StringSliceVarP(&options.labels, "labels", "l", nil, localizer.T(i18n.StartFlagLabels))
	_ = command.RegisterFlagCompletionFunc("project", completeStartProjectFlag)
	_ = command.RegisterFlagCompletionFunc("kind", completeStartIssueTypeFlag)
	return command
}
