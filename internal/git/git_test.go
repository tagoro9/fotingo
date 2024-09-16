package git

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	gogit "github.com/go-git/go-git/v5"
	gogitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/tagoro9/fotingo/internal/config"
	"github.com/tagoro9/fotingo/internal/jira"
)

// GitTestSuite is the test suite for git package
type GitTestSuite struct {
	suite.Suite
	repo     *gogit.Repository
	config   *viper.Viper
	messages chan string
	git      *git
}

type testCredentialProvider struct{}

func (m *testCredentialProvider) GetCredentials(_ string) (*http.BasicAuth, error) {
	return &http.BasicAuth{Username: "test", Password: "test"}, nil
}

func (suite *GitTestSuite) SetupTest() {
	suite.T().Setenv("GIT_TERMINAL_PROMPT", "0")

	// Create a temporary directory for the repo
	var err error

	storage := memory.NewStorage()
	fs := memfs.New()
	suite.repo, err = gogit.Init(storage, fs)
	assert.NoError(suite.T(), err)

	// Create a remote
	_, err = suite.repo.CreateRemote(&gogitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://github.com/tagoro9/fotingo.git"},
	})
	assert.NoError(suite.T(), err)

	// Create a main branch
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	// Create a dummy file and commit it
	_, err = worktree.Filesystem.Create("dummy.txt")
	assert.NoError(suite.T(), err)

	_, err = worktree.Add("dummy.txt")
	assert.NoError(suite.T(), err)

	_, err = worktree.Commit("feat: add dummy file", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	assert.NoError(suite.T(), err)

	// Create main branch reference
	headRef, err := suite.repo.Head()
	assert.NoError(suite.T(), err)

	mainRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), headRef.Hash())
	err = suite.repo.Storer.SetReference(mainRef)
	assert.NoError(suite.T(), err)

	// Seed a cached remote tracking ref to keep unit tests offline.
	originMainRef := plumbing.NewHashReference(plumbing.NewRemoteReferenceName("origin", "main"), headRef.Hash())
	err = suite.repo.Storer.SetReference(originMainRef)
	assert.NoError(suite.T(), err)
	originHeadRef := plumbing.NewSymbolicReference(
		plumbing.ReferenceName("refs/remotes/origin/HEAD"),
		plumbing.NewRemoteReferenceName("origin", "main"),
	)
	err = suite.repo.Storer.SetReference(originHeadRef)
	assert.NoError(suite.T(), err)

	suite.config = config.NewDefaultConfig()

	suite.messages = make(chan string, 10)

	suite.git = &git{
		repo:                     suite.repo,
		ViperConfigurableService: &config.ViperConfigurableService{Config: suite.config, Prefix: "git"},
		messages:                 &suite.messages,
		credentialProvider:       &testCredentialProvider{},
	}
}

func (suite *GitTestSuite) TearDownTest() {
	// Drain messages channel
	for len(suite.messages) > 0 {
		<-suite.messages
	}
}

func (suite *GitTestSuite) TestGetCurrentBranch() {
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	err = worktree.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("f/TEST-123-test-branch"),
		Create: true,
	})
	assert.NoError(suite.T(), err)

	branch, err := suite.git.GetCurrentBranch()

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "f/TEST-123-test-branch", branch)
}

func (suite *GitTestSuite) TestGetCurrentBranch_DetachedHead() {
	// Get the current HEAD commit
	head, err := suite.repo.Head()
	assert.NoError(suite.T(), err)

	// Checkout specific commit to create detached HEAD state
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	err = worktree.Checkout(&gogit.CheckoutOptions{
		Hash: head.Hash(),
	})
	assert.NoError(suite.T(), err)

	// Try to get current branch when in detached HEAD state
	_, err = suite.git.GetCurrentBranch()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "HEAD is not pointing to a branch")
}

func (suite *GitTestSuite) TestGetCurrentBranch_EmptyRepository() {
	storage := memory.NewStorage()
	fs := memfs.New()
	emptyRepo, err := gogit.Init(storage, fs)
	assert.NoError(suite.T(), err)

	g := &git{
		repo:                     emptyRepo,
		ViperConfigurableService: &config.ViperConfigurableService{Config: config.NewDefaultConfig(), Prefix: "git"},
		messages:                 &suite.messages,
		credentialProvider:       &testCredentialProvider{},
	}

	_, err = g.GetCurrentBranch()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "repository has no commits yet")
}

