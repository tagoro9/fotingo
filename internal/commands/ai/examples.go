package ai

// CommandExamples defines reusable command snippets that can be shared
// between help text and generated skill content.
type CommandExamples struct {
	InspectJSON             string
	StartExistingIssue      string
	StartCreateIssue        string
	SearchReviewers         string
	SearchAssignees         string
	SearchLabels            string
	ReviewDefault           string
	ReviewTemplateOverrides string
	ReviewWithParticipants  string
}

// DefaultCommandExamples returns canonical command snippets for AI guidance.
func DefaultCommandExamples() CommandExamples {
	return CommandExamples{
		InspectJSON:             "fotingo inspect --json",
		StartExistingIssue:      "fotingo start PROJ-123 -y",
		StartCreateIssue:        `fotingo start -p PROJ -k Task -t "Improve checkout decline error handling" -d "Problem: payment declines are hard to diagnose. Goal: clear user-facing messaging plus actionable logs. Acceptance criteria: improved copy, telemetry events, and regression tests for decline paths." -y`,
		SearchReviewers:         "fotingo search reviewers ali --json",
		SearchAssignees:         "fotingo search assignees bob --json",
		SearchLabels:            "fotingo search labels bug --json",
		ReviewDefault:           "fotingo review -y",
		ReviewTemplateOverrides: `fotingo review -y --template-summary "Improve checkout decline handling" --template-description "Why: reduce support tickets from unclear payment errors. What: clearer decline copy, structured telemetry, and test coverage for decline scenarios."`,
		ReviewWithParticipants:  "fotingo review -y -r alice -r team/platform --assignee bob --labels bug",
	}
}
