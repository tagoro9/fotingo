package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/jira"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// captureStdout redirects os.Stdout to a buffer, runs fn, then restores it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	origStdout := os.Stdout
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.String()
}

// saveGlobalFlags saves Global flags state and returns a restore function.
func saveGlobalFlags() func() {
	saved := Global
	return func() {
		Global = saved
	}
}

func setDefaultOutputFlags(t *testing.T) {
	t.Helper()

	restore := saveGlobalFlags()
	t.Cleanup(restore)

	Global.JSON = false
	Global.Quiet = false
	Global.Verbose = false
	Global.Debug = false
}

func withDefaultReviewTemplateResolver(t *testing.T) {
	t.Helper()
	resetReviewFlags()
	t.Cleanup(resetReviewFlags)

	originalResolver := resolveReviewTemplateFn
	resolveReviewTemplateFn = func() string { return defaultPRTemplate }
	t.Cleanup(func() {
		resolveReviewTemplateFn = originalResolver
	})
}

func TestRunWithSharedShell_PropagatesError(t *testing.T) {
	setDefaultOutputFlags(t)
	Global.Quiet = true

	expectedErr := errors.New("boom")
	err := runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
		out.InfoRaw(commandruntime.LogEmojiProgress, "phase 1")
		return expectedErr
	})

	assert.ErrorIs(t, err, expectedErr)
}

func TestRunWithSharedShell_MultipleMessages_NoDeadlock(t *testing.T) {
	setDefaultOutputFlags(t)
	Global.Quiet = true

	err := runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
		out.InfoRaw(commandruntime.LogEmojiProgress, "phase 1")
		out.InfoRaw(commandruntime.LogEmojiProgress, "phase 2")
		out.InfoRaw(commandruntime.LogEmojiProgress, "phase 3")
		return nil
	})

	assert.NoError(t, err)
}

type mockTerminalController struct {
	releaseErr   error
	restoreErr   error
	releaseCalls int
	restoreCalls int
}

func (m *mockTerminalController) ReleaseTerminal() error {
	m.releaseCalls++
	return m.releaseErr
}

func (m *mockTerminalController) RestoreTerminal() error {
	m.restoreCalls++
	return m.restoreErr
}

