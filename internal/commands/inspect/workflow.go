package inspect

import (
	"fmt"

	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/github"
	"github.com/tagoro9/fotingo/internal/jira"
)

// CommitInfo represents one commit in inspect command output.
type CommitInfo struct {
	Hash    string
	Message string
	Author  string
}

// BranchInfo represents branch-related inspect command output.
type BranchInfo struct {
	Name          string
	IssueID       string
	DefaultBranch string
}

// PullRequestInfo represents pull-request-related inspect command output.
type PullRequestInfo struct {
	Title       string
	Description string
	Number      int
	URL         string
	APIURL      string
	State       string
	Draft       bool
}

// IssueInfo represents issue-related inspect command output.
type IssueInfo struct {
	Key         string
	Summary     string
	Description string
	Status      string
	Type        string
	ParentKey   string
	EpicKey     string
	URL         string
}

// WorkflowResult is the internal inspect workflow result.
type WorkflowResult struct {
	Branch      *BranchInfo
	Issue       *IssueInfo
	PullRequest *PullRequestInfo
	Discussion  *github.PullRequestDiscussion
	IssueIDs    []string
	Commits     []CommitInfo
}

// WorkflowOptions controls inspect workflow selection behavior.
type WorkflowOptions struct {
	Branch string
	Issue  string
}

// PullRequestInspector resolves pull requests and discussion context for inspect pr.
type PullRequestInspector interface {
	DoesPRExistForBranch(branch string) (bool, *github.PullRequest, error)
	GetPullRequestDiscussion(prNumber int) (*github.PullRequestDiscussion, error)
}

// WorkflowDeps are inspect workflow dependencies.
type WorkflowDeps struct {
	NewGitClient     func(*viper.Viper, *chan string) (git.Git, error)
	NewGitHubClient  func(git.Git, *viper.Viper) (PullRequestInspector, error)
	NewJiraClient    func(*viper.Viper) (jira.Jira, error)
	FetchBranchIssue func(jira.Jira, string) (*jira.Issue, error)
}

// WorkflowRunner executes the inspect workflow.
type WorkflowRunner struct {
	Config  *viper.Viper
	Options WorkflowOptions
	Deps    WorkflowDeps
}

// Run executes inspect workflow and returns structured result data.
func (r WorkflowRunner) Run() (WorkflowResult, error) {
	output := WorkflowResult{}

	statusCh := make(chan string, 10)
	gitClient, err := r.Deps.NewGitClient(r.Config, &statusCh)
	if err != nil {
		return WorkflowResult{}, err
	}

	if r.Options.Issue != "" {
		jiraClient, err := r.Deps.NewJiraClient(r.Config)
		if err != nil {
			return WorkflowResult{}, err
		}

		issue, err := jiraClient.GetJiraIssue(r.Options.Issue)
		if err != nil {
			return WorkflowResult{}, err
		}

		output.Issue = &IssueInfo{
			Key:         issue.Key,
			Summary:     issue.Summary,
			Description: issue.Description,
			Status:      issue.Status,
			Type:        issue.Type,
			ParentKey:   issue.ParentKey,
			EpicKey:     issue.EpicKey,
			URL:         jiraClient.GetIssueURL(issue.Key),
		}
		return output, nil
	}

	branchName := r.Options.Branch
	if branchName == "" {
		branchName, err = gitClient.GetCurrentBranch()
		if err != nil {
			return WorkflowResult{}, err
		}
	}

	output.Branch = &BranchInfo{Name: branchName}

	defaultBranch, err := gitClient.GetDefaultBranch()
	if err == nil {
		output.Branch.DefaultBranch = defaultBranch
	}

	issueID, err := gitClient.GetIssueId()
	if err == nil {
		output.Branch.IssueID = issueID
	}

	if defaultBranch != "" && branchName != defaultBranch {
		commits, err := gitClient.GetCommitsSinceDefaultBranch()
		if err == nil {
			output.Commits = make([]CommitInfo, len(commits))
			for i, c := range commits {
				output.Commits[i] = CommitInfo{
					Hash:    c.Hash,
					Message: c.Message,
					Author:  c.Author,
				}
			}
			output.IssueIDs = ExtractIssueIDsFromCommits(commits)
		}
	}

	if output.Branch.IssueID != "" {
		jiraClient, err := r.Deps.NewJiraClient(r.Config)
		if err == nil {
			issue, err := r.Deps.FetchBranchIssue(jiraClient, output.Branch.IssueID)
			if err == nil {
				output.Issue = &IssueInfo{
					Key:         issue.Key,
					Summary:     issue.Summary,
					Description: issue.Description,
					Status:      issue.Status,
					Type:        issue.Type,
					ParentKey:   issue.ParentKey,
					EpicKey:     issue.EpicKey,
					URL:         jiraClient.GetIssueURL(issue.Key),
				}
			}
		}
	}

	if r.Deps.NewGitHubClient != nil {
		ghClient, err := r.Deps.NewGitHubClient(gitClient, r.Config)
		if err == nil {
			exists, pr, err := ghClient.DoesPRExistForBranch(branchName)
			if err == nil && exists && pr != nil {
				url := pr.HTMLURL
				if url == "" {
					url = pr.URL
				}
				output.PullRequest = &PullRequestInfo{
					Number:      pr.Number,
					Title:       pr.Title,
					Description: pr.Body,
					URL:         url,
					APIURL:      pr.URL,
					State:       pr.State,
					Draft:       pr.Draft,
				}
			}
		}
	}

	return output, nil
}

// RunPullRequest executes PR discussion inspection and returns structured result data.
func (r WorkflowRunner) RunPullRequest() (WorkflowResult, error) {
	statusCh := make(chan string, 10)
	gitClient, err := r.Deps.NewGitClient(r.Config, &statusCh)
	if err != nil {
		return WorkflowResult{}, err
	}
	if r.Deps.NewGitHubClient == nil {
		return WorkflowResult{}, fmt.Errorf("inspect workflow dependency NewGitHubClient is required")
	}

	branchName := r.Options.Branch
	if branchName == "" {
		branchName, err = gitClient.GetCurrentBranch()
		if err != nil {
			return WorkflowResult{}, err
		}
	}

	ghClient, err := r.Deps.NewGitHubClient(gitClient, r.Config)
	if err != nil {
		return WorkflowResult{}, err
	}

	exists, pr, err := ghClient.DoesPRExistForBranch(branchName)
	if err != nil {
		return WorkflowResult{}, err
	}
	if !exists || pr == nil {
		return WorkflowResult{}, fmt.Errorf("no open pull request found for branch %s", branchName)
	}

	discussion, err := ghClient.GetPullRequestDiscussion(pr.Number)
	if err != nil {
		return WorkflowResult{}, err
	}

	return WorkflowResult{
		Branch: &BranchInfo{Name: branchName},
		PullRequest: &PullRequestInfo{
			Title:       pr.Title,
			Description: pr.Body,
			Number:      pr.Number,
			URL:         pr.HTMLURL,
			APIURL:      pr.URL,
			State:       pr.State,
			Draft:       pr.Draft,
		},
		Discussion: discussion,
	}, nil
}
