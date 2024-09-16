package open

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
)

// ResolveRepoURL returns the remote repository URL for open repo workflows.
func ResolveRepoURL(gitClient git.Git) (string, error) {
	repository, err := gitClient.GetRemote()
	if err != nil {
		return "", err
	}
	return repository.GetURL().String(), nil
}

// ResolveBranchURL returns the hosted branch URL for open branch workflows.
func ResolveBranchURL(gitClient git.Git, unsupportedHostErr func(host string) error) (string, error) {
	repository, err := gitClient.GetRemote()
	if err != nil {
		return "", err
	}
	branch, err := gitClient.GetCurrentBranch()
	if err != nil {
		return "", err
	}
	switch repository.GetHostName() {
	case "github.com":
		return fmt.Sprintf("%s/tree/%s", repository.GetURL().String(), branch), nil
	default:
		return "", unsupportedHostErr(repository.GetHostName())
	}
}

// ResolvePRURL returns the pull request URL for open pr workflows.
func ResolvePRURL(githubClient github.Github) (string, error) {
	url, err := githubClient.GetPullRequestUrl()
	if err != nil {
		return "", err
	}
	return url, nil
}

// MapPRError translates open-pr errors into user-facing error variants.
func MapPRError(
	err error,
	noPRPattern *regexp.Regexp,
	noPRForBranchErr func(branch string, cause error) error,
	wrapErr func(cause error) error,
) error {
	if err == nil {
		return nil
	}

	matches := noPRPattern.FindStringSubmatch(err.Error())
	if len(matches) == 2 {
		branch := strings.TrimSpace(matches[1])
		if branch != "" {
			return noPRForBranchErr(branch, err)
		}
	}

	return wrapErr(err)
}
