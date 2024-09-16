package review

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/texttheater/golang-levenshtein/levenshtein"

	"github.com/tagoro9/fotingo/internal/github"
)

// MatchOption is a selectable resolution candidate for review metadata inputs.
type MatchOption struct {
	Resolved string
	Label    string
	Detail   string
	Fields   []string
	Kind     MatchKind
}

// MatchKind identifies the review metadata domain of a MatchOption.
type MatchKind string

const (
	MatchKindUser  MatchKind = "user"
	MatchKindTeam  MatchKind = "team"
	MatchKindLabel MatchKind = "label"
)

// BuildParticipantOptions loads collaborator/member/team options for reviewer/assignee matching.
func BuildParticipantOptions(ghClient github.Github) ([]MatchOption, []string, error) {
	userByLogin := map[string]github.User{}
	teamsByCanonical := map[string]github.Team{}
	warnings := make([]string, 0)

	collaborators, err := ghClient.GetCollaborators()
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to load repository collaborators: %v", err))
	} else {
		for _, collaborator := range collaborators {
			login := strings.TrimSpace(collaborator.Login)
			if login == "" {
				continue
			}
			collaborator.Login = login
			if existing, exists := userByLogin[login]; exists {
				userByLogin[login] = PreferParticipantUser(existing, collaborator)
				continue
			}
			userByLogin[login] = collaborator
		}
	}

	members, err := ghClient.GetOrgMembers()
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to load organization members: %v", err))
	} else {
		for _, member := range members {
			login := strings.TrimSpace(member.Login)
			if login == "" {
				continue
			}
			member.Login = login
			if existing, exists := userByLogin[login]; exists {
				userByLogin[login] = PreferParticipantUser(existing, member)
				continue
			}
			userByLogin[login] = member
		}
	}

	teams, err := ghClient.GetTeams()
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to load organization teams: %v", err))
	} else {
		for _, team := range teams {
			canonical := team.Canonical()
			if canonical == "" {
				continue
			}
			teamsByCanonical[canonical] = team
		}
	}

	if len(userByLogin) == 0 && len(teamsByCanonical) == 0 {
		return nil, warnings, fmt.Errorf("failed to load review participants from collaborators, organization members, and teams")
	}

	userKeys := make([]string, 0, len(userByLogin))
	for login := range userByLogin {
		userKeys = append(userKeys, login)
	}
	sort.Strings(userKeys)

	teamKeys := make([]string, 0, len(teamsByCanonical))
	for canonical := range teamsByCanonical {
		teamKeys = append(teamKeys, canonical)
	}
	sort.Strings(teamKeys)

	options := make([]MatchOption, 0, len(userKeys)+len(teamKeys))
	for _, login := range userKeys {
		user := userByLogin[login]
		options = append(options, MatchOption{
			Resolved: user.Login,
			Label:    user.Login,
			Detail:   user.Name,
			Fields: []string{
				user.Login,
				user.Name,
			},
			Kind: MatchKindUser,
		})
	}
	for _, canonical := range teamKeys {
		team := teamsByCanonical[canonical]
		options = append(options, MatchOption{
			Resolved: canonical,
			Label:    canonical,
			Detail:   team.Name,
			Fields: []string{
				canonical,
				team.Slug,
				team.Name,
				team.Description,
			},
			Kind: MatchKindTeam,
		})
	}

	return options, warnings, nil
}

// PreferParticipantUser prefers candidates that provide a richer name field.
func PreferParticipantUser(current github.User, candidate github.User) github.User {
	currentName := strings.TrimSpace(current.Name)
	candidateName := strings.TrimSpace(candidate.Name)
	if currentName == "" && candidateName != "" {
		return candidate
	}
	return current
}

