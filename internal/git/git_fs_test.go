package git

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	gogitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tagoro9/fotingo/internal/config"
	"github.com/tagoro9/fotingo/internal/jira"
)

// GitFsTestSuite uses real filesystem repos to test functions that shell out to git (stash)
type GitFsTestSuite struct {
	suite.Suite
	tmpDir    string
	repo      *gogit.Repository
	messages  chan string
	gitClient *git
}

func (s *GitFsTestSuite) SetupTest() {
	t := s.T()
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	var err error

	s.tmpDir = t.TempDir()
	repoDir := filepath.Join(s.tmpDir, "work")

	s.repo, err = gogit.PlainInit(repoDir, false)
	require.NoError(t, err)

	// Add a remote (needed for config lookups even if we don't use it for credentials)
	_, err = s.repo.CreateRemote(&gogitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://github.com/tagoro9/fotingo.git"},
	})
	require.NoError(t, err)

	// Create an initial commit
	wt, err := s.repo.Worktree()
	require.NoError(t, err)

	dummyPath := filepath.Join(repoDir, "dummy.txt")
	err = os.WriteFile(dummyPath, []byte("hello"), 0644)
	require.NoError(t, err)

	_, err = wt.Add("dummy.txt")
	require.NoError(t, err)

	commitHash, err := wt.Commit("feat: initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Create the main branch reference
	mainRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), commitHash)
	err = s.repo.Storer.SetReference(mainRef)
	require.NoError(t, err)

	viperCfg := config.NewDefaultConfig()
	s.messages = make(chan string, 100)

	s.gitClient = &git{
		repo:                     s.repo,
		ViperConfigurableService: &config.ViperConfigurableService{Config: viperCfg, Prefix: "git"},
		messages:                 &s.messages,
		credentialProvider:       &mockCredentialProvider{},
	}
}

func (s *GitFsTestSuite) TearDownTest() {
	for len(s.messages) > 0 {
		<-s.messages
	}
}

// repoDir returns the working directory of the test repo
func (s *GitFsTestSuite) repoDir() string {
	wt, err := s.repo.Worktree()
	require.NoError(s.T(), err)
	return wt.Filesystem.Root()
}

// createFileAndCommit is a helper that creates a file, stages and commits it
func (s *GitFsTestSuite) createFileAndCommit(name, content, message string) plumbing.Hash {
	t := s.T()
	wt, err := s.repo.Worktree()
	require.NoError(t, err)

	filePath := filepath.Join(s.repoDir(), name)
	err = os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	_, err = wt.Add(name)
	require.NoError(t, err)

	hash, err := wt.Commit(message, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)
	return hash
}

// --- StashChanges tests ---

func (s *GitFsTestSuite) TestStashChanges_StashesUntrackedFile() {
	t := s.T()

	// Create an untracked file
	filePath := filepath.Join(s.repoDir(), "uncommitted.txt")
	err := os.WriteFile(filePath, []byte("uncommitted content"), 0644)
	require.NoError(t, err)

	// Stash changes
	err = s.gitClient.StashChanges("test stash message")
	assert.NoError(t, err)

	// Verify working tree is clean
	hasChanges, err := s.gitClient.HasUncommittedChanges()
	assert.NoError(t, err)
	assert.False(t, hasChanges, "working tree should be clean after stash")

	// Verify the file is gone
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "untracked file should not exist after stash")
}

func (s *GitFsTestSuite) TestStashChanges_StashesModifiedTrackedFile() {
	t := s.T()

	// Modify the tracked dummy.txt file
	filePath := filepath.Join(s.repoDir(), "dummy.txt")
	err := os.WriteFile(filePath, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Stash changes
	err = s.gitClient.StashChanges("stash modified file")
	assert.NoError(t, err)

	// Verify the file is back to original content
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, "hello", string(content))
}