func TestRunInteractiveProcessWithControllerHandoff_Success(t *testing.T) {
	controller := &mockTerminalController{}
	called := false

	err := runInteractiveProcessWithControllerHandoff(controller, func() error {
		called = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 1, controller.releaseCalls)
	assert.Equal(t, 1, controller.restoreCalls)
}

func TestRunInteractiveProcessWithControllerHandoff_ReleaseFailure(t *testing.T) {
	controller := &mockTerminalController{releaseErr: errors.New("release failed")}
	called := false

	err := runInteractiveProcessWithControllerHandoff(controller, func() error {
		called = true
		return nil
	})

	require.Error(t, err)
	assert.False(t, called, "process should not run when release fails")
	assert.Equal(t, 1, controller.releaseCalls)
	assert.Equal(t, 0, controller.restoreCalls)

	var handoffErr *commandruntime.HandoffError
	require.ErrorAs(t, err, &handoffErr)
	assert.Equal(t, commandruntime.HandoffStageRelease, handoffErr.Stage)
}

func TestRunInteractiveProcessWithControllerHandoff_RestoreFailure(t *testing.T) {
	controller := &mockTerminalController{restoreErr: errors.New("restore failed")}

	err := runInteractiveProcessWithControllerHandoff(controller, func() error {
		return nil
	})

	require.Error(t, err)
	assert.Equal(t, 1, controller.releaseCalls)
	assert.Equal(t, 1, controller.restoreCalls)

	var handoffErr *commandruntime.HandoffError
	require.ErrorAs(t, err, &handoffErr)
	assert.Equal(t, commandruntime.HandoffStageRestore, handoffErr.Stage)
}

func TestRunInteractiveProcessWithControllerHandoff_ProcessAndRestoreFailure(t *testing.T) {
	controller := &mockTerminalController{restoreErr: errors.New("restore failed")}
	processErr := errors.New("editor failed")

	err := runInteractiveProcessWithControllerHandoff(controller, func() error {
		return processErr
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, processErr)

	var handoffErr *commandruntime.HandoffError
	require.ErrorAs(t, err, &handoffErr)
	assert.Equal(t, commandruntime.HandoffStageRestore, handoffErr.Stage)
}

func TestRunInteractiveProcessWithControllerHandoff_Reentrant(t *testing.T) {
	controller := &mockTerminalController{}
	done := make(chan error, 1)

	go func() {
		done <- runInteractiveProcessWithControllerHandoff(controller, func() error {
			return runInteractiveProcessWithControllerHandoff(controller, func() error {
				return nil
			})
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reentrant handoff")
	}

	assert.Equal(t, 1, controller.releaseCalls)
	assert.Equal(t, 1, controller.restoreCalls)
}

func TestFormatCommandError_DefaultIncludesHint(t *testing.T) {
	setDefaultOutputFlags(t)

	msg := formatCommandError(errors.New("failed to create release"))
	assert.Contains(t, msg, "failed to create release")
	assert.Contains(t, msg, "rerun with --debug for more details")
}

func TestFormatCommandError_DebugOmitsHint(t *testing.T) {
	setDefaultOutputFlags(t)
	Global.Debug = true

	msg := formatCommandError(errors.New("failed to create release"))
	assert.Contains(t, msg, "failed to create release")
	assert.NotContains(t, msg, "rerun with --debug for more details")
}

func TestFormatCommandError_DefaultUsesTopLevelExitMessage(t *testing.T) {
	setDefaultOutputFlags(t)

	msg := formatCommandError(fterrors.WrapGitHubError("friendly summary", errors.New("low-level details")))
	assert.Contains(t, msg, "friendly summary")
	assert.NotContains(t, msg, "low-level details")
}

func TestSummarizeCommandError_DefaultUsesTopLevelExitMessage(t *testing.T) {
	msg := summarizeCommandError(fterrors.WrapGitHubError("friendly summary", errors.New("low-level details")))
	assert.Equal(t, "friendly summary", msg)
}

func TestHumanizeDuration(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "125ms", commandruntime.HumanizeDuration(125*time.Millisecond))
	assert.Equal(t, "2.5s", commandruntime.HumanizeDuration(2500*time.Millisecond))
	assert.Equal(t, "2m 5s", commandruntime.HumanizeDuration(125*time.Second))
	assert.Equal(t, "1h 5m", commandruntime.HumanizeDuration(65*time.Minute))
}

func TestMessageSeverityDetection(t *testing.T) {
	t.Parallel()

	assert.True(t, commandruntime.IsErrorMessage("💥 boom"))
	assert.True(t, commandruntime.IsErrorMessage("network error while pushing"))
	assert.True(t, commandruntime.IsWarningMessage("⚠ warning"))
	assert.True(t, commandruntime.IsWarningMessage("warning: fallback to default editor"))
	assert.False(t, commandruntime.IsErrorMessage("all good"))
	assert.False(t, commandruntime.IsWarningMessage("all good"))
}

// ---------------------------------------------------------------------------
// commandModel TUI tests (start)
// ---------------------------------------------------------------------------

func TestOutputJSON_WritesValidJSON(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()

	data := StartOutput{
		Success: true,
		Issue: &IssueInfo{
			Key:     "TEST-1",
			Summary: "Some issue",
			Status:  "In Progress",
			Type:    "Bug",
		},
	}

	output := captureStdout(t, func() {
		OutputJSON(data)
	})

	var decoded StartOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err, "OutputJSON should produce valid JSON")
	assert.True(t, decoded.Success)
	assert.Equal(t, "TEST-1", decoded.Issue.Key)
}

func TestOutputJSONError_IncludesCodeAndType(t *testing.T) {
	testErr := fmt.Errorf("something went wrong")

	output := captureStdout(t, func() {
		OutputJSONError(testErr)
	})

	var decoded ErrorOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "something went wrong", decoded.Error)
	assert.Equal(t, 1, decoded.Code) // ExitGeneral for plain errors
	assert.Equal(t, "General error", decoded.Type)
}

func TestNewJSONOutput_Write(t *testing.T) {
	output := captureStdout(t, func() {
		j := NewJSONOutput()
		_ = j.Write(map[string]string{"key": "value"})
	})

	var decoded map[string]string
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "value", decoded["key"])
}

