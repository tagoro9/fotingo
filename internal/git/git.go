package git

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	gogitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	giturl "github.com/kubescape/go-git-url"
	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/config"
	"github.com/tagoro9/fotingo/internal/jira"
)

// Commit represents a git commit with its metadata
type Commit struct {
	Hash      string
	Message   string
	Author    string
	Additions int
	Deletions int
}

type Git interface {
	config.ConfigurableService
	// GetRemote returns the default git remote
	GetRemote() (giturl.IGitURL, error)
	// GetCurrentBranch returns the current branch name
	GetCurrentBranch() (string, error)
	// GetIssueId extracts the issue id from the current branch
	GetIssueId() (string, error)
	// CreateIssueBranch creates a new branch for the given issue
	CreateIssueBranch(issue *jira.Issue) (string, error)
	// Push pushes the current branch to the remote with upstream tracking
	Push() error
	// StashChanges stashes uncommitted changes with the provided message
	StashChanges(message string) error
	// PopStash pops the most recent stash
	PopStash() error
	// HasUncommittedChanges checks if there are uncommitted changes (staged or unstaged)
	HasUncommittedChanges() (bool, error)
	// GetCommitsSince returns commits since the given reference
	GetCommitsSince(ref string) ([]Commit, error)
	// DoesBranchExistInRemote checks if a branch exists on the remote
	DoesBranchExistInRemote(branch string) (bool, error)
	// GetDefaultBranch returns the default branch (usually main or master)
	GetDefaultBranch() (string, error)
	// FetchDefaultBranch fetches the remote default branch and refreshes local tracking refs.
	FetchDefaultBranch() error
	// GetCommitsSinceDefaultBranch returns commits since the current branch diverged from the default branch
	GetCommitsSinceDefaultBranch() ([]Commit, error)
	// GetIssuesFromCommits extracts issue IDs from commit messages
	GetIssuesFromCommits(commits []Commit) []string
}

// CredentialProvider abstracts git credential retrieval for testability.
type CredentialProvider interface {
	GetCredentials(remoteURL string) (*http.BasicAuth, error)
}

// RemoteConfigurable allows reconfiguring a remote URL on a Git client.
// This is used in tests to point the remote to a local bare repo for operations
// while keeping the original URL for display purposes.
type RemoteConfigurable interface {
	ReconfigureRemoteURL(remoteName, newURL string)
}

// execCredentialProvider retrieves credentials using the git credential fill command.
type execCredentialProvider struct {
	dir string
}

type git struct {
	repo *gogit.Repository
	*config.ViperConfigurableService
	messages           *chan string
	credentialProvider CredentialProvider
}