func (s *GitFsTestSuite) TestStashChanges_SendsMessage() {
	t := s.T()

	// Create a change to stash
	filePath := filepath.Join(s.repoDir(), "msg-test.txt")
	err := os.WriteFile(filePath, []byte("content"), 0644)
	require.NoError(t, err)

	err = s.gitClient.StashChanges("my stash message")
	assert.NoError(t, err)

	// Verify a message was sent to the channel
	assert.Greater(t, len(s.messages), 0)
	msg := <-s.messages
	assert.Contains(t, msg, "Stashing changes: my stash message")
}

func (s *GitFsTestSuite) TestStashChanges_NoChangesToStash() {
	t := s.T()

	// Stash with nothing to stash succeeds (git stash push returns 0 even with nothing)
	err := s.gitClient.StashChanges("nothing to stash")
	assert.NoError(t, err)
}

// --- PopStash tests ---

func (s *GitFsTestSuite) TestPopStash_RestoresStashedChanges() {
	t := s.T()

	// Create an untracked file and stash it
	filePath := filepath.Join(s.repoDir(), "stash-pop-test.txt")
	err := os.WriteFile(filePath, []byte("stash pop content"), 0644)
	require.NoError(t, err)

	err = s.gitClient.StashChanges("stash for pop test")
	require.NoError(t, err)

	// Verify file is gone
	_, err = os.Stat(filePath)
	require.True(t, os.IsNotExist(err))

	// Pop the stash
	err = s.gitClient.PopStash()
	assert.NoError(t, err)

	// Verify the file is restored
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, "stash pop content", string(content))
}

func (s *GitFsTestSuite) TestPopStash_FailsWithNoStash() {
	// Try to pop when there is no stash
	err := s.gitClient.PopStash()
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to pop stash")
}

func (s *GitFsTestSuite) TestPopStash_SendsMessage() {
	t := s.T()

	// Create and stash a change
	filePath := filepath.Join(s.repoDir(), "pop-msg.txt")
	err := os.WriteFile(filePath, []byte("content"), 0644)
	require.NoError(t, err)

	err = s.gitClient.StashChanges("for pop message test")
	require.NoError(t, err)

	// Drain messages from stash
	for len(s.messages) > 0 {
		<-s.messages
	}

	err = s.gitClient.PopStash()
	assert.NoError(t, err)

	assert.Greater(t, len(s.messages), 0)
	msg := <-s.messages
	assert.Contains(t, msg, "Popping stash")
}

func (s *GitFsTestSuite) TestPopStash_RestoresModifiedTrackedFile() {
	t := s.T()

	// Modify tracked file
	filePath := filepath.Join(s.repoDir(), "dummy.txt")
	err := os.WriteFile(filePath, []byte("modified for pop"), 0644)
	require.NoError(t, err)

	// Stash
	err = s.gitClient.StashChanges("stash modified tracked")
	require.NoError(t, err)

	// Verify original
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "hello", string(content))

	// Pop
	err = s.gitClient.PopStash()
	assert.NoError(t, err)

	// Verify restored
	content, err = os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, "modified for pop", string(content))
}

// --- StashChanges + PopStash round-trip ---

