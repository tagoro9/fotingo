package tracker

// Tracker defines the interface for interacting with an issue tracking system.
// Implementations may include Jira, GitHub Issues, Linear, etc.
type Tracker interface {
	// Name returns the name of the tracker implementation (e.g., "Jira", "GitHub").
	Name() string

	// GetCurrentUser returns the currently authenticated user.
	GetCurrentUser() (*User, error)

	// GetUserOpenIssues returns all open issues assigned to the current user.
	GetUserOpenIssues() ([]Issue, error)

	// GetIssue retrieves an issue by its ID or key.
	GetIssue(id string) (*Issue, error)

	// AssignIssue assigns an issue to the provided user ID.
	AssignIssue(id string, userID string) (*Issue, error)

	// CreateIssue creates a new issue with the given input data.
	CreateIssue(data CreateIssueInput) (*Issue, error)

	// GetProjectIssueTypes returns available issue types for the provided project.
	GetProjectIssueTypes(projectKey string) ([]ProjectIssueType, error)

	// SetIssueStatus transitions an issue to the specified status.
	SetIssueStatus(id string, status IssueStatus) (*Issue, error)

	// AddComment adds a comment to an issue.
	AddComment(id string, comment string) error

	// CreateRelease creates a new release in the tracker.
	CreateRelease(data CreateReleaseInput) (*Release, error)

	// SetFixVersion associates issues with a release.
	SetFixVersion(issueIDs []string, release *Release) error

	// IsValidIssueID checks if the given string is a valid issue ID format.
	IsValidIssueID(id string) bool

	// GetIssueURL returns the web URL for viewing an issue.
	GetIssueURL(id string) string
}