var execGitCommand = func(dir string, env []string, args ...string) (string, string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

// ReconfigureRemoteURL replaces all URLs for the named remote.
func (g *git) ReconfigureRemoteURL(remoteName, newURL string) {
	_ = g.repo.DeleteRemote(remoteName)
	_, _ = g.repo.CreateRemote(&gogitConfig.RemoteConfig{
		Name: remoteName,
		URLs: []string{newURL},
	})
}

func (g *git) GetRemote() (giturl.IGitURL, error) {
	remoteName := g.GetConfigString("remote")
	remote, remoteURL, err := g.getConfiguredRemote(remoteName)
	if err != nil {
		return nil, err
	}
	if remoteURL == "" {
		remoteURL = firstRemoteURL(remote)
	}
	gitUrl, err := giturl.NewGitURL(remoteURL)
	if err != nil {
		return nil, err
	}
	return gitUrl, nil
}

func (g *git) GetCurrentBranch() (string, error) {
	head, err := g.repo.Head()
	if err != nil {
		return "", normalizeRepositoryStateError(err)
	}
	if head.Name().IsBranch() {
		return head.Name().Short(), nil
	}
	return "", fmt.Errorf("HEAD is not pointing to a branch")
}

var IssueType = "ISSUE_TYPE"
var IssueKey = "ISSUE_KEY"
var IssueSanitizedSummary = "ISSUE_SANITIZED_SUMMARY"

// Define regex patterns for known template placeholders
var placeholderPatterns = map[string]struct {
	pattern string
	name    string
}{
	IssueType: {
		pattern: fmt.Sprintf(`(?P<%s>\w+)`, IssueType),
		name:    IssueType,
	},

	IssueKey: {
		pattern: fmt.Sprintf(`(?P<%s>\w+-\d+)`, IssueKey),
		name:    IssueKey,
	},
	IssueSanitizedSummary: {
		pattern: fmt.Sprintf(`(?P<%s>[\w-]+)`, IssueSanitizedSummary),
		name:    IssueSanitizedSummary,
	},
}

var fallbackIssueIDPattern = regexp.MustCompile(`(?i)(^|[^a-z0-9])([a-z][a-z0-9_]+-\d+)($|[^a-z0-9])`)

// TODO Is this needed now that we have the issue struct?
type TemplateIssue struct {
	ShortName        string
	Key              string
	Info             string
	SanitizedSummary string
}

// parseTemplateToRegex converts a user-defined template into a regex pattern
func parseTemplateToRegex(branchTemplate string) string {
	templateData := TemplateIssue{
		ShortName:        placeholderPatterns[IssueType].pattern,
		Key:              placeholderPatterns[IssueKey].pattern,
		Info:             placeholderPatterns[IssueKey].pattern,
		SanitizedSummary: placeholderPatterns[IssueSanitizedSummary].pattern,
	}
	// Render template as a go template passing templateData
	t := template.New("branch")
	t, err := t.Parse(branchTemplate)
	if err != nil {
		return ""
	}
	var data bytes.Buffer
	err = t.Execute(&data, struct {
		Issue TemplateIssue
	}{Issue: templateData})
	if err != nil {
		return ""
	}

	// Add anchors to match the entire string
	return "^" + data.String() + "$"
}

// extractValues uses regex to match and extract values based on the template
func extractValues(template, input string) (map[string]string, error) {
	regexPattern := parseTemplateToRegex(template)
	re := regexp.MustCompile(regexPattern)

	matches := re.FindStringSubmatch(input)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no matches found for input: %s", input)
	}

	result := make(map[string]string)
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = matches[i]
		}
	}

	return result, nil
}

func (g *git) GetIssueId() (string, error) {
	branch, err := g.GetCurrentBranch()
	if err != nil {
		return "", err
	}

	branchTemplate := g.GetConfigString("branchTemplate")
	var templateErr error
	if strings.TrimSpace(branchTemplate) != "" {
		values, err := extractValues(branchTemplate, branch)
		if err != nil {
			templateErr = err
		} else {
			issueID, ok := values[IssueKey]
			if ok && strings.TrimSpace(issueID) != "" {
				return normalizeIssueID(issueID), nil
			}
		}
	}

	issueID, ok := extractIssueIDFromBranchName(branch)
	if ok {
		return issueID, nil
	}

	if templateErr != nil {
		return "", fmt.Errorf("no issue id found in branch name: %s: %w", branch, templateErr)
	}
	return "", fmt.Errorf("no issue id found in branch name: %s", branch)
}

func extractIssueIDFromBranchName(branch string) (string, bool) {
	matches := fallbackIssueIDPattern.FindAllStringSubmatch(branch, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		candidate := strings.TrimSpace(match[2])
		if candidate == "" {
			continue
		}
		return normalizeIssueID(candidate), true
	}

	return "", false
}

func normalizeIssueID(issueID string) string {
	clean := strings.TrimSpace(issueID)
	parts := strings.SplitN(clean, "-", 2)
	if len(parts) != 2 {
		return clean
	}

	return strings.ToUpper(parts[0]) + "-" + parts[1]
}

