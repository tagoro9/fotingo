package ai

import (
	"embed"
	"fmt"
	"strings"

	"github.com/tagoro9/fotingo/internal/commandruntime"
)

//go:embed skills/*.md
var skillsFS embed.FS

// skillMetadataVersion is the semver for generated fotingo skill content.
// Bump this when generated skill guidance/frontmatter changes.
const skillMetadataVersion = "1.6.0"

// RenderSkill returns provider-specific skill markdown content from embedded assets.
func RenderSkill(provider Provider) (string, error) {
	if _, ok := providerSpec(provider); !ok {
		return "", fmt.Errorf("unsupported provider %q", provider)
	}

	core, err := skillsFS.ReadFile("skills/fotingo-core.md")
	if err != nil {
		return "", err
	}

	introTemplate, err := skillsFS.ReadFile("skills/skill-intro.md")
	if err != nil {
		return "", err
	}

	examples := DefaultCommandExamples()
	content := strings.TrimSpace(string(introTemplate)) + "\n\n" + strings.TrimSpace(string(core))
	buildVersion := strings.TrimSpace(commandruntime.GetBuildInfo().Version)
	if buildVersion == "" {
		buildVersion = "dev"
	}

	replacements := map[string]string{
		"{{PROVIDER_LABEL}}":                    ProviderLabel(provider),
		"{{SKILL_VERSION}}":                     skillMetadataVersion,
		"{{FOTINGO_VERSION}}":                   buildVersion,
		"{{EXAMPLE_INSPECT_JSON}}":              examples.InspectJSON,
		"{{EXAMPLE_START_EXISTING_ISSUE}}":      examples.StartExistingIssue,
		"{{EXAMPLE_START_CREATE_ISSUE}}":        examples.StartCreateIssue,
		"{{EXAMPLE_START_WORKTREE}}":            examples.StartWorktree,
		"{{EXAMPLE_SEARCH_REVIEWERS}}":          examples.SearchReviewers,
		"{{EXAMPLE_SEARCH_ASSIGNEES}}":          examples.SearchAssignees,
		"{{EXAMPLE_SEARCH_LABELS}}":             examples.SearchLabels,
		"{{EXAMPLE_REVIEW_DEFAULT}}":            examples.ReviewDefault,
		"{{EXAMPLE_REVIEW_BASE_BRANCH}}":        examples.ReviewBaseBranch,
		"{{EXAMPLE_REVIEW_TEMPLATE_OVERRIDES}}": examples.ReviewTemplateOverrides,
		"{{EXAMPLE_REVIEW_BODY_FROM_STDIN}}":    examples.ReviewBodyFromStdin,
		"{{EXAMPLE_REVIEW_SYNC_DEFAULT}}":       examples.ReviewSyncDefault,
		"{{EXAMPLE_REVIEW_WITH_PARTICIPANTS}}":  examples.ReviewWithParticipants,
	}
	for placeholder, value := range replacements {
		content = strings.ReplaceAll(content, placeholder, value)
	}

	return strings.TrimSpace(content) + "\n", nil
}
