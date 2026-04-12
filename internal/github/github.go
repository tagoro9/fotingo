package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cli/oauth"
	hub "github.com/google/go-github/v84/github"
	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/auth"
	"github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/config"
	"github.com/tagoro9/fotingo/internal/git"
	"github.com/tagoro9/fotingo/internal/ui"
)

// CreatePROptions contains the options for creating a pull request
type CreatePROptions struct {
	Title string
	Body  string
	Head  string
	Base  string
	Draft bool
}

// UpdatePROptions contains optional fields for editing an existing pull request.
type UpdatePROptions struct {
	Title *string
	Body  *string
}

// PullRequestBodyUpdate contains a PR body update for stack synchronization.
type PullRequestBodyUpdate struct {
	Number int
	Body   string
}

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Title     string
	Body      string
	Number    int
	NodeID    string
	URL       string
	HTMLURL   string
	HeadRef   string
	BaseRef   string
	Draft     bool
	State     string
	Merged    bool
	Mergeable *bool
}

// PullRequestDiscussion contains comments, reviews, and review conversations for a pull request.
type PullRequestDiscussion struct {
	Comments       []PullRequestIssueComment
	Reviews        []PullRequestReview
	ReviewComments []PullRequestReviewComment
	Conversations  []PullRequestConversation
}

// PullRequestIssueComment represents a top-level pull request issue comment.
type PullRequestIssueComment struct {
	ID                int64
	Author            string
	Body              string
	URL               string
	HTMLURL           string
	AuthorAssociation string
	CreatedAt         string
	UpdatedAt         string
}

// PullRequestReview represents a submitted pull request review.
type PullRequestReview struct {
	ID                int64
	Author            string
	State             string
	Body              string
	CommitID          string
	URL               string
	HTMLURL           string
	AuthorAssociation string
	SubmittedAt       string
}

// PullRequestReviewComment represents an inline pull request review comment.
type PullRequestReviewComment struct {
	ID                   int64
	NodeID               string
	ReviewID             int64
	InReplyToID          int64
	Author               string
	Body                 string
	Path                 string
	DiffHunk             string
	Side                 string
	StartSide            string
	Line                 int
	StartLine            int
	OriginalLine         int
	OriginalStartLine    int
	Position             int
	OriginalPosition     int
	CommitID             string
	OriginalCommitID     string
	SubjectType          string
	URL                  string
	HTMLURL              string
	PullRequestURL       string
	AuthorAssociation    string
	CreatedAt            string
	UpdatedAt            string
	ConversationID       string
	ConversationResolved *bool
}

// PullRequestConversation groups related inline review comments.
type PullRequestConversation struct {
	ID       string
	Resolved *bool
	Comments []PullRequestReviewComment
}

// Label represents a GitHub label
type Label struct {
	Name        string
	Description string
	Color       string
}

// User represents a GitHub user
type User struct {
	Login string
	Name  string
}

// Team represents a GitHub organization team.
type Team struct {
	Organization string
	Slug         string
	Name         string
	Description  string
}

// Canonical returns the canonical team identifier (<org>/<team-slug>).
func (t Team) Canonical() string {
	org := strings.TrimSpace(t.Organization)
	slug := strings.TrimSpace(t.Slug)
	if org == "" || slug == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", org, slug)
}

// CreateReleaseOptions contains the options for creating a release
type CreateReleaseOptions struct {
	TagName         string
	TargetCommitish string
	Name            string
	Body            string
	Draft           bool
	Prerelease      bool
}

// Release represents a GitHub release
type Release struct {
	ID      int64
	TagName string
	Name    string
	URL     string
	HTMLURL string
}

type Github interface {
	auth.OauthService
	config.ConfigurableService
	// GetPullRequestUrl returns the URL of the pull request for current branch or an error if it can't be found
	GetPullRequestUrl() (string, error)
	// GetCurrentUser returns the information about the current authenticated user
	GetCurrentUser() (*hub.User, error)
	// CreatePullRequest creates a new pull request
	CreatePullRequest(opts CreatePROptions) (*PullRequest, error)
	// UpdatePullRequest updates an existing pull request
	UpdatePullRequest(prNumber int, opts UpdatePROptions) (*PullRequest, error)
	// GetPullRequestDiscussion returns comments and reviews for an existing pull request
	GetPullRequestDiscussion(prNumber int) (*PullRequestDiscussion, error)
	// GetLabels returns all labels from the repository
	GetLabels() ([]Label, error)
	// AddLabelsToPR adds labels to a pull request
	AddLabelsToPR(prNumber int, labels []string) error
	// GetCollaborators returns repository collaborators for reviewer selection
	GetCollaborators() ([]User, error)
	// GetOrgMembers returns organization members for participant selection
	GetOrgMembers() ([]User, error)
	// GetTeams returns organization teams for team reviewer selection
	GetTeams() ([]Team, error)
	// RequestReviewers requests reviewers on a pull request
	RequestReviewers(prNumber int, reviewers []string, teamReviewers []string) error
	// RemoveReviewers removes pending reviewer requests from a pull request
	RemoveReviewers(prNumber int, reviewers []string, teamReviewers []string) error
	// AssignUsersToPR assigns users to a pull request
	AssignUsersToPR(prNumber int, assignees []string) error
	// RemoveAssigneesFromPR removes users assigned to a pull request
	RemoveAssigneesFromPR(prNumber int, assignees []string) error
	// MarkPullRequestReadyForReview moves a draft pull request to ready for review
	MarkPullRequestReadyForReview(prNodeID string) error
	// DoesPRExistForBranch checks if a PR exists for a given branch
	DoesPRExistForBranch(branch string) (bool, *PullRequest, error)
	// CreateRelease creates a GitHub release
	CreateRelease(opts CreateReleaseOptions) (*Release, error)
}

var oauthClientID = ""

var ErrAuthRequired = errors.New("github authentication required; run `fotingo login` interactively")

var ErrOAuthClientIDMissing = errors.New("github oauth client id is missing in this build; use `fotingo login` with API token auth or rebuild with oauth ldflags")

const (
	githubAuthMethodOAuth = "oauth"
	githubAuthMethodToken = "token"
)

type GithubConfig struct {
	Token string
}

type github struct {
	*auth.Authenticator
	*config.ViperConfigurableService
	owner                   string
	repo                    string
	git                     git.Git
	hub                     *hub.Client
	allowPrompt             bool
	promptAuth              func() (string, error)
	promptToken             func() (string, error)
	metadataCache           cache.Store
	cacheInitErr            error
	metadataFetchInfoLogger func(string)
}