// ResolveTokenMatches resolves requested tokens with optional interactive disambiguation.
func ResolveTokenMatches(
	kind string,
	requested []string,
	options []MatchOption,
	canPrompt bool,
	pick func(kind string, token string, matches []MatchOption) (string, error),
) ([]MatchOption, error) {
	tokens := NormalizeTokens(requested)
	resolved := make([]MatchOption, 0, len(tokens))

	for _, token := range tokens {
		matches := FindTokenMatches(token, options)
		if len(matches) == 0 {
			alternatives := FindTokenAlternatives(token, options, 5)
			if len(alternatives) == 0 {
				return nil, fmt.Errorf("no %s matches found for %q", kind, token)
			}
			return nil, fmt.Errorf(
				"no %s matches found for %q. Closest matches: %s",
				kind,
				token,
				FormatMatchList(alternatives),
			)
		}
		if len(matches) == 1 {
			resolved = append(resolved, matches[0])
			continue
		}

		if !canPrompt {
			return nil, fmt.Errorf(
				"ambiguous %s %q matched multiple candidates: %s (rerun without --yes to pick interactively)",
				kind,
				token,
				FormatMatchList(matches),
			)
		}

		selected, err := pick(kind, token, matches)
		if err != nil {
			return nil, err
		}
		selectedMatch, ok := FindOptionByResolved(matches, selected)
		if !ok {
			return nil, fmt.Errorf("selected %s %q is not a valid candidate for %q", kind, selected, token)
		}
		resolved = append(resolved, selectedMatch)
	}

	return DedupeMatchesPreserveOrder(resolved), nil
}

// FindTokenMatches returns the best-scored matches for a token.
func FindTokenMatches(token string, options []MatchOption) []MatchOption {
	scored := make([]scoredMatch, 0, len(options))
	for _, option := range options {
		score, matched := ScoreTokenMatch(token, option.Fields)
		if !matched {
			continue
		}
		scored = append(scored, scoredMatch{option: option, score: score})
	}

	if len(scored) == 0 {
		return nil
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score < scored[j].score
		}
		return strings.ToLower(scored[i].option.Label) < strings.ToLower(scored[j].option.Label)
	})

	bestScore := scored[0].score
	bestMatches := make([]MatchOption, 0, len(scored))
	for _, match := range scored {
		if match.score != bestScore {
			break
		}
		bestMatches = append(bestMatches, match.option)
	}

	return bestMatches
}

// FindTokenMatchesForCompletion returns ordered matches used for completion candidates.
func FindTokenMatchesForCompletion(token string, options []MatchOption) []MatchOption {
	scored := make([]scoredMatch, 0, len(options))
	for _, option := range options {
		score, matched := ScoreTokenMatch(token, option.Fields)
		if !matched {
			continue
		}
		scored = append(scored, scoredMatch{option: option, score: score})
	}

	if len(scored) == 0 {
		return nil
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score < scored[j].score
		}
		return strings.ToLower(scored[i].option.Label) < strings.ToLower(scored[j].option.Label)
	})

	matches := make([]MatchOption, 0, len(scored))
	for _, match := range scored {
		matches = append(matches, match.option)
	}
	return DedupeMatchesPreserveOrder(matches)
}

// FindTokenAlternatives returns fallback alternatives when no direct match exists.
func FindTokenAlternatives(token string, options []MatchOption, limit int) []MatchOption {
	if limit <= 0 {
		return nil
	}

	scored := make([]scoredMatch, 0, len(options))
	for _, option := range options {
		score, ok := ScoreTokenDistance(token, option.Fields)
		if !ok {
			continue
		}
		scored = append(scored, scoredMatch{option: option, score: score})
	}

	if len(scored) == 0 {
		return nil
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score < scored[j].score
		}
		return strings.ToLower(scored[i].option.Label) < strings.ToLower(scored[j].option.Label)
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	alternatives := make([]MatchOption, 0, len(scored))
	for _, match := range scored {
		alternatives = append(alternatives, match.option)
	}
	return DedupeMatchesPreserveOrder(alternatives)
}

