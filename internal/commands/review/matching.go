package review

import (
	"errors"
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

var errOrganizationMetadataUnsupported = errors.New(
	"organization-scoped participant metadata is unsupported for the repository owner",
)

// BuildLabelOptions loads repository label options for review metadata matching.
func BuildLabelOptions(ghClient github.Github) ([]MatchOption, error) {
	labels, err := ghClient.GetLabels()
	if err != nil {
		return nil, fmt.Errorf("failed to load repository labels: %w", err)
	}

	options := make([]MatchOption, 0, len(labels))
	for _, label := range labels {
		options = append(options, MatchOption{
			Resolved: label.Name,
			Label:    label.Name,
			Detail:   label.Description,
			Fields: []string{
				label.Name,
				label.Description,
				label.Color,
			},
			Kind: MatchKindLabel,
		})
	}

	return options, nil
}

// BuildParticipantOptions loads organization-scoped participant options and
// supplements them with repository collaborators.
func BuildParticipantOptions(ghClient github.Github) ([]MatchOption, []string, error) {
	return buildParticipantOptions(ghClient, true)
}

// BuildOrgScopedParticipantOptions loads organization members and optional teams
// when the current repository owner supports organization metadata.
func BuildOrgScopedParticipantOptions(
	ghClient github.Github,
	includeTeams bool,
) ([]MatchOption, []string, error) {
	builder := newParticipantOptionBuilder()
	warnings := make([]string, 0)

	supported, err := repositoryOwnerSupportsOrganizationMetadata(ghClient)
	if err != nil {
		return nil, warnings, fmt.Errorf(
			"failed to determine whether the repository owner supports organization metadata: %w",
			err,
		)
	}
	if !supported {
		return nil, warnings, errOrganizationMetadataUnsupported
	}

	members, err := ghClient.GetOrgMembers()
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to load organization members: %v", err))
	} else {
		builder.addUsers(members)
	}

	if includeTeams {
		teams, err := ghClient.GetTeams()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to load organization teams: %v", err))
		} else {
			builder.addTeams(teams)
		}
	}

	options := builder.options()
	if len(options) == 0 {
		if includeTeams {
			return nil, warnings, fmt.Errorf("failed to load review participants from organization members and teams")
		}
		return nil, warnings, fmt.Errorf("failed to load review participants from organization members")
	}

	return options, warnings, nil
}

// BuildParticipantOptionsForQuery loads participant options for query-based
// reviewer and assignee search.
func BuildParticipantOptionsForQuery(
	ghClient github.Github,
	query string,
	includeTeams bool,
) ([]MatchOption, []string, error) {
	_ = query
	return buildParticipantOptions(ghClient, includeTeams)
}

// BuildParticipantOptionsForTokens loads participant options for token-based
// reviewer and assignee resolution.
func BuildParticipantOptionsForTokens(
	ghClient github.Github,
	requested []string,
	includeTeams bool,
) ([]MatchOption, []string, error) {
	_ = requested
	return buildParticipantOptions(ghClient, includeTeams)
}

func buildParticipantOptions(ghClient github.Github, includeTeams bool) ([]MatchOption, []string, error) {
	orgOptions, warnings, orgErr := BuildOrgScopedParticipantOptions(ghClient, includeTeams)

	collaboratorOptions, collaboratorErr := buildCollaboratorParticipantOptions(ghClient)
	if collaboratorErr != nil {
		warnings = append(warnings, fmt.Sprintf("failed to load repository collaborators: %v", collaboratorErr))
	}

	options := mergeParticipantOptions(orgOptions, collaboratorOptions)
	if len(options) > 0 {
		return options, warnings, nil
	}

	if isOrganizationMetadataUnsupported(orgErr) {
		if collaboratorErr != nil {
			return nil, warnings, collaboratorErr
		}
		return nil, warnings, fmt.Errorf("failed to load review participants from repository collaborators")
	}
	if orgErr != nil && collaboratorErr != nil {
		return nil, warnings, fmt.Errorf(
			"failed to load review participants from collaborators, organization members, and teams",
		)
	}
	if orgErr != nil {
		return nil, warnings, orgErr
	}
	if collaboratorErr != nil {
		return nil, warnings, collaboratorErr
	}
	return nil, warnings, fmt.Errorf(
		"failed to load review participants from collaborators, organization members, and teams",
	)
}