const (
	defaultLabelsCacheTTL        = 30 * time.Minute
	defaultCollaboratorsCacheTTL = 720 * time.Hour
	defaultOrgMembersCacheTTL    = 720 * time.Hour
	defaultTeamsCacheTTL         = 720 * time.Hour
	defaultUserProfilesCacheTTL  = 720 * time.Hour
	maxNameLookupPerFetch        = 500
)

type cachedUserProfile struct {
	Resolved bool `json:"resolved"`
	User     User `json:"user"`
}

type cachedUserProfileName struct {
	Resolved bool   `json:"resolved"`
	Name     string `json:"name,omitempty"`
}

type userNameEnrichmentStats struct {
	Changed        bool
	CacheHits      int
	NetworkLookups int
	RemainingEmpty int
}

var newMetadataCacheStore = func(cfg *viper.Viper) (cache.Store, error) {
	path := ""
	if cfg != nil {
		path = strings.TrimSpace(cfg.GetString("cache.path"))
	}

	if path == "" {
		store, err := cache.NewDefault()
		if err != nil {
			return nil, err
		}
		return store, nil
	}

	store, err := cache.New(cache.WithPath(path), cache.WithLogger(nil))
	if err != nil {
		return nil, err
	}
	return store, nil
}

func (g *github) GetCurrentUser() (*hub.User, error) {
	user, _, err := g.hub.Users.Get(context.Background(), "")
	return user, err
}

func (g *github) GetPullRequestUrl() (string, error) {
	branch, err := g.git.GetCurrentBranch()
	if err != nil {
		return "", err
	}
	list, _, err := g.hub.PullRequests.List(context.TODO(), g.owner, g.repo, &hub.PullRequestListOptions{
		Head: fmt.Sprintf("%s:%s", g.owner, branch),
	})
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", fmt.Errorf("no pull request found for branch %s", branch)
	}
	return list[0].GetHTMLURL(), nil
}

// CreatePullRequest creates a new pull request with the given options
func (g *github) CreatePullRequest(opts CreatePROptions) (*PullRequest, error) {
	newPR := &hub.NewPullRequest{
		Title: &opts.Title,
		Body:  &opts.Body,
		Head:  &opts.Head,
		Base:  &opts.Base,
		Draft: &opts.Draft,
	}

	pr, _, err := g.hub.PullRequests.Create(context.Background(), g.owner, g.repo, newPR)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	return mapPullRequest(pr), nil
}

// UpdatePullRequest updates an existing pull request with the provided fields.
func (g *github) UpdatePullRequest(prNumber int, opts UpdatePROptions) (*PullRequest, error) {
	edit := &hub.PullRequest{}
	if opts.Title != nil {
		edit.Title = opts.Title
	}
	if opts.Body != nil {
		edit.Body = opts.Body
	}

	pr, _, err := g.hub.PullRequests.Edit(context.Background(), g.owner, g.repo, prNumber, edit)
	if err != nil {
		return nil, fmt.Errorf("failed to update pull request: %w", err)
	}

	return mapPullRequest(pr), nil
}

// UpdatePullRequestBodies updates multiple pull request bodies in order.
func (g *github) UpdatePullRequestBodies(updates []PullRequestBodyUpdate) ([]*PullRequest, error) {
	updated := make([]*PullRequest, 0, len(updates))
	for _, update := range updates {
		body := update.Body
		pr, err := g.UpdatePullRequest(update.Number, UpdatePROptions{Body: &body})
		if err != nil {
			return nil, fmt.Errorf("failed to update pull request #%d body: %w", update.Number, err)
		}
		updated = append(updated, pr)
	}
	return updated, nil
}

// GetPullRequestDiscussion returns comments and reviews for an existing pull request.
func (g *github) GetPullRequestDiscussion(prNumber int) (*PullRequestDiscussion, error) {
	issueComments, err := g.listPullRequestIssueComments(prNumber)
	if err != nil {
		return nil, err
	}

	reviews, err := g.listPullRequestReviews(prNumber)
	if err != nil {
		return nil, err
	}

	reviewComments, err := g.listPullRequestReviewComments(prNumber)
	if err != nil {
		return nil, err
	}

	return &PullRequestDiscussion{
		Comments:       issueComments,
		Reviews:        reviews,
		ReviewComments: reviewComments,
		Conversations:  GroupPullRequestReviewComments(reviewComments),
	}, nil
}

