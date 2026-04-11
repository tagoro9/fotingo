package ai

// CommandExamples defines reusable command snippets that can be shared
// between help text and generated skill content.
type CommandExamples struct {
	InspectJSON             string
	InspectPullRequestJSON  string
	StartExistingIssue      string
	StartCreateIssue        string
	StartWorktree           string
	SearchReviewers         string
	SearchAssignees         string
	SearchLabels            string
	ReviewDefault           string
	ReviewBaseBranch        string
	ReviewTemplateOverrides string
	ReviewBodyFromStdin     string
	ReviewSyncDefault       string
	ReviewSyncMetadata      string
	ReviewWithParticipants  string
}

// DefaultCommandExamples returns canonical command snippets for AI guidance.
func DefaultCommandExamples() CommandExamples {
	return CommandExamples{
		InspectJSON:             "fotingo inspect --json",
		InspectPullRequestJSON:  "fotingo inspect pr --json",
		StartExistingIssue:      "fotingo start PROJ-123 -y",
		StartCreateIssue:        `fotingo start -p PROJ -k Task -t "Improve checkout decline error handling" -d "Problem: payment declines are hard to diagnose. Goal: clear user-facing messaging plus actionable logs. Acceptance criteria: improved copy, telemetry events, and regression tests for decline paths." -y`,
		StartWorktree:           "fotingo start PROJ-123 --worktree -y --json",
		SearchReviewers:         "fotingo search reviewers ali --json",
		SearchAssignees:         "fotingo search assignees bob --json",
		SearchLabels:            "fotingo search labels bug --json",
		ReviewDefault:           "fotingo review -y",
		ReviewBaseBranch:        "fotingo review -y --branch release/2026.04",
		ReviewTemplateOverrides: `fotingo review -y --template-summary "Improve checkout decline handling" --template-description "Why: reduce support tickets from unclear payment errors.\n\nWhat changed:\n- clearer decline copy\n- structured telemetry\n- regression coverage"`,
		ReviewBodyFromStdin:     `printf '## Summary\n\nImprove checkout decline handling\n\n## Description\n\nDetailed reviewer notes.\n' | fotingo review -y --description -`,
		ReviewSyncDefault:       "fotingo review sync -y",
		ReviewSyncMetadata:      "fotingo review sync -y -r alice --remove-reviewers team/platform --assignee bob --remove-assignee carol --ready-for-review",
		ReviewWithParticipants:  "fotingo review -y -r alice -r team/platform --assignee bob --labels bug",
	}
}