func TestNewJSONOutput_WriteError(t *testing.T) {
	testErr := fmt.Errorf("bad thing")

	output := captureStdout(t, func() {
		j := NewJSONOutput()
		_ = j.WriteError(testErr)
	})

	var decoded ErrorOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "bad thing", decoded.Error)
}

func TestNewJSONOutput_WriteSuccess(t *testing.T) {
	output := captureStdout(t, func() {
		j := NewJSONOutput()
		_ = j.WriteSuccess(map[string]bool{"ok": true})
	})

	var decoded map[string]bool
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)
	assert.True(t, decoded["ok"])
}

// ---------------------------------------------------------------------------
// Output helpers: ShouldOutputJSON / ShouldSuppressOutput
// ---------------------------------------------------------------------------

func TestShouldOutputJSON_DefaultFalse(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()

	Global.JSON = false
	assert.False(t, ShouldOutputJSON())
}

func TestShouldSuppressOutput_Combinations(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()

	tests := []struct {
		json, quiet, expected bool
	}{
		{false, false, false},
		{true, false, true},
		{false, true, true},
		{true, true, true},
	}

	for _, tt := range tests {
		Global.JSON = tt.json
		Global.Quiet = tt.quiet
		assert.Equal(t, tt.expected, ShouldSuppressOutput())
	}
}

// ---------------------------------------------------------------------------
// outputOpenJSON
// ---------------------------------------------------------------------------

func TestOutputOpenJSON_Success(t *testing.T) {
	output := captureStdout(t, func() {
		err := outputOpenJSON("repo", "https://github.com/test/repo", nil)
		assert.NoError(t, err)
	})

	var decoded OpenOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)
	assert.True(t, decoded.Success)
	assert.Equal(t, "repo", decoded.Target)
	assert.Equal(t, "https://github.com/test/repo", decoded.URL)
	assert.False(t, decoded.Opened, "JSON mode should not open browser")
}

func TestOutputOpenJSON_Error(t *testing.T) {
	output := captureStdout(t, func() {
		err := outputOpenJSON("pr", "", fmt.Errorf("no PR found"))
		assert.Error(t, err)
	})

	var decoded OpenOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)
	assert.False(t, decoded.Success)
	assert.Equal(t, "pr", decoded.Target)
	assert.Equal(t, "no PR found", decoded.Error)
}

func TestOutputOpenJSON_AllTargets(t *testing.T) {
	t.Parallel()

	targets := []string{"branch", "issue", "pr", "repo"}

	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			output := OpenOutput{
				Success: true,
				Target:  target,
				URL:     "https://example.com/" + target,
			}

			data, err := json.Marshal(output)
			require.NoError(t, err)

			var decoded OpenOutput
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, target, decoded.Target)
		})
	}
}

// ---------------------------------------------------------------------------
// outputStartJSON
// ---------------------------------------------------------------------------

