// Package testutil provides mock HTTP server utilities for testing Jira client code.
package testutil

// MockUser represents a Jira user for testing.
type MockUser struct {
	AccountID    string
	DisplayName  string
	EmailAddress string
	AvatarURLs   MockAvatarURLs
	Active       bool
}

// MockAvatarURLs contains avatar URLs in different sizes.
type MockAvatarURLs struct {
	Size16x16 string
	Size24x24 string
	Size32x32 string
	Size48x48 string
}

// ToAPIResponse converts the MockUser to a Jira API response format.
func (u *MockUser) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"accountId":    u.AccountID,
		"displayName":  u.DisplayName,
		"emailAddress": u.EmailAddress,
		"active":       u.Active,
		"avatarUrls": map[string]interface{}{
			"16x16": u.AvatarURLs.Size16x16,
			"24x24": u.AvatarURLs.Size24x24,
			"32x32": u.AvatarURLs.Size32x32,
			"48x48": u.AvatarURLs.Size48x48,
		},
	}
}

// MockIssue represents a Jira issue for testing.
type MockIssue struct {
	ID          string
	Key         string
	Summary     string
	Description string
	Status      MockStatus
	IssueType   MockIssueType
	Project     MockProject
	Assignee    *MockUser
	Reporter    *MockUser
	Labels      []string
	FixVersions []MockVersion
	Resolution  string
	Parent      *MockParent
}

// ToAPIResponse converts the MockIssue to a Jira API response format.
func (i *MockIssue) ToAPIResponse() map[string]interface{} {
	fields := map[string]interface{}{
		"summary":     i.Summary,
		"description": i.Description,
		"status":      i.Status.ToAPIResponse(),
		"issuetype":   i.IssueType.ToAPIResponse(),
		"project":     i.Project.ToAPIResponse(),
		"labels":      i.Labels,
	}

	if i.Assignee != nil {
		fields["assignee"] = i.Assignee.ToAPIResponse()
	}

	if i.Reporter != nil {
		fields["reporter"] = i.Reporter.ToAPIResponse()
	}

	if len(i.FixVersions) > 0 {
		var versions []map[string]interface{}
		for _, v := range i.FixVersions {
			versions = append(versions, v.ToAPIResponse())
		}
		fields["fixVersions"] = versions
	}

	if i.Resolution != "" {
		fields["resolution"] = map[string]interface{}{
			"name": i.Resolution,
		}
	}

	if i.Parent != nil {
		fields["parent"] = i.Parent.ToAPIResponse()
	}

	return map[string]interface{}{
		"id":     i.ID,
		"key":    i.Key,
		"self":   "https://example.atlassian.net/rest/api/2/issue/" + i.ID,
		"fields": fields,
	}
}

// MockStatus represents an issue status for testing.
type MockStatus struct {
	ID          string
	Name        string
	Description string
}

// ToAPIResponse converts the MockStatus to a Jira API response format.
func (s *MockStatus) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":          s.ID,
		"name":        s.Name,
		"description": s.Description,
	}
}

// MockIssueType represents an issue type for testing.
type MockIssueType struct {
	ID          string
	Name        string
	Description string
	Subtask     bool
}

// ToAPIResponse converts the MockIssueType to a Jira API response format.
func (t *MockIssueType) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":          t.ID,
		"name":        t.Name,
		"description": t.Description,
		"subtask":     t.Subtask,
	}
}

// MockProject represents a Jira project for testing.
type MockProject struct {
	ID   string
	Key  string
	Name string
}

// ToAPIResponse converts the MockProject to a Jira API response format.
func (p *MockProject) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":   p.ID,
		"key":  p.Key,
		"name": p.Name,
		"self": "https://example.atlassian.net/rest/api/2/project/" + p.ID,
	}
}

// MockTransition represents a workflow transition for testing.
type MockTransition struct {
	ID   string
	Name string
	To   MockStatus
}

// ToAPIResponse converts the MockTransition to a Jira API response format.
func (t *MockTransition) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":   t.ID,
		"name": t.Name,
		"to":   t.To.ToAPIResponse(),
	}
}

// MockVersion represents a project version for testing.
type MockVersion struct {
	ID          string
	Name        string
	Description string
	ProjectID   int
	Released    bool
	Archived    bool
}

