package inspect

import (
	"regexp"

	"github.com/tagoro9/fotingo/internal/git"
)

var issueIDPattern = regexp.MustCompile(`[A-Z][A-Z0-9_]+-\d+`)

// IssueIDPattern returns the compiled Jira-style issue key matcher.
func IssueIDPattern() *regexp.Regexp {
	return issueIDPattern
}

// ExtractIssueIDsFromCommits extracts unique issue IDs from commit messages.
func ExtractIssueIDsFromCommits(commits []git.Commit) []string {
	seen := make(map[string]bool)
	var issueIDs []string

	for _, commit := range commits {
		matches := issueIDPattern.FindAllString(commit.Message, -1)
		for _, match := range matches {
			if !seen[match] {
				seen[match] = true
				issueIDs = append(issueIDs, match)
			}
		}
	}

	return issueIDs
}