func (suite *GitTestSuite) TestGetCommitsSince_EmptyRepository() {
	storage := memory.NewStorage()
	fs := memfs.New()
	emptyRepo, err := gogit.Init(storage, fs)
	assert.NoError(suite.T(), err)

	g := &git{
		repo:                     emptyRepo,
		ViperConfigurableService: &config.ViperConfigurableService{Config: config.NewDefaultConfig(), Prefix: "git"},
		messages:                 &suite.messages,
		credentialProvider:       &testCredentialProvider{},
	}

	_, err = g.GetCommitsSince("origin/main")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "repository has no commits yet")
}

func (suite *GitTestSuite) TestGetIssueId() {
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	err = worktree.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("f/TEST-123_test-branch"),
		Create: true,
	})
	assert.NoError(suite.T(), err)

	issueId, err := suite.git.GetIssueId()

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "TEST-123", issueId)
}

func (suite *GitTestSuite) TestGetIssueId_NormalizesLowercaseTemplateMatch() {
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	err = worktree.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("f/test-123_test-branch"),
		Create: true,
	})
	assert.NoError(suite.T(), err)

	issueID, err := suite.git.GetIssueId()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "TEST-123", issueID)
}

func (suite *GitTestSuite) TestGetIssueId_FallbackFromBranchToken() {
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	err = worktree.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(
			"f/devex-464_add_observability_tooling_from_public_mcp_into_the_internal_mcp_+_a_dumm",
		),
		Create: true,
	})
	assert.NoError(suite.T(), err)

	issueID, err := suite.git.GetIssueId()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "DEVEX-464", issueID)
}

func (suite *GitTestSuite) TestGetIssueId_NoBranchMatch() {
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	// Create a branch that doesn't match the template
	err = worktree.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("invalid-branch-name"),
		Create: true,
	})
	assert.NoError(suite.T(), err)

	_, err = suite.git.GetIssueId()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no issue id found in branch name")
	assert.Contains(suite.T(), err.Error(), "no matches found")
}

func (suite *GitTestSuite) TestGetIssueId_DetachedHead() {
	// Get the current HEAD commit
	head, err := suite.repo.Head()
	assert.NoError(suite.T(), err)

	// Checkout specific commit to create detached HEAD state
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	err = worktree.Checkout(&gogit.CheckoutOptions{
		Hash: head.Hash(),
	})
	assert.NoError(suite.T(), err)

	// Try to get issue ID when in detached HEAD state
	_, err = suite.git.GetIssueId()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "HEAD is not pointing to a branch")
}

func (suite *GitTestSuite) TestParseTemplateToRegex() {
	// Execute
	template := "{{.Issue.ShortName}}/{{.Issue.Info}}-{{.Issue.SanitizedSummary}}"
	regex := parseTemplateToRegex(template)

	// Assert
	assert.Contains(suite.T(), regex, "(?P<ISSUE_TYPE>\\w+)")
	assert.Contains(suite.T(), regex, "(?P<ISSUE_KEY>\\w+-\\d+)")
	assert.Contains(suite.T(), regex, "(?P<ISSUE_SANITIZED_SUMMARY>[\\w-]+)")
}

func (suite *GitTestSuite) TestParseTemplateToRegex_InvalidTemplate() {
	// Test with invalid template syntax
	template := "{{.Issue.Invalid"
	regex := parseTemplateToRegex(template)

	// Should return empty string on parse error
	assert.Empty(suite.T(), regex)
}

func (suite *GitTestSuite) TestExtractValues() {
	// Setup
	template := "{{.Issue.ShortName}}/{{.Issue.Info}}-{{.Issue.SanitizedSummary}}"
	branchName := "feature/TEST-123-test-branch"

	// Execute
	values, err := extractValues(template, branchName)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "feature", values[IssueType])
	assert.Equal(suite.T(), "TEST-123", values[IssueKey])
	assert.Equal(suite.T(), "test-branch", values[IssueSanitizedSummary])
}

func (suite *GitTestSuite) TestExtractValues_NoMatch() {
	// Setup
	template := "{{.Issue.ShortName}}/{{.Issue.Info}}-{{.Issue.SanitizedSummary}}"
	branchName := "this-does-not-match"

	// Execute
	_, err := extractValues(template, branchName)

	// Assert
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no matches found")
}

