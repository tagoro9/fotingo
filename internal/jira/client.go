package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	jiraClient "github.com/andygrunwald/go-jira"
	"github.com/spf13/viper"
	"github.com/tagoro9/fotingo/internal/auth"
	"github.com/tagoro9/fotingo/internal/cache"
	"github.com/tagoro9/fotingo/internal/config"
	"github.com/tagoro9/fotingo/internal/tracker"
	"github.com/tagoro9/fotingo/internal/ui"
	"github.com/texttheater/golang-levenshtein/levenshtein"
	"github.com/trivago/tgo/tcontainer"
	"golang.org/x/oauth2"
)

const jiraEpicLinkSchemaCustom = "com.pyxis.greenhopper.jira:gh-epic-link"

const (
	jiraAuthMethodOAuth = "oauth"
	jiraAuthMethodToken = "token"
)

type IssueStatus string

type Issue struct {
	Id          string
	Key         string
	Summary     string
	Description string
	Status      string
	Type        string
	ParentKey   string
	EpicKey     string
	Assignee    *tracker.User
}

// Info returns the issue key for branch template compatibility.
// Legacy branch templates reference {{.Issue.Info}}.
func (i *Issue) Info() string {
	if i == nil {
		return ""
	}
	return i.Key
}

// ShortName returns a short prefix for the issue type
func (i *Issue) ShortName() string {
	switch strings.ToLower(i.Type) {
	case "bug":
		return "b"
	case "feature", "story", "sub-task":
		return "f"
	case "spike":
		return "s"
	case "task":
		return "c"
	case "tech debt":
		return "d"
	default:
		return "f" // Default to feature prefix
	}
}

// SanitizedSummary returns a sanitized summary of the issue
func (i *Issue) SanitizedSummary() string {
	summary := strings.ToLower(i.Summary)
	// Remove special characters
	summary = regexp.MustCompile(`["$&'*,:;<>?@[\]`+"`"+`~''""]`).ReplaceAllString(summary, "")
	// Replace slashes, dots and multiple dashes with single dash
	summary = regexp.MustCompile(`\/|\.|--=`).ReplaceAllString(summary, "-")
	// Replace spaces, parentheses and multiple underscores with single underscore
	summary = regexp.MustCompile(`\s|\(|\)|__+`).ReplaceAllString(summary, "_")
	// Remove trailing underscores/dashes
	summary = regexp.MustCompile(`([_-])$`).ReplaceAllString(summary, "")
	// Truncate to 72 chars
	if len(summary) > 72 {
		summary = summary[:72]
	}
	return summary
}

func newIssue(issue *jiraClient.Issue) *Issue {
	if issue == nil {
		return &Issue{}
	}

	summary := ""
	description := ""
	status := ""
	issueType := ""
	parentKey := ""
	epicKey := ""
	var assignee *tracker.User

	if issue.Fields != nil {
		summary = issue.Fields.Summary
		description = issue.Fields.Description
		if issue.Fields.Status != nil {
			status = issue.Fields.Status.Name
		}
		issueType = issue.Fields.Type.Name
		if issue.Fields.Parent != nil {
			parentKey = strings.TrimSpace(issue.Fields.Parent.Key)
		}
		if issue.Fields.Epic != nil {
			epicKey = strings.TrimSpace(issue.Fields.Epic.Key)
		}
		if epicKey == "" && parentKey != "" && !strings.EqualFold(issueType, "sub-task") && !strings.EqualFold(issueType, "subtask") {
			// In modern Jira projects, epic hierarchy can be represented via parent on non-subtasks.
			epicKey = parentKey
		}
		if issue.Fields.Assignee != nil {
			assignee = toTrackerUser(issue.Fields.Assignee)
		}
	}

	return &Issue{
		Id:          issue.ID,
		Key:         issue.Key,
		Summary:     summary,
		Description: description,
		// TODO Convert to IssueStatus
		Status:    status,
		Type:      issueType,
		ParentKey: parentKey,
		EpicKey:   epicKey,
		Assignee:  assignee,
	}
}

// jiraIssueStatusToTracker converts a Jira status name to a tracker.IssueStatus.
func jiraIssueStatusToTracker(status string) tracker.IssueStatus {
	statusLower := strings.ToLower(status)
	switch {
	case strings.Contains(statusLower, "backlog"):
		return tracker.IssueStatusBacklog
	case strings.Contains(statusLower, "to do") || strings.Contains(statusLower, "todo") || strings.Contains(statusLower, "selected"):
		return tracker.IssueStatusToDo
	case strings.Contains(statusLower, "progress"):
		return tracker.IssueStatusInProgress
	case strings.Contains(statusLower, "review"):
		return tracker.IssueStatusInReview
	case strings.Contains(statusLower, "done") || strings.Contains(statusLower, "complete") || strings.Contains(statusLower, "resolved"):
		return tracker.IssueStatusDone
	default:
		return tracker.IssueStatusBacklog
	}
}

// trackerStatusToJira converts a tracker.IssueStatus to the Jira IssueStatus.
func trackerStatusToJira(status tracker.IssueStatus) IssueStatus {
	switch status {
	case tracker.IssueStatusBacklog:
		return StatusBacklog
	case tracker.IssueStatusToDo:
		return StatusSelectedForDevelopment
	case tracker.IssueStatusInProgress:
		return StatusInProgress
	case tracker.IssueStatusInReview:
		return StatusInReview
	case tracker.IssueStatusDone:
		return StatusDone
	default:
		return StatusBacklog
	}
}

