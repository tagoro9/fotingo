package commands

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// updateGolden is a flag to update golden files when running tests with -update
var updateGolden = flag.Bool("update", false, "update golden files")

// goldenDir is the directory containing golden files
const goldenDir = "testdata"

// goldenCompare compares the actual output against the golden file.
// If the -update flag is set, it updates the golden file instead.
func goldenCompare(t *testing.T, goldenFile string, actual []byte) {
	t.Helper()

	goldenPath := filepath.Join(goldenDir, goldenFile)

	if *updateGolden {
		// Create the testdata directory if it doesn't exist
		err := os.MkdirAll(goldenDir, 0755)
		require.NoError(t, err, "failed to create testdata directory")

		err = os.WriteFile(goldenPath, actual, 0644)
		require.NoError(t, err, "failed to update golden file")
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if os.IsNotExist(err) {
		t.Fatalf("Golden file %s does not exist. Run with -update flag to create it.", goldenPath)
	}
	require.NoError(t, err, "failed to read golden file")

	// Normalize line endings for comparison
	expectedNorm := normalizeLineEndings(string(expected))
	actualNorm := normalizeLineEndings(string(actual))

	if expectedNorm != actualNorm {
		t.Errorf("Output does not match golden file %s.\nExpected:\n%s\n\nActual:\n%s", goldenPath, expectedNorm, actualNorm)
	}
}

// normalizeLineEndings converts all line endings to \n for comparison
func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// TestHelpOutput tests the --help output for all commands and compares against golden files
func TestHelpOutput(t *testing.T) {
	// Save original values and restore after test
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	tests := []struct {
		name       string
		args       []string
		goldenFile string
	}{
		{
			name:       "root help",
			args:       []string{"fotingo", "--help"},
			goldenFile: "help_root.golden",
		},
		{
			name:       "start help",
			args:       []string{"fotingo", "start", "--help"},
			goldenFile: "help_start.golden",
		},
		{
			name:       "review help",
			args:       []string{"fotingo", "review", "--help"},
			goldenFile: "help_review.golden",
		},
		{
			name:       "search help",
			args:       []string{"fotingo", "search", "--help"},
			goldenFile: "help_search.golden",
		},
		{
			name:       "open help",
			args:       []string{"fotingo", "open", "--help"},
			goldenFile: "help_open.golden",
		},
		{
			name:       "inspect help",
			args:       []string{"fotingo", "inspect", "--help"},
			goldenFile: "help_inspect.golden",
		},
		{
			name:       "completion help",
			args:       []string{"fotingo", "completion", "--help"},
			goldenFile: "help_completion.golden",
		},
		{
			name:       "config help",
			args:       []string{"fotingo", "config", "--help"},
			goldenFile: "help_config.golden",
		},
		{
			name:       "ai help",
			args:       []string{"fotingo", "ai", "--help"},
			goldenFile: "help_ai.golden",
		},
		{
			name:       "cache help",
			args:       []string{"fotingo", "cache", "--help"},
			goldenFile: "help_cache.golden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset command for clean execution
			resetCommandStateForExecute(Fotingo)
			buf := new(bytes.Buffer)
			Fotingo.SetOut(buf)
			Fotingo.SetErr(buf)
			Fotingo.SetArgs(tt.args[1:]) // Skip the program name

			// Execute and capture output
			err := Fotingo.Execute()
			require.NoError(t, err)

			goldenCompare(t, tt.goldenFile, buf.Bytes())
		})
	}
}

// TestJSONOutputGoldenFiles tests JSON output against golden files
func TestJSONOutputGoldenFiles(t *testing.T) {
	t.Parallel()

	t.Run("start success json", func(t *testing.T) {
		t.Parallel()

		output := StartOutput{
			Success: true,
			Issue: &IssueInfo{
				Key:     "PROJ-123",
				Summary: "Fix login bug",
				Status:  "In Progress",
				Type:    "Bug",
				URL:     "https://jira.example.com/browse/PROJ-123",
			},
			Branch: &StartBranchInfo{
				Name:    "feature/PROJ-123-fix-login-bug",
				Created: true,
			},
		}

		data, err := json.MarshalIndent(output, "", "  ")
		require.NoError(t, err)

		goldenCompare(t, "json_start_success.golden", append(data, '\n'))
	})

	t.Run("start error json", func(t *testing.T) {
		t.Parallel()

		output := StartOutput{
			Success: false,
			Error:   "failed to initialize Jira client: authentication required",
		}

		data, err := json.MarshalIndent(output, "", "  ")
		require.NoError(t, err)

		goldenCompare(t, "json_start_error.golden", append(data, '\n'))
	})

	t.Run("review success json", func(t *testing.T) {
		t.Parallel()

		output := ReviewOutput{
			Success: true,
			PullRequest: &PullRequestInfo{
				Number: 42,
				URL:    "https://github.com/owner/repo/pull/42",
				Title:  "[PROJ-123] Fix login bug",
				Draft:  false,
				State:  "open",
			},
			Issue: &IssueInfo{
				Key:     "PROJ-123",
				Summary: "Fix login bug",
				Status:  "In Review",
				Type:    "Bug",
				URL:     "https://jira.example.com/browse/PROJ-123",
			},
			Labels:    []string{"bug", "priority"},
			Reviewers: []string{"alice", "bob"},
		}

		data, err := json.MarshalIndent(output, "", "  ")
		require.NoError(t, err)

		goldenCompare(t, "json_review_success.golden", append(data, '\n'))
	})

	t.Run("review existed json", func(t *testing.T) {
		t.Parallel()

		output := ReviewOutput{
			Success: true,
			PullRequest: &PullRequestInfo{
				Number: 42,
				URL:    "https://github.com/owner/repo/pull/42",
				Title:  "[PROJ-123] Fix login bug",
				Draft:  false,
				State:  "open",
			},
			Existed: true,
		}

		data, err := json.MarshalIndent(output, "", "  ")
		require.NoError(t, err)

		goldenCompare(t, "json_review_existed.golden", append(data, '\n'))
	})

	t.Run("open success json", func(t *testing.T) {
		t.Parallel()

		output := OpenOutput{
			Success: true,
			Target:  "pr",
			URL:     "https://github.com/owner/repo/pull/42",
			Opened:  true,
		}

		data, err := json.MarshalIndent(output, "", "  ")
		require.NoError(t, err)

		goldenCompare(t, "json_open_success.golden", append(data, '\n'))
	})

	t.Run("error json", func(t *testing.T) {
		t.Parallel()

		output := ErrorOutput{
			Error: "failed to create branch",
			Code:  4,
			Type:  "Git error",
		}

		data, err := json.MarshalIndent(output, "", "  ")
		require.NoError(t, err)

		goldenCompare(t, "json_error.golden", append(data, '\n'))
	})
}
