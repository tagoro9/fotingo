// Package tracker provides a generic abstraction for issue tracking systems.
package tracker

// IssueType represents the type of an issue.
type IssueType string

// Issue types supported by the tracker abstraction.
const (
	IssueTypeStory   IssueType = "Story"
	IssueTypeBug     IssueType = "Bug"
	IssueTypeTask    IssueType = "Task"
	IssueTypeSubTask IssueType = "SubTask"
	IssueTypeEpic    IssueType = "Epic"
)

// IssueStatus represents the workflow status of an issue.
type IssueStatus string

// Issue statuses supported by the tracker abstraction.
const (
	IssueStatusBacklog    IssueStatus = "Backlog"
	IssueStatusToDo       IssueStatus = "ToDo"
	IssueStatusInProgress IssueStatus = "InProgress"
	IssueStatusInReview   IssueStatus = "InReview"
	IssueStatusDone       IssueStatus = "Done"
)

// User represents a user in the issue tracking system.
type User struct {
	ID        string
	Name      string
	Email     string
	AvatarURL string
}

// Issue represents an issue in the tracking system.
type Issue struct {
	ID          string
	Key         string
	Summary     string
	Description string
	Type        IssueType
	Status      IssueStatus
	Assignee    *User
	URL         string
}

// ProjectIssueType represents an available issue type for a project.
type ProjectIssueType struct {
	ID          string
	Name        string
	Description string
	Subtask     bool
}

// CreateIssueInput contains the data needed to create a new issue.
type CreateIssueInput struct {
	Title       string
	Project     string
	Type        IssueType
	TypeName    string
	ParentID    string
	EpicID      string
	Labels      []string
	Description string
}

// Release represents a release or version in the tracking system.
type Release struct {
	ID          string
	Name        string
	Description string
	URL         string
}

// CreateReleaseInput contains the data needed to create a new release.
type CreateReleaseInput struct {
	Name        string
	Description string
	IssueIDs    []string
}