func (s *GitFsTestSuite) TestStashAndPop_RoundTrip() {
	t := s.T()

	// Create multiple untracked files
	for i := range 3 {
		filePath := filepath.Join(s.repoDir(), fmt.Sprintf("roundtrip-%d.txt", i))
		err := os.WriteFile(filePath, []byte(fmt.Sprintf("content %d", i)), 0644)
		require.NoError(t, err)
	}

	// Also modify a tracked file
	dummyPath := filepath.Join(s.repoDir(), "dummy.txt")
	err := os.WriteFile(dummyPath, []byte("modified for stash"), 0644)
	require.NoError(t, err)

	// Verify uncommitted changes exist
	hasChanges, err := s.gitClient.HasUncommittedChanges()
	require.NoError(t, err)
	require.True(t, hasChanges)

	// Stash
	err = s.gitClient.StashChanges("round trip test")
	require.NoError(t, err)

	// Verify clean
	hasChanges, err = s.gitClient.HasUncommittedChanges()
	require.NoError(t, err)
	require.False(t, hasChanges)

	// Verify tracked file is back to original
	content, err := os.ReadFile(dummyPath)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(content))

	// Pop
	err = s.gitClient.PopStash()
	require.NoError(t, err)

	// Verify changes are restored
	hasChanges, err = s.gitClient.HasUncommittedChanges()
	assert.NoError(t, err)
	assert.True(t, hasChanges)

	// Verify tracked file is modified again
	content, err = os.ReadFile(dummyPath)
	assert.NoError(t, err)
	assert.Equal(t, "modified for stash", string(content))

	// Verify untracked files are back
	for i := range 3 {
		filePath := filepath.Join(s.repoDir(), fmt.Sprintf("roundtrip-%d.txt", i))
		content, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("content %d", i), string(content))
	}
}

func (s *GitFsTestSuite) TestStashChanges_WithStagedChanges() {
	t := s.T()

	// Create a file, stage it but do not commit
	wt, err := s.repo.Worktree()
	require.NoError(t, err)

	filePath := filepath.Join(s.repoDir(), "staged.txt")
	err = os.WriteFile(filePath, []byte("staged content"), 0644)
	require.NoError(t, err)

	_, err = wt.Add("staged.txt")
	require.NoError(t, err)

	// Stash
	err = s.gitClient.StashChanges("stash staged")
	assert.NoError(t, err)

	// Verify clean
	hasChanges, err := s.gitClient.HasUncommittedChanges()
	assert.NoError(t, err)
	assert.False(t, hasChanges)
}

func (s *GitFsTestSuite) TestPopStash_MultipleStashes() {
	t := s.T()

	// Create first change and stash
	filePath1 := filepath.Join(s.repoDir(), "stash1.txt")
	err := os.WriteFile(filePath1, []byte("first stash"), 0644)
	require.NoError(t, err)

	err = s.gitClient.StashChanges("first stash")
	require.NoError(t, err)

	// Create second change and stash
	filePath2 := filepath.Join(s.repoDir(), "stash2.txt")
	err = os.WriteFile(filePath2, []byte("second stash"), 0644)
	require.NoError(t, err)

	err = s.gitClient.StashChanges("second stash")
	require.NoError(t, err)

	// Pop should restore the most recent stash (second)
	err = s.gitClient.PopStash()
	assert.NoError(t, err)

	_, err = os.Stat(filePath2)
	assert.NoError(t, err, "second stashed file should be restored")

	// Pop again to restore the first stash
	err = s.gitClient.PopStash()
	assert.NoError(t, err)

	_, err = os.Stat(filePath1)
	assert.NoError(t, err, "first stashed file should be restored")
}

// --- Push error path tests ---

func (s *GitFsTestSuite) TestPush_InvalidRemote() {
	s.gitClient.Config.Set("git.remote", "nonexistent")
	err := s.gitClient.Push()
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to get remote")
}

func (s *GitFsTestSuite) TestPush_DetachedHead() {
	t := s.T()

	// Checkout detached HEAD
	head, err := s.repo.Head()
	require.NoError(t, err)

	wt, err := s.repo.Worktree()
	require.NoError(t, err)

	err = wt.Checkout(&gogit.CheckoutOptions{
		Hash: head.Hash(),
	})
	require.NoError(t, err)

	err = s.gitClient.Push()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get current branch")
}

// --- DoesBranchExistInRemote error path tests ---

func (s *GitFsTestSuite) TestDoesBranchExistInRemote_InvalidRemote() {
	s.gitClient.Config.Set("git.remote", "nonexistent")
	_, err := s.gitClient.DoesBranchExistInRemote("main")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to get remote")
}

