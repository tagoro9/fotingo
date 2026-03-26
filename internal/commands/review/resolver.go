package review

import (
	"fmt"
	"strings"

	"github.com/tagoro9/fotingo/internal/github"
)

// PickMatchFunc resolves one ambiguous token by selecting a concrete match.
type PickMatchFunc func(kind string, token string, matches []MatchOption) (string, error)

// ResolveLabels resolves requested label tokens to known repository labels.
func ResolveLabels(
	ghClient github.Github,
	requested []string,
	canPrompt bool,
	pick PickMatchFunc,
) ([]string, []string, error) {
	options, err := BuildLabelOptions(ghClient)
	if err != nil {
		return nil, nil, err
	}

	tokens := NormalizeTokens(requested)
	resolved := make([]string, 0, len(tokens))
	missing := make([]string, 0)

	for _, token := range tokens {
		matches := FindTokenMatches(token, options)
		if len(matches) == 0 {
			missing = append(missing, token)
			continue
		}
		if len(matches) == 1 {
			resolved = append(resolved, matches[0].Resolved)
			continue
		}

		if !canPrompt {
			return nil, nil, fmt.Errorf(
				"ambiguous label %q matched multiple candidates: %s (rerun without --yes to pick interactively)",
				token,
				FormatMatchList(matches),
			)
		}
		if pick == nil {
			return nil, nil, fmt.Errorf("ambiguous label %q requires picker callback", token)
		}

		selected, err := pick("label", token, matches)
		if err != nil {
			return nil, nil, err
		}
		resolved = append(resolved, selected)
	}

	return DedupeStringsPreserveOrder(resolved), DedupeStringsPreserveOrder(missing), nil
}

// ResolveReviewers resolves reviewer tokens to user and team reviewers.
func ResolveReviewers(
	ghClient github.Github,
	requested []string,
	canPrompt bool,
	pick PickMatchFunc,
) ([]string, []string, []string, error) {
	options, warnings, err := BuildParticipantOptionsForTokens(ghClient, requested, true)
	if err != nil {
		return nil, nil, warnings, err
	}

	selected, err := ResolveTokenMatches("reviewer", requested, options, canPrompt, pick)
	if err != nil {
		return nil, nil, warnings, err
	}

	userReviewers := make([]string, 0, len(selected))
	teamReviewers := make([]string, 0, len(selected))
	for _, match := range selected {
		switch match.Kind {
		case MatchKindTeam:
			teamReviewers = append(teamReviewers, match.Resolved)
		default:
			userReviewers = append(userReviewers, match.Resolved)
		}
	}

	return DedupeStringsPreserveOrder(userReviewers), DedupeStringsPreserveOrder(teamReviewers), warnings, nil
}

// ResolveAssignees resolves assignee tokens to user logins.
func ResolveAssignees(
	ghClient github.Github,
	requested []string,
	canPrompt bool,
	pick PickMatchFunc,
) ([]string, []string, error) {
	options, warnings, err := BuildParticipantOptionsForTokens(ghClient, requested, true)
	if err != nil {
		return nil, warnings, err
	}

	selected, err := ResolveTokenMatches("assignee", requested, options, canPrompt, pick)
	if err != nil {
		return nil, warnings, err
	}

	assignees := make([]string, 0, len(selected))
	for _, match := range selected {
		if match.Kind == MatchKindTeam {
			return nil, warnings, fmt.Errorf("team %q cannot be used as an assignee", match.Resolved)
		}
		assignees = append(assignees, match.Resolved)
	}

	return DedupeStringsPreserveOrder(assignees), warnings, nil
}

// ResolveTokens resolves tokens and returns the resolved values only.
func ResolveTokens(
	kind string,
	requested []string,
	options []MatchOption,
	canPrompt bool,
	pick PickMatchFunc,
) ([]string, error) {
	resolvedMatches, err := ResolveTokenMatches(kind, requested, options, canPrompt, pick)
	if err != nil {
		return nil, err
	}

	resolved := make([]string, 0, len(resolvedMatches))
	for _, match := range resolvedMatches {
		resolved = append(resolved, match.Resolved)
	}
	return DedupeStringsPreserveOrder(resolved), nil
}

// ToTeamSlugs converts canonical org/team values to team slugs.
func ToTeamSlugs(canonicalTeams []string) []string {
	slugs := make([]string, 0, len(canonicalTeams))
	for _, canonical := range canonicalTeams {
		value := strings.TrimSpace(canonical)
		if value == "" {
			continue
		}
		parts := strings.SplitN(value, "/", 2)
		if len(parts) == 2 {
			slugs = append(slugs, strings.TrimSpace(parts[1]))
			continue
		}
		slugs = append(slugs, value)
	}
	return DedupeStringsPreserveOrder(slugs)
}