// jiraIssueTypeToTracker converts a Jira issue type name to a tracker.IssueType.
func jiraIssueTypeToTracker(issueType string) tracker.IssueType {
	typeLower := strings.ToLower(issueType)
	switch {
	case strings.Contains(typeLower, "story") || strings.Contains(typeLower, "feature"):
		return tracker.IssueTypeStory
	case strings.Contains(typeLower, "bug"):
		return tracker.IssueTypeBug
	case strings.Contains(typeLower, "sub-task") || strings.Contains(typeLower, "subtask"):
		return tracker.IssueTypeSubTask
	case strings.Contains(typeLower, "epic"):
		return tracker.IssueTypeEpic
	case strings.Contains(typeLower, "task"):
		return tracker.IssueTypeTask
	default:
		return tracker.IssueTypeTask
	}
}

// trackerIssueTypeToJira converts a tracker.IssueType to a Jira issue type name.
func trackerIssueTypeToJira(issueType tracker.IssueType) string {
	switch issueType {
	case tracker.IssueTypeStory:
		return "Story"
	case tracker.IssueTypeBug:
		return "Bug"
	case tracker.IssueTypeSubTask:
		return "Sub-task"
	case tracker.IssueTypeEpic:
		return "Epic"
	case tracker.IssueTypeTask:
		return "Task"
	default:
		return "Task"
	}
}

// toTrackerIssue converts a Jira Issue to a tracker.Issue.
func (j *jira) toTrackerIssue(issue *Issue) *tracker.Issue {
	return &tracker.Issue{
		ID:          issue.Id,
		Key:         issue.Key,
		Summary:     issue.Summary,
		Description: issue.Description,
		Type:        jiraIssueTypeToTracker(issue.Type),
		Status:      jiraIssueStatusToTracker(issue.Status),
		Assignee:    issue.Assignee,
		URL:         j.GetIssueURL(issue.Key),
	}
}

// toTrackerUser converts a Jira user to a tracker.User.
func toTrackerUser(user *jiraClient.User) *tracker.User {
	if user == nil {
		return nil
	}

	avatarURL := ""
	if user.AvatarUrls.Four8X48 != "" {
		avatarURL = user.AvatarUrls.Four8X48
	}
	return &tracker.User{
		ID:        user.AccountID,
		Name:      user.DisplayName,
		Email:     user.EmailAddress,
		AvatarURL: avatarURL,
	}
}

// issueIDPattern matches Jira issue IDs like "PROJ-123"
var issueIDPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]+-\d+$`)
var jiraStatusCodePattern = regexp.MustCompile(`(?i)status code:\s*(\d+)`)

const (
	StatusBacklog                IssueStatus = "Backlog"
	StatusSelectedForDevelopment IssueStatus = "Selected for Development"
	StatusInProgress             IssueStatus = "In Progress"
	StatusInReview               IssueStatus = "In Review"
	StatusDone                   IssueStatus = "Done"
)

// Jira interface defines the contract for Jira operations.
// This interface embeds tracker.Tracker for compatibility with the generic tracker abstraction.
type Jira interface {
	auth.OauthService
	config.ConfigurableService
	tracker.Tracker

	// GetConfig returns the Viper configuration for this Jira client.
	GetConfig() *viper.Viper

	// GetIssueUrl returns the URL of the issue given its id.
	// Deprecated: Use GetIssueURL from tracker.Tracker interface instead.
	GetIssueUrl(issueId string) (string, error)

	// GetJiraIssue returns the Jira-specific Issue type.
	// Use this when you need access to Jira-specific fields like ShortName() or SanitizedSummary().
	GetJiraIssue(issueId string) (*Issue, error)

	// SetJiraIssueStatus transitions an issue to the specified Jira status.
	// Use this when you need to work with Jira-specific IssueStatus values.
	SetJiraIssueStatus(issueId string, status IssueStatus) (*Issue, error)

	// SearchIssues searches issues in Jira for link-resolution flows.
	SearchIssues(projectKey string, query string, issueTypes []tracker.IssueType, limit int) ([]tracker.Issue, error)
}

// Compile-time assertion that jira implements tracker.Tracker
var _ tracker.Tracker = (*jira)(nil)

type jira struct {
	*authenticator
	*config.ViperConfigurableService
	client           *jiraClient.Client
	jiraRootURL      string
	allowPrompt      bool
	promptRoot       func() (string, error)
	promptAuthMethod func() (string, error)
	promptAPICreds   func() (string, string, error)
	metadataCache    cache.Store
	cacheInitErr     error
}

type jiraSite struct {
	Id        string   `json:"id"`
	Url       string   `json:"url"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	AvatarUrl string   `json:"avatarUrl"`
}

const defaultIssueTypesCacheTTL = 24 * time.Hour

var newIssueTypesCacheStore = func(cfg *viper.Viper) (cache.Store, error) {
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

// Name returns the name of this tracker implementation.
func (j *jira) Name() string {
	return "Jira"
}

// GetCurrentUser returns the information about the current authenticated user.
// This implements the tracker.Tracker interface.
func (j *jira) GetCurrentUser() (*tracker.User, error) {
	self, _, err := j.client.User.GetSelf()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}
	return toTrackerUser(self), nil
}

func (j *jira) Authenticate() (token *auth.AccessToken, err error) {
	rootURL, err := j.resolveJiraRootURL()
	if err != nil {
		return nil, err
	}

	login, apiToken, hasAPICredentials := j.configuredAPICredentials()

	if j.hasOAuthCredentials() {
		if j.hasStoredOAuthToken() {
			return j.authenticateWithOAuth(rootURL)
		}
		if hasAPICredentials {
			return j.authenticateWithAPIToken(rootURL, login, apiToken)
		}
		if !j.allowPrompt || !isInputTerminalFn() {
			return nil, ErrAuthRequired
		}

		authPrompt := j.promptAuthMethod
		if authPrompt == nil {
			authPrompt = promptJiraAuthMethod
		}
		method, promptErr := authPrompt()
		if promptErr != nil {
			return nil, promptErr
		}

		switch strings.ToLower(strings.TrimSpace(method)) {
		case "", jiraAuthMethodOAuth:
			return j.authenticateWithOAuth(rootURL)
		case jiraAuthMethodToken:
			return j.promptAndAuthenticateWithAPIToken(rootURL, true)
		default:
			return nil, fmt.Errorf("unsupported jira auth method: %s", method)
		}
	}

	if hasAPICredentials {
		return j.authenticateWithAPIToken(rootURL, login, apiToken)
	}
	if !j.allowPrompt || !isInputTerminalFn() {
		return nil, ErrAuthRequired
	}

	return j.promptAndAuthenticateWithAPIToken(rootURL, false)
}