// GetCredentials returns HTTP Basic Auth credentials for the given remote URL by invoking git credential fill
func (e *execCredentialProvider) GetCredentials(remoteURL string) (*http.BasicAuth, error) {
	cmd := exec.Command("git", "credential", "fill")
	cmd.Env = gitNonInteractiveEnv()
	if e.dir != "" {
		cmd.Dir = e.dir
	}

	input := fmt.Sprintf("url=%s\n\n", remoteURL)
	stdin := bytes.NewBufferString(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdin = stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrOutput := strings.TrimSpace(stderr.String())
		if stderrOutput == "" {
			return nil, fmt.Errorf(
				"git credential helper failed in non-interactive mode for %s: %w",
				remoteURL,
				err,
			)
		}
		return nil, fmt.Errorf(
			"git credential helper failed in non-interactive mode for %s: %s",
			remoteURL,
			stderrOutput,
		)
	}

	// Parse the output which is in key=value format
	credentials := make(map[string]string)
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			credentials[parts[0]] = parts[1]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read git credential output for %s: %w", remoteURL, err)
	}

	username := credentials["username"]
	password := credentials["password"]

	if username == "" || password == "" {
		return nil, fmt.Errorf(
			"no usable git credentials found for %s in non-interactive mode; "+
				"configure a credential helper or authenticate with Git before running fotingo",
			remoteURL,
		)
	}
	if looksLikeCredentialPrompt(username) || looksLikeCredentialPrompt(password) {
		return nil, fmt.Errorf(
			"git credential helper returned interactive prompt text instead of stored credentials for %s; "+
				"configure a credential helper or authenticate with Git before running fotingo",
			remoteURL,
		)
	}

	return &http.BasicAuth{
		Username: username,
		Password: password,
	}, nil
}

func looksLikeCredentialPrompt(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}

	return strings.HasPrefix(trimmed, "Username for '") || strings.HasPrefix(trimmed, "Password for '")
}

// firstRemoteURL returns the first configured URL for a remote, if available.
func firstRemoteURL(remoteRef *gogit.Remote) string {
	if remoteRef == nil {
		return ""
	}
	config := remoteRef.Config()
	if config == nil || len(config.URLs) == 0 {
		return ""
	}
	return strings.TrimSpace(config.URLs[0])
}

// worktreeRoot returns the filesystem root for the repository worktree.
func (g *git) worktreeRoot() (string, error) {
	worktree, err := g.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	return worktree.Filesystem.Root(), nil
}

// getRemoteURLFromGitCommand resolves a remote URL using the git CLI in the current worktree.
func (g *git) getRemoteURLFromGitCommand(remote string) (string, error) {
	worktreeRoot, err := g.worktreeRoot()
	if err != nil {
		return "", err
	}

	stdout, stderr, err := execGitCommand(worktreeRoot, gitNonInteractiveEnv(), "remote", "get-url", remote)
	if err != nil {
		if stderr == "" {
			return "", fmt.Errorf("failed to inspect remote %s: %w", remote, err)
		}
		return "", fmt.Errorf("failed to inspect remote %s: %s", remote, stderr)
	}
	if stdout == "" {
		return "", fmt.Errorf("remote %s has no configured URL", remote)
	}

	return stdout, nil
}

// getConfiguredRemote resolves a usable remote, falling back to git CLI inspection when needed.
func (g *git) getConfiguredRemote(remote string) (*gogit.Remote, string, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return nil, "", fmt.Errorf("git remote is not configured")
	}

	if remoteRef, err := g.repo.Remote(remote); err == nil {
		remoteURL := firstRemoteURL(remoteRef)
		if remoteURL == "" {
			return nil, "", fmt.Errorf("remote %s has no configured URL", remote)
		}
		return remoteRef, remoteURL, nil
	}

	remoteURL, err := g.getRemoteURLFromGitCommand(remote)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get remote %s: %w", remote, err)
	}

	return gogit.NewRemote(g.repo.Storer, &gogitConfig.RemoteConfig{
		Name:  remote,
		URLs:  []string{remoteURL},
		Fetch: []gogitConfig.RefSpec{gogitConfig.RefSpec(fmt.Sprintf("+refs/heads/*:refs/remotes/%s/*", remote))},
	}), remoteURL, nil
}

// fetch fetches updates from the specified remote repository
func (g *git) fetch(remote string) error {
	remoteRef, remoteURL, err := g.getConfiguredRemote(remote)
	if err != nil {
		return err
	}

	credentials, err := g.credentialProvider.GetCredentials(remoteURL)
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	err = remoteRef.Fetch(&gogit.FetchOptions{
		RemoteName: remote,
		Auth:       credentials,
	})

	if err == gogit.NoErrAlreadyUpToDate {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to fetch from remote: %w", err)
	}

	return nil
}