// ToAPIResponse converts the MockVersion to a Jira API response format.
func (v *MockVersion) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":          v.ID,
		"name":        v.Name,
		"description": v.Description,
		"projectId":   v.ProjectID,
		"released":    v.Released,
		"archived":    v.Archived,
	}
}

// MockComment represents a comment on an issue for testing.
type MockComment struct {
	ID     string
	Body   string
	Author *MockUser
}

// MockParent represents a parent issue for sub-tasks.
type MockParent struct {
	ID  string
	Key string
}

// ToAPIResponse converts the MockParent to a Jira API response format.
func (p *MockParent) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":  p.ID,
		"key": p.Key,
	}
}

// -----------------------------------------------------------------------
// Fixture Factories - Helper functions to create common test fixtures
// -----------------------------------------------------------------------

// DefaultUser returns a default test user.
func DefaultUser() *MockUser {
	return &MockUser{
		AccountID:    "user-123",
		DisplayName:  "John Doe",
		EmailAddress: "john.doe@example.com",
		Active:       true,
		AvatarURLs: MockAvatarURLs{
			Size16x16: "https://example.com/avatar_16.png",
			Size24x24: "https://example.com/avatar_24.png",
			Size32x32: "https://example.com/avatar_32.png",
			Size48x48: "https://example.com/avatar_48.png",
		},
	}
}

// DefaultProject returns a default test project.
func DefaultProject() *MockProject {
	return &MockProject{
		ID:   "10000",
		Key:  "TEST",
		Name: "Test Project",
	}
}

// NewUser creates a new MockUser with the given parameters.
func NewUser(accountID, displayName, email string) *MockUser {
	return &MockUser{
		AccountID:    accountID,
		DisplayName:  displayName,
		EmailAddress: email,
		Active:       true,
		AvatarURLs: MockAvatarURLs{
			Size48x48: "https://example.com/avatar_" + accountID + ".png",
		},
	}
}

// NewProject creates a new MockProject with the given parameters.
func NewProject(id, key, name string) *MockProject {
	return &MockProject{
		ID:   id,
		Key:  key,
		Name: name,
	}
}

// NewIssue creates a new MockIssue with common defaults.
func NewIssue(key, summary, statusName, typeName string) *MockIssue {
	parts := splitIssueKey(key)
	return &MockIssue{
		ID:      "100" + parts.num,
		Key:     key,
		Summary: summary,
		Status: MockStatus{
			ID:   statusIDFromName(statusName),
			Name: statusName,
		},
		IssueType: MockIssueType{
			ID:   typeIDFromName(typeName),
			Name: typeName,
		},
		Project: MockProject{
			ID:   "10000",
			Key:  parts.project,
			Name: parts.project + " Project",
		},
	}
}

// NewIssueWithAssignee creates a new MockIssue with an assignee.
func NewIssueWithAssignee(key, summary, statusName, typeName string, assignee *MockUser) *MockIssue {
	issue := NewIssue(key, summary, statusName, typeName)
	issue.Assignee = assignee
	return issue
}

// NewStoryIssue creates a Story issue with common defaults.
func NewStoryIssue(key, summary, statusName string) *MockIssue {
	return NewIssue(key, summary, statusName, "Story")
}

// NewBugIssue creates a Bug issue with common defaults.
func NewBugIssue(key, summary, statusName string) *MockIssue {
	return NewIssue(key, summary, statusName, "Bug")
}

// NewTaskIssue creates a Task issue with common defaults.
func NewTaskIssue(key, summary, statusName string) *MockIssue {
	return NewIssue(key, summary, statusName, "Task")
}

// NewSubTaskIssue creates a Sub-task issue with a parent.
func NewSubTaskIssue(key, summary, statusName, parentKey string) *MockIssue {
	issue := NewIssue(key, summary, statusName, "Sub-task")
	issue.IssueType.Subtask = true
	issue.Parent = &MockParent{
		Key: parentKey,
	}
	return issue
}

// NewTransition creates a new MockTransition.
func NewTransition(id, name, toStatusID, toStatusName string) MockTransition {
	return MockTransition{
		ID:   id,
		Name: name,
		To: MockStatus{
			ID:   toStatusID,
			Name: toStatusName,
		},
	}
}

