package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalreview "github.com/tagoro9/fotingo/internal/commands/review"
	internalstart "github.com/tagoro9/fotingo/internal/commands/start"
	"github.com/tagoro9/fotingo/internal/tracker"
)

func TestIsKnownOutputLevel(t *testing.T) {
	assert.True(t, isKnownOutputLevel(commandruntime.OutputLevelInfo))
	assert.True(t, isKnownOutputLevel(commandruntime.OutputLevelVerbose))
	assert.True(t, isKnownOutputLevel(commandruntime.OutputLevelDebug))
	assert.False(t, isKnownOutputLevel(commandruntime.OutputLevel(999)))
}

func TestShouldEmitCommandLevel(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()

	Global.JSON = false
	Global.Quiet = false
	Global.Verbose = false
	Global.Debug = false

	assert.True(t, shouldEmitCommandLevel(commandruntime.OutputLevelInfo))
	assert.False(t, shouldEmitCommandLevel(commandruntime.OutputLevelVerbose))
	assert.False(t, shouldEmitCommandLevel(commandruntime.OutputLevelDebug))

	Global.Verbose = true
	assert.True(t, shouldEmitCommandLevel(commandruntime.OutputLevelVerbose))
	assert.False(t, shouldEmitCommandLevel(commandruntime.OutputLevelDebug))

	Global.Verbose = false
	Global.Debug = true
	assert.True(t, shouldEmitCommandLevel(commandruntime.OutputLevelVerbose))
	assert.True(t, shouldEmitCommandLevel(commandruntime.OutputLevelDebug))

	Global.Quiet = true
	assert.False(t, shouldEmitCommandLevel(commandruntime.OutputLevelInfo))
	assert.False(t, shouldEmitCommandLevel(commandruntime.OutputLevelVerbose))
	assert.False(t, shouldEmitCommandLevel(commandruntime.OutputLevelDebug))
}

func TestNormalizeReviewTokens(t *testing.T) {
	tokens := internalreview.NormalizeTokens([]string{" alice ", "", "bob", "alice"})
	assert.Equal(t, []string{"alice", "bob", "alice"}, tokens)
}

func TestIssueMatchesAllowedTypes(t *testing.T) {
	assert.True(t, internalstart.IssueMatchesAllowedTypes(tracker.IssueTypeTask, nil))
	assert.True(t, internalstart.IssueMatchesAllowedTypes(tracker.IssueTypeBug, []tracker.IssueType{tracker.IssueTypeBug}))
	assert.False(t, internalstart.IssueMatchesAllowedTypes(tracker.IssueTypeStory, []tracker.IssueType{tracker.IssueTypeBug}))
}

func TestGetIssueTypeIcon(t *testing.T) {
	assert.NotEmpty(t, getIssueTypeIcon(tracker.IssueTypeStory))
	assert.NotEmpty(t, getIssueTypeIcon(tracker.IssueTypeBug))
	assert.NotEmpty(t, getIssueTypeIcon(tracker.IssueTypeTask))
	assert.NotEmpty(t, getIssueTypeIcon(tracker.IssueTypeSubTask))
	assert.NotEmpty(t, getIssueTypeIcon(tracker.IssueTypeEpic))
}

func TestEnsureJiraRootConfiguredWhenAlreadySet(t *testing.T) {
	orig := fotingoConfig.GetString("jira.root")
	fotingoConfig.Set("jira.root", "https://example.atlassian.net")
	t.Cleanup(func() { fotingoConfig.Set("jira.root", orig) })

	err := ensureJiraRootConfigured()
	require.NoError(t, err)
}