// organizationMetadataSupporter reports whether the repository owner can expose
// organization member and team metadata.
type organizationMetadataSupporter interface {
	SupportsOrganizationMetadata() (bool, error)
}

func repositoryOwnerSupportsOrganizationMetadata(ghClient github.Github) (bool, error) {
	checker, ok := ghClient.(organizationMetadataSupporter)
	if !ok {
		return true, nil
	}
	return checker.SupportsOrganizationMetadata()
}

func isOrganizationMetadataUnsupported(err error) bool {
	return errors.Is(err, errOrganizationMetadataUnsupported)
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

type participantOptionBuilder struct {
	userByLogin      map[string]github.User
	teamsByCanonical map[string]github.Team
}

func newParticipantOptionBuilder() participantOptionBuilder {
	return participantOptionBuilder{
		userByLogin:      map[string]github.User{},
		teamsByCanonical: map[string]github.Team{},
	}
}

func (b *participantOptionBuilder) addUsers(users []github.User) {
	for _, user := range users {
		login := strings.TrimSpace(user.Login)
		if login == "" {
			continue
		}
		user.Login = login
		if existing, exists := b.userByLogin[login]; exists {
			b.userByLogin[login] = PreferParticipantUser(existing, user)
			continue
		}
		b.userByLogin[login] = user
	}
}

func (b *participantOptionBuilder) addTeams(teams []github.Team) {
	for _, team := range teams {
		canonical := team.Canonical()
		if canonical == "" {
			continue
		}
		b.teamsByCanonical[canonical] = team
	}
}

func (b participantOptionBuilder) options() []MatchOption {
	userKeys := make([]string, 0, len(b.userByLogin))
	for login := range b.userByLogin {
		userKeys = append(userKeys, login)
	}
	sort.Strings(userKeys)

	teamKeys := make([]string, 0, len(b.teamsByCanonical))
	for canonical := range b.teamsByCanonical {
		teamKeys = append(teamKeys, canonical)
	}
	sort.Strings(teamKeys)

	options := make([]MatchOption, 0, len(userKeys)+len(teamKeys))
	for _, login := range userKeys {
		user := b.userByLogin[login]
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
		team := b.teamsByCanonical[canonical]
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

	return options
}

func buildCollaboratorParticipantOptions(ghClient github.Github) ([]MatchOption, error) {
	collaborators, err := ghClient.GetCollaborators()
	if err != nil {
		return nil, fmt.Errorf("failed to load repository collaborators: %w", err)
	}

	builder := newParticipantOptionBuilder()
	builder.addUsers(collaborators)
	return builder.options(), nil
}

func mergeParticipantOptions(base []MatchOption, extras []MatchOption) []MatchOption {
	builder := newParticipantOptionBuilder()
	addParticipantMatchOptions(&builder, base)
	addParticipantMatchOptions(&builder, extras)
	return builder.options()
}

func addParticipantMatchOptions(builder *participantOptionBuilder, options []MatchOption) {
	for _, option := range options {
		switch option.Kind {
		case MatchKindTeam:
			parts := strings.SplitN(strings.TrimSpace(option.Resolved), "/", 2)
			team := github.Team{
				Name: option.Detail,
				Slug: strings.TrimSpace(option.Resolved),
			}
			if len(parts) == 2 {
				team.Organization = strings.TrimSpace(parts[0])
				team.Slug = strings.TrimSpace(parts[1])
			}
			builder.addTeams([]github.Team{team})
		default:
			builder.addUsers([]github.User{{
				Login: strings.TrimSpace(option.Resolved),
				Name:  strings.TrimSpace(option.Detail),
			}})
		}
	}
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