// --- GetCommitsSinceDefaultBranch tests ---
// We can test this by simulating the remote ref in the local storer

func (s *GitFsTestSuite) TestGetCommitsSinceDefaultBranch_InvalidRemote() {
	s.gitClient.Config.Set("git.remote", "nonexistent")
	_, err := s.gitClient.GetCommitsSinceDefaultBranch()
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to get default branch")
}

// --- GetDefaultBranch error path tests ---

func (s *GitFsTestSuite) TestGetDefaultBranch_InvalidRemote() {
	s.gitClient.Config.Set("git.remote", "nonexistent")
	_, err := s.gitClient.GetDefaultBranch()
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to get remote")
}

// --- fetch error path tests ---

func (s *GitFsTestSuite) TestFetch_InvalidRemote() {
	err := s.gitClient.fetch("nonexistent")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to get remote")
}

// --- HasUncommittedChanges filesystem tests ---

func (s *GitFsTestSuite) TestHasUncommittedChanges_WithNewUntrackedFile() {
	t := s.T()

	filePath := filepath.Join(s.repoDir(), "untracked.txt")
	err := os.WriteFile(filePath, []byte("untracked"), 0644)
	require.NoError(t, err)

	hasChanges, err := s.gitClient.HasUncommittedChanges()
	assert.NoError(t, err)
	assert.True(t, hasChanges)
}

func (s *GitFsTestSuite) TestHasUncommittedChanges_CleanAfterCommit() {
	t := s.T()

	s.createFileAndCommit("committed.txt", "committed", "feat: committed file")

	hasChanges, err := s.gitClient.HasUncommittedChanges()
	assert.NoError(t, err)
	assert.False(t, hasChanges)
}

func (s *GitFsTestSuite) TestHasUncommittedChanges_WithDeletedFile() {
	t := s.T()

	// Delete a tracked file
	filePath := filepath.Join(s.repoDir(), "dummy.txt")
	err := os.Remove(filePath)
	require.NoError(t, err)

	hasChanges, err := s.gitClient.HasUncommittedChanges()
	assert.NoError(t, err)
	assert.True(t, hasChanges)
}

func (s *GitFsTestSuite) TestHasUncommittedChanges_WithGitIgnoredFile() {
	t := s.T()
	wt, err := s.repo.Worktree()
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(s.repoDir(), ".gitignore"), []byte("*.log\n"), 0644)
	require.NoError(t, err)

	_, err = wt.Add(".gitignore")
	require.NoError(t, err)

	_, err = wt.Commit("chore: add gitignore", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	filePath := filepath.Join(s.repoDir(), "debug.log")
	err = os.WriteFile(filePath, []byte("ignored"), 0644)
	require.NoError(t, err)

	hasChanges, err := s.gitClient.HasUncommittedChanges()
	assert.NoError(t, err)
	assert.False(t, hasChanges)
}

func (s *GitFsTestSuite) TestHasUncommittedChanges_WithGlobalGitIgnoredFile() {
	t := s.T()

	s.configureGlobalIgnore("*.cache\n")

	filePath := filepath.Join(s.repoDir(), "tmp.cache")
	err := os.WriteFile(filePath, []byte("ignored globally"), 0644)
	require.NoError(t, err)

	hasChanges, err := s.gitClient.HasUncommittedChanges()
	assert.NoError(t, err)
	assert.False(t, hasChanges)
}