func (suite *GitTestSuite) TestExtractValues_InvalidTemplate() {
	// Setup with invalid template - parseTemplateToRegex returns "" for invalid templates,
	// which results in a regex that matches everything with no named groups
	template := "{{.Issue.Invalid"
	branchName := "feature/TEST-123-test-branch"

	// Execute
	values, err := extractValues(template, branchName)

	// Assert - invalid template produces empty regex which matches, but has no named groups
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), values)
}

func (suite *GitTestSuite) TestCreateIssueBranch() {
	// Setup
	issue := &jira.Issue{
		Key:     "TEST-123",
		Type:    "feature",
		Summary: "test-branch",
	}

	branchName, err := suite.git.CreateIssueBranch(issue)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "f/test-123_test-branch", branchName)

	_, err = suite.repo.Reference(plumbing.NewBranchReferenceName(branchName), true)
	assert.NoError(suite.T(), err)
}

func (suite *GitTestSuite) TestCreateIssueBranch_BranchAlreadyExists() {
	// Create a branch first
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	err = worktree.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("f/test-456_existing-branch"),
		Create: true,
	})
	assert.NoError(suite.T(), err)

	// Switch back to main
	err = worktree.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("main"),
	})
	assert.NoError(suite.T(), err)

	// Try to create the same branch
	issue := &jira.Issue{
		Key:     "TEST-456",
		Type:    "feature",
		Summary: "existing-branch",
	}

	_, err = suite.git.CreateIssueBranch(issue)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "already exists")
}

func (suite *GitTestSuite) TestCreateIssueBranch_LongBranchName() {
	// Create an issue with a very long summary
	longSummary := "this-is-a-very-long-summary-that-will-exceed-the-maximum-branch-name-length-and-should-be-truncated-to-avoid-git-reference-name-issues-we-need-to-make-sure-this-is-handled-properly-and-the-branch-name-is-trimmed-to-244-characters-which-is-a-safe-limit-for-git-reference-names"
	issue := &jira.Issue{
		Key:     "TEST-789",
		Type:    "feature",
		Summary: longSummary,
	}

	branchName, err := suite.git.buildBranchName(issue)

	assert.NoError(suite.T(), err)
	// Branch name should be truncated to 244 characters
	assert.LessOrEqual(suite.T(), len(branchName), 244)
}

func (suite *GitTestSuite) TestCreateIssueBranch_NoTemplate() {
	// Remove branch template from config
	suite.config.Set("git.branchTemplate", "")

	issue := &jira.Issue{
		Key:     "TEST-999",
		Type:    "feature",
		Summary: "test",
	}

	_, err := suite.git.CreateIssueBranch(issue)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "branch template not configured")

	// Restore the template for other tests
	suite.config.Set("git.branchTemplate", "{{.Issue.ShortName}}/{{.Issue.Info}}_{{.Issue.SanitizedSummary}}")
}

func (suite *GitTestSuite) TestGetRemote() {
	// Test with default remote
	gitUrl, err := suite.git.GetRemote()
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), gitUrl)
	assert.Equal(suite.T(), "github.com", gitUrl.GetHostName())
	assert.Equal(suite.T(), "tagoro9", gitUrl.GetOwnerName())
	assert.Equal(suite.T(), "fotingo", gitUrl.GetRepoName())

	// Test with non-existent remote
	suite.config.Set("git.remote", "nonexistent")
	_, err = suite.git.GetRemote()
	assert.Error(suite.T(), err)
}

func (suite *GitTestSuite) TestGetDefaultBranch() {
	// Test with default branch
	branch, err := suite.git.GetDefaultBranch()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "main", branch)

	// Create a new remote with a different default branch
	_, err = suite.repo.CreateRemote(&gogitConfig.RemoteConfig{
		Name: "custom",
		URLs: []string{"https://github.com/tagoro9/fotingo.git"},
	})
	assert.NoError(suite.T(), err)

	headRef, err := suite.repo.Head()
	assert.NoError(suite.T(), err)
	customMainRef := plumbing.NewHashReference(plumbing.NewRemoteReferenceName("custom", "main"), headRef.Hash())
	err = suite.repo.Storer.SetReference(customMainRef)
	assert.NoError(suite.T(), err)
	customHeadRef := plumbing.NewSymbolicReference(
		plumbing.ReferenceName("refs/remotes/custom/HEAD"),
		plumbing.NewRemoteReferenceName("custom", "main"),
	)
	err = suite.repo.Storer.SetReference(customHeadRef)
	assert.NoError(suite.T(), err)

	// Set the custom remote as the default
	suite.config.Set("git.remote", "custom")
	branch, err = suite.git.GetDefaultBranch()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "main", branch)
}