// branchExists checks if a branch exists in the repository (local or remote)
func (g *git) branchExists(branchName string) (bool, error) {
	// First check local branches
	branches, err := g.repo.Branches()
	if err != nil {
		return false, fmt.Errorf("failed to list branches: %w", err)
	}

	exists := false
	err = branches.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().Short() == branchName {
			exists = true
			return nil
		}
		return nil
	})

	if err != nil {
		return false, fmt.Errorf("failed to iterate branches: %w", err)
	}

	if exists {
		return true, nil
	}

	// Then check cached remote branches first to avoid unnecessary network calls.
	remote := g.GetConfigString("remote")
	remoteBranchRef := plumbing.NewRemoteReferenceName(remote, branchName)
	if _, err := g.repo.Reference(remoteBranchRef, true); err == nil {
		return true, nil
	}

	remoteBranches, err := g.repo.References()
	if err != nil {
		return false, fmt.Errorf("failed to list remote references: %w", err)
	}
	remotePrefix := fmt.Sprintf("refs/remotes/%s/", remote)
	hasCachedRemoteRefs := false
	err = remoteBranches.ForEach(func(ref *plumbing.Reference) error {
		if strings.HasPrefix(ref.Name().String(), remotePrefix) {
			hasCachedRemoteRefs = true
			return storer.ErrStop
		}
		return nil
	})
	if err != nil && err != storer.ErrStop {
		return false, fmt.Errorf("failed to inspect remote references: %w", err)
	}
	if hasCachedRemoteRefs {
		return false, nil
	}

	// No cached refs available, fetch and check remote branches.
	err = g.fetch(remote)
	if err != nil {
		return false, fmt.Errorf("failed to fetch from remote: %w", err)
	}

	remoteBranches, err = g.repo.References()
	if err != nil {
		return false, fmt.Errorf("failed to list remote references: %w", err)
	}

	err = remoteBranches.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().Short() == fmt.Sprintf("%s/%s", remote, branchName) {
			exists = true
			return nil
		}
		return nil
	})

	if err != nil {
		return false, fmt.Errorf("failed to iterate remote references: %w", err)
	}

	return exists, nil
}

// GetDefaultBranch returns the default branch (usually main or master) of the repository
func (g *git) GetDefaultBranch() (string, error) {
	remote := strings.TrimSpace(g.GetConfigString("remote"))
	if remote == "" {
		return "", fmt.Errorf("git remote is not configured")
	}

	branch, showErr := g.getRemoteDefaultBranchFromGitCommand(remote)
	if showErr == nil {
		return branch, nil
	}

	// Prefer cached remote HEAD when available to avoid unnecessary network calls.
	cachedHeadRefName := plumbing.ReferenceName(fmt.Sprintf("refs/remotes/%s/HEAD", remote))
	if headRef, err := g.repo.Reference(cachedHeadRefName, false); err == nil {
		if headRef.Type() == plumbing.SymbolicReference {
			target := normalizeDefaultBranchName(remote, headRef.Target().Short())
			if target != "" {
				return target, nil
			}
		}
	}

	remoteRef, remoteURL, err := g.getConfiguredRemote(remote)
	if err != nil {
		return "", err
	}

	credentials, err := g.credentialProvider.GetCredentials(remoteURL)
	if err != nil {
		return "", fmt.Errorf(
			"failed to get credentials for remote %s while resolving default branch: %w",
			remote,
			err,
		)
	}

	refs, err := remoteRef.List(&gogit.ListOptions{
		Auth: credentials,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list remote refs for %s: %w", remote, err)
	}

	// Look for remote HEAD reference that points to the default branch.
	for _, ref := range refs {
		if ref.Name() != plumbing.HEAD {
			continue
		}
		target := normalizeDefaultBranchName(remote, ref.Target().Short())
		if target != "" && target != "HEAD" {
			return target, nil
		}
	}

	return "", fmt.Errorf("could not determine default branch for remote %s: %w", remote, showErr)
}

func (g *git) getRemoteDefaultBranchFromGitCommand(remote string) (string, error) {
	worktreeRoot, err := g.worktreeRoot()
	if err != nil {
		return "", err
	}

	stdout, stderr, err := execGitCommand(worktreeRoot, gitNonInteractiveEnv(), "remote", "show", remote)
	if err != nil {
		if stderr == "" {
			return "", fmt.Errorf("failed to inspect remote %s: %w", remote, err)
		}
		return "", fmt.Errorf("failed to inspect remote %s: %s", remote, stderr)
	}

	branch, err := parseRemoteHeadBranch(stdout)
	if err != nil {
		return "", fmt.Errorf("failed to parse remote %s HEAD branch: %w", remote, err)
	}

	return branch, nil
}

func parseRemoteHeadBranch(output string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "HEAD branch:") {
			continue
		}
		branch := strings.TrimSpace(strings.TrimPrefix(line, "HEAD branch:"))
		branch = strings.TrimPrefix(branch, "refs/heads/")
		if branch == "" || branch == "(unknown)" {
			return "", fmt.Errorf("remote HEAD branch is unavailable")
		}
		return branch, nil
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read remote output: %w", err)
	}

	return "", fmt.Errorf("HEAD branch line not found")
}