func (s *GitFsTestSuite) TestHasUncommittedChanges_WithGlobalIgnoreAndNonIgnoredFile() {
	t := s.T()

	s.configureGlobalIgnore("*.cache\n")

	err := os.WriteFile(filepath.Join(s.repoDir(), "tmp.cache"), []byte("ignored globally"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(s.repoDir(), "tracked-by-status.txt"), []byte("not ignored"), 0644)
	require.NoError(t, err)

	hasChanges, err := s.gitClient.HasUncommittedChanges()
	assert.NoError(t, err)
	assert.True(t, hasChanges)
}

func (s *GitFsTestSuite) configureGlobalIgnore(patterns string) {
	t := s.T()

	homeDir := t.TempDir()
	excludesPath := filepath.Join(homeDir, ".gitignore_global")
	gitConfigPath := filepath.Join(homeDir, ".gitconfig")

	err := os.WriteFile(excludesPath, []byte(patterns), 0644)
	require.NoError(t, err)

	configContent := fmt.Sprintf("[core]\n\texcludesfile = %s\n", excludesPath)
	err = os.WriteFile(gitConfigPath, []byte(configContent), 0644)
	require.NoError(t, err)

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
}

// --- GetCommitsSince additional tests ---

func (s *GitFsTestSuite) TestGetCommitsSince_NoCommitsSinceHead() {
	t := s.T()

	head, err := s.repo.Head()
	require.NoError(t, err)

	commits, err := s.gitClient.GetCommitsSince(head.Hash().String())
	assert.NoError(t, err)
	assert.Empty(t, commits)
}

func (s *GitFsTestSuite) TestGetCommitsSince_CommitHashIncluded() {
	t := s.T()

	head, err := s.repo.Head()
	require.NoError(t, err)
	initialHash := head.Hash().String()

	// Create two commits
	s.createFileAndCommit("f1.txt", "c1", "feat: first")
	s.createFileAndCommit("f2.txt", "c2", "feat: second")

	commits, err := s.gitClient.GetCommitsSince(initialHash)
	assert.NoError(t, err)
	assert.Len(t, commits, 2)

	// Verify commit hashes are present and non-empty
	for _, c := range commits {
		assert.NotEmpty(t, c.Hash)
		assert.NotEmpty(t, c.Author)
	}
}

// --- branchExists local-only tests ---

func (s *GitFsTestSuite) TestBranchExists_ExistingLocalBranch() {
	t := s.T()

	exists, err := s.gitClient.branchExists("main")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func (s *GitFsTestSuite) TestBranchExists_NewlyCreatedBranch() {
	t := s.T()

	wt, err := s.repo.Worktree()
	require.NoError(t, err)

	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("new-local-branch"),
		Create: true,
	})
	require.NoError(t, err)

	// Switch back to main
	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("main"),
	})
	require.NoError(t, err)

	// Check the newly created branch exists
	exists, err := s.gitClient.branchExists("new-local-branch")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestGitFsSuite(t *testing.T) {
	suite.Run(t, new(GitFsTestSuite))
}

// mockCredentialProvider returns no-op credentials for local file:// remotes.
type mockCredentialProvider struct{}

func (m *mockCredentialProvider) GetCredentials(_ string) (*http.BasicAuth, error) {
	return &http.BasicAuth{
		Username: "test",
		Password: "test",
	}, nil
}

// failingCredentialProvider always returns an error.
type failingCredentialProvider struct{}

func (f *failingCredentialProvider) GetCredentials(_ string) (*http.BasicAuth, error) {
	return nil, fmt.Errorf("credential retrieval failed")
}

// GitCredentialTestSuite tests functions that depend on credentials using a local bare remote.
type GitCredentialTestSuite struct {
	suite.Suite
	tmpDir    string
	bareRepo  *gogit.Repository
	workRepo  *gogit.Repository
	messages  chan string
	gitClient *git
}

