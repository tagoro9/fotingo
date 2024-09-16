package commandruntime

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/jira"
)

type jiraRetryClient struct {
	issue     *jira.Issue
	errByCall map[int]error
	calls     int
}

func (m *jiraRetryClient) GetJiraIssue(_ string) (*jira.Issue, error) {
	m.calls++
	if err, ok := m.errByCall[m.calls]; ok {
		return nil, err
	}
	return m.issue, nil
}

func TestGetJiraIssueWithRetry_RetriesAndSucceeds(t *testing.T) {
	client := &jiraRetryClient{
		issue: &jira.Issue{Key: "TEST-123"},
		errByCall: map[int]error{
			1: errors.New("temporary network error"),
			2: errors.New("jira timeout"),
		},
	}

	calls := 0
	issue, err := GetJiraIssueWithRetry(client, "TEST-123", JiraIssueLookupConfig{
		MaxAttempts: 3,
		Delay:       0,
		Sleep:       func(_ time.Duration) {},
	}, func(_ string, _ ...any) {
		calls++
	})
	require.NoError(t, err)
	require.NotNil(t, issue)
	assert.Equal(t, "TEST-123", issue.Key)
	assert.Equal(t, 3, client.calls)
	assert.Equal(t, 3, calls)
}

func TestGetJiraIssueWithRetry_ReturnsLastError(t *testing.T) {
	client := &jiraRetryClient{
		errByCall: map[int]error{
			1: errors.New("temporary network error"),
			2: errors.New("jira timeout"),
			3: errors.New("still failing"),
		},
	}

	issue, err := GetJiraIssueWithRetry(client, "TEST-123", JiraIssueLookupConfig{
		MaxAttempts: 3,
		Delay:       0,
		Sleep:       func(_ time.Duration) {},
	}, nil)
	require.Error(t, err)
	assert.Nil(t, issue)
	assert.Equal(t, "still failing", err.Error())
	assert.Equal(t, 3, client.calls)
}

func TestGetJiraIssueWithRetry_NormalizesAttempts(t *testing.T) {
	client := &jiraRetryClient{
		issue: &jira.Issue{Key: "TEST-123"},
	}

	issue, err := GetJiraIssueWithRetry(client, "TEST-123", JiraIssueLookupConfig{
		MaxAttempts: 0,
		Delay:       0,
		Sleep:       func(_ time.Duration) {},
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, issue)
	assert.Equal(t, 1, client.calls)
}