func (g *github) listPullRequestIssueComments(prNumber int) ([]PullRequestIssueComment, error) {
	sort := "created"
	direction := "asc"
	opts := &hub.IssueListCommentsOptions{
		Sort:      &sort,
		Direction: &direction,
		ListOptions: hub.ListOptions{
			PerPage: 100,
		},
	}

	var mapped []PullRequestIssueComment
	for {
		comments, resp, err := g.hub.Issues.ListComments(context.Background(), g.owner, g.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list pull request issue comments: %w", err)
		}
		for _, comment := range comments {
			mapped = append(mapped, mapPullRequestIssueComment(comment))
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return mapped, nil
}

func (g *github) listPullRequestReviews(prNumber int) ([]PullRequestReview, error) {
	opts := &hub.ListOptions{PerPage: 100}
	var mapped []PullRequestReview
	for {
		reviews, resp, err := g.hub.PullRequests.ListReviews(context.Background(), g.owner, g.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list pull request reviews: %w", err)
		}
		for _, review := range reviews {
			mapped = append(mapped, mapPullRequestReview(review))
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return mapped, nil
}

func (g *github) listPullRequestReviewComments(prNumber int) ([]PullRequestReviewComment, error) {
	opts := &hub.PullRequestListCommentsOptions{
		Sort:      "created",
		Direction: "asc",
		ListOptions: hub.ListOptions{
			PerPage: 100,
		},
	}

	var mapped []PullRequestReviewComment
	for {
		comments, resp, err := g.hub.PullRequests.ListComments(context.Background(), g.owner, g.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list pull request review comments: %w", err)
		}
		for _, comment := range comments {
			mapped = append(mapped, mapPullRequestReviewComment(comment))
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return mapped, nil
}

func mapPullRequestIssueComment(comment *hub.IssueComment) PullRequestIssueComment {
	return PullRequestIssueComment{
		ID:                comment.GetID(),
		Author:            comment.GetUser().GetLogin(),
		Body:              comment.GetBody(),
		URL:               comment.GetURL(),
		HTMLURL:           comment.GetHTMLURL(),
		AuthorAssociation: comment.GetAuthorAssociation(),
		CreatedAt:         formatGitHubTimestamp(comment.GetCreatedAt()),
		UpdatedAt:         formatGitHubTimestamp(comment.GetUpdatedAt()),
	}
}

func mapPullRequestReview(review *hub.PullRequestReview) PullRequestReview {
	return PullRequestReview{
		ID:                review.GetID(),
		Author:            review.GetUser().GetLogin(),
		State:             review.GetState(),
		Body:              review.GetBody(),
		CommitID:          review.GetCommitID(),
		URL:               review.GetPullRequestURL(),
		HTMLURL:           review.GetHTMLURL(),
		AuthorAssociation: review.GetAuthorAssociation(),
		SubmittedAt:       formatGitHubTimestamp(review.GetSubmittedAt()),
	}
}

func mapPullRequestReviewComment(comment *hub.PullRequestComment) PullRequestReviewComment {
	return PullRequestReviewComment{
		ID:                comment.GetID(),
		NodeID:            comment.GetNodeID(),
		ReviewID:          comment.GetPullRequestReviewID(),
		InReplyToID:       comment.GetInReplyTo(),
		Author:            comment.GetUser().GetLogin(),
		Body:              comment.GetBody(),
		Path:              comment.GetPath(),
		DiffHunk:          comment.GetDiffHunk(),
		Side:              comment.GetSide(),
		StartSide:         comment.GetStartSide(),
		Line:              comment.GetLine(),
		StartLine:         comment.GetStartLine(),
		OriginalLine:      comment.GetOriginalLine(),
		OriginalStartLine: comment.GetOriginalStartLine(),
		Position:          comment.GetPosition(),
		OriginalPosition:  comment.GetOriginalPosition(),
		CommitID:          comment.GetCommitID(),
		OriginalCommitID:  comment.GetOriginalCommitID(),
		SubjectType:       comment.GetSubjectType(),
		URL:               comment.GetURL(),
		HTMLURL:           comment.GetHTMLURL(),
		PullRequestURL:    comment.GetPullRequestURL(),
		AuthorAssociation: comment.GetAuthorAssociation(),
		CreatedAt:         formatGitHubTimestamp(comment.GetCreatedAt()),
		UpdatedAt:         formatGitHubTimestamp(comment.GetUpdatedAt()),
		ConversationID:    pullRequestReviewConversationID(comment),
	}
}

func formatGitHubTimestamp(timestamp hub.Timestamp) string {
	if timestamp.IsZero() {
		return ""
	}
	return timestamp.Format(time.RFC3339)
}

func pullRequestReviewConversationID(comment *hub.PullRequestComment) string {
	if comment == nil {
		return ""
	}
	if comment.GetInReplyTo() > 0 {
		return fmt.Sprintf("review-comment-%d", comment.GetInReplyTo())
	}
	if comment.GetID() > 0 {
		return fmt.Sprintf("review-comment-%d", comment.GetID())
	}
	if nodeID := strings.TrimSpace(comment.GetNodeID()); nodeID != "" {
		return "review-comment-" + nodeID
	}
	return ""
}

// GroupPullRequestReviewComments groups inline review comments into conversation-like threads.
func GroupPullRequestReviewComments(comments []PullRequestReviewComment) []PullRequestConversation {
	if len(comments) == 0 {
		return nil
	}

	conversationByID := make(map[string]int, len(comments))
	conversations := make([]PullRequestConversation, 0)
	for _, comment := range comments {
		conversationID := strings.TrimSpace(comment.ConversationID)
		if conversationID == "" {
			if comment.InReplyToID > 0 {
				conversationID = fmt.Sprintf("review-comment-%d", comment.InReplyToID)
			} else {
				conversationID = fmt.Sprintf("review-comment-%d", comment.ID)
			}
			comment.ConversationID = conversationID
		}

		idx, ok := conversationByID[conversationID]
		if !ok {
			conversationByID[conversationID] = len(conversations)
			conversations = append(conversations, PullRequestConversation{ID: conversationID})
			idx = len(conversations) - 1
		}
		conversations[idx].Comments = append(conversations[idx].Comments, comment)
	}

	return conversations
}

// GetLabels returns all labels from the repository
func (g *github) GetLabels() ([]Label, error) {
	cacheKey := g.metadataCacheKey("labels")
	var cacheErr error
	if g.cacheInitErr != nil {
		cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to initialize metadata cache: %w", g.cacheInitErr))
	}

	if g.metadataCache != nil {
		var cachedLabels []Label
		hit, err := g.metadataCache.Get(cacheKey, &cachedLabels)
		if err != nil {
			cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to read labels cache: %w", err))
		} else if hit {
			return cachedLabels, nil
		}
	}

	labels, err := g.fetchLabels()
	if err != nil {
		if cacheErr != nil {
			return nil, errors.Join(cacheErr, err)
		}
		return nil, err
	}

	if g.metadataCache != nil {
		ttl := g.metadataTTL("cache.labelsTTL", defaultLabelsCacheTTL)
		_ = g.metadataCache.SetWithTTL(cacheKey, labels, ttl)
	}

	return labels, nil
}

func (g *github) fetchLabels() ([]Label, error) {
	var allLabels []Label
	opts := &hub.ListOptions{PerPage: 100}

	for {
		labels, resp, err := g.hub.Issues.ListLabels(context.Background(), g.owner, g.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list labels: %w", err)
		}

		for _, label := range labels {
			allLabels = append(allLabels, Label{
				Name:        label.GetName(),
				Description: label.GetDescription(),
				Color:       label.GetColor(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allLabels, nil
}

// AddLabelsToPR adds labels to a pull request
func (g *github) AddLabelsToPR(prNumber int, labels []string) error {
	_, _, err := g.hub.Issues.AddLabelsToIssue(context.Background(), g.owner, g.repo, prNumber, labels)
	if err != nil {
		return fmt.Errorf("failed to add labels to pull request: %w", err)
	}
	return nil
}

// GetCollaborators returns repository collaborators for reviewer selection
func (g *github) GetCollaborators() ([]User, error) {
	startedAt := time.Now()
	cacheKey := g.metadataCacheKey("collaborators")
	var cacheErr error
	if g.cacheInitErr != nil {
		cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to initialize metadata cache: %w", g.cacheInitErr))
	}

	if g.metadataCache != nil {
		var cachedCollaborators []User
		hit, err := g.metadataCache.Get(cacheKey, &cachedCollaborators)
		if err != nil {
			cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to read collaborators cache: %w", err))
		} else if hit {
			enrichedCollaborators, stats := g.enrichUsersWithProfileNames(cachedCollaborators, maxNameLookupPerFetch, false)
			if stats.Changed {
				ttl := g.metadataTTL("cache.collaboratorsTTL", defaultCollaboratorsCacheTTL)
				_ = g.metadataCache.SetWithTTL(cacheKey, enrichedCollaborators, ttl)
			}
			g.logCachedMetadataLoad(
				fmt.Sprintf(
					"Loaded %d GitHub repository collaborators for %s/%s from cache",
					len(enrichedCollaborators),
					g.owner,
					g.repo,
				),
				time.Since(startedAt),
			)
			g.logUserNameEnrichment(
				"repository collaborators",
				fmt.Sprintf("%s/%s", g.owner, g.repo),
				stats,
				false,
			)
			return enrichedCollaborators, nil
		}
	}

	collaborators, err := g.fetchCollaborators()
	if err != nil {
		if cacheErr != nil {
			return nil, errors.Join(cacheErr, err)
		}
		return nil, err
	}

	if g.metadataCache != nil {
		ttl := g.metadataTTL("cache.collaboratorsTTL", defaultCollaboratorsCacheTTL)
		_ = g.metadataCache.SetWithTTL(cacheKey, collaborators, ttl)
	}

	return collaborators, nil
}

func (g *github) fetchCollaborators() ([]User, error) {
	startedAt := time.Now()
	g.logMetadataFetchInfo(
		fmt.Sprintf("Fetching GitHub repository collaborators for %s/%s (cache miss or expired)", g.owner, g.repo),
	)

	var allCollaborators []User
	opts := &hub.ListCollaboratorsOptions{
		ListOptions: hub.ListOptions{PerPage: 100},
	}

	for {
		collaborators, resp, err := g.hub.Repositories.ListCollaborators(context.Background(), g.owner, g.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list collaborators: %w", err)
		}

		for _, collaborator := range collaborators {
			allCollaborators = append(allCollaborators, User{
				Login: collaborator.GetLogin(),
				Name:  collaborator.GetName(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	allCollaborators, stats := g.enrichUsersWithProfileNames(allCollaborators, maxNameLookupPerFetch, true)
	g.logUserNameEnrichment(
		"repository collaborators",
		fmt.Sprintf("%s/%s", g.owner, g.repo),
		stats,
		true,
	)
	g.logMetadataFetchInfo(
		fmt.Sprintf(
			"Fetched %d GitHub repository collaborators for %s/%s in %s",
			len(allCollaborators),
			g.owner,
			g.repo,
			formatMetadataDuration(time.Since(startedAt)),
		),
	)

	return allCollaborators, nil
}

// GetOrgMembers returns organization members for reviewer selection.
func (g *github) GetOrgMembers() ([]User, error) {
	startedAt := time.Now()
	cacheKey := g.metadataOwnerCacheKey("org-members")
	var cacheErr error
	if g.cacheInitErr != nil {
		cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to initialize metadata cache: %w", g.cacheInitErr))
	}

	if g.metadataCache != nil {
		var cachedMembers []User
		hit, err := g.metadataCache.Get(cacheKey, &cachedMembers)
		if err != nil {
			cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to read org members cache: %w", err))
		} else if hit {
			enrichedMembers, stats := g.enrichUsersWithProfileNames(cachedMembers, maxNameLookupPerFetch, false)
			if stats.Changed {
				ttl := g.metadataTTL("cache.orgMembersTTL", defaultOrgMembersCacheTTL)
				_ = g.metadataCache.SetWithTTL(cacheKey, enrichedMembers, ttl)
			}
			g.logCachedMetadataLoad(
				fmt.Sprintf(
					"Loaded %d GitHub organization members for %s from cache",
					len(enrichedMembers),
					g.owner,
				),
				time.Since(startedAt),
			)
			g.logUserNameEnrichment("organization members", g.owner, stats, false)
			return enrichedMembers, nil
		}
	}

	members, err := g.fetchOrgMembers()
	if err != nil {
		if cacheErr != nil {
			return nil, errors.Join(cacheErr, err)
		}
		return nil, err
	}

	if g.metadataCache != nil {
		ttl := g.metadataTTL("cache.orgMembersTTL", defaultOrgMembersCacheTTL)
		_ = g.metadataCache.SetWithTTL(cacheKey, members, ttl)
	}

	return members, nil
}

func (g *github) fetchOrgMembers() ([]User, error) {
	startedAt := time.Now()
	g.logMetadataFetchInfo(
		fmt.Sprintf("Fetching GitHub organization members for %s (cache miss or expired)", g.owner),
	)

	var allMembers []User
	opts := &hub.ListMembersOptions{
		ListOptions: hub.ListOptions{PerPage: 100},
	}

	for {
		members, resp, err := g.hub.Organizations.ListMembers(context.Background(), g.owner, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list organization members: %w", err)
		}

		for _, member := range members {
			allMembers = append(allMembers, User{
				Login: member.GetLogin(),
				Name:  member.GetName(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	allMembers, stats := g.enrichUsersWithProfileNames(allMembers, maxNameLookupPerFetch, true)
	g.logUserNameEnrichment("organization members", g.owner, stats, true)
	g.logMetadataFetchInfo(
		fmt.Sprintf(
			"Fetched %d GitHub organization members for %s in %s",
			len(allMembers),
			g.owner,
			formatMetadataDuration(time.Since(startedAt)),
		),
	)

	return allMembers, nil
}

// SetMetadataFetchInfoLogger configures an optional info logger for long-running metadata fetches.
func (g *github) SetMetadataFetchInfoLogger(logf func(string)) {
	g.metadataFetchInfoLogger = logf
}

func (g *github) logMetadataFetchInfo(message string) {
	if g == nil || g.metadataFetchInfoLogger == nil {
		return
	}
	g.metadataFetchInfoLogger(strings.TrimSpace(message))
}

func (g *github) enrichUsersWithProfileNames(
	users []User,
	maxLookups int,
	allowNetworkLookup bool,
) ([]User, userNameEnrichmentStats) {
	stats := userNameEnrichmentStats{}
	if maxLookups <= 0 {
		stats.RemainingEmpty = countUsersMissingNames(users)
		return users, stats
	}

	remainingLookups := maxLookups
	for index := range users {
		login := strings.TrimSpace(users[index].Login)
		if login == "" || strings.TrimSpace(users[index].Name) != "" {
			continue
		}

		if resolvedName, hit := g.cachedUserProfileName(login); hit {
			stats.CacheHits++
			if strings.TrimSpace(resolvedName) == "" {
				continue
			}
			users[index].Name = strings.TrimSpace(resolvedName)
			stats.Changed = true
			continue
		}

		if !allowNetworkLookup || remainingLookups == 0 {
			continue
		}

		remainingLookups--
		stats.NetworkLookups++
		resolvedName, err := g.fetchUserProfileName(login)
		if err != nil {
			continue
		}
		resolvedName = strings.TrimSpace(resolvedName)
		if resolvedName == "" {
			continue
		}

		users[index].Name = resolvedName
		stats.Changed = true
	}

	stats.RemainingEmpty = countUsersMissingNames(users)
	return users, stats
}

func (g *github) fetchUserProfileName(login string) (string, error) {
	profile, err := g.fetchUserProfile(login)
	if err != nil {
		return "", err
	}

	return profile.Name, nil
}

func (g *github) fetchUserProfile(login string) (User, error) {
	trimmedLogin := strings.TrimSpace(login)
	if trimmedLogin == "" {
		return User{}, fmt.Errorf("login is required")
	}

	if cachedProfile, hit := g.cachedUserProfile(trimmedLogin); hit {
		return cachedProfile, nil
	}

	user, _, err := g.hub.Users.Get(context.Background(), trimmedLogin)
	if err != nil {
		return User{}, err
	}

	profile := User{
		Login: strings.TrimSpace(user.GetLogin()),
		Name:  strings.TrimSpace(user.GetName()),
	}
	if profile.Login == "" {
		profile.Login = trimmedLogin
	}
	g.cacheUserProfile(profile)

	return profile, nil
}

// GetTeams returns organization teams for team reviewer selection.
func (g *github) GetTeams() ([]Team, error) {
	startedAt := time.Now()
	cacheKey := g.metadataOwnerCacheKey("teams")
	var cacheErr error
	if g.cacheInitErr != nil {
		cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to initialize metadata cache: %w", g.cacheInitErr))
	}

	if g.metadataCache != nil {
		var cachedTeams []Team
		hit, err := g.metadataCache.Get(cacheKey, &cachedTeams)
		if err != nil {
			cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to read teams cache: %w", err))
		} else if hit {
			g.logCachedMetadataLoad(
				fmt.Sprintf("Loaded %d GitHub organization teams for %s from cache", len(cachedTeams), g.owner),
				time.Since(startedAt),
			)
			return cachedTeams, nil
		}
	}

	teams, err := g.fetchTeams()
	if err != nil {
		if cacheErr != nil {
			return nil, errors.Join(cacheErr, err)
		}
		return nil, err
	}

	if g.metadataCache != nil {
		ttl := g.metadataTTL("cache.teamsTTL", defaultTeamsCacheTTL)
		_ = g.metadataCache.SetWithTTL(cacheKey, teams, ttl)
	}

	return teams, nil
}

func (g *github) fetchTeams() ([]Team, error) {
	startedAt := time.Now()
	g.logMetadataFetchInfo(
		fmt.Sprintf("Fetching GitHub organization teams for %s (cache miss or expired)", g.owner),
	)

	var allTeams []Team
	opts := &hub.ListOptions{PerPage: 100}

	for {
		teams, resp, err := g.hub.Teams.ListTeams(context.Background(), g.owner, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list organization teams: %w", err)
		}

		for _, team := range teams {
			allTeams = append(allTeams, Team{
				Organization: g.owner,
				Slug:         team.GetSlug(),
				Name:         team.GetName(),
				Description:  team.GetDescription(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	g.logMetadataFetchInfo(
		fmt.Sprintf(
			"Fetched %d GitHub organization teams for %s in %s",
			len(allTeams),
			g.owner,
			formatMetadataDuration(time.Since(startedAt)),
		),
	)
	return allTeams, nil
}

// SupportsOrganizationMetadata reports whether the current repository owner is
// a GitHub organization, which determines whether organization members and
// teams can be queried for participant resolution.
func (g *github) SupportsOrganizationMetadata() (bool, error) {
	cacheKey := g.metadataOwnerCacheKey("org-metadata-supported")
	var cacheErr error
	if g.cacheInitErr != nil {
		cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to initialize metadata cache: %w", g.cacheInitErr))
	}

	if g.metadataCache != nil {
		var cachedSupported bool
		hit, err := g.metadataCache.Get(cacheKey, &cachedSupported)
		if err != nil {
			cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to read owner metadata support cache: %w", err))
		} else if hit {
			return cachedSupported, nil
		}
	}

	supported, err := g.fetchOrganizationMetadataSupport()
	if err != nil {
		if cacheErr != nil {
			return false, errors.Join(cacheErr, err)
		}
		return false, err
	}

	if g.metadataCache != nil {
		ttl := g.metadataTTL("cache.orgMembersTTL", defaultOrgMembersCacheTTL)
		_ = g.metadataCache.SetWithTTL(cacheKey, supported, ttl)
	}

	return supported, nil
}

func (g *github) fetchOrganizationMetadataSupport() (bool, error) {
	repository, _, err := g.hub.Repositories.Get(context.Background(), g.owner, g.repo)
	if err != nil {
		return false, fmt.Errorf("failed to inspect repository owner type: %w", err)
	}

	owner := repository.GetOwner()
	ownerType := strings.TrimSpace(owner.GetType())
	if ownerType == "" {
		return false, fmt.Errorf("failed to determine repository owner type for %s/%s", g.owner, g.repo)
	}

	return strings.EqualFold(ownerType, "Organization"), nil
}

func (g *github) metadataCacheKey(kind string) string {
	return fmt.Sprintf("github:%s:%s/%s", kind, g.owner, g.repo)
}

func (g *github) metadataOwnerCacheKey(kind string) string {
	return fmt.Sprintf("github:%s:%s", kind, g.owner)
}

func (g *github) metadataGlobalCacheKey(kind string) string {
	return fmt.Sprintf("github:%s", kind)
}

func (g *github) metadataUserProfileCacheKey(login string) string {
	return fmt.Sprintf("%s:%s", g.metadataGlobalCacheKey("user-profile"), normalizeMetadataLogin(login))
}

func (g *github) metadataLegacyUserProfileNameCacheKey(login string) string {
	return fmt.Sprintf("%s:%s", g.metadataGlobalCacheKey("user-profile-name"), normalizeMetadataLogin(login))
}

func normalizeMetadataLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

func (g *github) cachedUserProfile(login string) (User, bool) {
	if g == nil || g.metadataCache == nil {
		return User{}, false
	}

	trimmedLogin := strings.TrimSpace(login)
	if trimmedLogin == "" {
		return User{}, false
	}

	var cached cachedUserProfile
	hit, err := g.metadataCache.Get(g.metadataUserProfileCacheKey(trimmedLogin), &cached)
	if err == nil && hit && cached.Resolved {
		profile := cached.User
		if strings.TrimSpace(profile.Login) == "" {
			profile.Login = trimmedLogin
		}
		profile.Name = strings.TrimSpace(profile.Name)
		return profile, true
	}

	var legacy cachedUserProfileName
	legacyHit, legacyErr := g.metadataCache.Get(g.metadataLegacyUserProfileNameCacheKey(trimmedLogin), &legacy)
	if legacyErr != nil || !legacyHit || !legacy.Resolved {
		return User{}, false
	}

	profile := User{
		Login: trimmedLogin,
		Name:  strings.TrimSpace(legacy.Name),
	}
	g.cacheUserProfile(profile)
	return profile, true
}

func (g *github) cachedUserProfileName(login string) (string, bool) {
	profile, hit := g.cachedUserProfile(login)
	if !hit {
		return "", false
	}

	return profile.Name, true
}

func (g *github) cacheUserProfile(profile User) {
	if g == nil || g.metadataCache == nil {
		return
	}

	login := strings.TrimSpace(profile.Login)
	if login == "" {
		return
	}

	ttl := g.metadataTTL("cache.userProfilesTTL", defaultUserProfilesCacheTTL)
	_ = g.metadataCache.SetWithTTL(
		g.metadataUserProfileCacheKey(login),
		cachedUserProfile{
			Resolved: true,
			User: User{
				Login: login,
				Name:  strings.TrimSpace(profile.Name),
			},
		},
		ttl,
	)
}

func (g *github) logCachedMetadataLoad(message string, duration time.Duration) {
	g.logMetadataFetchInfo(fmt.Sprintf("%s in %s", strings.TrimSpace(message), formatMetadataDuration(duration)))
}

func (g *github) logUserNameEnrichment(
	kind string,
	scope string,
	stats userNameEnrichmentStats,
	allowNetworkLookup bool,
) {
	if stats.CacheHits > 0 {
		g.logMetadataFetchInfo(
			fmt.Sprintf(
				"Applied %d cached GitHub profile names to %s for %s",
				stats.CacheHits,
				kind,
				scope,
			),
		)
	}

	if allowNetworkLookup && stats.NetworkLookups > 0 {
		g.logMetadataFetchInfo(
			fmt.Sprintf(
				"Fetched %d GitHub user profiles to enrich %s for %s",
				stats.NetworkLookups,
				kind,
				scope,
			),
		)
	}

	if !allowNetworkLookup && stats.RemainingEmpty > 0 {
		g.logMetadataFetchInfo(
			fmt.Sprintf(
				"Skipped fresh GitHub profile lookups for %d cached %s without names",
				stats.RemainingEmpty,
				kind,
			),
		)
	}
}

func countUsersMissingNames(users []User) int {
	total := 0
	for _, user := range users {
		if strings.TrimSpace(user.Login) == "" || strings.TrimSpace(user.Name) != "" {
			continue
		}
		total++
	}
	return total
}

func formatMetadataDuration(duration time.Duration) string {
	if duration < time.Millisecond {
		return duration.Round(time.Microsecond).String()
	}
	return duration.Round(time.Millisecond).String()
}

func (g *github) metadataTTL(configKey string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(g.GetConfigString(configKey))
	if raw == "" {
		return fallback
	}

	ttl, err := time.ParseDuration(raw)
	if err != nil || ttl <= 0 {
		return fallback
	}

	return ttl
}

type latestReleaseResponse struct {
	TagName string `json:"tag_name"`
}

var latestReleaseAPIBaseURL = "https://api.github.com"

// FetchLatestReleaseTag fetches the latest release tag for a public repository
// without requiring authentication.
func FetchLatestReleaseTag(ctx context.Context, client *http.Client, owner string, repo string) (string, error) {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(repo) == "" {
		return "", fmt.Errorf("owner and repo are required")
	}

	httpClient := client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 3 * time.Second}
	}
	httpClient = wrapGitHubHTTPClient(httpClient)

	baseURL := strings.TrimRight(strings.TrimSpace(latestReleaseAPIBaseURL), "/")
	endpoint := fmt.Sprintf("%s/repos/%s/%s/releases/latest", baseURL, strings.TrimSpace(owner), strings.TrimSpace(repo))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "fotingo/version-checker")

	response, err := httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("latest release request failed with %s", response.Status)
	}

	var payload latestReleaseResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.TagName) == "" {
		return "", fmt.Errorf("latest release tag is empty")
	}

	return payload.TagName, nil
}

// RequestReviewers requests reviewers on a pull request
func (g *github) RequestReviewers(prNumber int, reviewers []string, teamReviewers []string) error {
	if len(reviewers) == 0 && len(teamReviewers) == 0 {
		return nil
	}

	reviewersRequest := hub.ReviewersRequest{
		Reviewers:     reviewers,
		TeamReviewers: teamReviewers,
	}

	_, _, err := g.hub.PullRequests.RequestReviewers(context.Background(), g.owner, g.repo, prNumber, reviewersRequest)
	if err != nil {
		return fmt.Errorf("failed to request reviewers: %w", err)
	}
	return nil
}

// RemoveReviewers removes pending reviewer requests from a pull request.
func (g *github) RemoveReviewers(prNumber int, reviewers []string, teamReviewers []string) error {
	if len(reviewers) == 0 && len(teamReviewers) == 0 {
		return nil
	}

	reviewersRequest := hub.ReviewersRequest{
		Reviewers:     reviewers,
		TeamReviewers: teamReviewers,
	}

	_, err := g.hub.PullRequests.RemoveReviewers(context.Background(), g.owner, g.repo, prNumber, reviewersRequest)
	if err != nil {
		return fmt.Errorf("failed to remove reviewers: %w", err)
	}
	return nil
}

// AssignUsersToPR assigns users to a pull request.
func (g *github) AssignUsersToPR(prNumber int, assignees []string) error {
	if len(assignees) == 0 {
		return nil
	}

	_, _, err := g.hub.Issues.AddAssignees(context.Background(), g.owner, g.repo, prNumber, assignees)
	if err != nil {
		return fmt.Errorf("failed to assign users to pull request: %w", err)
	}
	return nil
}

// RemoveAssigneesFromPR removes users assigned to a pull request.
func (g *github) RemoveAssigneesFromPR(prNumber int, assignees []string) error {
	if len(assignees) == 0 {
		return nil
	}

	_, _, err := g.hub.Issues.RemoveAssignees(context.Background(), g.owner, g.repo, prNumber, assignees)
	if err != nil {
		return fmt.Errorf("failed to remove assignees from pull request: %w", err)
	}
	return nil
}

type markReadyForReviewGraphQLRequest struct {
	Query     string            `json:"query"`
	Variables map[string]string `json:"variables"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type markReadyForReviewGraphQLResponse struct {
	Errors []graphQLError `json:"errors"`
}

// MarkPullRequestReadyForReview moves a draft pull request to ready for review.
func (g *github) MarkPullRequestReadyForReview(prNodeID string) error {
	nodeID := strings.TrimSpace(prNodeID)
	if nodeID == "" {
		return fmt.Errorf("pull request node ID is required to mark ready for review")
	}

	request := markReadyForReviewGraphQLRequest{
		Query: `mutation($pullRequestId: ID!) {
  markPullRequestReadyForReview(input: {pullRequestId: $pullRequestId}) {
    pullRequest {
      id
    }
  }
}`,
		Variables: map[string]string{"pullRequestId": nodeID},
	}

	req, err := g.hub.NewRequest("POST", "/graphql", request)
	if err != nil {
		return fmt.Errorf("failed to create ready-for-review request: %w", err)
	}

	var response markReadyForReviewGraphQLResponse
	if _, err := g.hub.Do(context.Background(), req, &response); err != nil {
		return fmt.Errorf("failed to mark pull request ready for review: %w", err)
	}
	if len(response.Errors) > 0 {
		messages := make([]string, 0, len(response.Errors))
		for _, graphQLErr := range response.Errors {
			if strings.TrimSpace(graphQLErr.Message) != "" {
				messages = append(messages, graphQLErr.Message)
			}
		}
		if len(messages) == 0 {
			messages = append(messages, "unknown GraphQL error")
		}
		return fmt.Errorf("failed to mark pull request ready for review: %s", strings.Join(messages, "; "))
	}

	return nil
}

// DoesPRExistForBranch checks if a PR exists for a given branch
func (g *github) DoesPRExistForBranch(branch string) (bool, *PullRequest, error) {
	pr, exists, err := g.FindOpenPullRequestByHeadBranch(branch)
	if err != nil {
		return false, nil, err
	}
	return exists, pr, nil
}

// FindOpenPullRequestByHeadBranch returns the open PR for the given head branch, when present.
func (g *github) FindOpenPullRequestByHeadBranch(branch string) (*PullRequest, bool, error) {
	list, _, err := g.hub.PullRequests.List(context.Background(), g.owner, g.repo, &hub.PullRequestListOptions{
		Head:  fmt.Sprintf("%s:%s", g.owner, branch),
		State: "open",
	})
	if err != nil {
		return nil, false, fmt.Errorf("failed to check for existing pull request: %w", err)
	}

	if len(list) == 0 {
		return nil, false, nil
	}

	return mapPullRequest(list[0]), true, nil
}

// ListOpenPullRequestsByBaseBranch returns open PRs whose base branch matches the given branch.
func (g *github) ListOpenPullRequestsByBaseBranch(branch string) ([]PullRequest, error) {
	var mapped []PullRequest
	opts := &hub.PullRequestListOptions{
		Base:  branch,
		State: "open",
		ListOptions: hub.ListOptions{
			PerPage: 100,
		},
	}
	for {
		list, resp, err := g.hub.PullRequests.List(context.Background(), g.owner, g.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list pull requests by base branch: %w", err)
		}
		for _, pr := range list {
			mapped = append(mapped, *mapPullRequest(pr))
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return mapped, nil
}

// ListOpenPullRequestsByStackID returns open PRs whose body contains the fotingo stack id marker.
func (g *github) ListOpenPullRequestsByStackID(stackID string) ([]PullRequest, error) {
	stackID = strings.TrimSpace(stackID)
	if stackID == "" {
		return nil, fmt.Errorf("stack id is required")
	}

	var mapped []PullRequest
	opts := &hub.PullRequestListOptions{
		State: "open",
		ListOptions: hub.ListOptions{
			PerPage: 100,
		},
	}
	needle := fmt.Sprintf(`fotingo:stack id="%s"`, stackID)
	for {
		list, resp, err := g.hub.PullRequests.List(context.Background(), g.owner, g.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list pull requests by stack id: %w", err)
		}
		for _, pr := range list {
			if strings.Contains(pr.GetBody(), needle) {
				mapped = append(mapped, *mapPullRequest(pr))
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return mapped, nil
}

func mapPullRequest(pr *hub.PullRequest) *PullRequest {
	if pr == nil {
		return nil
	}

	mapped := &PullRequest{
		Title:   pr.GetTitle(),
		Body:    pr.GetBody(),
		Number:  pr.GetNumber(),
		NodeID:  pr.GetNodeID(),
		URL:     pr.GetURL(),
		HTMLURL: pr.GetHTMLURL(),
		Draft:   pr.GetDraft(),
		State:   pr.GetState(),
		Merged:  pr.GetMerged(),
	}
	if pr.Head != nil {
		mapped.HeadRef = pr.Head.GetRef()
	}
	if pr.Base != nil {
		mapped.BaseRef = pr.Base.GetRef()
	}
	if pr.Mergeable != nil {
		mergeable := pr.GetMergeable()
		mapped.Mergeable = &mergeable
	}
	return mapped
}

// CreateRelease creates a GitHub release
func (g *github) CreateRelease(opts CreateReleaseOptions) (*Release, error) {
	releaseRequest := &hub.RepositoryRelease{
		TagName:         &opts.TagName,
		TargetCommitish: &opts.TargetCommitish,
		Name:            &opts.Name,
		Body:            &opts.Body,
		Draft:           &opts.Draft,
		Prerelease:      &opts.Prerelease,
	}

	release, _, err := g.hub.Repositories.CreateRelease(context.Background(), g.owner, g.repo, releaseRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create release: %w", err)
	}

	return &Release{
		ID:      release.GetID(),
		TagName: release.GetTagName(),
		Name:    release.GetName(),
		URL:     release.GetURL(),
		HTMLURL: release.GetHTMLURL(),
	}, nil
}

// NewWithHTTPClient returns a new GitHub client using the provided HTTP client and base URL.
// This bypasses OAuth authentication and is intended for testing with mock servers.
func NewWithHTTPClient(g git.Git, cfg *viper.Viper, httpClient *http.Client, baseURL string) (Github, error) {
	remote, err := g.GetRemote()
	if err != nil {
		return nil, err
	}
	configurableService := &config.ViperConfigurableService{Config: cfg, Prefix: "github"}

	client, err := hub.NewClient(wrapGitHubHTTPClient(httpClient)).WithEnterpriseURLs(baseURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	metadataCache, cacheInitErr := newMetadataCacheStore(cfg)
	if cacheInitErr != nil {
		metadataCache = nil
	}

	return &github{
		ViperConfigurableService: configurableService,
		git:                      g,
		hub:                      client,
		owner:                    remote.GetOwnerName(),
		repo:                     remote.GetRepoName(),
		metadataCache:            metadataCache,
		cacheInitErr:             cacheInitErr,
	}, nil
}

// NewWithHTTPClientAndRepo creates a GitHub client with explicit owner/repo, bypassing remote URL parsing.
func NewWithHTTPClientAndRepo(g git.Git, cfg *viper.Viper, httpClient *http.Client, baseURL, owner, repo string) (Github, error) {
	configurableService := &config.ViperConfigurableService{Config: cfg, Prefix: "github"}

	client, err := hub.NewClient(wrapGitHubHTTPClient(httpClient)).WithEnterpriseURLs(baseURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	metadataCache, cacheInitErr := newMetadataCacheStore(cfg)
	if cacheInitErr != nil {
		metadataCache = nil
	}

	return &github{
		ViperConfigurableService: configurableService,
		git:                      g,
		hub:                      client,
		owner:                    owner,
		repo:                     repo,
		metadataCache:            metadataCache,
		cacheInitErr:             cacheInitErr,
	}, nil
}

func New(git git.Git, cfg *viper.Viper) (Github, error) {
	return NewWithOptions(git, cfg, true)
}

func NewAuthOnly(cfg *viper.Viper) (Github, error) {
	return NewAuthOnlyWithOptions(cfg, true)
}

func NewAuthOnlyWithOptions(cfg *viper.Viper, allowPrompt bool) (Github, error) {
	configurableService := &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	oauthHost, err := oauth.NewGitHubHost("https://github.com")
	if err != nil {
		return nil, fmt.Errorf("failed to configure github oauth host: %w", err)
	}

	gh := &github{
		ViperConfigurableService: configurableService,
		Authenticator: auth.NewAuthenticator(
			oauthClientID,
			"",
			[]string{"user", "repo"},
			oauthHost,
			func() string {
				localConfig := configurableService.GetConfig()
				if localConfig == nil {
					return ""
				}
				return localConfig.GetString("token")
			},
			func(token string) error {
				return configurableService.SaveConfig("token", token)
			},
		),
		git:         nil,
		hub:         nil,
		owner:       "",
		repo:        "",
		allowPrompt: allowPrompt,
		promptAuth:  promptGitHubAuthMethod,
		promptToken: promptGitHubToken,
	}
	metadataCache, cacheInitErr := newMetadataCacheStore(cfg)
	if cacheInitErr != nil {
		metadataCache = nil
	}
	gh.metadataCache = metadataCache
	gh.cacheInitErr = cacheInitErr

	token, err := gh.resolveToken()
	if err != nil {
		return nil, err
	}
	gh.hub = hub.NewClient(wrapGitHubHTTPClient(nil)).WithAuthToken(token)
	return gh, nil
}

func NewWithOptions(git git.Git, cfg *viper.Viper, allowPrompt bool) (Github, error) {
	remote, err := git.GetRemote()
	if err != nil {
		return nil, err
	}
	configurableService := &config.ViperConfigurableService{Config: cfg, Prefix: "github"}
	oauthHost, err := oauth.NewGitHubHost("https://github.com")
	if err != nil {
		return nil, fmt.Errorf("failed to configure github oauth host: %w", err)
	}
	gh := &github{
		ViperConfigurableService: configurableService,
		Authenticator: auth.NewAuthenticator(oauthClientID, "", []string{"user", "repo"}, oauthHost,
			func() string {
				localConfig := configurableService.GetConfig()
				if localConfig == nil {
					return ""
				}
				return localConfig.GetString("token")
			}, func(token string) error {
				return configurableService.SaveConfig("token", token)
			}),
		git:         git,
		hub:         nil,
		owner:       remote.GetOwnerName(),
		repo:        remote.GetRepoName(),
		allowPrompt: allowPrompt,
		promptAuth:  promptGitHubAuthMethod,
		promptToken: promptGitHubToken,
	}
	metadataCache, cacheInitErr := newMetadataCacheStore(cfg)
	if cacheInitErr != nil {
		metadataCache = nil
	}
	gh.metadataCache = metadataCache
	gh.cacheInitErr = cacheInitErr

	token, err := gh.resolveToken()
	if err != nil {
		return nil, err
	}
	gh.hub = hub.NewClient(wrapGitHubHTTPClient(nil)).WithAuthToken(token)

	return gh, nil
}

func (g *github) resolveToken() (string, error) {
	configToken := strings.TrimSpace(g.GetConfigString("token"))
	if configToken != "" {
		if oauthToken, ok := parseStoredOAuthToken(configToken); ok {
			return oauthToken, nil
		}
		return configToken, nil
	}

	if envToken := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); envToken != "" {
		return envToken, nil
	}

	if !g.allowPrompt || !isInputTerminal() {
		return "", ErrAuthRequired
	}

	methodPrompt := g.promptAuth
	if methodPrompt == nil {
		methodPrompt = promptGitHubAuthMethod
	}

	method, err := methodPrompt()
	if err != nil {
		return "", err
	}

	switch strings.ToLower(strings.TrimSpace(method)) {
	case "", githubAuthMethodOAuth:
		if err := validateOAuthClientID(); err != nil {
			return "", err
		}
		token, authErr := g.Authenticate()
		if authErr != nil {
			return "", authErr
		}
		return token.Token, nil
	case githubAuthMethodToken:
		tokenPrompt := g.promptToken
		if tokenPrompt == nil {
			tokenPrompt = promptGitHubToken
		}
		token, tokenErr := tokenPrompt()
		if tokenErr != nil {
			return "", tokenErr
		}
		if saveErr := g.SaveConfig("token", token); saveErr != nil {
			return "", saveErr
		}
		return token, nil
	default:
		return "", fmt.Errorf("unsupported github auth method: %s", method)
	}
}

func validateOAuthClientID() error {
	if strings.TrimSpace(oauthClientID) == "" {
		return ErrOAuthClientIDMissing
	}
	return nil
}

func parseStoredOAuthToken(raw string) (string, bool) {
	var token auth.AccessToken
	if err := json.Unmarshal([]byte(raw), &token); err != nil {
		return "", false
	}
	if strings.TrimSpace(token.Token) == "" {
		return "", false
	}
	return token.Token, true
}

func promptGitHubAuthMethod() (string, error) {
	useOAuth, err := ui.Confirm("Authenticate GitHub with OAuth? (No to use classic PAT)", true)
	if err != nil {
		return "", err
	}
	if useOAuth {
		return githubAuthMethodOAuth, nil
	}
	return githubAuthMethodToken, nil
}

func promptGitHubToken() (string, error) {
	input := ui.NewInputProgram(
		ui.WithPrompt("GitHub classic PAT (repo scope required)"),
		ui.WithPlaceholder("Create at https://github.com/settings/tokens"),
		ui.WithValidation(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("github api token is required")
			}
			return nil
		}),
	)

	value, cancelled, err := input.RunWithCancel()
	if err != nil {
		return "", err
	}
	if cancelled || strings.TrimSpace(value) == "" {
		return "", ErrAuthRequired
	}

	return strings.TrimSpace(value), nil
}

func isInputTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