func TestOutputStartJSON_SuccessStruct(t *testing.T) {
	t.Parallel()

	// Test the StartOutput struct serialization directly rather than calling
	// outputStartJSON, because it internally calls jira.New() which requires auth.
	output := StartOutput{
		Success: true,
		Issue: &IssueInfo{
			Key:     "TEST-1",
			Summary: "Fix bug",
			Status:  "In Progress",
			Type:    "Bug",
			URL:     "https://jira.example.com/browse/TEST-1",
		},
		Branch: &StartBranchInfo{
			Name:         "b/TEST-1_fix_bug",
			Created:      true,
			WorktreePath: "/tmp/fotingo-b-test-1_fix_bug",
		},
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	var decoded StartOutput
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.True(t, decoded.Success)
	assert.Equal(t, "TEST-1", decoded.Issue.Key)
	assert.Equal(t, "b/TEST-1_fix_bug", decoded.Branch.Name)
	assert.True(t, decoded.Branch.Created)
	assert.Equal(t, "/tmp/fotingo-b-test-1_fix_bug", decoded.Branch.WorktreePath)
}

func TestOutputStartJSON_Error(t *testing.T) {
	result := startResult{
		err: fmt.Errorf("jira connection failed"),
	}

	output := captureStdout(t, func() {
		err := outputStartJSON(result)
		assert.Error(t, err)
	})

	var decoded StartOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)
	assert.False(t, decoded.Success)
	assert.Equal(t, "jira connection failed", decoded.Error)
	assert.Nil(t, decoded.Issue)
	assert.Nil(t, decoded.Branch)
}

func TestOutputStartJSON_NoBranchStruct(t *testing.T) {
	t.Parallel()

	// Test the struct serialization for a start without branch.
	output := StartOutput{
		Success: true,
		Issue: &IssueInfo{
			Key:     "TEST-1",
			Summary: "Task",
			Status:  "In Progress",
			Type:    "Task",
		},
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	var decoded StartOutput
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.True(t, decoded.Success)
	assert.NotNil(t, decoded.Issue)
	assert.Nil(t, decoded.Branch, "branch should be nil when no branch was created")
}

// ---------------------------------------------------------------------------
// outputReviewJSON
// ---------------------------------------------------------------------------

func TestOutputReviewJSON_Success(t *testing.T) {
	// Test the output struct serialization directly since constructing
	// a full reviewResult requires a github.PullRequest with internal state.
	output := ReviewOutput{
		Success: true,
		PullRequest: &PullRequestInfo{
			Number: 42,
			URL:    "https://github.com/owner/repo/pull/42",
			Title:  "[TEST-1] Fix bug",
			State:  "open",
		},
		Issue: &IssueInfo{
			Key:     "TEST-1",
			Summary: "Fix bug",
			Status:  "In Review",
			Type:    "Bug",
		},
		Labels:    []string{"bug"},
		Reviewers: []string{"alice"},
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	var decoded ReviewOutput
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.Success)
	assert.Equal(t, 42, decoded.PullRequest.Number)
	assert.Equal(t, "TEST-1", decoded.Issue.Key)
	assert.Equal(t, []string{"bug"}, decoded.Labels)
	assert.Equal(t, []string{"alice"}, decoded.Reviewers)
	assert.False(t, decoded.Existed)
}

func TestOutputReviewJSON_Error(t *testing.T) {
	result := reviewResult{
		err: fmt.Errorf("push failed"),
	}

	output := captureStdout(t, func() {
		err := outputReviewJSON(result)
		assert.Error(t, err)
	})

	var decoded ReviewOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)
	assert.False(t, decoded.Success)
	assert.Equal(t, "push failed", decoded.Error)
}