func (j *jira) authenticateWithOAuth(rootURL string) (*auth.AccessToken, error) {
	if err := j.validateOAuthConfig(); err != nil {
		return nil, err
	}

	token, err := j.authenticator.Authenticate()
	if err != nil {
		return nil, err
	}

	httpClient := j.authConfig.Client(context.TODO(), &oauth2.Token{
		AccessToken:  token.Token,
		RefreshToken: token.RefreshToken,
		TokenType:    token.Type,
		Expiry:       token.Expiry,
	})
	httpClient = wrapJiraHTTPClient(httpClient)

	siteID, err := j.resolveOAuthSiteID(rootURL, httpClient)
	if err != nil {
		return nil, err
	}

	jiraClientWithAuth, err := jiraClient.NewClient(httpClient, fmt.Sprintf("https://api.atlassian.com/ex/jira/%s", siteID))
	if err != nil {
		return nil, err
	}
	j.client = jiraClientWithAuth
	return token, nil
}

func (j *jira) authenticateWithAPIToken(rootURL, login, token string) (*auth.AccessToken, error) {
	login = strings.TrimSpace(login)
	token = strings.TrimSpace(token)
	if login == "" || token == "" {
		return nil, ErrAuthRequired
	}

	transport := &jiraClient.BasicAuthTransport{
		Username: login,
		Password: token,
	}
	jiraClientWithAuth, err := jiraClient.NewClient(wrapJiraHTTPClient(transport.Client()), rootURL)
	if err != nil {
		return nil, err
	}

	j.client = jiraClientWithAuth
	return &auth.AccessToken{}, nil
}

func (j *jira) promptAndAuthenticateWithAPIToken(rootURL string, clearOAuthToken bool) (*auth.AccessToken, error) {
	credentialsPrompt := j.promptAPICreds
	if credentialsPrompt == nil {
		credentialsPrompt = promptJiraAPICredentials
	}
	promptLogin, promptToken, promptErr := credentialsPrompt()
	if promptErr != nil {
		return nil, promptErr
	}
	if err := j.SaveConfig("user.login", promptLogin); err != nil {
		return nil, err
	}
	if err := j.SaveConfig("user.token", promptToken); err != nil {
		return nil, err
	}
	if clearOAuthToken {
		if err := j.SaveConfig("token", ""); err != nil {
			return nil, err
		}
	}
	return j.authenticateWithAPIToken(rootURL, promptLogin, promptToken)
}

func (j *jira) configuredAPICredentials() (string, string, bool) {
	login := strings.TrimSpace(j.GetConfigString("user.login"))
	apiToken := strings.TrimSpace(j.GetConfigString("user.token"))

	if login == "" || apiToken == "" {
		return "", "", false
	}

	return login, apiToken, true
}

func (j *jira) hasStoredOAuthToken() bool {
	return isSerializedOAuthToken(j.GetConfigString("token"))
}

func (j *jira) hasOAuthCredentials() bool {
	if j.authenticator == nil || j.authConfig == nil {
		return false
	}
	return strings.TrimSpace(j.authConfig.ClientID) != "" && strings.TrimSpace(j.authConfig.ClientSecret) != ""
}

func isSerializedOAuthToken(raw string) bool {
	var token auth.AccessToken
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &token); err != nil {
		return false
	}
	return strings.TrimSpace(token.Token) != ""
}

func oauthCredentialsAvailableInBuild() bool {
	return strings.TrimSpace(jiraOAuthClientID) != "" && strings.TrimSpace(jiraOAuthClientSecret) != ""
}

// ShouldWarnIgnoredStoredOAuthToken reports when a stored OAuth token will be
// ignored because the current binary is not OAuth-capable.
func ShouldWarnIgnoredStoredOAuthToken(cfg *viper.Viper) bool {
	if cfg == nil || oauthCredentialsAvailableInBuild() {
		return false
	}
	return isSerializedOAuthToken(strings.TrimSpace(cfg.GetString("jira.token")))
}