func gitNonInteractiveEnv() []string {
	env := os.Environ()
	env = setEnvValue(env, "GIT_TERMINAL_PROMPT", "0")
	env = setEnvValue(env, "GCM_INTERACTIVE", "Never")
	env = setEnvValue(env, "GIT_ASKPASS", "echo")
	env = setEnvValue(env, "SSH_ASKPASS", "echo")
	return env
}

func setEnvValue(env []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = prefix + value
			return env
		}
	}

	return append(env, prefix+value)
}

func normalizeRepositoryStateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, plumbing.ErrReferenceNotFound) || strings.Contains(err.Error(), "reference not found") {
		return fmt.Errorf("repository has no commits yet; create an initial commit first")
	}
	return err
}

func normalizeDefaultBranchName(remote, branch string) string {
	normalized := strings.TrimSpace(branch)
	normalized = strings.TrimPrefix(normalized, "refs/heads/")
	normalized = strings.TrimPrefix(normalized, fmt.Sprintf("%s/", remote))
	return normalized
}

func (g *git) CreateIssueBranch(issue *jira.Issue) (string, error) {
	branchName, err := g.buildBranchName(issue)
	if err != nil {
		return "", err
	}

	defaultBranch, err := g.GetDefaultBranch()
	if err != nil {
		return "", fmt.Errorf("failed to get default branch: %w", err)
	}

	exists, err := g.branchExists(branchName)
	if err != nil {
		return "", fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if exists {
		return "", fmt.Errorf("branch %s already exists", branchName)
	}
	// Get the worktree to create the branch
	worktree, err := g.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	remote := g.GetConfigString("remote")
	remoteDefaultRef := plumbing.NewRemoteReferenceName(remote, defaultBranch)
	branchRef, err := g.repo.Reference(remoteDefaultRef, true)
	if err != nil {
		// Fall back to local default branch ref before hitting the network.
		branchRef, err = g.repo.Reference(plumbing.NewBranchReferenceName(defaultBranch), true)
		if err != nil {
			(*g.messages) <- fmt.Sprintf("Fetching from remote %s", remote)
			err = g.fetch(remote)
			if err != nil {
				(*g.messages) <- fmt.Sprintf("Failed to fetch from remote: %s", err)
				return "", fmt.Errorf("failed to fetch from remote: %w", err)
			}

			branchRef, err = g.repo.Reference(remoteDefaultRef, true)
			if err != nil {
				(*g.messages) <- fmt.Sprintf("Failed to get default branch reference: %s", err)
				return "", fmt.Errorf("failed to get default branch reference: %w", err)
			}
		}
	}

	// TODO If there are modified files, create a stash with them
	// TODO Check if the branch already exists

	(*g.messages) <- fmt.Sprintf("Creating branch %s for issue %s", branchName, issue.Key)

	// Create and checkout the new branch from the default branch
	err = worktree.Checkout(&gogit.CheckoutOptions{
		Hash:   branchRef.Hash(),
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: true,
	})

	if err != nil {
		return "", fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}

	return branchName, nil
}

// FetchDefaultBranch refreshes remote tracking refs for the configured default branch.
func (g *git) FetchDefaultBranch() error {
	defaultBranch, err := g.GetDefaultBranch()
	if err != nil {
		return fmt.Errorf("failed to get default branch: %w", err)
	}

	remote := g.GetConfigString("remote")
	(*g.messages) <- fmt.Sprintf("Fetching from remote %s", remote)
	if err := g.fetch(remote); err != nil {
		return fmt.Errorf("failed to fetch from remote: %w", err)
	}

	remoteDefaultRef := plumbing.NewRemoteReferenceName(remote, defaultBranch)
	if _, err := g.repo.Reference(remoteDefaultRef, true); err != nil {
		return fmt.Errorf("failed to get default branch reference: %w", err)
	}

	return nil
}

func (g *git) buildBranchName(issue *jira.Issue) (string, error) {
	branchTemplate := g.GetConfigString("branchTemplate")
	if branchTemplate == "" {
		return "", fmt.Errorf("branch template not configured")
	}

	t, err := template.New("branch").Parse(branchTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse branch template: %w", err)
	}

	var data bytes.Buffer
	err = t.Execute(&data, struct {
		Issue *jira.Issue
	}{Issue: issue})
	if err != nil {
		return "", fmt.Errorf("failed to execute branch template: %w", err)
	}

	branchName := strings.ToLower(data.String())

	// Trim branch name to 244 characters to avoid git reference name issues
	if len(branchName) > 244 {
		branchName = branchName[:244]
	}

	return branchName, nil
}

// Push pushes the current branch to the remote with upstream tracking
func (g *git) Push() error {
	remote := g.GetConfigString("remote")

	currentBranch, err := g.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	remoteRef, remoteURL, err := g.getConfiguredRemote(remote)
	if err != nil {
		return err
	}

	credentials, err := g.credentialProvider.GetCredentials(remoteURL)
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	(*g.messages) <- fmt.Sprintf("Pushing branch %s to remote %s", currentBranch, remote)

	refSpec := gogitConfig.RefSpec(fmt.Sprintf(
		"refs/heads/%s:refs/heads/%s",
		currentBranch,
		currentBranch,
	))

	err = remoteRef.Push(&gogit.PushOptions{
		RemoteName: remote,
		RefSpecs:   []gogitConfig.RefSpec{refSpec},
		Auth:       credentials,
	})

	if err != nil && err != gogit.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to push to remote: %w", err)
	}

	if setErr := g.setBranchUpstream(remote, currentBranch); setErr != nil {
		return fmt.Errorf("failed to set upstream for branch %s: %w", currentBranch, setErr)
	}

	if err == gogit.NoErrAlreadyUpToDate {
		(*g.messages) <- "Branch is already up to date"
		return nil
	}

	return nil
}