func TestOutputReviewJSON_Existed(t *testing.T) {
	output := ReviewOutput{
		Success: true,
		PullRequest: &PullRequestInfo{
			Number: 10,
			URL:    "https://github.com/o/r/pull/10",
			State:  "existing",
		},
		Existed: true,
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	var decoded ReviewOutput
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.True(t, decoded.Existed)
	assert.Equal(t, "existing", decoded.PullRequest.State)
}

// ---------------------------------------------------------------------------
// JSON output structs serialization
// ---------------------------------------------------------------------------

func TestStartOutput_OmitsEmptyFields(t *testing.T) {
	t.Parallel()

	output := StartOutput{
		Success: false,
		Error:   "something failed",
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	// Issue and Branch should be omitted
	assert.NotContains(t, string(data), "issue")
	assert.NotContains(t, string(data), "branch")
	assert.Contains(t, string(data), "error")
}

func TestReviewOutput_OmitsEmptyFields(t *testing.T) {
	t.Parallel()

	output := ReviewOutput{
		Success: true,
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "pullRequest")
	assert.NotContains(t, string(data), "issue")
	assert.NotContains(t, string(data), "labels")
	assert.NotContains(t, string(data), "reviewers")
	assert.NotContains(t, string(data), "error")
	assert.NotContains(t, string(data), "existed")
}

func TestOpenOutput_OmitsEmptyFields(t *testing.T) {
	t.Parallel()

	output := OpenOutput{
		Success: false,
		Target:  "pr",
		Error:   "not found",
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "url")
	assert.NotContains(t, string(data), "opened")
	assert.Contains(t, string(data), "error")
}

func TestErrorOutput_OmitsZeroCode(t *testing.T) {
	t.Parallel()

	output := ErrorOutput{
		Error: "generic error",
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	// Code 0 and empty Type should be omitted
	assert.NotContains(t, string(data), "code")
	assert.NotContains(t, string(data), "type")
}

func TestPullRequestInfo_Serialization(t *testing.T) {
	t.Parallel()

	pr := PullRequestInfo{
		Number: 99,
		URL:    "https://github.com/o/r/pull/99",
		Title:  "My PR",
		Draft:  true,
		State:  "open",
	}

	data, err := json.Marshal(pr)
	require.NoError(t, err)

	var decoded PullRequestInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 99, decoded.Number)
	assert.True(t, decoded.Draft)
	assert.Equal(t, "My PR", decoded.Title)
}

// ---------------------------------------------------------------------------
// formatIssuesByCategory
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// buildPRBody
// ---------------------------------------------------------------------------

func TestBuildPRBody_Default(t *testing.T) {
	withDefaultReviewTemplateResolver(t)

	issue := &jira.Issue{
		Key:         "TEST-1",
		Summary:     "Fix the thing",
		Description: "Detailed description",
		Status:      "In Progress",
		Type:        "Bug",
	}

	body := buildPRBody("f/TEST-1_fix_the_thing", issue, nil)
	assert.Contains(t, body, "**Summary**")
	assert.Contains(t, body, "Fix the thing")
	assert.Contains(t, body, "**Description**")
	assert.Contains(t, body, "Detailed description")
	assert.Contains(t, body, "Fixes TEST-1")
	assert.Contains(t, body, "**Changes**")
	assert.Contains(t, body, "TEST-1")
	assert.Contains(t, body, "🚀 PR created with [fotingo](https://github.com/tagoro9/fotingo)")
}

func TestBuildPRBody_NilIssue(t *testing.T) {
	withDefaultReviewTemplateResolver(t)

	body := buildPRBody("my-branch", nil, nil)
	assert.Contains(t, body, "my-branch")
	assert.Contains(t, body, "**Description**")
	assert.NotContains(t, body, "{summary}")
	assert.NotContains(t, body, "{description}")
	assert.NotContains(t, body, "{fixedIssues}")
}

func TestBuildPRBody_ContainsChangesSection(t *testing.T) {
	withDefaultReviewTemplateResolver(t)

	body := buildPRBody("branch", nil, nil)
	assert.Contains(t, body, "**Changes**")
}

// ---------------------------------------------------------------------------
// buildReleaseNotes
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// defaultPRTemplate and defaultReleaseTemplate
// ---------------------------------------------------------------------------

func TestDefaultPRTemplate_ContainsRequiredSections(t *testing.T) {
	t.Parallel()

	assert.Contains(t, defaultPRTemplate, "**Summary**")
	assert.Contains(t, defaultPRTemplate, "<!-- fotingo:start summary -->")
	assert.Contains(t, defaultPRTemplate, "<!-- fotingo:start description -->")
	assert.Contains(t, defaultPRTemplate, "<!-- fotingo:start fixed-issues -->")
	assert.Contains(t, defaultPRTemplate, "<!-- fotingo:start changes -->")
	assert.NotContains(t, defaultPRTemplate, "{summary}")
	assert.NotContains(t, defaultPRTemplate, "{description}")
	assert.NotContains(t, defaultPRTemplate, "{fixedIssues}")
	assert.Contains(t, defaultPRTemplate, "**Description**")
	assert.Contains(t, defaultPRTemplate, "**Changes**")
	assert.NotContains(t, defaultPRTemplate, "{changes}")
	assert.Contains(t, defaultPRTemplate, "{fotingo.banner}")
}

func TestDefaultReleaseTemplate_ContainsRequiredPlaceholders(t *testing.T) {
	t.Parallel()

	assert.Contains(t, defaultReleaseTemplate, "{version}")
	assert.Contains(t, defaultReleaseTemplate, "{fixedIssuesByCategory}")
	assert.Contains(t, defaultReleaseTemplate, "{fotingo.banner}")
}

// ---------------------------------------------------------------------------
// extractIssueIDsFromCommits (additional edge cases)
// ---------------------------------------------------------------------------

func TestExtractIssueIDsFromCommits_Empty(t *testing.T) {
	t.Parallel()

	result := extractIssueIDsFromCommits(nil)
	assert.Nil(t, result)
}

func TestExtractIssueIDsFromCommits_PreservesOrder(t *testing.T) {
	t.Parallel()

	commits := []git.Commit{
		{Message: "BETA-999 first seen"},
		{Message: "ALPHA-1 second seen"},
	}

	result := extractIssueIDsFromCommits(commits)
	require.Len(t, result, 2)
	assert.Equal(t, "BETA-999", result[0])
	assert.Equal(t, "ALPHA-1", result[1])
}

// ---------------------------------------------------------------------------
// inspect: outputJSON and outputError
// ---------------------------------------------------------------------------

func TestOutputJSON_InspectOutput(t *testing.T) {
	data := InspectOutput{
		Branch: &BranchInfo{
			Name:          "f/TEST-1_my_branch",
			IssueID:       "TEST-1",
			DefaultBranch: "main",
		},
		PullRequest: &InspectPRInfo{
			Number:      12,
			Title:       "Inspect output PR",
			Description: "Inspect output PR body",
			URL:         "https://github.com/test/repo/pull/12",
		},
		IssueIDs: []string{"TEST-1"},
	}

	output := captureStdout(t, func() {
		outputJSON(data)
	})

	var decoded InspectOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "f/TEST-1_my_branch", decoded.Branch.Name)
	assert.Equal(t, "TEST-1", decoded.Branch.IssueID)
	require.NotNil(t, decoded.PullRequest)
	assert.Equal(t, 12, decoded.PullRequest.Number)
	assert.Equal(t, "Inspect output PR body", decoded.PullRequest.Description)
	assert.Equal(t, []string{"TEST-1"}, decoded.IssueIDs)
}

func TestOutputError_JSON(t *testing.T) {
	output := captureStdout(t, func() {
		outputError(fmt.Errorf("issue not found"))
	})

	var decoded struct {
		Error string `json:"error"`
	}
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "issue not found", decoded.Error)
}

