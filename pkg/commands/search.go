package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	internalreview "github.com/tagoro9/fotingo/internal/commands/review"
	"github.com/tagoro9/fotingo/internal/github"
)

type searchDomain string

const (
	searchDomainReviewers searchDomain = "reviewers"
	searchDomainAssignees searchDomain = "assignees"
	searchDomainLabels    searchDomain = "labels"
	searchResultsLimit                 = 5
)

var newSearchGitHubClient = func() (github.Github, error) {
	return newCompletionGitHubClient()
}

func init() {
	searchCmd.AddCommand(
		newSearchMetadataCommand(
			searchDomainReviewers,
			"Search review reviewers non-interactively",
			"Search review reviewers using the same fuzzy matching logic as `fotingo review`.",
			`fotingo search reviewers ali`,
		),
		newSearchMetadataCommand(
			searchDomainAssignees,
			"Search review assignees non-interactively",
			"Search review assignees using the same fuzzy matching logic as `fotingo review`.",
			`fotingo search assignees alice`,
		),
		newSearchMetadataCommand(
			searchDomainLabels,
			"Search review labels non-interactively",
			"Search review labels using the same fuzzy matching logic as `fotingo review`.",
			`fotingo search labels bug`,
		),
	)
	Fotingo.AddCommand(searchCmd)
}

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search review metadata",
	Long: "Search review reviewers, assignees, and labels using the same metadata " +
		"and fuzzy ranking used by `fotingo review`.",
}

type metadataFetchInfoLoggerSetter interface {
	SetMetadataFetchInfoLogger(func(string))
}

func newSearchMetadataCommand(
	domain searchDomain,
	short string,
	long string,
	example string,
) *cobra.Command {
	return &cobra.Command{
		Use:                   fmt.Sprintf("%s <query>", domain),
		Short:                 short,
		Long:                  long,
		Example:               example,
		DisableFlagsInUseLine: true,
		Args:                  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if ShouldOutputJSON() {
				return runSearchMetadataCommand(cmd.OutOrStdout(), domain, args, nil)
			}

			if shouldUseInteractiveSearchUIFn() {
				return runInteractiveSearchMetadataCommandFn(cmd.OutOrStdout(), domain, args)
			}

			return runSearchMetadataCommand(cmd.OutOrStdout(), domain, args, nil)
		},
	}
}

func runSearchMetadataCommand(
	writer io.Writer,
	domain searchDomain,
	args []string,
	progress func(string),
) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return errors.New("search query is required")
	}

	results, err := searchReviewMetadata(domain, query, progress)
	if err != nil {
		return err
	}

	if ShouldOutputJSON() {
		OutputJSON(buildSearchOutput(domain, query, results))
		return nil
	}

	printSearchResults(writer, domain, query, results)
	return nil
}

func searchReviewMetadata(
	domain searchDomain,
	query string,
	progress func(string),
) ([]reviewMatchOption, error) {
	ghClient, err := newSearchGitHubClient()
	if err != nil {
		return nil, err
	}
	if progress != nil {
		if setter, ok := ghClient.(metadataFetchInfoLoggerSetter); ok {
			setter.SetMetadataFetchInfoLogger(progress)
		}
	}

	options, err := loadSearchOptions(ghClient, domain, query)
	if err != nil {
		return nil, err
	}

	matches := internalreview.FindTokenMatchesForCompletion(query, options)
	if len(matches) > searchResultsLimit {
		matches = matches[:searchResultsLimit]
	}

	return matches, nil
}

func loadSearchOptions(
	ghClient github.Github,
	domain searchDomain,
	query string,
) ([]reviewMatchOption, error) {
	switch domain {
	case searchDomainReviewers:
		options, _, err := internalreview.BuildParticipantOptionsForQuery(ghClient, query, true)
		return options, err
	case searchDomainAssignees:
		options, _, err := internalreview.BuildParticipantOptionsForQuery(ghClient, query, false)
		if err != nil {
			return nil, err
		}

		userOptions := make([]reviewMatchOption, 0, len(options))
		for _, option := range options {
			if option.Kind == reviewMatchKindTeam {
				continue
			}
			userOptions = append(userOptions, option)
		}
		return userOptions, nil
	case searchDomainLabels:
		return internalreview.BuildLabelOptions(ghClient)
	default:
		return nil, fmt.Errorf("unsupported search domain %q", domain)
	}
}

func buildSearchOutput(domain searchDomain, query string, results []reviewMatchOption) SearchOutput {
	output := SearchOutput{
		Success: true,
		Domain:  string(domain),
		Query:   query,
		Results: []SearchResultInfo{},
	}

	for _, result := range results {
		output.Results = append(output.Results, SearchResultInfo{
			Resolved: strings.TrimSpace(result.Resolved),
			Label:    searchMatchLabel(result),
			Detail:   strings.TrimSpace(result.Detail),
			Kind:     strings.TrimSpace(string(result.Kind)),
		})
	}

	return output
}

func printSearchResults(writer io.Writer, domain searchDomain, query string, results []reviewMatchOption) {
	for _, line := range renderSearchResultLines(domain, query, results) {
		_, _ = fmt.Fprintln(writer, line)
	}
}

// renderSearchResultLines normalizes search results into reusable display lines
// for both the plain-text CLI path and the interactive search TUI.
func renderSearchResultLines(domain searchDomain, query string, results []reviewMatchOption) []string {
	if len(results) == 0 {
		return []string{fmt.Sprintf("No %s matches found for %q.", domain, query)}
	}

	lines := []string{fmt.Sprintf("Top %s matches for %q:", domain, query)}
	for idx, result := range results {
		label := searchMatchLabel(result)
		line := fmt.Sprintf("%d. %s", idx+1, label)
		if kind := strings.TrimSpace(string(result.Kind)); kind != "" {
			line = fmt.Sprintf("%s (%s)", line, kind)
		}
		line = fmt.Sprintf("%s  resolved: %s", line, strings.TrimSpace(result.Resolved))
		if detail := strings.TrimSpace(result.Detail); detail != "" && !strings.EqualFold(detail, label) {
			line = fmt.Sprintf("%s  detail: %s", line, detail)
		}
		lines = append(lines, line)
	}

	return lines
}

func searchMatchLabel(result reviewMatchOption) string {
	label := strings.TrimSpace(result.Label)
	if label == "" {
		return strings.TrimSpace(result.Resolved)
	}
	return label
}
