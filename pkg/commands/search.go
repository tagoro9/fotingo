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
			return runSearchMetadataCommand(cmd.OutOrStdout(), domain, args)
		},
	}
}

func runSearchMetadataCommand(writer io.Writer, domain searchDomain, args []string) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return errors.New("search query is required")
	}

	results, err := searchReviewMetadata(domain, query)
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

func searchReviewMetadata(domain searchDomain, query string) ([]reviewMatchOption, error) {
	ghClient, err := newSearchGitHubClient()
	if err != nil {
		return nil, err
	}

	options, err := loadSearchOptions(ghClient, domain)
	if err != nil {
		return nil, err
	}

	matches := internalreview.FindTokenMatchesForCompletion(query, options)
	if len(matches) > searchResultsLimit {
		matches = matches[:searchResultsLimit]
	}

	return matches, nil
}

func loadSearchOptions(ghClient github.Github, domain searchDomain) ([]reviewMatchOption, error) {
	switch domain {
	case searchDomainReviewers:
		options, _, err := internalreview.BuildParticipantOptions(ghClient)
		return options, err
	case searchDomainAssignees:
		options, _, err := internalreview.BuildParticipantOptions(ghClient)
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
	if len(results) == 0 {
		_, _ = fmt.Fprintf(writer, "No %s matches found for %q.\n", domain, query)
		return
	}

	_, _ = fmt.Fprintf(writer, "Top %s matches for %q:\n", domain, query)
	for idx, result := range results {
		label := searchMatchLabel(result)
		_, _ = fmt.Fprintf(writer, "%d. %s", idx+1, label)
		if kind := strings.TrimSpace(string(result.Kind)); kind != "" {
			_, _ = fmt.Fprintf(writer, " (%s)", kind)
		}
		_, _ = fmt.Fprintln(writer)
		_, _ = fmt.Fprintf(writer, "   resolved: %s\n", strings.TrimSpace(result.Resolved))
		if detail := strings.TrimSpace(result.Detail); detail != "" && !strings.EqualFold(detail, label) {
			_, _ = fmt.Fprintf(writer, "   detail: %s\n", detail)
		}
	}
}

func searchMatchLabel(result reviewMatchOption) string {
	label := strings.TrimSpace(result.Label)
	if label == "" {
		return strings.TrimSpace(result.Resolved)
	}
	return label
}
