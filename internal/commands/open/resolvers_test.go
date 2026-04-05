package open

import (
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapPRError(t *testing.T) {
	pattern := regexp.MustCompile(`(?i)no pull request found for branch\s+([^\s:]+)`)
	baseErr := errors.New("no pull request found for branch feature/ABC-1")

	err := MapPRError(
		baseErr,
		pattern,
		func(branch string, cause error) error {
			assert.Equal(t, "feature/ABC-1", branch)
			return errors.Join(errors.New("mapped"), cause)
		},
		func(cause error) error {
			return errors.Join(errors.New("wrapped"), cause)
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mapped")
	assert.Contains(t, err.Error(), "feature/ABC-1")
}

func TestMapPRErrorFallback(t *testing.T) {
	baseErr := errors.New("other")
	err := MapPRError(
		baseErr,
		regexp.MustCompile(`x`),
		func(_ string, _ error) error { return errors.New("mapped") },
		func(cause error) error { return errors.Join(errors.New("wrapped"), cause) },
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "wrapped")
}

func TestCollectLinkedIssueIDs_PreservesOrderAndDeduplicates(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		[]string{"FOTINGO-26", "FOTINGO-31", "FOTINGO-44"},
		CollectLinkedIssueIDs(
			"FOTINGO-26",
			[]string{"FOTINGO-31", "FOTINGO-26", "", "FOTINGO-44", "FOTINGO-31"},
		),
	)
}

func TestCollectLinkedIssueIDs_AllowsCommitOnlyResolution(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		[]string{"FOTINGO-26"},
		CollectLinkedIssueIDs("", []string{"FOTINGO-26"}),
	)
}