// ---------------------------------------------------------------------------
// startResult / reviewResult struct behavior
// ---------------------------------------------------------------------------

func TestStartResult_Struct(t *testing.T) {
	t.Parallel()

	result := startResult{
		issue: &jira.Issue{
			Key:     "TEST-1",
			Summary: "Do thing",
		},
		branchName: "f/TEST-1_do_thing",
		created:    true,
		err:        nil,
	}

	assert.Nil(t, result.err)
	assert.Equal(t, "TEST-1", result.issue.Key)
	assert.True(t, result.created)
}

func TestReviewResult_Struct(t *testing.T) {
	t.Parallel()

	result := reviewResult{
		labels:    []string{"bug"},
		reviewers: []string{"alice", "bob"},
		existed:   true,
		err:       nil,
	}

	assert.True(t, result.existed)
	assert.Equal(t, []string{"bug"}, result.labels)
	assert.Nil(t, result.err)
}

// ---------------------------------------------------------------------------
// Inspect: InspectOutput JSON round-trip
// ---------------------------------------------------------------------------

func TestInspectOutput_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := InspectOutput{
		Branch: &BranchInfo{
			Name:          "f/PROJ-42_awesome_feature",
			IssueID:       "PROJ-42",
			DefaultBranch: "main",
		},
		Issue: &IssueInfo{
			Key:         "PROJ-42",
			Summary:     "Awesome feature",
			Description: "Implement awesome feature end-to-end.",
			Status:      "In Progress",
			Type:        "Story",
			URL:         "https://jira.example.com/browse/PROJ-42",
		},
		PullRequest: &InspectPRInfo{
			Number:      77,
			Title:       "Inspect PR context",
			Description: "Inspect PR context body",
			URL:         "https://github.com/o/r/pull/77",
		},
		IssueIDs: []string{"PROJ-42", "PROJ-43"},
		Commits: []CommitInfo{
			{Hash: "abc123", Message: "feat: PROJ-42 initial work", Author: "dev"},
			{Hash: "def456", Message: "fix: PROJ-43 related fix", Author: "dev"},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded InspectOutput
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Branch.Name, decoded.Branch.Name)
	assert.Equal(t, original.Branch.IssueID, decoded.Branch.IssueID)
	assert.Equal(t, original.Issue.Key, decoded.Issue.Key)
	assert.Equal(t, original.Issue.Description, decoded.Issue.Description)
	require.NotNil(t, decoded.PullRequest)
	assert.Equal(t, original.PullRequest.Number, decoded.PullRequest.Number)
	assert.Equal(t, original.PullRequest.Description, decoded.PullRequest.Description)
	assert.Equal(t, original.IssueIDs, decoded.IssueIDs)
	assert.Len(t, decoded.Commits, 2)
	assert.Equal(t, "abc123", decoded.Commits[0].Hash)
}