func (suite *GitTestSuite) TestBranchExists() {
	// Test with existing branch
	exists, err := suite.git.branchExists("main")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)

	// Test with non-existent branch
	exists, err = suite.git.branchExists("nonexistent-branch")
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
}

func (suite *GitTestSuite) TestFindClosestRepository() {
	// Test with current directory (should find the test repo)
	repo, err := findClosestRepository("")
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), repo)

	// Test with non-existent directory
	_, err = findClosestRepository("/non-existent-path")
	assert.Error(suite.T(), err)
}

func (suite *GitTestSuite) TestNew() {
	// Test with valid config
	git, err := New(suite.config, &suite.messages)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), git)

	// Test with nil config
	_, err = New(nil, &suite.messages)
	assert.Error(suite.T(), err, "expected error with nil config")

	// Test with nil messages
	_, err = New(suite.config, nil)
	assert.Error(suite.T(), err, "expected error with nil messages")
}

func (suite *GitTestSuite) TestHasUncommittedChanges_CleanWorkTree() {
	// Test with clean work tree
	hasChanges, err := suite.git.HasUncommittedChanges()
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), hasChanges)
}

func (suite *GitTestSuite) TestHasUncommittedChanges_WithStagedChanges() {
	// Create a new file and stage it
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	file, err := worktree.Filesystem.Create("new-staged-file.txt")
	assert.NoError(suite.T(), err)
	_, err = file.Write([]byte("staged content"))
	assert.NoError(suite.T(), err)
	err = file.Close()
	assert.NoError(suite.T(), err)

	_, err = worktree.Add("new-staged-file.txt")
	assert.NoError(suite.T(), err)

	// Test with staged changes
	hasChanges, err := suite.git.HasUncommittedChanges()
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), hasChanges)
}

func (suite *GitTestSuite) TestHasUncommittedChanges_WithUnstagedChanges() {
	// Modify an existing file without staging
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	file, err := worktree.Filesystem.OpenFile("dummy.txt", 2, 0644)
	assert.NoError(suite.T(), err)
	_, err = file.Write([]byte("modified content"))
	assert.NoError(suite.T(), err)
	err = file.Close()
	assert.NoError(suite.T(), err)

	// Test with unstaged changes
	hasChanges, err := suite.git.HasUncommittedChanges()
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), hasChanges)
}

func (suite *GitTestSuite) TestGetCommitsSince() {
	// Get the initial commit hash
	head, err := suite.repo.Head()
	assert.NoError(suite.T(), err)
	initialCommitHash := head.Hash().String()

	// Create a new commit
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	file, err := worktree.Filesystem.Create("commit-test.txt")
	assert.NoError(suite.T(), err)
	_, err = file.Write([]byte("test content"))
	assert.NoError(suite.T(), err)
	err = file.Close()
	assert.NoError(suite.T(), err)

	_, err = worktree.Add("commit-test.txt")
	assert.NoError(suite.T(), err)

	_, err = worktree.Commit("feat: add commit test file", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	assert.NoError(suite.T(), err)

	// Get commits since initial commit
	commits, err := suite.git.GetCommitsSince(initialCommitHash)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), commits, 1)
	assert.Equal(suite.T(), "feat: add commit test file", commits[0].Message)
	assert.Equal(suite.T(), "Test User", commits[0].Author)
}