// IsUnauthorizedError returns true when the Jira API error corresponds to HTTP 401.
func IsUnauthorizedError(err error) bool {
	if err == nil {
		return false
	}
	matches := jiraStatusCodePattern.FindStringSubmatch(err.Error())
	if len(matches) >= 2 {
		statusCode, parseErr := strconv.Atoi(matches[1])
		if parseErr == nil {
			return statusCode == http.StatusUnauthorized
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "unauthorized")
}

// GetIssueURL returns the URL of the issue given its id.
// This implements the tracker.Tracker interface.
func (j *jira) GetIssueURL(issueId string) string {
	rootURL, err := j.resolveJiraRootURL()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s/browse/%s", rootURL, strings.ToUpper(issueId))
}

// GetIssueUrl returns the URL of the issue given its id.
// Deprecated: Use GetIssueURL instead.
func (j *jira) GetIssueUrl(issueId string) (string, error) {
	return j.GetIssueURL(issueId), nil
}

// GetIssue returns the issue given its id.
// This implements the tracker.Tracker interface.
func (j *jira) GetIssue(issueId string) (*tracker.Issue, error) {
	jiraIssue, err := j.GetJiraIssue(issueId)
	if err != nil {
		return nil, err
	}
	return j.toTrackerIssue(jiraIssue), nil
}

// AssignIssue assigns an issue to the provided user ID.
// This implements the tracker.Tracker interface.
func (j *jira) AssignIssue(issueID string, userID string) (*tracker.Issue, error) {
	updateData := map[string]interface{}{
		"fields": map[string]interface{}{
			"assignee": map[string]string{
				"accountId": userID,
			},
		},
	}

	_, err := j.client.Issue.UpdateIssue(issueID, updateData)
	if err != nil {
		return nil, fmt.Errorf("failed to assign issue %s: %w", issueID, err)
	}

	return j.GetIssue(issueID)
}

// GetJiraIssue returns the Jira-specific Issue type.
// Use this when you need access to Jira-specific fields like ShortName() or SanitizedSummary().
func (j *jira) GetJiraIssue(issueId string) (*Issue, error) {
	issue, _, err := j.client.Issue.Get(issueId, &jiraClient.GetQueryOptions{
		Expand: "transitions, renderedFields",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get issue %s: %w", issueId, err)
	}
	return newIssue(issue), nil
}

// TransitionMapping maps IssueStatus to regex patterns that should match transition names
type TransitionMapping struct {
	Status  IssueStatus
	Pattern *regexp.Regexp
	Exact   bool // If true, only use exact matches
}

var defaultTransitionMappings = []TransitionMapping{
	{
		Status:  StatusInProgress,
		Pattern: regexp.MustCompile(`(?i)(start|begin|progress)`),
	},
	{
		Status:  StatusDone,
		Pattern: regexp.MustCompile(`(?i)(done|complete|finish|resolved)`),
	},
	{
		Status:  StatusInReview,
		Pattern: regexp.MustCompile(`(?i)(review|verify)`),
	},
}

func (j *jira) getIssueTransitions(issueId string) ([]jiraClient.Transition, error) {
	transitions, _, err := j.client.Issue.GetTransitions(issueId)
	return transitions, err
}

// SetIssueStatus transitions an issue to the specified tracker status.
// This implements the tracker.Tracker interface.
func (j *jira) SetIssueStatus(issueId string, status tracker.IssueStatus) (*tracker.Issue, error) {
	jiraStatus := trackerStatusToJira(status)
	jiraIssue, err := j.SetJiraIssueStatus(issueId, jiraStatus)
	if err != nil {
		return nil, err
	}
	return j.toTrackerIssue(jiraIssue), nil
}

// SetJiraIssueStatus transitions an issue to the specified Jira status by finding
// the closest matching transition name and applying it.
// Use this when you need to work with Jira-specific IssueStatus values.
func (j *jira) SetJiraIssueStatus(issueId string, targetStatus IssueStatus) (*Issue, error) {
	// Get the issue with its available transitions
	transitions, err := j.getIssueTransitions(issueId)
	if err != nil {
		// TODO Analyze the error and see if it's a 404 to return a better message / code
		return nil, fmt.Errorf("failed to get issue %s: %w", issueId, err)
	}

	// Find the best matching transition
	var bestMatch *jiraClient.Transition
	bestScore := 0.0

	// First try exact matches
	for _, transition := range transitions {
		if strings.EqualFold(transition.Name, string(targetStatus)) {
			bestMatch = &transition
			break
		}
	}

	// If no exact match, try predefined mappings
	if bestMatch == nil {
		for _, mapping := range defaultTransitionMappings {
			if mapping.Status == targetStatus {
				for _, transition := range transitions {
					if mapping.Pattern.MatchString(transition.Name) {
						bestMatch = &transition
						break
					}
				}
				if bestMatch != nil {
					break
				}
			}
		}
	}

	// If still no match, use string similarity as fallback
	if bestMatch == nil {
		for _, transition := range transitions {
			score := 1.0 - (float64(levenshtein.DistanceForStrings(
				[]rune(strings.ToLower(transition.Name)),
				[]rune(strings.ToLower(string(targetStatus))),
				levenshtein.DefaultOptions,
			)) / float64(maxInt(len(transition.Name), len(targetStatus))))

			if score > bestScore {
				bestScore = score
				bestMatch = &transition
			}
		}

		// Only accept similarity matches above threshold
		if bestScore < 0.5 {
			bestMatch = nil
		}
	}

	if bestMatch == nil {
		return nil, fmt.Errorf("no matching transition found for status '%s' in issue %s", targetStatus, issueId)
	}

	// Perform the transition
	_, err = j.client.Issue.DoTransition(issueId, bestMatch.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to transition issue %s to '%s': %w", issueId, bestMatch.Name, err)
	}

	return j.GetJiraIssue(issueId)
}

// GetUserOpenIssues returns all open issues assigned to the current user.
// This implements the tracker.Tracker interface.
func (j *jira) GetUserOpenIssues() ([]tracker.Issue, error) {
	jql := "assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC"
	searchOptions := &jiraClient.SearchOptionsV2{
		Fields:     []string{"description", "key", "project", "status", "summary", "type"},
		MaxResults: 50,
	}

	issues, _, err := j.client.Issue.SearchV2JQL(jql, searchOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to search for open issues: %w", err)
	}

	result := make([]tracker.Issue, 0, len(issues))
	for _, issue := range issues {
		jiraIssue := newIssue(&issue)
		result = append(result, *j.toTrackerIssue(jiraIssue))
	}

	return result, nil
}

// CreateIssue creates a new issue with the given input data.
// This implements the tracker.Tracker interface.
func (j *jira) CreateIssue(data tracker.CreateIssueInput) (*tracker.Issue, error) {
	issueTypeName := j.resolveCreateIssueTypeName(data.Project, data.Type, data.TypeName)

	issueFields := &jiraClient.IssueFields{
		Summary:     data.Title,
		Description: data.Description,
		Project: jiraClient.Project{
			Key: data.Project,
		},
		Type: jiraClient.IssueType{
			Name: issueTypeName,
		},
	}

	// Add labels if provided
	if len(data.Labels) > 0 {
		issueFields.Labels = data.Labels
	}

	// Set parent for sub-tasks
	if data.ParentID != "" && data.Type == tracker.IssueTypeSubTask {
		issueFields.Parent = &jiraClient.Parent{
			Key: data.ParentID,
		}
	}

	if strings.TrimSpace(data.EpicID) != "" {
		epicID := strings.TrimSpace(data.EpicID)
		epicFieldID, err := j.resolveEpicLinkFieldID()
		if err != nil {
			if errors.Is(err, errEpicLinkFieldNotFound) {
				// Modern Jira projects can model epic as parent and omit the legacy Epic Link field.
				if issueFields.Parent == nil {
					issueFields.Parent = &jiraClient.Parent{Key: epicID}
				}
			} else {
				return nil, fmt.Errorf("failed to resolve epic link field: %w", err)
			}
		} else {
			if issueFields.Unknowns == nil {
				issueFields.Unknowns = tcontainer.NewMarshalMap()
			}
			issueFields.Unknowns[epicFieldID] = epicID
		}
	}

	issue := &jiraClient.Issue{
		Fields: issueFields,
	}

	createdIssue, _, err := j.client.Issue.Create(issue)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	jiraIssue := newIssue(createdIssue)
	return j.toTrackerIssue(jiraIssue), nil
}

func (j *jira) resolveCreateIssueTypeName(projectKey string, issueType tracker.IssueType, preferred string) string {
	normalizedPreferred := normalizeIssueTypeName(preferred)
	if normalizedPreferred != "" {
		return normalizedPreferred
	}

	projectIssueTypes, err := j.GetProjectIssueTypes(projectKey)
	if err == nil {
		for _, projectIssueType := range projectIssueTypes {
			name := strings.TrimSpace(projectIssueType.Name)
			if name == "" {
				continue
			}

			if issueType == tracker.IssueTypeSubTask && projectIssueType.Subtask {
				return name
			}
			if jiraIssueTypeToTracker(name) == issueType {
				return name
			}
		}
	}

	return trackerIssueTypeToJira(issueType)
}

func normalizeIssueTypeName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}

	switch {
	case strings.EqualFold(name, "story"):
		return "Story"
	case strings.EqualFold(name, "bug"):
		return "Bug"
	case strings.EqualFold(name, "task"):
		return "Task"
	case strings.EqualFold(name, "epic"):
		return "Epic"
	case strings.EqualFold(name, "subtask"):
		return "Subtask"
	case strings.EqualFold(name, "sub-task"):
		return "Sub-task"
	default:
		return name
	}
}