func (g *git) setBranchUpstream(remote, branch string) error {
	repoConfig, err := g.repo.Config()
	if err != nil {
		return fmt.Errorf("failed to read repository config: %w", err)
	}

	if repoConfig.Branches == nil {
		repoConfig.Branches = make(map[string]*gogitConfig.Branch)
	}

	branchConfig, ok := repoConfig.Branches[branch]
	if !ok || branchConfig == nil {
		branchConfig = &gogitConfig.Branch{Name: branch}
		repoConfig.Branches[branch] = branchConfig
	}

	branchConfig.Remote = remote
	branchConfig.Merge = plumbing.NewBranchReferenceName(branch)

	if err := g.repo.SetConfig(repoConfig); err != nil {
		return fmt.Errorf("failed to write repository config: %w", err)
	}

	(*g.messages) <- fmt.Sprintf("Set upstream for %s to %s/%s", branch, remote, branch)
	return nil
}

// StashChanges stashes uncommitted changes with the provided message, including untracked files
func (g *git) StashChanges(message string) error {
	worktree, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// go-git does not have native stash support, so we use the git command directly
	// This includes untracked files (-u flag)
	cmd := exec.Command("git", "stash", "push", "-u", "-m", message)
	cmd.Dir = worktree.Filesystem.Root()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	(*g.messages) <- fmt.Sprintf("Stashing changes: %s", message)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stash changes: %w: %s", err, stderr.String())
	}

	return nil
}

// PopStash pops the most recent stash
func (g *git) PopStash() error {
	worktree, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// go-git does not have native stash support, so we use the git command directly
	cmd := exec.Command("git", "stash", "pop")
	cmd.Dir = worktree.Filesystem.Root()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	(*g.messages) <- "Popping stash"

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pop stash: %w: %s", err, stderr.String())
	}

	return nil
}

