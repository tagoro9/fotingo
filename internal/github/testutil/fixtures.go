// Package testutil provides mock HTTP server utilities for testing GitHub client code.
package testutil

import "time"

// MockUser represents a GitHub user for testing.
type MockUser struct {
	ID        int64
	Login     string
	Name      string
	Email     string
	AvatarURL string
	HTMLURL   string
	Type      string
}

// ToAPIResponse converts the MockUser to a GitHub API response format.
func (u *MockUser) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":         u.ID,
		"login":      u.Login,
		"name":       u.Name,
		"email":      u.Email,
		"avatar_url": u.AvatarURL,
		"html_url":   u.HTMLURL,
		"type":       u.Type,
	}
}

// MockRepository represents a GitHub repository for testing.
type MockRepository struct {
	ID            int64
	Name          string
	FullName      string
	Description   string
	Private       bool
	HTMLURL       string
	CloneURL      string
	DefaultBranch string
	Owner         *MockUser
}

// ToAPIResponse converts the MockRepository to a GitHub API response format.
func (r *MockRepository) ToAPIResponse() map[string]interface{} {
	resp := map[string]interface{}{
		"id":             r.ID,
		"name":           r.Name,
		"full_name":      r.FullName,
		"description":    r.Description,
		"private":        r.Private,
		"html_url":       r.HTMLURL,
		"clone_url":      r.CloneURL,
		"default_branch": r.DefaultBranch,
	}

	if r.Owner != nil {
		resp["owner"] = r.Owner.ToAPIResponse()
	}

	return resp
}

// MockPullRequest represents a GitHub pull request for testing.
type MockPullRequest struct {
	ID        int64
	Number    int
	Title     string
	Body      string
	State     string
	HTMLURL   string
	URL       string
	Head      MockPRRef
	Base      MockPRRef
	Draft     bool
	Mergeable bool
	User      *MockUser
	Assignees []*MockUser
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MockPRRef represents a pull request head/base reference.
type MockPRRef struct {
	Ref string
	SHA string
}

// ToAPIResponse converts the MockPullRequest to a GitHub API response format.
func (pr *MockPullRequest) ToAPIResponse() map[string]interface{} {
	resp := map[string]interface{}{
		"id":        pr.ID,
		"number":    pr.Number,
		"title":     pr.Title,
		"body":      pr.Body,
		"state":     pr.State,
		"html_url":  pr.HTMLURL,
		"url":       pr.URL,
		"draft":     pr.Draft,
		"mergeable": pr.Mergeable,
		"head": map[string]interface{}{
			"ref": pr.Head.Ref,
			"sha": pr.Head.SHA,
		},
		"base": map[string]interface{}{
			"ref": pr.Base.Ref,
		},
		"created_at": pr.CreatedAt.Format(time.RFC3339),
		"updated_at": pr.UpdatedAt.Format(time.RFC3339),
	}

	if pr.User != nil {
		resp["user"] = pr.User.ToAPIResponse()
	}
	if len(pr.Assignees) > 0 {
		assignees := make([]map[string]interface{}, 0, len(pr.Assignees))
		for _, assignee := range pr.Assignees {
			assignees = append(assignees, assignee.ToAPIResponse())
		}
		resp["assignees"] = assignees
	}

	return resp
}

// MockLabel represents a GitHub label for testing.
type MockLabel struct {
	ID          int64
	Name        string
	Description string
	Color       string
}

// MockTeam represents a GitHub organization team for testing.
type MockTeam struct {
	ID           int64
	Organization string
	Slug         string
	Name         string
	Description  string
}

// ToAPIResponse converts the MockTeam to a GitHub API response format.
func (t *MockTeam) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":          t.ID,
		"slug":        t.Slug,
		"name":        t.Name,
		"description": t.Description,
		"organization": map[string]interface{}{
			"login": t.Organization,
		},
	}
}

// ToAPIResponse converts the MockLabel to a GitHub API response format.
func (l *MockLabel) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":          l.ID,
		"name":        l.Name,
		"description": l.Description,
		"color":       l.Color,
	}
}

// MockRelease represents a GitHub release for testing.
type MockRelease struct {
	ID              int64
	TagName         string
	Name            string
	Body            string
	Draft           bool
	Prerelease      bool
	TargetCommitish string
	URL             string
	HTMLURL         string
	CreatedAt       time.Time
}

