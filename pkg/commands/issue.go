package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/jira"
)

var newIssueActions = func() (jira.IssueActions, error) {
	client, err := jira.NewWithOptions(fotingoConfig, !Global.Yes && !ShouldOutputJSON())
	if err != nil {
		return nil, err
	}
	actions, ok := client.(jira.IssueActions)
	if !ok {
		return nil, fmt.Errorf("jira client does not support issue actions")
	}
	return actions, nil
}

func init() { Fotingo.AddCommand(newIssueCommand()) }

func newIssueCommand() *cobra.Command {
	issue := &cobra.Command{Use: "issue", Short: "Create, comment on, or label Jira issues without Git context"}
	var key, bodyFile string
	comment := &cobra.Command{
		Use: "comment [text]", Short: "Add a literal plain-text comment to a Jira issue",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := jira.ValidateIssueKey(key); err != nil {
				return err
			}
			hasFile := cmd.Flags().Changed("body-file")
			if hasFile == (len(args) == 1) {
				return fmt.Errorf("provide either comment text or --body-file")
			}
			var body string
			if hasFile {
				var data []byte
				var err error
				if bodyFile == "-" {
					data, err = io.ReadAll(cmd.InOrStdin())
				} else {
					data, err = os.ReadFile(bodyFile)
				}
				if err != nil {
					return fmt.Errorf("read comment body: %w", err)
				}
				body = string(data)
			} else {
				body = args[0]
			}
			if strings.TrimSpace(body) == "" {
				return fmt.Errorf("comment body must not be empty")
			}
			client, err := newIssueActions()
			if err != nil {
				return err
			}
			result, err := client.CommentOnIssue(key, body)
			if err != nil {
				return err
			}
			if ShouldOutputJSON() {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			if !ShouldSuppressOutput() {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "Comment added:", result.URL)
			}
			return err
		},
	}
	comment.Flags().StringVarP(&key, "issue", "i", "", "Jira issue key (required)")
	comment.Flags().StringVar(&bodyFile, "body-file", "", "Read literal text from a file, or - for stdin")
	issue.AddCommand(newIssueCreateCommand(), comment, newIssueLabelCommand())
	return issue
}

func newIssueLabelCommand() *cobra.Command {
	var key string
	var labels []string
	command := &cobra.Command{
		Use: "label", Short: "Add labels to a Jira issue without changing its status", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := jira.ValidateIssueKey(key); err != nil {
				return err
			}
			if len(labels) == 0 {
				return fmt.Errorf("at least one --add label is required")
			}
			for _, label := range labels {
				if strings.TrimSpace(label) == "" || strings.ContainsAny(label, " \t\r\n") {
					return fmt.Errorf("labels must be non-empty and contain no whitespace")
				}
			}
			client, err := newIssueActions()
			if err != nil {
				return err
			}
			result, err := client.LabelIssue(key, labels)
			if err != nil {
				return err
			}
			if ShouldOutputJSON() {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			if !ShouldSuppressOutput() {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s labels: %s\n", key, strings.Join(result.Labels, ", "))
			}
			return err
		},
	}
	command.Flags().StringVarP(&key, "issue", "i", "", "Jira issue key (required)")
	command.Flags().StringArrayVar(&labels, "add", nil, "Label to add (repeat for multiple labels)")
	return command
}