// ---------------------------------------------------------------------------
// Jira Issue helpers (from client.go - tested via commands package)
// ---------------------------------------------------------------------------

func TestIssue_ShortName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		typeName string
		want     string
	}{
		{"bug", "Bug", "b"},
		{"story", "Story", "f"},
		{"feature", "Feature", "f"},
		{"task", "Task", "c"},
		{"spike", "Spike", "s"},
		{"tech debt", "Tech Debt", "d"},
		{"sub-task", "Sub-task", "f"},
		{"unknown", "CustomType", "f"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			issue := &jira.Issue{Type: tt.typeName}
			assert.Equal(t, tt.want, issue.ShortName())
		})
	}
}

func TestIssue_SanitizedSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		summary string
		want    string
	}{
		{"spaces to underscores", "Fix the bug", "fix_the_bug"},
		{"special chars removed", "Fix 'the' bug", "fix_the_bug"},
		{"parentheses to underscores", "Fix (the) bug", "fix__the__bug"},
		{"truncate at 72", strings.Repeat("a", 100), strings.Repeat("a", 72)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			issue := &jira.Issue{Summary: tt.summary}
			assert.Equal(t, tt.want, issue.SanitizedSummary())
		})
	}
}

// ---------------------------------------------------------------------------
// GlobalFlags struct
// ---------------------------------------------------------------------------

func TestGlobalFlags_DefaultValues(t *testing.T) {
	// Global should have zero-value defaults when not modified
	defaults := GlobalFlags{}
	assert.Equal(t, "", defaults.Branch)
	assert.False(t, defaults.Yes)
	assert.False(t, defaults.JSON)
	assert.False(t, defaults.Quiet)
	assert.False(t, defaults.NoColor)
}

// ---------------------------------------------------------------------------
// TUI model Update with spinner.TickMsg
// ---------------------------------------------------------------------------

func TestOutputReviewJSON_NilPR(t *testing.T) {
	result := reviewResult{
		err: nil,
	}

	output := captureStdout(t, func() {
		err := outputReviewJSON(result)
		assert.NoError(t, err)
	})

	var decoded ReviewOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)
	assert.True(t, decoded.Success)
	assert.Nil(t, decoded.PullRequest)
	assert.Nil(t, decoded.Issue)
}

// ---------------------------------------------------------------------------
// outputStartJSON error path
// ---------------------------------------------------------------------------