func (s *GitCredentialTestSuite) SetupTest() {
	t := s.T()
	var err error

	s.tmpDir = t.TempDir()

	// Create a bare repo as the "remote"
	bareDir := filepath.Join(s.tmpDir, "bare.git")
	s.bareRepo, err = gogit.PlainInit(bareDir, true)
	require.NoError(t, err)

	// Create the working repo and add the bare repo as remote
	workDir := filepath.Join(s.tmpDir, "work")
	s.workRepo, err = gogit.PlainInit(workDir, false)
	require.NoError(t, err)

	_, err = s.workRepo.CreateRemote(&gogitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{bareDir},
	})
	require.NoError(t, err)

	// Create an initial commit
	wt, err := s.workRepo.Worktree()
	require.NoError(t, err)

	dummyPath := filepath.Join(workDir, "dummy.txt")
	err = os.WriteFile(dummyPath, []byte("hello"), 0644)
	require.NoError(t, err)

	_, err = wt.Add("dummy.txt")
	require.NoError(t, err)

	_, err = wt.Commit("feat: initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Push the initial commit to establish the remote main branch
	err = s.workRepo.Push(&gogit.PushOptions{
		RemoteName: "origin",
		RefSpecs: []gogitConfig.RefSpec{
			"refs/heads/master:refs/heads/master",
		},
	})
	require.NoError(t, err)

	// Set HEAD on bare repo to point to master
	bareHeadRef := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("master"))
	err = s.bareRepo.Storer.SetReference(bareHeadRef)
	require.NoError(t, err)

	viperCfg := config.NewDefaultConfig()
	s.messages = make(chan string, 100)

	s.gitClient = &git{
		repo:                     s.workRepo,
		ViperConfigurableService: &config.ViperConfigurableService{Config: viperCfg, Prefix: "git"},
		messages:                 &s.messages,
		credentialProvider:       &mockCredentialProvider{},
	}
}

func (s *GitCredentialTestSuite) TearDownTest() {
	for len(s.messages) > 0 {
		<-s.messages
	}
}

func (s *GitCredentialTestSuite) workDir() string {
	wt, err := s.workRepo.Worktree()
	require.NoError(s.T(), err)
	return wt.Filesystem.Root()
}

func (s *GitCredentialTestSuite) createFileAndCommit(name, content, message string) plumbing.Hash {
	t := s.T()
	wt, err := s.workRepo.Worktree()
	require.NoError(t, err)

	filePath := filepath.Join(s.workDir(), name)
	err = os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	_, err = wt.Add(name)
	require.NoError(t, err)

	hash, err := wt.Commit(message, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)
	return hash
}

// --- fetch tests ---

func (s *GitCredentialTestSuite) TestFetch_Success() {
	err := s.gitClient.fetch("origin")
	assert.NoError(s.T(), err)
}

func (s *GitCredentialTestSuite) TestFetch_AlreadyUpToDate() {
	// First fetch to sync
	err := s.gitClient.fetch("origin")
	require.NoError(s.T(), err)

	// Second fetch should be already up to date (no error)
	err = s.gitClient.fetch("origin")
	assert.NoError(s.T(), err)
}

func (s *GitCredentialTestSuite) TestFetch_CredentialError() {
	s.gitClient.credentialProvider = &failingCredentialProvider{}
	err := s.gitClient.fetch("origin")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to get credentials")
}

// --- Push tests ---

func (s *GitCredentialTestSuite) TestPush_Success() {
	s.createFileAndCommit("push-test.txt", "push content", "feat: push test")

	err := s.gitClient.Push()
	assert.NoError(s.T(), err)

	cfg, cfgErr := s.workRepo.Config()
	require.NoError(s.T(), cfgErr)
	require.Contains(s.T(), cfg.Branches, "master")
	assert.Equal(s.T(), "origin", cfg.Branches["master"].Remote)
	assert.Equal(s.T(), plumbing.NewBranchReferenceName("master"), cfg.Branches["master"].Merge)
}

func (s *GitCredentialTestSuite) TestPush_AlreadyUpToDate() {
	// Push initial state (already synced in setup)
	err := s.gitClient.Push()
	assert.NoError(s.T(), err)
}

func (s *GitCredentialTestSuite) TestPush_CredentialError() {
	s.gitClient.credentialProvider = &failingCredentialProvider{}
	err := s.gitClient.Push()
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to get credentials")
}