// HasUncommittedChanges checks if there are uncommitted changes (staged or unstaged)
func (g *git) HasUncommittedChanges() (bool, error) {
	worktree, err := g.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree status: %w", err)
	}

	globalIgnoreMatcher, err := g.loadGlobalIgnoreMatcher()
	if err != nil {
		return false, fmt.Errorf("failed to load global gitignore patterns: %w", err)
	}

	for path, fileStatus := range status {
		if fileStatus == nil {
			continue
		}

		if fileStatus.Staging == gogit.Unmodified && fileStatus.Worktree == gogit.Unmodified {
			continue
		}

		// Keep tracked/staged changes authoritative and only filter untracked noise.
		if !isGlobalIgnoreCandidate(fileStatus) {
			return true, nil
		}

		isDir, err := isPathDir(worktree.Filesystem, path)
		if err != nil {
			return false, fmt.Errorf("failed to inspect %q: %w", path, err)
		}

		if !matchesGitIgnore(globalIgnoreMatcher, path, isDir) {
			return true, nil
		}
	}

	return false, nil
}

func (g *git) loadGlobalIgnoreMatcher() (gitignore.Matcher, error) {
	patterns, err := gitignore.LoadGlobalPatterns(osfs.New("/"))
	if err != nil {
		return nil, err
	}
	if len(patterns) == 0 {
		return nil, nil
	}

	return gitignore.NewMatcher(patterns), nil
}

func isGlobalIgnoreCandidate(fileStatus *gogit.FileStatus) bool {
	return fileStatus.Worktree == gogit.Untracked &&
		(fileStatus.Staging == gogit.Untracked || fileStatus.Staging == gogit.Unmodified)
}

func isPathDir(filesystem billy.Filesystem, path string) (bool, error) {
	fileInfo, err := filesystem.Stat(path)
	if err == nil {
		return fileInfo.IsDir(), nil
	}
	if errorsIsNotExist(err) {
		return false, nil
	}
	return false, err
}

func matchesGitIgnore(matcher gitignore.Matcher, path string, isDir bool) bool {
	if matcher == nil {
		return false
	}

	cleanPath := filepath.ToSlash(filepath.Clean(path))
	segments := strings.Split(cleanPath, "/")
	return matcher.Match(segments, isDir)
}

func errorsIsNotExist(err error) bool {
	return err != nil && (os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist))
}

// GetCommitsSince returns commits since the given reference
func (g *git) GetCommitsSince(ref string) ([]Commit, error) {
	// Get HEAD first so empty repositories return a clear error.
	head, err := g.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", normalizeRepositoryStateError(err))
	}

	// Resolve the reference to a hash
	refHash, err := g.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve reference %s: %w", ref, err)
	}

	// Get commit iterator starting from HEAD
	commitIter, err := g.repo.Log(&gogit.LogOptions{
		From: head.Hash(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}

	var commits []Commit
	err = commitIter.ForEach(func(c *object.Commit) error {
		// Stop when we reach the reference commit
		if c.Hash == *refHash {
			return storer.ErrStop
		}

		additions, deletions := getCommitLineStats(c)
		commits = append(commits, Commit{
			Hash:      c.Hash.String(),
			Message:   strings.TrimSpace(c.Message),
			Author:    c.Author.Name,
			Additions: additions,
			Deletions: deletions,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return commits, nil
}

func getCommitLineStats(commit *object.Commit) (int, int) {
	stats, err := commit.Stats()
	if err != nil {
		return 0, 0
	}

	additions := 0
	deletions := 0
	for _, stat := range stats {
		additions += stat.Addition
		deletions += stat.Deletion
	}

	return additions, deletions
}

// DoesBranchExistInRemote checks if a branch exists on the remote
func (g *git) DoesBranchExistInRemote(branch string) (bool, error) {
	remote := g.GetConfigString("remote")

	remoteRef, remoteURL, err := g.getConfiguredRemote(remote)
	if err != nil {
		return false, err
	}

	credentials, err := g.credentialProvider.GetCredentials(remoteURL)
	if err != nil {
		return false, fmt.Errorf("failed to get credentials: %w", err)
	}

	refs, err := remoteRef.List(&gogit.ListOptions{
		Auth: credentials,
	})
	if err != nil {
		return false, fmt.Errorf("failed to list remote refs: %w", err)
	}

	branchRefName := plumbing.NewBranchReferenceName(branch)
	for _, ref := range refs {
		if ref.Name() == branchRefName {
			return true, nil
		}
	}

	return false, nil
}

// GetCommitsSinceDefaultBranch returns commits since the current branch diverged from the default branch
func (g *git) GetCommitsSinceDefaultBranch() ([]Commit, error) {
	defaultBranch, err := g.GetDefaultBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get default branch: %w", err)
	}

	remote := g.GetConfigString("remote")
	remoteRef := fmt.Sprintf("%s/%s", remote, defaultBranch)
	remoteHash, err := g.repo.ResolveRevision(plumbing.Revision(remoteRef))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve remote reference %s: %w", remoteRef, err)
	}

	head, err := g.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", normalizeRepositoryStateError(err))
	}

	headCommit, err := g.repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve HEAD commit: %w", err)
	}

	defaultCommit, err := g.repo.CommitObject(*remoteHash)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve default branch commit %s: %w", remoteRef, err)
	}

	mergeBases, err := headCommit.MergeBase(defaultCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve merge base with %s: %w", remoteRef, err)
	}
	if len(mergeBases) == 0 {
		return nil, fmt.Errorf("failed to resolve merge base with %s: no common ancestor", remoteRef)
	}

	return g.GetCommitsSince(mergeBases[0].Hash.String())
}