func (suite *GitTestSuite) TestGetCommitsSince_MultipleCommits() {
	// Get the initial commit hash
	head, err := suite.repo.Head()
	assert.NoError(suite.T(), err)
	initialCommitHash := head.Hash().String()

	// Create multiple commits
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	for i := 1; i <= 3; i++ {
		filename := fmt.Sprintf("file%d.txt", i)
		file, err := worktree.Filesystem.Create(filename)
		assert.NoError(suite.T(), err)
		_, err = fmt.Fprintf(file, "content %d", i)
		assert.NoError(suite.T(), err)
		err = file.Close()
		assert.NoError(suite.T(), err)

		_, err = worktree.Add(filename)
		assert.NoError(suite.T(), err)

		_, err = worktree.Commit(fmt.Sprintf("feat: add file %d", i), &gogit.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		assert.NoError(suite.T(), err)
	}

	// Get commits since initial commit
	commits, err := suite.git.GetCommitsSince(initialCommitHash)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), commits, 3)

	// Commits are returned in reverse chronological order (newest first)
	assert.Equal(suite.T(), "feat: add file 3", commits[0].Message)
	assert.Equal(suite.T(), "feat: add file 2", commits[1].Message)
	assert.Equal(suite.T(), "feat: add file 1", commits[2].Message)
}

func (suite *GitTestSuite) TestGetCommitsSince_InvalidRef() {
	// Test with invalid reference
	_, err := suite.git.GetCommitsSince("invalid-ref")
	assert.Error(suite.T(), err)
}

func (suite *GitTestSuite) TestGetIssuesFromCommits() {
	tests := []struct {
		name     string
		commits  []Commit
		expected []string
	}{
		{
			name: "single issue in commit message",
			commits: []Commit{
				{Hash: "abc123", Message: "feat: implement login TEST-123", Author: "Test"},
			},
			expected: []string{"TEST-123"},
		},
		{
			name: "multiple issues in single commit",
			commits: []Commit{
				{Hash: "abc123", Message: "fix: resolve TEST-123 and TEST-456", Author: "Test"},
			},
			expected: []string{"TEST-123", "TEST-456"},
		},
		{
			name: "issues across multiple commits",
			commits: []Commit{
				{Hash: "abc123", Message: "feat: implement TEST-123", Author: "Test"},
				{Hash: "def456", Message: "fix: resolve PROJ-789", Author: "Test"},
			},
			expected: []string{"TEST-123", "PROJ-789"},
		},
		{
			name: "duplicate issues are deduplicated",
			commits: []Commit{
				{Hash: "abc123", Message: "feat: implement TEST-123", Author: "Test"},
				{Hash: "def456", Message: "fix: more work on TEST-123", Author: "Test"},
			},
			expected: []string{"TEST-123"},
		},
		{
			name: "lowercase issue IDs are not matched (uppercase only)",
			commits: []Commit{
				{Hash: "abc123", Message: "feat: implement test-123", Author: "Test"},
			},
			expected: nil,
		},
		{
			name: "no issues found",
			commits: []Commit{
				{Hash: "abc123", Message: "chore: update dependencies", Author: "Test"},
			},
			expected: nil,
		},
		{
			name:     "empty commits",
			commits:  []Commit{},
			expected: nil,
		},
		{
			name: "issue at start of message",
			commits: []Commit{
				{Hash: "abc123", Message: "TEST-123: fix bug", Author: "Test"},
			},
			expected: []string{"TEST-123"},
		},
		{
			name: "issue in brackets",
			commits: []Commit{
				{Hash: "abc123", Message: "[TEST-123] fix bug", Author: "Test"},
			},
			expected: []string{"TEST-123"},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := suite.git.GetIssuesFromCommits(tt.commits)
			assert.Equal(suite.T(), tt.expected, result)
		})
	}
}

func (suite *GitTestSuite) TestGetRemote_InvalidRemote() {
	// Test with non-existent remote
	suite.config.Set("git.remote", "nonexistent")
	_, err := suite.git.GetRemote()
	assert.Error(suite.T(), err)

	// Reset to default
	suite.config.Set("git.remote", "origin")
}

func (suite *GitTestSuite) TestGetRemote_InvalidURL() {
	// Create a remote with invalid URL
	_, err := suite.repo.CreateRemote(&gogitConfig.RemoteConfig{
		Name: "invalid",
		URLs: []string{"not-a-valid-url"},
	})
	assert.NoError(suite.T(), err)

	suite.config.Set("git.remote", "invalid")
	_, err = suite.git.GetRemote()
	assert.Error(suite.T(), err)

	// Reset to default
	suite.config.Set("git.remote", "origin")
}

func (suite *GitTestSuite) TestFindClosestRepository_InvalidPath() {
	// Test with a path that doesn't exist and has no parent git repo
	_, err := findClosestRepository("/nonexistent/path/that/does/not/exist")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no Git repository found")
}