func (s *GitCredentialTestSuite) TestPush_UsesConfiguredRemoteForUpstream() {
	t := s.T()

	customRemoteDir := filepath.Join(s.tmpDir, "custom.git")
	customRemote, err := gogit.PlainInit(customRemoteDir, true)
	require.NoError(t, err)

	_, err = s.workRepo.CreateRemote(&gogitConfig.RemoteConfig{
		Name: "custom",
		URLs: []string{customRemoteDir},
	})
	require.NoError(t, err)

	s.gitClient.Config.Set("git.remote", "custom")

	err = s.gitClient.Push()
	require.NoError(t, err)

	cfg, err := s.workRepo.Config()
	require.NoError(t, err)
	require.Contains(t, cfg.Branches, "master")
	assert.Equal(t, "custom", cfg.Branches["master"].Remote)
	assert.Equal(t, plumbing.NewBranchReferenceName("master"), cfg.Branches["master"].Merge)

	_, err = customRemote.Reference(plumbing.NewBranchReferenceName("master"), true)
	assert.NoError(t, err)
}

// --- GetDefaultBranch tests ---

func (s *GitCredentialTestSuite) TestGetDefaultBranch_Success() {
	branch, err := s.gitClient.GetDefaultBranch()
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "master", branch)
}

func (s *GitCredentialTestSuite) TestGetDefaultBranch_CredentialError() {
	s.gitClient.credentialProvider = &failingCredentialProvider{}

	// Remove cached remote refs to force GetDefaultBranch through the credentialed remote-list path.
	_ = s.workRepo.Storer.RemoveReference(plumbing.ReferenceName("refs/remotes/origin/HEAD"))
	_ = s.workRepo.Storer.RemoveReference(plumbing.ReferenceName("refs/remotes/origin/main"))
	_ = s.workRepo.Storer.RemoveReference(plumbing.ReferenceName("refs/remotes/origin/master"))
	_ = s.workRepo.Storer.RemoveReference(plumbing.ReferenceName("refs/heads/main"))
	_ = s.workRepo.Storer.RemoveReference(plumbing.ReferenceName("refs/heads/master"))
	_ = s.workRepo.DeleteRemote("origin")
	_, _ = s.workRepo.CreateRemote(&gogitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://example.invalid/tagoro9/fotingo.git"},
	})

	_, err := s.gitClient.GetDefaultBranch()
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to get credentials")
}

// --- DoesBranchExistInRemote tests ---

func (s *GitCredentialTestSuite) TestDoesBranchExistInRemote_Found() {
	exists, err := s.gitClient.DoesBranchExistInRemote("master")
	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

func (s *GitCredentialTestSuite) TestDoesBranchExistInRemote_NotFound() {
	exists, err := s.gitClient.DoesBranchExistInRemote("nonexistent-branch")
	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *GitCredentialTestSuite) TestDoesBranchExistInRemote_CredentialError() {
	s.gitClient.credentialProvider = &failingCredentialProvider{}
	_, err := s.gitClient.DoesBranchExistInRemote("master")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to get credentials")
}

// --- GetCommitsSinceDefaultBranch tests ---

func (s *GitCredentialTestSuite) TestGetCommitsSinceDefaultBranch_NoNewCommits() {
	t := s.T()

	// Fetch first so the remote ref exists locally
	err := s.gitClient.fetch("origin")
	require.NoError(t, err)

	commits, err := s.gitClient.GetCommitsSinceDefaultBranch()
	assert.NoError(t, err)
	assert.Empty(t, commits)
}

func (s *GitCredentialTestSuite) TestGetCommitsSinceDefaultBranch_WithNewCommits() {
	t := s.T()

	// Fetch so remote ref is available
	err := s.gitClient.fetch("origin")
	require.NoError(t, err)

	// Create a new branch and add commits
	wt, err := s.workRepo.Worktree()
	require.NoError(t, err)

	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("feature-branch"),
		Create: true,
	})
	require.NoError(t, err)

	s.createFileAndCommit("feature.txt", "feature", "feat: TEST-100 new feature")
	s.createFileAndCommit("feature2.txt", "feature2", "fix: TEST-101 bug fix")

	commits, err := s.gitClient.GetCommitsSinceDefaultBranch()
	assert.NoError(t, err)
	assert.Len(t, commits, 2)
}