func (j *jira) resolveEpicLinkFieldID() (string, error) {
	const cacheKind = "epic-link-field-id"

	if j.metadataCache != nil {
		var fieldID string
		hit, err := j.metadataCache.Get(j.metadataCacheKey(cacheKind, ""), &fieldID)
		if err == nil && hit && strings.TrimSpace(fieldID) != "" {
			return strings.TrimSpace(fieldID), nil
		}
	}

	fields, _, err := j.client.Field.GetList()
	if err != nil {
		return "", fmt.Errorf("failed to fetch jira fields: %w", err)
	}

	for _, field := range fields {
		fieldID := strings.TrimSpace(field.ID)
		if fieldID == "" {
			continue
		}

		customSchema := strings.TrimSpace(field.Schema.Custom)
		if strings.EqualFold(customSchema, jiraEpicLinkSchemaCustom) {
			if j.metadataCache != nil {
				_ = j.metadataCache.SetWithTTL(j.metadataCacheKey(cacheKind, ""), fieldID, 0)
			}
			return fieldID, nil
		}

		name := strings.ToLower(strings.TrimSpace(field.Name))
		if strings.Contains(name, "epic link") {
			if j.metadataCache != nil {
				_ = j.metadataCache.SetWithTTL(j.metadataCacheKey(cacheKind, ""), fieldID, 0)
			}
			return fieldID, nil
		}
	}

	return "", errEpicLinkFieldNotFound
}