// ToAPIResponse converts the MockRelease to a GitHub API response format.
func (r *MockRelease) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":               r.ID,
		"tag_name":         r.TagName,
		"name":             r.Name,
		"body":             r.Body,
		"draft":            r.Draft,
		"prerelease":       r.Prerelease,
		"target_commitish": r.TargetCommitish,
		"url":              r.URL,
		"html_url":         r.HTMLURL,
		"created_at":       r.CreatedAt.Format(time.RFC3339),
	}
}

// -----------------------------------------------------------------------
// Fixture Factories - Helper functions to create common test fixtures
// -----------------------------------------------------------------------

// DefaultUser returns a default test user.
func DefaultUser() *MockUser {
	return &MockUser{
		ID:        1,
		Login:     "testuser",
		Name:      "Test User",
		Email:     "testuser@example.com",
		AvatarURL: "https://avatars.githubusercontent.com/u/1",
		HTMLURL:   "https://github.com/testuser",
		Type:      "User",
	}
}

// DefaultRepository returns a default test repository.
func DefaultRepository() *MockRepository {
	owner := DefaultUser()
	return &MockRepository{
		ID:            100,
		Name:          "testrepo",
		FullName:      "testowner/testrepo",
		Description:   "A test repository",
		Private:       false,
		HTMLURL:       "https://github.com/testowner/testrepo",
		CloneURL:      "https://github.com/testowner/testrepo.git",
		DefaultBranch: "main",
		Owner:         owner,
	}
}

// NewUser creates a new MockUser with the given parameters.
func NewUser(id int64, login, name string) *MockUser {
	return &MockUser{
		ID:        id,
		Login:     login,
		Name:      name,
		Email:     login + "@example.com",
		AvatarURL: "https://avatars.githubusercontent.com/u/" + itoa(int(id)),
		HTMLURL:   "https://github.com/" + login,
		Type:      "User",
	}
}

// NewRepository creates a new MockRepository with the given parameters.
func NewRepository(id int64, owner, name string) *MockRepository {
	return &MockRepository{
		ID:            id,
		Name:          name,
		FullName:      owner + "/" + name,
		Description:   "Repository " + name,
		Private:       false,
		HTMLURL:       "https://github.com/" + owner + "/" + name,
		CloneURL:      "https://github.com/" + owner + "/" + name + ".git",
		DefaultBranch: "main",
		Owner:         NewUser(1, owner, owner),
	}
}