func (suite *GitTestSuite) TestFindClosestRepository_Subdirectory() {
	// Test that findClosestRepository walks up from a subdirectory
	// Use a known subdirectory within this project
	repo, err := findClosestRepository("internal/git")
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), repo)
}

func (suite *GitTestSuite) TestGetCommitsSince_InvalidReference() {
	_, err := suite.git.GetCommitsSince("refs/heads/nonexistent")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to resolve reference")
}

func (suite *GitTestSuite) TestGetIssuesFromCommits_ComplexPatterns() {
	tests := []struct {
		name     string
		commits  []Commit
		expected []string
	}{
		{
			name: "issue with underscores in project name",
			commits: []Commit{
				{Hash: "abc", Message: "fix: resolve MY_PROJ-123", Author: "Test"},
			},
			expected: []string{"MY_PROJ-123"},
		},
		{
			name: "issue with numbers in project name",
			commits: []Commit{
				{Hash: "abc", Message: "feat: implement PROJ2-456", Author: "Test"},
			},
			expected: []string{"PROJ2-456"},
		},
		{
			name: "mixed case does not match - requires uppercase project key",
			commits: []Commit{
				{Hash: "abc", Message: "fix: Test-123 should not match", Author: "Test"},
			},
			expected: nil,
		},
		{
			name: "multiple occurrences of same issue in one commit",
			commits: []Commit{
				{Hash: "abc", Message: "TEST-123: fixes TEST-123 completely", Author: "Test"},
			},
			expected: []string{"TEST-123"},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := suite.git.GetIssuesFromCommits(tt.commits)
			assert.Equal(suite.T(), tt.expected, result)
		})
	}
}

func (suite *GitTestSuite) TestCreateIssueBranch_InvalidTemplate() {
	// Set an invalid Go template
	suite.config.Set("git.branchTemplate", "{{.Issue.Invalid")

	issue := &jira.Issue{
		Key:     "TEST-999",
		Type:    "feature",
		Summary: "test",
	}

	_, err := suite.git.CreateIssueBranch(issue)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to parse branch template")

	// Restore
	suite.config.Set("git.branchTemplate", "{{.Issue.ShortName}}/{{.Issue.Info}}_{{.Issue.SanitizedSummary}}")
}

func (suite *GitTestSuite) TestGetIssueId_TemplateWithoutKey() {
	// Use a template that does not include the Info placeholder
	suite.config.Set("git.branchTemplate", "{{.Issue.ShortName}}/{{.Issue.SanitizedSummary}}")

	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	err = worktree.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("f/some-summary"),
		Create: true,
	})
	assert.NoError(suite.T(), err)

	_, err = suite.git.GetIssueId()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no issue id found in branch name")

	// Restore
	suite.config.Set("git.branchTemplate", "{{.Issue.ShortName}}/{{.Issue.Info}}_{{.Issue.SanitizedSummary}}")
}

func (suite *GitTestSuite) TestGetCommitsSince_NoNewCommitsSinceHead() {
	// Get commits since HEAD (should be empty)
	head, err := suite.repo.Head()
	assert.NoError(suite.T(), err)

	commits, err := suite.git.GetCommitsSince(head.Hash().String())
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), commits)
}

func (suite *GitTestSuite) TestParseTemplateToRegex_WithVariousTemplates() {
	tests := []struct {
		name     string
		template string
		isEmpty  bool
	}{
		{
			name:     "template with unknown function fails to parse",
			template: "{{.Issue.ShortName | unknownFunc}}",
			isEmpty:  true,
		},
		{
			name:     "valid template with all fields",
			template: "{{.Issue.ShortName}}/{{.Issue.Info}}_{{.Issue.SanitizedSummary}}",
			isEmpty:  false,
		},
		{
			name:     "template with only key",
			template: "{{.Issue.Info}}",
			isEmpty:  false,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			regex := parseTemplateToRegex(tt.template)
			if tt.isEmpty {
				assert.Empty(suite.T(), regex)
			} else {
				assert.NotEmpty(suite.T(), regex)
			}
		})
	}
}

func (suite *GitTestSuite) TestGitNonInteractiveEnv_Defaults() {
	env := gitNonInteractiveEnv()

	assert.Contains(suite.T(), env, "GIT_TERMINAL_PROMPT=0")
	assert.Contains(suite.T(), env, "GCM_INTERACTIVE=Never")
	assert.Contains(suite.T(), env, "GIT_ASKPASS=echo")
	assert.Contains(suite.T(), env, "SSH_ASKPASS=echo")
}

