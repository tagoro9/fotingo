package start

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tagoro9/fotingo/internal/tracker"
)

// IssueLinkClient is the subset of tracker behavior required for link resolution.
type IssueLinkClient interface {
	IsValidIssueID(id string) bool
	GetIssue(id string) (*tracker.Issue, error)
	SearchIssues(projectKey string, query string, issueTypes []tracker.IssueType, limit int) ([]tracker.Issue, error)
}

// ResolveIssueLinkOptions configures interactive/non-interactive issue-link resolution.
type ResolveIssueLinkOptions struct {
	ProjectKey       string
	RawQuery         string
	AllowedTypes     []tracker.IssueType
	Interactive      bool
	PickerTitle      string
	SelectIssueLink  func([]tracker.Issue, string) (*tracker.Issue, error)
	PromptRefineLink func(currentQuery string) (string, bool, error)
	Errors           ResolveIssueLinkErrors
}

// ResolveIssueLinkErrors allows the caller to inject localized/domain errors.
type ResolveIssueLinkErrors struct {
	QueryRequired func() error
	SearchIssues  func(error) error
	LinkNotFound  func(string) error
	LinkAmbiguous func(string) error
	Cancelled     func() error
}

// ResolveIssueLink resolves an issue key from free-form query text.
func ResolveIssueLink(client IssueLinkClient, opts ResolveIssueLinkOptions) (string, error) {
	query := strings.TrimSpace(opts.RawQuery)
	if query == "" {
		return "", resolveLinkErr(opts.Errors.QueryRequired, errors.New("issue query is required"))
	}

	currentQuery := query
	for {
		normalizedQuery := strings.ToUpper(currentQuery)
		if client.IsValidIssueID(normalizedQuery) {
			issue, err := client.GetIssue(normalizedQuery)
			if err == nil && issue != nil && IssueMatchesAllowedTypes(issue.Type, opts.AllowedTypes) {
				return strings.TrimSpace(issue.Key), nil
			}
		}

		candidates, err := client.SearchIssues(opts.ProjectKey, currentQuery, opts.AllowedTypes, 50)
		if err != nil {
			return "", resolveLinkErrWithArg(opts.Errors.SearchIssues, err, fmt.Errorf("failed to search issues: %w", err))
		}
		if len(candidates) == 0 {
			return "", resolveLinkErrWithArg(opts.Errors.LinkNotFound, currentQuery, fmt.Errorf("issue %q not found", currentQuery))
		}

		exactMatches := make([]tracker.Issue, 0, 1)
		for _, candidate := range candidates {
			if strings.EqualFold(strings.TrimSpace(candidate.Key), normalizedQuery) {
				exactMatches = append(exactMatches, candidate)
			}
		}
		if len(exactMatches) == 1 {
			return exactMatches[0].Key, nil
		}

		if len(candidates) == 1 {
			return strings.TrimSpace(candidates[0].Key), nil
		}

		if !opts.Interactive {
			return "", resolveLinkErrWithArg(opts.Errors.LinkAmbiguous, currentQuery, fmt.Errorf("issue link %q is ambiguous", currentQuery))
		}

		if opts.SelectIssueLink == nil || opts.PromptRefineLink == nil {
			return "", errors.New("interactive issue resolution requires select and refine callbacks")
		}

		selected, err := opts.SelectIssueLink(candidates, opts.PickerTitle)
		if err != nil {
			return "", err
		}
		if selected != nil {
			return strings.TrimSpace(selected.Key), nil
		}

		refinedQuery, cancelled, err := opts.PromptRefineLink(currentQuery)
		if err != nil {
			return "", err
		}
		if cancelled {
			return "", resolveLinkErr(opts.Errors.Cancelled, errors.New("issue selection cancelled"))
		}

		currentQuery = strings.TrimSpace(refinedQuery)
		if currentQuery == "" {
			return "", resolveLinkErr(opts.Errors.QueryRequired, errors.New("issue query is required"))
		}
	}
}

func resolveLinkErr(factory func() error, fallback error) error {
	if factory == nil {
		return fallback
	}
	return factory()
}

func resolveLinkErrWithArg[T any](factory func(T) error, arg T, fallback error) error {
	if factory == nil {
		return fallback
	}
	return factory(arg)
}
