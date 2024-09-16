package commandruntime

import (
	"time"

	"github.com/tagoro9/fotingo/internal/jira"
)

// JiraIssueLookupClient is the subset required for retrying Jira issue lookups.
type JiraIssueLookupClient interface {
	GetJiraIssue(issueID string) (*jira.Issue, error)
}

// JiraIssueLookupConfig controls retry behavior when looking up branch-derived Jira issues.
type JiraIssueLookupConfig struct {
	MaxAttempts int
	Delay       time.Duration
	Sleep       func(time.Duration)
}

// GetJiraIssueWithRetry fetches a Jira issue with bounded retries.
func GetJiraIssueWithRetry(
	jiraClient JiraIssueLookupClient,
	issueID string,
	cfg JiraIssueLookupConfig,
	debugf func(format string, args ...any),
) (*jira.Issue, error) {
	attempts := cfg.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}

	sleep := cfg.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		issue, err := jiraClient.GetJiraIssue(issueID)
		if err == nil {
			if attempt > 1 && debugf != nil {
				debugf("jira issue lookup succeeded for %s on attempt %d", issueID, attempt)
			}
			return issue, nil
		}

		lastErr = err
		if attempt < attempts {
			if debugf != nil {
				debugf(
					"jira issue lookup failed for %s on attempt %d/%d: %v",
					issueID,
					attempt,
					attempts,
					err,
				)
			}
			sleep(cfg.Delay)
		}
	}

	return nil, lastErr
}