// SearchIssues searches issues by project, optional text query and issue types.
func (j *jira) SearchIssues(projectKey string, query string, issueTypes []tracker.IssueType, limit int) ([]tracker.Issue, error) {
	if limit <= 0 {
		limit = 50
	}

	clauses := make([]string, 0, 3)

	normalizedProject := strings.ToUpper(strings.TrimSpace(projectKey))
	if normalizedProject != "" {
		clauses = append(clauses, fmt.Sprintf(`project = "%s"`, escapeJQLString(normalizedProject)))
	}

	typeFilters := make([]string, 0, len(issueTypes))
	for _, issueType := range issueTypes {
		name := strings.TrimSpace(trackerIssueTypeToJira(issueType))
		if name == "" {
			continue
		}
		typeFilters = append(typeFilters, fmt.Sprintf(`"%s"`, escapeJQLString(name)))
	}
	if len(typeFilters) > 0 {
		clauses = append(clauses, fmt.Sprintf("issuetype in (%s)", strings.Join(typeFilters, ", ")))
	}

	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery != "" {
		escapedQuery := escapeJQLString(trimmedQuery)
		normalizedKey := strings.ToUpper(escapedQuery)
		clauses = append(clauses, fmt.Sprintf(`(key = "%s" OR summary ~ "\"%s\"")`, normalizedKey, escapedQuery))
	}

	jql := "ORDER BY updated DESC"
	if len(clauses) > 0 {
		jql = fmt.Sprintf("%s ORDER BY updated DESC", strings.Join(clauses, " AND "))
	}

	searchOptions := &jiraClient.SearchOptionsV2{
		Fields:     []string{"description", "key", "project", "status", "summary", "type"},
		MaxResults: limit,
	}

	issues, _, err := j.client.Issue.SearchV2JQL(jql, searchOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	result := make([]tracker.Issue, 0, len(issues))
	for _, issue := range issues {
		jiraIssue := newIssue(&issue)
		result = append(result, *j.toTrackerIssue(jiraIssue))
	}

	return result, nil
}

func escapeJQLString(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(escaped, `"`, `\"`)
}

// GetProjectIssueTypes returns available issue types for a project.
// This implements the tracker.Tracker interface.
func (j *jira) GetProjectIssueTypes(projectKey string) ([]tracker.ProjectIssueType, error) {
	normalizedProjectKey := strings.ToUpper(strings.TrimSpace(projectKey))
	cacheKey := j.metadataCacheKey("issue-types", normalizedProjectKey)
	var cacheErr error
	if j.cacheInitErr != nil {
		cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to initialize Jira metadata cache: %w", j.cacheInitErr))
	}

	if j.metadataCache != nil {
		var cachedIssueTypes []tracker.ProjectIssueType
		hit, err := j.metadataCache.Get(cacheKey, &cachedIssueTypes)
		if err != nil {
			cacheErr = errors.Join(cacheErr, fmt.Errorf("failed to read Jira issue-type cache: %w", err))
		} else if hit {
			return cachedIssueTypes, nil
		}
	}

	issueTypes, err := j.fetchProjectIssueTypes(normalizedProjectKey)
	if err != nil {
		if cacheErr != nil {
			return nil, errors.Join(cacheErr, err)
		}
		return nil, err
	}

	if j.metadataCache != nil {
		ttl := j.metadataTTL("cache.issueTypesTTL", defaultIssueTypesCacheTTL)
		// Cache storage issues should not fail issue type retrieval.
		_ = j.metadataCache.SetWithTTL(cacheKey, issueTypes, ttl)
	}

	return issueTypes, nil
}

func (j *jira) fetchProjectIssueTypes(projectKey string) ([]tracker.ProjectIssueType, error) {
	meta, _, err := j.client.Issue.GetCreateMeta(strings.ToUpper(projectKey))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issue types for project %s: %w", projectKey, err)
	}

	project := meta.GetProjectWithKey(projectKey)
	if project == nil {
		return nil, fmt.Errorf("project %s was not found in Jira create metadata", projectKey)
	}

	issueTypes := make([]tracker.ProjectIssueType, 0, len(project.IssueTypes))
	for _, issueType := range project.IssueTypes {
		if issueType == nil || strings.TrimSpace(issueType.Name) == "" {
			continue
		}
		issueTypes = append(issueTypes, tracker.ProjectIssueType{
			ID:          issueType.Id,
			Name:        issueType.Name,
			Description: issueType.Description,
			Subtask:     issueType.Subtasks,
		})
	}

	if len(issueTypes) == 0 {
		return nil, fmt.Errorf("no issue types available for project %s", projectKey)
	}

	return issueTypes, nil
}

func (j *jira) metadataCacheKey(kind string, projectKey string) string {
	rootURL, err := j.resolveJiraRootURL()
	if err != nil || strings.TrimSpace(rootURL) == "" {
		rootURL = "unknown-site"
	}

	return fmt.Sprintf(
		"jira:%s:%s:%s",
		kind,
		url.QueryEscape(strings.ToLower(strings.TrimSpace(rootURL))),
		strings.ToUpper(strings.TrimSpace(projectKey)),
	)
}

func (j *jira) resolveOAuthSiteID(rootURL string, httpClient *http.Client) (string, error) {
	normalizedRoot, err := normalizeJiraRootURL(rootURL, true)
	if err != nil {
		return "", err
	}

	if siteID, ok := j.cachedOAuthSiteID(normalizedRoot); ok {
		return siteID, nil
	}

	legacySiteID := strings.TrimSpace(j.GetConfigString("siteId"))
	if legacySiteID != "" {
		legacySiteRoot := strings.TrimSpace(j.GetConfigString("siteRoot"))
		if legacySiteRoot == "" {
			j.cacheOAuthSiteID(normalizedRoot, legacySiteID)
			return legacySiteID, nil
		}

		normalizedLegacyRoot, legacyErr := normalizeJiraRootURL(legacySiteRoot, true)
		if legacyErr == nil && normalizedLegacyRoot == normalizedRoot {
			j.cacheOAuthSiteID(normalizedRoot, legacySiteID)
			return legacySiteID, nil
		}
	}

	siteID, err := getSiteIdByRootURL(normalizedRoot, httpClient)
	if err != nil {
		return "", err
	}

	j.cacheOAuthSiteID(normalizedRoot, siteID)
	return siteID, nil
}

func (j *jira) cachedOAuthSiteID(normalizedRoot string) (string, bool) {
	if j.metadataCache == nil || strings.TrimSpace(normalizedRoot) == "" {
		return "", false
	}

	var siteID string
	hit, err := j.metadataCache.Get(j.oauthSiteIDCacheKey(normalizedRoot), &siteID)
	if err != nil || !hit {
		return "", false
	}

	trimmed := strings.TrimSpace(siteID)
	if trimmed == "" {
		return "", false
	}

	return trimmed, true
}

func (j *jira) cacheOAuthSiteID(normalizedRoot string, siteID string) {
	if j.metadataCache == nil || strings.TrimSpace(normalizedRoot) == "" || strings.TrimSpace(siteID) == "" {
		return
	}

	// Persist derived OAuth site IDs without expiration.
	_ = j.metadataCache.SetWithTTL(j.oauthSiteIDCacheKey(normalizedRoot), strings.TrimSpace(siteID), 0)
}