// DefaultTransitions returns a common set of workflow transitions.
func DefaultTransitions() []MockTransition {
	return []MockTransition{
		NewTransition("11", "To Do", "10001", "To Do"),
		NewTransition("21", "In Progress", "10002", "In Progress"),
		NewTransition("31", "In Review", "10003", "In Review"),
		NewTransition("41", "Done", "10004", "Done"),
	}
}

// InProgressTransitions returns transitions typically available when starting work.
func InProgressTransitions() []MockTransition {
	return []MockTransition{
		NewTransition("21", "Start Progress", "10002", "In Progress"),
		NewTransition("41", "Done", "10004", "Done"),
	}
}

// ReviewTransitions returns transitions typically available after development.
func ReviewTransitions() []MockTransition {
	return []MockTransition{
		NewTransition("31", "Ready for Review", "10003", "In Review"),
		NewTransition("21", "Back to Progress", "10002", "In Progress"),
		NewTransition("41", "Done", "10004", "Done"),
	}
}

// NewVersion creates a new MockVersion.
func NewVersion(id, name, description string, projectID int) *MockVersion {
	return &MockVersion{
		ID:          id,
		Name:        name,
		Description: description,
		ProjectID:   projectID,
		Released:    false,
		Archived:    false,
	}
}

// -----------------------------------------------------------------------
// Sample Fixtures - Pre-built test data sets
// -----------------------------------------------------------------------

// SampleUsers returns a set of sample users for testing.
func SampleUsers() []*MockUser {
	return []*MockUser{
		{
			AccountID:    "user-001",
			DisplayName:  "Alice Developer",
			EmailAddress: "alice@example.com",
			Active:       true,
			AvatarURLs: MockAvatarURLs{
				Size48x48: "https://example.com/alice.png",
			},
		},
		{
			AccountID:    "user-002",
			DisplayName:  "Bob Tester",
			EmailAddress: "bob@example.com",
			Active:       true,
			AvatarURLs: MockAvatarURLs{
				Size48x48: "https://example.com/bob.png",
			},
		},
		{
			AccountID:    "user-003",
			DisplayName:  "Carol Manager",
			EmailAddress: "carol@example.com",
			Active:       true,
			AvatarURLs: MockAvatarURLs{
				Size48x48: "https://example.com/carol.png",
			},
		},
	}
}

// SampleProjects returns a set of sample projects for testing.
func SampleProjects() []*MockProject {
	return []*MockProject{
		{ID: "10000", Key: "TEST", Name: "Test Project"},
		{ID: "10001", Key: "DEV", Name: "Development Project"},
		{ID: "10002", Key: "PROD", Name: "Production Project"},
	}
}

// SampleIssues returns a set of sample issues for testing.
func SampleIssues() []*MockIssue {
	alice := SampleUsers()[0]
	return []*MockIssue{
		NewIssueWithAssignee("TEST-1", "Implement user authentication", "In Progress", "Story", alice),
		NewIssueWithAssignee("TEST-2", "Fix login page bug", "To Do", "Bug", alice),
		NewIssueWithAssignee("TEST-3", "Add unit tests for auth module", "Backlog", "Task", alice),
		NewStoryIssue("TEST-4", "Design new dashboard", "Done"),
		NewBugIssue("TEST-5", "Memory leak in background service", "In Review"),
	}
}

// -----------------------------------------------------------------------
// Utility functions
// -----------------------------------------------------------------------

type issueKeyParts struct {
	project string
	num     string
}

func splitIssueKey(key string) issueKeyParts {
	for i, c := range key {
		if c == '-' {
			return issueKeyParts{
				project: key[:i],
				num:     key[i+1:],
			}
		}
	}
	return issueKeyParts{project: key, num: "1"}
}

func statusIDFromName(name string) string {
	switch name {
	case "Backlog":
		return "10000"
	case "To Do":
		return "10001"
	case "In Progress":
		return "10002"
	case "In Review":
		return "10003"
	case "Done":
		return "10004"
	default:
		return "10000"
	}
}

func typeIDFromName(name string) string {
	switch name {
	case "Story":
		return "10001"
	case "Bug":
		return "10002"
	case "Task":
		return "10003"
	case "Sub-task":
		return "10004"
	case "Epic":
		return "10005"
	default:
		return "10003"
	}
}