func (suite *GitTestSuite) TestSetEnvValue_ReplacesAndAppends() {
	initial := []string{"FOO=1", "GIT_TERMINAL_PROMPT=1"}
	updated := setEnvValue(initial, "GIT_TERMINAL_PROMPT", "0")
	assert.Equal(suite.T(), []string{"FOO=1", "GIT_TERMINAL_PROMPT=0"}, updated)

	appended := setEnvValue([]string{"FOO=1"}, "BAR", "2")
	assert.Equal(suite.T(), []string{"FOO=1", "BAR=2"}, appended)
}

func (suite *GitTestSuite) TestExecCredentialProvider_ReportsStderrOnFailure() {
	scriptDir := suite.T().TempDir()
	scriptPath := filepath.Join(scriptDir, "git")
	err := os.WriteFile(scriptPath, []byte(`#!/bin/sh
echo "fatal: helper exploded" >&2
exit 128
`), 0755)
	assert.NoError(suite.T(), err)

	suite.T().Setenv("PATH", scriptDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider := &execCredentialProvider{}
	_, err = provider.GetCredentials("https://github.com/tagoro9/fotingo-playground.git")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "git credential helper failed in non-interactive mode")
	assert.Contains(suite.T(), err.Error(), "fatal: helper exploded")
	assert.Contains(suite.T(), err.Error(), "https://github.com/tagoro9/fotingo-playground.git")
}

func (suite *GitTestSuite) TestExecCredentialProvider_RejectsPromptPlaceholderCredentials() {
	scriptDir := suite.T().TempDir()
	scriptPath := filepath.Join(scriptDir, "git")
	err := os.WriteFile(scriptPath, []byte(`#!/bin/sh
cat <<'EOF'
protocol=https
host=github.com
username=Username for 'https://github.com':
password=Password for 'https://github.com':
EOF
`), 0755)
	assert.NoError(suite.T(), err)

	suite.T().Setenv("PATH", scriptDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider := &execCredentialProvider{}
	_, err = provider.GetCredentials("https://github.com/tagoro9/fotingo-playground.git")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "returned interactive prompt text instead of stored credentials")
}

func (suite *GitTestSuite) TestLooksLikeCredentialPrompt() {
	assert.True(suite.T(), looksLikeCredentialPrompt("Username for 'https://github.com':"))
	assert.True(suite.T(), looksLikeCredentialPrompt("Password for 'https://github.com':"))
	assert.False(suite.T(), looksLikeCredentialPrompt("octocat"))
	assert.False(suite.T(), looksLikeCredentialPrompt(""))
}

func (suite *GitTestSuite) TestHasUncommittedChanges_WithNewFile() {
	// Create a new untracked file
	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	file, err := worktree.Filesystem.Create("new-untracked-file.txt")
	assert.NoError(suite.T(), err)
	_, err = file.Write([]byte("untracked content"))
	assert.NoError(suite.T(), err)
	err = file.Close()
	assert.NoError(suite.T(), err)

	hasChanges, err := suite.git.HasUncommittedChanges()
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), hasChanges)
}

func (suite *GitTestSuite) TestGetCommitsSince_WhitespaceInMessage() {
	head, err := suite.repo.Head()
	assert.NoError(suite.T(), err)
	initialHash := head.Hash().String()

	worktree, err := suite.repo.Worktree()
	assert.NoError(suite.T(), err)

	file, err := worktree.Filesystem.Create("whitespace-test.txt")
	assert.NoError(suite.T(), err)
	_, err = file.Write([]byte("content"))
	assert.NoError(suite.T(), err)
	err = file.Close()
	assert.NoError(suite.T(), err)

	_, err = worktree.Add("whitespace-test.txt")
	assert.NoError(suite.T(), err)

	// Commit with trailing whitespace/newline
	_, err = worktree.Commit("  feat: message with whitespace  \n", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	assert.NoError(suite.T(), err)

	commits, err := suite.git.GetCommitsSince(initialHash)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), commits, 1)
	// Message should be trimmed
	assert.Equal(suite.T(), "feat: message with whitespace", commits[0].Message)
}

func TestGitSuite(t *testing.T) {
	suite.Run(t, new(GitTestSuite))
}