func (j *jira) oauthSiteIDCacheKey(normalizedRoot string) string {
	return fmt.Sprintf(
		"jira:site-id:%s",
		url.QueryEscape(strings.ToLower(strings.TrimSpace(normalizedRoot))),
	)
}

func (j *jira) metadataTTL(configKey string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(j.GetConfigString(configKey))
	if raw == "" {
		return fallback
	}

	ttl, err := time.ParseDuration(raw)
	if err != nil || ttl <= 0 {
		return fallback
	}

	return ttl
}

// AddComment adds a comment to an issue.
// This implements the tracker.Tracker interface.
func (j *jira) AddComment(issueID, comment string) error {
	jiraComment := &jiraClient.Comment{
		Body: comment,
	}

	_, _, err := j.client.Issue.AddComment(issueID, jiraComment)
	if err != nil {
		return fmt.Errorf("failed to add comment to issue %s: %w", issueID, err)
	}

	return nil
}

// CreateRelease creates a new release (version) in Jira for the given project.
// This implements the tracker.Tracker interface.
func (j *jira) CreateRelease(data tracker.CreateReleaseInput) (*tracker.Release, error) {
	if len(data.IssueIDs) == 0 {
		return nil, fmt.Errorf("at least one issue ID is required to create a release")
	}

	// Get the project key from the first issue
	firstIssue, err := j.GetJiraIssue(data.IssueIDs[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get issue %s: %w", data.IssueIDs[0], err)
	}

	// Extract project key from issue key (e.g., "PROJ-123" -> "PROJ")
	projectKey := strings.Split(firstIssue.Key, "-")[0]

	// Get project to retrieve its ID
	project, _, err := j.client.Project.Get(projectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectKey, err)
	}

	released := true
	// Create the version
	version := &jiraClient.Version{
		Name:        data.Name,
		Description: data.Description,
		ProjectID:   parseInt(project.ID),
		Released:    &released,
	}

	createdVersion, _, err := j.client.Version.Create(version)
	if err != nil {
		return nil, fmt.Errorf("failed to create version %s: %w", data.Name, err)
	}

	return &tracker.Release{
		ID:          createdVersion.ID,
		Name:        createdVersion.Name,
		Description: createdVersion.Description,
		URL:         j.getReleaseURL(projectKey, createdVersion.ID),
	}, nil
}

// getReleaseURL returns the URL to the release page in Jira
func (j *jira) getReleaseURL(projectKey, versionID string) string {
	rootURL, err := j.resolveJiraRootURL()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s/projects/%s/versions/%s", rootURL, projectKey, versionID)
}

// parseInt parses a string to int, returning 0 on error
func parseInt(s string) int {
	var result int
	_, _ = fmt.Sscanf(s, "%d", &result)
	return result
}

// SetFixVersion associates issues with a release by setting the fix version.
// This implements the tracker.Tracker interface.
func (j *jira) SetFixVersion(issueIDs []string, release *tracker.Release) error {
	if release == nil {
		return fmt.Errorf("release cannot be nil")
	}

	for _, issueID := range issueIDs {
		// Use the UpdateIssue method to set the fixVersions field
		updateData := map[string]interface{}{
			"update": map[string]interface{}{
				"fixVersions": []map[string]interface{}{
					{
						"add": map[string]string{
							"name": release.Name,
						},
					},
				},
			},
		}

		_, err := j.client.Issue.UpdateIssue(issueID, updateData)
		if err != nil {
			return fmt.Errorf("failed to set fix version on issue %s: %w", issueID, err)
		}
	}

	return nil
}

// IsValidIssueID checks if the given string is a valid Jira issue ID format.
// This implements the tracker.Tracker interface.
// Valid Jira issue IDs follow the pattern: PROJECT-123 (uppercase letters, hyphen, digits).
func (j *jira) IsValidIssueID(id string) bool {
	return issueIDPattern.MatchString(strings.ToUpper(id))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// getSiteId makes a request to jira to get all the sites the
// authenticated user has access to. In order to use the Jira
// API with an oauth token, we have to make calls using the id of the site
// instead of their specific URL
func getSiteIdByRootURL(rootURL string, httpClient *http.Client) (string, error) {
	cloudEndpoint := "https://api.atlassian.com/oauth/token/accessible-resources"
	client := wrapJiraHTTPClient(httpClient)
	response, err := client.Get(cloudEndpoint)
	if err != nil {
		return "", err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("couldn't get sites: %s", response.Status)
	}
	var sites []jiraSite
	if err = json.NewDecoder(response.Body).Decode(&sites); err != nil {
		return "", err
	}

	normalizedRoot, err := normalizeJiraRootURL(rootURL, false)
	if err != nil {
		return "", err
	}

	for _, jiraSite := range sites {
		normalizedSiteURL, err := normalizeJiraRootURL(jiraSite.Url, false)
		if err != nil {
			continue
		}
		if normalizedSiteURL == normalizedRoot {
			return jiraSite.Id, nil
		}
	}
	return "", fmt.Errorf("couldn't find site for Jira URL: %s", rootURL)
}

// NewWithHTTPClient returns a new Jira client using the provided HTTP client and base URL.
// This bypasses OAuth authentication and is intended for testing with mock servers.
func NewWithHTTPClient(cfg *viper.Viper, httpClient *http.Client, baseURL string) (Jira, error) {
	configurableService := &config.ViperConfigurableService{Config: cfg, Prefix: "jira"}
	metadataCache, cacheErr := newIssueTypesCacheStore(cfg)

	client, err := jiraClient.NewClient(wrapJiraHTTPClient(httpClient), baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jira client: %w", err)
	}

	j := &jira{
		ViperConfigurableService: configurableService,
		client:                   client,
		allowPrompt:              false,
		metadataCache:            metadataCache,
		cacheInitErr:             cacheErr,
	}

	rootURL := configurableService.GetConfigString("root")
	if rootURL == "" {
		rootURL = baseURL
	}
	normalized, err := normalizeJiraRootURL(rootURL, true)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize Jira root URL: %w", err)
	}
	j.jiraRootURL = normalized

	return j, nil
}

