package git

import (
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// Bound automatic recovery to five fetches, adding at most 992 ancestry levels.
var shallowDeepenSteps = [...]int{32, 64, 128, 256, 512}

func (g *git) fetchShallowDefaultBranch(remote, branch string) error {
	root, err := g.worktreeRoot()
	if err != nil {
		return err
	}
	refspec := fmt.Sprintf("refs/heads/%s:refs/remotes/%s/%s", branch, remote, branch)
	_, stderr, err := execGitCommand(root, gitNonInteractiveEnv(), "fetch", "--depth=1", "--no-tags", remote, refspec)
	if err != nil {
		return fmt.Errorf("failed to fetch shallow default branch %s: %s: %w", branch, stderr, err)
	}
	return nil
}

// getShallowCommitsSince pins both tips and deepens only relevant shallow boundaries.
// Native Git honors shallow roots, unlike object traversal through absent parents.
func (g *git) getShallowCommitsSince(head, ref plumbing.Hash) ([]Commit, error) {
	root, err := g.worktreeRoot()
	if err != nil {
		return nil, err
	}
	previousBoundaries := ""
	for attempt := 0; ; attempt++ {
		boundaries, err := g.comparisonShallowBoundaries(root, head, ref)
		if err != nil {
			return nil, err
		}
		if len(boundaries) == 0 {
			return g.readShallowCommitRange(root, head, ref)
		}
		if attempt == len(shallowDeepenSteps) {
			return nil, fmt.Errorf("shallow comparison is still incomplete after %d fetches; deepen the relevant history manually and retry", attempt)
		}
		key := strings.Join(boundaries, " ")
		if key == previousBoundaries {
			return nil, fmt.Errorf("remote supplied no additional shallow history; deepen from a complete remote and retry")
		}
		previousBoundaries = key
		remote := strings.TrimSpace(g.GetConfigString("remote"))
		if remote == "" {
			return nil, fmt.Errorf("cannot deepen shallow history: git remote is not configured")
		}
		// Boundary commits came from the remote, even when HEAD contains unpushed commits.
		// Fetch hashes without destinations so local branches and comparison tips stay fixed.
		args := []string{"fetch", "--deepen=" + strconv.Itoa(shallowDeepenSteps[attempt]), "--no-tags", "--no-write-fetch-head", remote}
		args = append(args, boundaries...)
		_, stderr, err := execGitCommand(root, gitNonInteractiveEnv(), args...)
		if err != nil {
			return nil, fmt.Errorf("failed to deepen shallow history; fetch the required history and retry: %s: %w", stderr, err)
		}
	}
}

// comparisonShallowBoundaries returns truncated paths on either side of the comparison
// that are not excluded by a known common ancestor. Finding a merge base alone is
// insufficient: a merged side branch may still hide commits behind a shallow root.
func (g *git) comparisonShallowBoundaries(root string, head, ref plumbing.Hash) ([]string, error) {
	bases, stderr, err := execGitCommand(root, gitNonInteractiveEnv(), "merge-base", "--all", head.String(), ref.String())
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return nil, fmt.Errorf("failed to resolve shallow merge base: %s: %w", stderr, err)
		}
	}
	args := []string{"rev-list", head.String(), ref.String()}
	if bases != "" {
		args = append(args, "--not")
		args = append(args, strings.Fields(bases)...)
	}
	visible, stderr, err := execGitCommand(root, gitNonInteractiveEnv(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect shallow comparison: %s: %w", stderr, err)
	}
	shallow, err := g.repo.Storer.Shallow()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect shallow history: %w", err)
	}
	shallowSet := make(map[string]bool, len(shallow))
	for _, hash := range shallow {
		shallowSet[hash.String()] = true
	}
	var boundaries []string
	for _, hash := range strings.Fields(visible) {
		if shallowSet[hash] {
			boundaries = append(boundaries, hash)
		}
	}
	if bases == "" && len(boundaries) == 0 {
		return nil, fmt.Errorf("failed to resolve merge base: no common ancestor")
	}
	sort.Strings(boundaries)
	return boundaries, nil
}

func (g *git) readShallowCommitRange(root string, head, ref plumbing.Hash) ([]Commit, error) {
	output, stderr, err := execGitCommand(root, gitNonInteractiveEnv(), "rev-list", head.String(), "--not", ref.String())
	if err != nil {
		return nil, fmt.Errorf("failed to read shallow commit range: %s: %w", stderr, err)
	}
	var commits []Commit
	for _, hash := range strings.Fields(output) {
		commit, err := g.repo.CommitObject(plumbing.NewHash(hash))
		if err != nil {
			return nil, fmt.Errorf("failed to read comparison commit %s: %w", hash, err)
		}
		additions, deletions := getCommitLineStats(commit)
		commits = append(commits, Commit{Hash: hash, Message: strings.TrimSpace(commit.Message), Author: commit.Author.Name, Additions: additions, Deletions: deletions})
	}
	return commits, nil
}