// NewPullRequest creates a new MockPullRequest with the given parameters.
func NewPullRequest(number int, title, head, base, state string) *MockPullRequest {
	now := time.Now()
	return &MockPullRequest{
		ID:        int64(number * 1000),
		Number:    number,
		Title:     title,
		Body:      "Pull request body for " + title,
		State:     state,
		HTMLURL:   "https://github.com/testowner/testrepo/pull/" + itoa(number),
		URL:       "https://api.github.com/repos/testowner/testrepo/pulls/" + itoa(number),
		Head:      MockPRRef{Ref: head, SHA: "abc123def456"},
		Base:      MockPRRef{Ref: base},
		Draft:     false,
		Mergeable: true,
		User:      DefaultUser(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewDraftPullRequest creates a new draft MockPullRequest.
func NewDraftPullRequest(number int, title, head, base string) *MockPullRequest {
	pr := NewPullRequest(number, title, head, base, "open")
	pr.Draft = true
	return pr
}

// NewLabel creates a new MockLabel with the given parameters.
func NewLabel(id int64, name, description, color string) *MockLabel {
	return &MockLabel{
		ID:          id,
		Name:        name,
		Description: description,
		Color:       color,
	}
}

// NewTeam creates a new MockTeam with the given parameters.
func NewTeam(id int64, organization, slug, name, description string) *MockTeam {
	return &MockTeam{
		ID:           id,
		Organization: organization,
		Slug:         slug,
		Name:         name,
		Description:  description,
	}
}

// NewRelease creates a new MockRelease with the given parameters.
func NewRelease(id int64, tagName, name, targetCommitish string) *MockRelease {
	return &MockRelease{
		ID:              id,
		TagName:         tagName,
		Name:            name,
		Body:            "Release notes for " + tagName,
		Draft:           false,
		Prerelease:      false,
		TargetCommitish: targetCommitish,
		URL:             "https://api.github.com/repos/testowner/testrepo/releases/" + itoa(int(id)),
		HTMLURL:         "https://github.com/testowner/testrepo/releases/tag/" + tagName,
		CreatedAt:       time.Now(),
	}
}

// -----------------------------------------------------------------------
// Sample Fixtures - Pre-built test data sets
// -----------------------------------------------------------------------

// SampleUsers returns a set of sample users for testing.
func SampleUsers() []*MockUser {
	return []*MockUser{
		NewUser(1, "alice", "Alice Developer"),
		NewUser(2, "bob", "Bob Reviewer"),
		NewUser(3, "carol", "Carol Manager"),
		NewUser(4, "dave", "Dave Tester"),
	}
}

// SampleCollaborators returns a set of sample collaborators for testing.
func SampleCollaborators() []*MockUser {
	return []*MockUser{
		NewUser(1, "collaborator1", "Collaborator One"),
		NewUser(2, "collaborator2", "Collaborator Two"),
		NewUser(3, "collaborator3", "Collaborator Three"),
	}
}

// SampleLabels returns a set of sample labels for testing.
func SampleLabels() []*MockLabel {
	return []*MockLabel{
		NewLabel(1, "bug", "Something isn't working", "d73a4a"),
		NewLabel(2, "enhancement", "New feature or request", "a2eeef"),
		NewLabel(3, "documentation", "Improvements or additions to documentation", "0075ca"),
		NewLabel(4, "good first issue", "Good for newcomers", "7057ff"),
		NewLabel(5, "help wanted", "Extra attention is needed", "008672"),
		NewLabel(6, "priority: high", "High priority issue", "ff0000"),
		NewLabel(7, "priority: low", "Low priority issue", "00ff00"),
	}
}

// SampleTeams returns a set of sample teams for testing.
func SampleTeams(org string) []*MockTeam {
	return []*MockTeam{
		NewTeam(1, org, "platform", "Platform", "Platform engineering"),
		NewTeam(2, org, "frontend", "Frontend", "Frontend web clients"),
	}
}

// SamplePullRequests returns a set of sample pull requests for testing.
func SamplePullRequests() []*MockPullRequest {
	return []*MockPullRequest{
		NewPullRequest(1, "Add new feature", "feature-branch", "main", "open"),
		NewPullRequest(2, "Fix critical bug", "bugfix-branch", "main", "open"),
		NewDraftPullRequest(3, "Work in progress", "wip-branch", "main"),
		{
			ID:        4000,
			Number:    4,
			Title:     "Merged feature",
			Body:      "This PR was merged",
			State:     "closed",
			HTMLURL:   "https://github.com/testowner/testrepo/pull/4",
			URL:       "https://api.github.com/repos/testowner/testrepo/pulls/4",
			Head:      MockPRRef{Ref: "merged-branch", SHA: "def456"},
			Base:      MockPRRef{Ref: "main"},
			Draft:     false,
			Mergeable: true,
			User:      DefaultUser(),
			CreatedAt: time.Now().Add(-24 * time.Hour),
			UpdatedAt: time.Now(),
		},
	}
}

// SampleReleases returns a set of sample releases for testing.
func SampleReleases() []*MockRelease {
	return []*MockRelease{
		NewRelease(1, "v1.0.0", "Version 1.0.0", "main"),
		NewRelease(2, "v1.1.0", "Version 1.1.0", "main"),
		{
			ID:              3,
			TagName:         "v2.0.0-beta",
			Name:            "Version 2.0.0 Beta",
			Body:            "Beta release notes",
			Draft:           false,
			Prerelease:      true,
			TargetCommitish: "develop",
			URL:             "https://api.github.com/repos/testowner/testrepo/releases/3",
			HTMLURL:         "https://github.com/testowner/testrepo/releases/tag/v2.0.0-beta",
			CreatedAt:       time.Now(),
		},
	}
}

// -----------------------------------------------------------------------
// Utility functions
// -----------------------------------------------------------------------

// itoa converts an int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