// New returns a new instance of an authenticated Jira client
func New(cfg *viper.Viper) (Jira, error) {
	return NewWithOptions(cfg, true)
}

// NewWithOptions returns an authenticated Jira client with prompt behavior controls.
func NewWithOptions(cfg *viper.Viper, allowPrompt bool) (Jira, error) {
	configurableService := &config.ViperConfigurableService{Config: cfg, Prefix: "jira"}
	metadataCache, cacheErr := newIssueTypesCacheStore(cfg)

	j := &jira{
		ViperConfigurableService: configurableService,
		allowPrompt:              allowPrompt,
		promptRoot:               promptForJiraRootURL,
		promptAuthMethod:         promptJiraAuthMethod,
		promptAPICreds:           promptJiraAPICredentials,
		metadataCache:            metadataCache,
		cacheInitErr:             cacheErr,
		authenticator: createAuthenticator(func() string {
			localConfig := configurableService.GetConfig()
			if localConfig == nil {
				return ""
			}
			return localConfig.GetString("token")
		}, func(token string) error {
			return configurableService.SaveConfig("token", token)
		}, allowPrompt),
		client: nil,
	}
	_, err := j.Authenticate()
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (j *jira) resolveJiraRootURL() (string, error) {
	if j.jiraRootURL != "" {
		return j.jiraRootURL, nil
	}

	configRoot := j.GetConfigString("root")
	if configRoot != "" {
		normalized, err := normalizeJiraRootURL(configRoot, false)
		if err != nil {
			return "", err
		}
		j.jiraRootURL = normalized
		return j.jiraRootURL, nil
	}

	if !j.allowPrompt {
		return "", errMissingJiraRoot
	}
	if !isInputTerminalFn() {
		return "", errMissingJiraRoot
	}

	promptFn := j.promptRoot
	if promptFn == nil {
		promptFn = promptForJiraRootURL
	}

	prompted, err := promptFn()
	if err != nil {
		return "", err
	}

	normalized, err := normalizeJiraRootURL(prompted, false)
	if err != nil {
		return "", err
	}

	if err := j.SaveConfig("root", normalized); err != nil {
		return "", err
	}

	j.jiraRootURL = normalized
	return j.jiraRootURL, nil
}

var (
	errMissingJiraRoot         = errors.New("missing Jira site URL; set jira.root or FOTINGO_JIRA_ROOT")
	errEpicLinkFieldNotFound   = errors.New("epic link field not found")
	ErrAuthRequired            = errors.New("jira authentication required; run `fotingo login` interactively")
	ErrOAuthCredentialsMissing = errors.New("jira oauth client credentials are missing in this build; use `fotingo login` with API token auth or rebuild with oauth ldflags")
)

func promptForJiraRootURL() (string, error) {
	input := ui.NewInputProgram(
		ui.WithPrompt("Jira site URL"),
		ui.WithPlaceholder("https://yourcompany.atlassian.net"),
		ui.WithValidation(func(value string) error {
			_, err := normalizeJiraRootURL(value, false)
			return err
		}),
	)

	value, cancelled, err := input.RunWithCancel()
	if err != nil {
		return "", err
	}
	if cancelled || strings.TrimSpace(value) == "" {
		return "", errMissingJiraRoot
	}

	return value, nil
}

func promptJiraAPICredentials() (string, string, error) {
	loginInput := ui.NewInputProgram(
		ui.WithPrompt("Jira account email"),
		ui.WithPlaceholder("Atlassian account email"),
		ui.WithValidation(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("jira user email is required")
			}
			return nil
		}),
	)

	login, cancelled, err := loginInput.RunWithCancel()
	if err != nil {
		return "", "", err
	}
	if cancelled || strings.TrimSpace(login) == "" {
		return "", "", ErrAuthRequired
	}

	tokenInput := ui.NewInputProgram(
		ui.WithPrompt("Jira API token (Atlassian)"),
		ui.WithPlaceholder("Create at https://id.atlassian.com/manage-profile/security/api-tokens"),
		ui.WithValidation(func(value string) error {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("jira api token is required")
			}
			return nil
		}),
	)

	token, cancelled, err := tokenInput.RunWithCancel()
	if err != nil {
		return "", "", err
	}
	if cancelled || strings.TrimSpace(token) == "" {
		return "", "", ErrAuthRequired
	}

	return strings.TrimSpace(login), strings.TrimSpace(token), nil
}

func promptJiraAuthMethod() (string, error) {
	useOAuth, err := ui.Confirm("Authenticate Jira with OAuth? (No to use API token)", true)
	if err != nil {
		return "", err
	}
	if useOAuth {
		return jiraAuthMethodOAuth, nil
	}
	return jiraAuthMethodToken, nil
}

func normalizeJiraRootURL(raw string, allowHTTP bool) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errMissingJiraRoot
	}

	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid Jira site URL: %w", err)
	}

	if parsed.Host == "" {
		return "", fmt.Errorf("invalid Jira site URL: host is required")
	}

	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}

	if parsed.Scheme != "https" && (!allowHTTP || parsed.Scheme != "http") {
		return "", fmt.Errorf("invalid Jira site URL: scheme must be https")
	}

	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("invalid Jira site URL: path is not allowed")
	}

	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return strings.TrimRight(parsed.String(), "/"), nil
}

func isInputTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

var isInputTerminalFn = isInputTerminal