// issueIDPatternInCommit matches Jira-style issue IDs in commit messages (e.g., "PROJECT-123")
var issueIDPatternInCommit = regexp.MustCompile(`\b([A-Z][A-Z0-9_]+-\d+)\b`)

// GetIssuesFromCommits extracts unique issue IDs from commit messages
// It looks for patterns like "fixes PROJECT-123", "PROJECT-123", etc.
func (g *git) GetIssuesFromCommits(commits []Commit) []string {
	seen := make(map[string]struct{})
	var issues []string

	for _, commit := range commits {
		matches := issueIDPatternInCommit.FindAllStringSubmatch(commit.Message, -1)
		for _, match := range matches {
			if len(match) > 1 {
				issueID := strings.ToUpper(match[1])
				if _, exists := seen[issueID]; !exists {
					seen[issueID] = struct{}{}
					issues = append(issues, issueID)
				}
			}
		}
	}

	return issues
}

func repoWorkingDir(repo *gogit.Repository) (string, error) {
	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	return worktree.Filesystem.Root(), nil
}

// findClosestRepository finds the closest Git repository to the given path and returns a Repository instance
// of it
func findClosestRepository(path string) (*gogit.Repository, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	for {
		repo, err := gogit.PlainOpenWithOptions(absPath, &gogit.PlainOpenOptions{
			DetectDotGit:          true,
			EnableDotGitCommonDir: true,
		})
		if err == nil {
			return repo, nil
		}

		// Move up one directory
		parentDir := filepath.Dir(absPath)
		// Reached the root directory
		if parentDir == absPath {
			return nil, fmt.Errorf("no Git repository found in %s or any parent directory", path)
		}
		absPath = parentDir
	}
}

// New returns a new instance of a Git client in the current working directory
func New(cfg *viper.Viper, messages *chan string) (Git, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if messages == nil {
		return nil, fmt.Errorf("messages channel cannot be nil")
	}

	configurableService := &config.ViperConfigurableService{Config: cfg, Prefix: "git"}
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	repo, err := findClosestRepository(dir)
	// TODO Make this work outside a git repo (maybe storing a flag?)
	if err != nil {
		return nil, err
	}
	worktreeDir, err := repoWorkingDir(repo)
	if err != nil {
		return nil, err
	}
	return &git{
		ViperConfigurableService: configurableService,
		repo:                     repo,
		messages:                 messages,
		credentialProvider:       &execCredentialProvider{dir: worktreeDir},
	}, nil
}

// NewWithCredentialProvider returns a new Git client with a custom credential provider.
// This is useful for testing functions that require credentials without invoking git credential fill.
func NewWithCredentialProvider(cfg *viper.Viper, messages *chan string, cp CredentialProvider) (Git, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if messages == nil {
		return nil, fmt.Errorf("messages channel cannot be nil")
	}
	if cp == nil {
		return nil, fmt.Errorf("credential provider cannot be nil")
	}

	configurableService := &config.ViperConfigurableService{Config: cfg, Prefix: "git"}
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	repo, err := findClosestRepository(dir)
	if err != nil {
		return nil, err
	}
	return &git{
		ViperConfigurableService: configurableService,
		repo:                     repo,
		messages:                 messages,
		credentialProvider:       cp,
	}, nil
}
