package commands

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	"github.com/tagoro9/fotingo/internal/jira"
)

type jiraRetryMock struct {
	mockJira
	issue     *jira.Issue
	errByCall map[int]error
	calls     int
}

func (m *jiraRetryMock) GetJiraIssue(_ string) (*jira.Issue, error) {
	m.calls++
	if err := m.errByCall[m.calls]; err != nil {
		return nil, err
	}
	if m.issue != nil {
		return m.issue, nil
	}
	return m.jiraIssue, m.jiraIssueErr
}

func withJiraRetryTestConfig(t *testing.T) {
	t.Helper()

	origAttempts := jiraIssueLookupMaxAttempts
	origDelay := jiraIssueLookupDelay
	origSleep := jiraIssueLookupSleep
	t.Cleanup(func() {
		jiraIssueLookupMaxAttempts = origAttempts
		jiraIssueLookupDelay = origDelay
		jiraIssueLookupSleep = origSleep
	})

	jiraIssueLookupMaxAttempts = 3
	jiraIssueLookupDelay = 0
	jiraIssueLookupSleep = func(_ time.Duration) {}
}

func TestGetBranchDerivedJiraIssueWithRetry_RetriesAndSucceeds(t *testing.T) {
	withJiraRetryTestConfig(t)
	jiraIssueLookupSleep = func(_ time.Duration) {}

	statusCh := make(chan string, 8)
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)
	m := &jiraRetryMock{
		issue: &jira.Issue{Key: "TEST-123", Summary: "Retry me"},
		errByCall: map[int]error{
			1: errors.New("temporary network error"),
			2: errors.New("jira timeout"),
		},
	}

	issue, err := getBranchDerivedJiraIssueWithRetry(m, "TEST-123", &out)
	require.NoError(t, err)
	require.NotNil(t, issue)
	assert.Equal(t, "TEST-123", issue.Key)
	assert.Equal(t, 3, m.calls)
}

func TestGetBranchDerivedJiraIssueWithRetry_ReturnsLastErrorAfterMaxAttempts(t *testing.T) {
	withJiraRetryTestConfig(t)
	jiraIssueLookupSleep = func(_ time.Duration) {}

	m := &jiraRetryMock{
		errByCall: map[int]error{
			1: errors.New("temporary network error"),
			2: errors.New("jira timeout"),
			3: errors.New("still failing"),
		},
	}

	issue, err := getBranchDerivedJiraIssueWithRetry(m, "TEST-123", nil)
	require.Error(t, err)
	assert.Nil(t, issue)
	assert.Equal(t, "still failing", err.Error())
	assert.Equal(t, 3, m.calls)
}

func TestHandleOpenIssue_RetriesBranchIssueLookup(t *testing.T) {
	withJiraRetryTestConfig(t)
	jiraIssueLookupSleep = func(_ time.Duration) {}

	g := &mockGit{
		currentBranch: "f/TEST-123_retry_issue",
		issueId:       "TEST-123",
	}
	j := &jiraRetryMock{
		issue: &jira.Issue{Key: "TEST-123", Summary: "Retry me", Status: "In Progress"},
		errByCall: map[int]error{
			1: errors.New("temporary network error"),
		},
	}
	j.issueURL = "https://jira.example.com/browse/TEST-123"

	url, err := handleOpenIssue(g, j)
	require.NoError(t, err)
	assert.Equal(t, "https://jira.example.com/browse/TEST-123", url)
	assert.Equal(t, 2, j.calls)
}

func TestFetchReviewBranchIssue_RetriesBranchIssueLookup(t *testing.T) {
	withJiraRetryTestConfig(t)
	jiraIssueLookupSleep = func(_ time.Duration) {}

	statusCh := make(chan string, 8)
	out := commandruntime.NewLocalizedEmitter(statusCh, shouldEmitCommandLevel, localizer.T)
	j := &jiraRetryMock{
		issue: &jira.Issue{Key: "TEST-123", Summary: "Retry me"},
		errByCall: map[int]error{
			1: errors.New("temporary network error"),
		},
	}

	issue, err := fetchReviewBranchIssue(j, "TEST-123", out.Debugf)
	require.NoError(t, err)
	require.NotNil(t, issue)
	assert.Equal(t, "TEST-123", issue.Key)
	assert.Equal(t, 2, j.calls)
}

func TestFetchInspectBranchIssue_RetriesBranchIssueLookup(t *testing.T) {
	withJiraRetryTestConfig(t)
	jiraIssueLookupSleep = func(_ time.Duration) {}

	j := &jiraRetryMock{
		issue: &jira.Issue{Key: "TEST-123", Summary: "Retry me"},
		errByCall: map[int]error{
			1: errors.New("temporary network error"),
		},
	}

	issue, err := fetchInspectBranchIssue(j, "TEST-123")
	require.NoError(t, err)
	require.NotNil(t, issue)
	assert.Equal(t, "TEST-123", issue.Key)
	assert.Equal(t, 2, j.calls)
}