// ScoreTokenMatch returns match score and whether token matched any fields.
func ScoreTokenMatch(token string, fields []string) (int, bool) {
	normalizedToken := strings.ToLower(strings.TrimSpace(token))
	if normalizedToken == "" {
		return 0, false
	}

	bestScore := 0
	matched := false
	initialized := false
	for _, rawField := range fields {
		field := strings.ToLower(strings.TrimSpace(rawField))
		if field == "" {
			continue
		}

		candidates := append([]string{field}, FieldTokenCandidates(field)...)
		for _, candidate := range candidates {
			switch {
			case candidate == normalizedToken:
				score := 0
				if candidate != field {
					score = 20
				}
				if !initialized || score < bestScore {
					bestScore = score
					initialized = true
				}
				matched = true
			case strings.HasPrefix(candidate, normalizedToken):
				score := 100 + (len(candidate) - len(normalizedToken))
				if candidate != field {
					score += 20
				}
				if !initialized || score < bestScore {
					bestScore = score
					initialized = true
				}
				matched = true
			case strings.Contains(candidate, normalizedToken):
				score := 200 + strings.Index(candidate, normalizedToken)
				if candidate != field {
					score += 20
				}
				if !initialized || score < bestScore {
					bestScore = score
					initialized = true
				}
				matched = true
			default:
				distance := levenshtein.DistanceForStrings(
					[]rune(normalizedToken),
					[]rune(candidate),
					levenshtein.DefaultOptions,
				)
				threshold := len([]rune(normalizedToken))/2 + 2
				if distance <= threshold {
					score := 300 + distance
					if candidate != field {
						score += 20
					}
					if !initialized || score < bestScore {
						bestScore = score
						initialized = true
					}
					matched = true
				}
			}
		}
	}

	return bestScore, matched
}

// ScoreTokenDistance returns distance score for fuzzy alternatives.
func ScoreTokenDistance(token string, fields []string) (int, bool) {
	normalizedToken := strings.ToLower(strings.TrimSpace(token))
	if normalizedToken == "" {
		return 0, false
	}

	bestScore := 0
	initialized := false
	for _, rawField := range fields {
		field := strings.ToLower(strings.TrimSpace(rawField))
		if field == "" {
			continue
		}

		candidates := append([]string{field}, FieldTokenCandidates(field)...)
		for _, candidate := range candidates {
			score := 0
			switch {
			case candidate == normalizedToken:
				score = 0
			case strings.Contains(candidate, normalizedToken):
				score = 200 + strings.Index(candidate, normalizedToken)
			default:
				distance := levenshtein.DistanceForStrings(
					[]rune(normalizedToken),
					[]rune(candidate),
					levenshtein.DefaultOptions,
				)
				score = 400 + distance
			}

			if candidate != field {
				score += 20
			}

			if !initialized || score < bestScore {
				bestScore = score
				initialized = true
			}
		}
	}

	return bestScore, initialized
}

// FieldTokenCandidates splits a field into auxiliary searchable tokens.
func FieldTokenCandidates(field string) []string {
	tokens := strings.FieldsFunc(field, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	return DedupeStringsPreserveOrder(tokens)
}

// NormalizeTokens trims and drops empty requested tokens.
func NormalizeTokens(tokens []string) []string {
	normalized := make([]string, 0, len(tokens))
	for _, token := range tokens {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

// DedupeStringsPreserveOrder deduplicates strings while keeping first-seen order.
func DedupeStringsPreserveOrder(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	deduped := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		deduped = append(deduped, value)
	}
	return deduped
}

// DedupeMatchesPreserveOrder deduplicates match options by kind+resolved key.
func DedupeMatchesPreserveOrder(values []MatchOption) []MatchOption {
	seen := make(map[string]struct{}, len(values))
	deduped := make([]MatchOption, 0, len(values))
	for _, value := range values {
		key := string(value.Kind) + ":" + value.Resolved
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, value)
	}
	return deduped
}

// FindOptionByResolved finds a specific option by resolved value.
func FindOptionByResolved(options []MatchOption, resolved string) (MatchOption, bool) {
	for _, option := range options {
		if option.Resolved == resolved {
			return option, true
		}
	}
	return MatchOption{}, false
}

// FormatMatchList formats a match-option slice for error messages.
func FormatMatchList(matches []MatchOption) string {
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, FormatMatchOption(match))
	}
	sort.Strings(values)
	return strings.Join(values, ", ")
}

// FormatMatchOption formats one match option for user-facing output.
func FormatMatchOption(match MatchOption) string {
	value := strings.TrimSpace(match.Resolved)
	detail := strings.TrimSpace(match.Detail)
	if (match.Kind == MatchKindUser || match.Kind == MatchKindTeam) &&
		detail != "" &&
		!strings.EqualFold(detail, value) {
		return fmt.Sprintf("%s (%s)", value, detail)
	}
	return value
}

type scoredMatch struct {
	option MatchOption
	score  int
}