func (s *GitCredentialTestSuite) TestGetCommitsSinceDefaultBranch_UsesMergeBaseScope() {
	t := s.T()

	err := s.gitClient.fetch("origin")
	require.NoError(t, err)

	wt, err := s.workRepo.Worktree()
	require.NoError(t, err)

	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("feature-branch"),
		Create: true,
	})
	require.NoError(t, err)

	s.createFileAndCommit("feature.txt", "feature", "feat: TEST-200 new feature")
	s.createFileAndCommit("feature2.txt", "feature2", "fix: TEST-201 bug fix")

	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("master"),
	})
	require.NoError(t, err)

	s.createFileAndCommit("main-only.txt", "main-only", "chore: TEST-999 main only change")
	err = s.workRepo.Push(&gogit.PushOptions{
		RemoteName: "origin",
		RefSpecs: []gogitConfig.RefSpec{
			"refs/heads/master:refs/heads/master",
		},
	})
	require.NoError(t, err)

	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("feature-branch"),
	})
	require.NoError(t, err)

	err = s.gitClient.fetch("origin")
	require.NoError(t, err)

	commits, err := s.gitClient.GetCommitsSinceDefaultBranch()
	require.NoError(t, err)
	require.Len(t, commits, 2)
	assert.Equal(t, "fix: TEST-201 bug fix", commits[0].Message)
	assert.Equal(t, "feat: TEST-200 new feature", commits[1].Message)
}

// --- CreateIssueBranch tests ---

func (s *GitCredentialTestSuite) TestCreateIssueBranch_Success() {
	t := s.T()

	issue := &jira.Issue{
		Key:     "TEST-42",
		Summary: "Test feature",
		Type:    "Story",
	}

	branchName, err := s.gitClient.CreateIssueBranch(issue)
	assert.NoError(t, err)
	assert.Contains(t, branchName, "test-42")
	assert.Contains(t, branchName, "test_feature")
}

func (s *GitCredentialTestSuite) TestCreateIssueBranch_BranchAlreadyExists() {
	t := s.T()

	issue := &jira.Issue{
		Key:     "TEST-99",
		Summary: "Duplicate branch",
		Type:    "Bug",
	}

	// Create the branch first
	_, err := s.gitClient.CreateIssueBranch(issue)
	require.NoError(t, err)

	// Checkout back to master
	wt, err := s.workRepo.Worktree()
	require.NoError(t, err)
	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("master"),
	})
	require.NoError(t, err)

	// Try to create same branch again
	_, err = s.gitClient.CreateIssueBranch(issue)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// --- NewWithCredentialProvider tests ---

func (s *GitCredentialTestSuite) TestNewWithCredentialProvider_NilConfig() {
	_, err := NewWithCredentialProvider(nil, &s.messages, &mockCredentialProvider{})
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "config cannot be nil")
}

func (s *GitCredentialTestSuite) TestNewWithCredentialProvider_NilMessages() {
	viperCfg := config.NewDefaultConfig()
	_, err := NewWithCredentialProvider(viperCfg, nil, &mockCredentialProvider{})
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "messages channel cannot be nil")
}

func (s *GitCredentialTestSuite) TestNewWithCredentialProvider_NilProvider() {
	viperCfg := config.NewDefaultConfig()
	_, err := NewWithCredentialProvider(viperCfg, &s.messages, nil)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "credential provider cannot be nil")
}

func TestGitCredentialSuite(t *testing.T) {
	suite.Run(t, new(GitCredentialTestSuite))
}