func TestOutputStartJSON_ErrorPath(t *testing.T) {
	result := startResult{
		err: fmt.Errorf("connection refused"),
	}

	output := captureStdout(t, func() {
		err := outputStartJSON(result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection refused")
	})

	var decoded StartOutput
	err := json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)
	assert.False(t, decoded.Success)
	assert.Equal(t, "connection refused", decoded.Error)
}

// ---------------------------------------------------------------------------
// buildPRBody with Jira client providing URL
// ---------------------------------------------------------------------------

func TestBuildPRBody_WithIssueAndNoJiraClient(t *testing.T) {
	withDefaultReviewTemplateResolver(t)

	issue := &jira.Issue{
		Key:         "PROJ-99",
		Summary:     "Add feature",
		Description: "Feature details",
		Status:      "In Progress",
		Type:        "Story",
	}

	body := buildPRBody("f/PROJ-99_add_feature", issue, nil)
	// Issue key and summary should be rendered
	assert.Contains(t, body, "PROJ-99")
	assert.Contains(t, body, "Add feature")
	assert.Contains(t, body, "Feature details")
	assert.Contains(t, body, "Fixes PROJ-99")
	assert.NotContains(t, body, "{summary}")
	assert.NotContains(t, body, "{description}")
	assert.NotContains(t, body, "{fixedIssues}")
}

// ---------------------------------------------------------------------------
// Start command: accepts max 1 arg
// ---------------------------------------------------------------------------

func TestStartCommand_AcceptsNoArgs(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()

	// Verify the command accepts 0 or 1 args by checking Args validator
	err := startCmd.Args(startCmd, []string{})
	assert.NoError(t, err, "start should accept 0 args")

	err = startCmd.Args(startCmd, []string{"TEST-1"})
	assert.NoError(t, err, "start should accept 1 arg")

	err = startCmd.Args(startCmd, []string{"TEST-1", "TEST-2"})
	assert.Error(t, err, "start should reject 2 args")
}

// ---------------------------------------------------------------------------
// Open command: args validation
// ---------------------------------------------------------------------------

func TestOpenCommand_ArgsValidation(t *testing.T) {
	restore := saveGlobalFlags()
	defer restore()

	// Exactly 1 arg required
	err := openCmd.Args(openCmd, []string{})
	assert.Error(t, err, "open should reject 0 args")

	err = openCmd.Args(openCmd, []string{"repo"})
	assert.NoError(t, err, "open should accept 1 valid arg")

	err = openCmd.Args(openCmd, []string{"invalid"})
	assert.Error(t, err, "open should reject invalid arg")

	err = openCmd.Args(openCmd, []string{"repo", "branch"})
	assert.Error(t, err, "open should reject 2 args")
}

// ---------------------------------------------------------------------------
// Release command: args validation
// ---------------------------------------------------------------------------

func TestReleaseCommand_ArgsValidation(t *testing.T) {
	t.Parallel()

	err := releaseCmd.Args(releaseCmd, []string{})
	assert.Error(t, err, "release should reject 0 args")

	err = releaseCmd.Args(releaseCmd, []string{"v1.0.0"})
	assert.NoError(t, err, "release should accept 1 arg")

	err = releaseCmd.Args(releaseCmd, []string{"v1.0.0", "extra"})
	assert.Error(t, err, "release should reject 2 args")
}

// ---------------------------------------------------------------------------
// Completion command: args validation
// ---------------------------------------------------------------------------

func TestCompletionCommand_ArgsValidation(t *testing.T) {
	t.Parallel()

	err := completionCmd.Args(completionCmd, []string{})
	assert.Error(t, err, "completion should reject 0 args")

	err = completionCmd.Args(completionCmd, []string{"bash"})
	assert.NoError(t, err, "completion should accept 'bash'")

	err = completionCmd.Args(completionCmd, []string{"invalid"})
	assert.Error(t, err, "completion should reject invalid shell")
}

// ---------------------------------------------------------------------------
// Multiple status messages accumulate correctly in TUI models
// ---------------------------------------------------------------------------
