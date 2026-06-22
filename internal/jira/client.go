package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/techinpark/jira-cli/internal/adf"
	"github.com/techinpark/jira-cli/internal/httpx"
)

type Client struct {
	httpClient *httpx.Client
}

type ListProjectsOptions struct {
	Query string
	Limit int
}

type SearchOptions struct {
	JQL    string
	Fields []string
	Limit  int
}

type CreateIssueInput struct {
	Project     string
	IssueType   string
	Summary     string
	Description string
	Assignee    string // resolved accountId
	Priority    string
	Parent      string
	Due         string
	Labels      []string
	Fields      map[string]any
}

type UpdateIssueInput struct {
	Summary     string
	Description *string
	Assignee    string // resolved accountId
	Priority    string
	Parent      string
	Due         string
	Labels      []string
	Fields      map[string]any
}

type MoveIssueInput struct {
	Transition string
	Comment    string
	Fields     map[string]any
}

type WorklogInput struct {
	TimeSpent string
	Started   string
	Comment   string
}

func NewClient(httpClient *httpx.Client) *Client {
	return &Client{httpClient: httpClient}
}

func (c *Client) CheckAuth(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.httpClient.Do(ctx, http.MethodGet, "/rest/api/3/myself", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListProjects(ctx context.Context, opts ListProjectsOptions) ([]Project, error) {
	query := url.Values{}
	if opts.Query != "" {
		query.Set("query", opts.Query)
	}
	if opts.Limit > 0 {
		query.Set("maxResults", strconv.Itoa(opts.Limit))
	}
	var out struct {
		Values []projectDocument `json:"values"`
	}
	if err := c.httpClient.Do(ctx, http.MethodGet, "/rest/api/3/project/search", query, nil, &out); err != nil {
		return nil, err
	}
	projects := make([]Project, 0, len(out.Values))
	for _, item := range out.Values {
		projects = append(projects, item.toProject())
	}
	return projects, nil
}

func (c *Client) GetProject(ctx context.Context, key string) (Project, error) {
	var out projectDocument
	err := c.httpClient.Do(ctx, http.MethodGet, "/rest/api/3/project/"+url.PathEscape(key), nil, nil, &out)
	return out.toProject(), err
}

func (c *Client) GetIssue(ctx context.Context, key string, fields []string) (Issue, error) {
	query := url.Values{}
	if len(fields) > 0 {
		query.Set("fields", strings.Join(fields, ","))
	}
	var out issueDocument
	err := c.httpClient.Do(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key), query, nil, &out)
	return out.toIssue(), err
}

func (c *Client) SearchIssues(ctx context.Context, opts SearchOptions) (SearchResult, error) {
	body := map[string]any{
		"jql":        opts.JQL,
		"maxResults": defaultLimit(opts.Limit, 50),
	}
	if len(opts.Fields) > 0 {
		body["fields"] = opts.Fields
	}
	var out struct {
		Issues        []issueDocument `json:"issues"`
		NextPageToken string          `json:"nextPageToken"`
		Names         map[string]any  `json:"names"`
		Schema        map[string]any  `json:"schema"`
	}
	if err := c.httpClient.Do(ctx, http.MethodPost, "/rest/api/3/search/jql", nil, body, &out); err != nil {
		return SearchResult{}, err
	}
	issues := make([]Issue, 0, len(out.Issues))
	for _, item := range out.Issues {
		issues = append(issues, item.toIssue())
	}
	return SearchResult{
		Issues:        issues,
		NextPageToken: out.NextPageToken,
		Names:         out.Names,
		Schema:        out.Schema,
	}, nil
}

func (c *Client) CreateIssue(ctx context.Context, input CreateIssueInput) (IssueRef, error) {
	fields := map[string]any{
		"project":   refByNameOrID(input.Project, "key"),
		"issuetype": refByNameOrID(input.IssueType, "name"),
		"summary":   input.Summary,
	}
	if input.Description != "" {
		fields["description"] = adf.PlainTextDoc(input.Description)
	}
	applyIssueFields(fields, input.Assignee, input.Priority, input.Parent, input.Due, input.Labels)
	for key, value := range input.Fields {
		fields[key] = value
	}
	var out IssueRef
	err := c.httpClient.Do(ctx, http.MethodPost, "/rest/api/3/issue", nil, map[string]any{"fields": fields}, &out)
	return out, err
}

func (c *Client) UpdateIssue(ctx context.Context, key string, input UpdateIssueInput) error {
	fields := map[string]any{}
	if input.Summary != "" {
		fields["summary"] = input.Summary
	}
	if input.Description != nil {
		fields["description"] = adf.PlainTextDoc(*input.Description)
	}
	applyIssueFields(fields, input.Assignee, input.Priority, input.Parent, input.Due, input.Labels)
	for name, value := range input.Fields {
		fields[name] = value
	}
	return c.httpClient.Do(ctx, http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(key), nil, map[string]any{"fields": fields}, nil)
}

func (c *Client) DeleteIssue(ctx context.Context, key string, deleteSubtasks bool) error {
	query := url.Values{}
	if deleteSubtasks {
		query.Set("deleteSubtasks", "true")
	}
	return c.httpClient.Do(ctx, http.MethodDelete, "/rest/api/3/issue/"+url.PathEscape(key), query, nil, nil)
}

// AddAttachments uploads one or more local files to an issue. Jira has no way to
// embed attachments when an issue is created, so callers create the issue first
// and then attach files to the returned key.
func (c *Client) AddAttachments(ctx context.Context, issueKey string, paths []string) ([]Attachment, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no attachment files provided")
	}
	if len(paths) > 60 {
		return nil, fmt.Errorf("too many attachments: %d files requested, Jira allows at most 60 per request", len(paths))
	}

	parts := make([]httpx.FilePart, 0, len(paths))
	files := make([]*os.File, 0, len(paths))
	defer func() {
		for _, f := range files {
			_ = f.Close()
		}
	}()

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("attachment %q: %w", path, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("attachment %q is a directory", path)
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("attachment %q: %w", path, err)
		}
		files = append(files, f)
		base := filepath.Base(path)
		parts = append(parts, httpx.FilePart{
			FieldName:   "file",
			FileName:    base,
			ContentType: mime.TypeByExtension(filepath.Ext(base)),
			Reader:      f,
		})
	}

	var out []attachmentDocument
	if err := c.httpClient.Upload(ctx, http.MethodPost, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/attachments", parts, &out); err != nil {
		return nil, err
	}

	attachments := make([]Attachment, 0, len(out))
	for _, item := range out {
		attachments = append(attachments, item.toAttachment())
	}
	return attachments, nil
}

// ListAttachments returns the attachments currently on an issue.
func (c *Client) ListAttachments(ctx context.Context, issueKey string) ([]Attachment, error) {
	query := url.Values{}
	query.Set("fields", "attachment")
	var out struct {
		Fields struct {
			Attachment []attachmentDocument `json:"attachment"`
		} `json:"fields"`
	}
	if err := c.httpClient.Do(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(issueKey), query, nil, &out); err != nil {
		return nil, err
	}
	attachments := make([]Attachment, 0, len(out.Fields.Attachment))
	for _, item := range out.Fields.Attachment {
		attachments = append(attachments, item.toAttachment())
	}
	return attachments, nil
}

// AttachmentMeta returns metadata for a single attachment by ID.
func (c *Client) AttachmentMeta(ctx context.Context, attachmentID string) (Attachment, error) {
	var out attachmentDocument
	err := c.httpClient.Do(ctx, http.MethodGet, "/rest/api/3/attachment/"+url.PathEscape(attachmentID), nil, nil, &out)
	return out.toAttachment(), err
}

// DownloadAttachment streams an attachment's binary content to w and returns the
// attachment's filename, looked up from its metadata.
func (c *Client) DownloadAttachment(ctx context.Context, attachmentID string, w io.Writer) (string, error) {
	meta, err := c.AttachmentMeta(ctx, attachmentID)
	if err != nil {
		return "", err
	}
	if err := c.httpClient.Download(ctx, "/rest/api/3/attachment/content/"+url.PathEscape(attachmentID), w); err != nil {
		return "", err
	}
	return meta.Filename, nil
}

// DeleteAttachment removes an attachment by ID.
func (c *Client) DeleteAttachment(ctx context.Context, attachmentID string) error {
	return c.httpClient.Do(ctx, http.MethodDelete, "/rest/api/3/attachment/"+url.PathEscape(attachmentID), nil, nil, nil)
}

// Myself returns the authenticated user.
func (c *Client) Myself(ctx context.Context) (User, error) {
	var out userDocument
	err := c.httpClient.Do(ctx, http.MethodGet, "/rest/api/3/myself", nil, nil, &out)
	return out.toUser(), err
}

// SearchUsers finds users matching a query (display name or email).
func (c *Client) SearchUsers(ctx context.Context, query string, limit int) ([]User, error) {
	q := url.Values{}
	q.Set("query", query)
	if limit > 0 {
		q.Set("maxResults", strconv.Itoa(limit))
	}
	var out []userDocument
	if err := c.httpClient.Do(ctx, http.MethodGet, "/rest/api/3/user/search", q, nil, &out); err != nil {
		return nil, err
	}
	users := make([]User, 0, len(out))
	for _, item := range out {
		users = append(users, item.toUser())
	}
	return users, nil
}

// ResolveUserAccountID turns a user reference into a Jira Cloud accountId.
// "me" (or "@me") resolves to the authenticated user, a value containing "@" is
// treated as an email and looked up via user search, and anything else is
// assumed to already be an accountId.
func (c *Client) ResolveUserAccountID(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	switch {
	case ref == "":
		return "", fmt.Errorf("empty user reference")
	case ref == "me" || ref == "@me":
		me, err := c.Myself(ctx)
		if err != nil {
			return "", err
		}
		return me.AccountID, nil
	case strings.Contains(ref, "@"):
		users, err := c.SearchUsers(ctx, ref, 2)
		if err != nil {
			return "", err
		}
		for _, u := range users {
			if strings.EqualFold(u.Email, ref) {
				return u.AccountID, nil
			}
		}
		switch len(users) {
		case 0:
			return "", fmt.Errorf("no user found for %q", ref)
		case 1:
			return users[0].AccountID, nil
		default:
			return "", fmt.Errorf("multiple users match %q; specify an accountId", ref)
		}
	default:
		return ref, nil
	}
}

// AssignIssue sets the assignee of an issue. An empty accountID unassigns the
// issue (sets the field to null).
func (c *Client) AssignIssue(ctx context.Context, issueKey, accountID string) error {
	body := map[string]any{"accountId": nil}
	if accountID != "" {
		body["accountId"] = accountID
	}
	return c.httpClient.Do(ctx, http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/assignee", nil, body, nil)
}

func (c *Client) ListComments(ctx context.Context, issueKey string) ([]Comment, error) {
	var out pageOfComments
	err := c.httpClient.Do(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/comment", nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return out.items(), nil
}

func (c *Client) AddComment(ctx context.Context, issueKey, body string) (Comment, error) {
	var out commentDocument
	err := c.httpClient.Do(ctx, http.MethodPost, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/comment", nil, map[string]any{
		"body": adf.PlainTextDoc(body),
	}, &out)
	return out.toComment(), err
}

func (c *Client) UpdateComment(ctx context.Context, issueKey, commentID, body string) (Comment, error) {
	var out commentDocument
	err := c.httpClient.Do(ctx, http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/comment/"+url.PathEscape(commentID), nil, map[string]any{
		"body": adf.PlainTextDoc(body),
	}, &out)
	return out.toComment(), err
}

func (c *Client) DeleteComment(ctx context.Context, issueKey, commentID string) error {
	return c.httpClient.Do(ctx, http.MethodDelete, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/comment/"+url.PathEscape(commentID), nil, nil, nil)
}

func (c *Client) ListTransitions(ctx context.Context, issueKey string) ([]Transition, error) {
	var out struct {
		Transitions []transitionDocument `json:"transitions"`
	}
	err := c.httpClient.Do(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/transitions", nil, nil, &out)
	if err != nil {
		return nil, err
	}
	transitions := make([]Transition, 0, len(out.Transitions))
	for _, item := range out.Transitions {
		transitions = append(transitions, item.toTransition())
	}
	return transitions, nil
}

func (c *Client) MoveIssue(ctx context.Context, issueKey string, input MoveIssueInput) error {
	transitionID := input.Transition
	if !isDigits(transitionID) {
		transitions, err := c.ListTransitions(ctx, issueKey)
		if err != nil {
			return err
		}
		found := false
		for _, item := range transitions {
			if strings.EqualFold(item.Name, transitionID) {
				transitionID = item.ID
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("transition %q not found", input.Transition)
		}
	}

	body := map[string]any{
		"transition": map[string]any{"id": transitionID},
	}
	if len(input.Fields) > 0 {
		body["fields"] = input.Fields
	}
	if input.Comment != "" {
		body["update"] = map[string]any{
			"comment": []map[string]any{
				{
					"add": map[string]any{
						"body": adf.PlainTextDoc(input.Comment),
					},
				},
			},
		}
	}
	return c.httpClient.Do(ctx, http.MethodPost, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/transitions", nil, body, nil)
}

func (c *Client) ListWorklogs(ctx context.Context, issueKey string) ([]Worklog, error) {
	var out struct {
		Worklogs []worklogDocument `json:"worklogs"`
	}
	err := c.httpClient.Do(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/worklog", nil, nil, &out)
	if err != nil {
		return nil, err
	}
	worklogs := make([]Worklog, 0, len(out.Worklogs))
	for _, item := range out.Worklogs {
		worklogs = append(worklogs, item.toWorklog())
	}
	return worklogs, nil
}

func (c *Client) AddWorklog(ctx context.Context, issueKey string, input WorklogInput) (Worklog, error) {
	var out worklogDocument
	err := c.httpClient.Do(ctx, http.MethodPost, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/worklog", nil, worklogPayload(input), &out)
	return out.toWorklog(), err
}

func (c *Client) UpdateWorklog(ctx context.Context, issueKey, worklogID string, input WorklogInput) (Worklog, error) {
	var out worklogDocument
	err := c.httpClient.Do(ctx, http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/worklog/"+url.PathEscape(worklogID), nil, worklogPayload(input), &out)
	return out.toWorklog(), err
}

func (c *Client) DeleteWorklog(ctx context.Context, issueKey, worklogID string) error {
	return c.httpClient.Do(ctx, http.MethodDelete, "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/worklog/"+url.PathEscape(worklogID), nil, nil, nil)
}

func (c *Client) Raw(ctx context.Context, method, path string, query url.Values, body any) (any, error) {
	var out any
	err := c.httpClient.Do(ctx, method, path, query, body, &out)
	return out, err
}

type projectDocument struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	ProjectType string `json:"projectTypeKey"`
	Archived    bool   `json:"archived"`
	Lead        struct {
		DisplayName string `json:"displayName"`
	} `json:"lead"`
}

func (d projectDocument) toProject() Project {
	return Project{
		ID:       d.ID,
		Key:      d.Key,
		Name:     d.Name,
		Type:     d.ProjectType,
		Lead:     d.Lead.DisplayName,
		Archived: d.Archived,
	}
}

type issueDocument struct {
	ID     string         `json:"id"`
	Key    string         `json:"key"`
	Fields map[string]any `json:"fields"`
}

func (d issueDocument) toIssue() Issue {
	fields := d.Fields
	if fields == nil {
		fields = map[string]any{}
	}
	return Issue{
		ID:         d.ID,
		Key:        d.Key,
		Summary:    stringField(fields, "summary"),
		Status:     nestedStringField(fields, "status", "name"),
		IssueType:  nestedStringField(fields, "issuetype", "name"),
		ProjectKey: nestedStringField(fields, "project", "key"),
		Assignee:   nestedStringField(fields, "assignee", "displayName"),
		Reporter:   nestedStringField(fields, "reporter", "displayName"),
		Fields:     fields,
	}
}

type pageOfComments struct {
	Comments []commentDocument `json:"comments"`
	Values   []commentDocument `json:"values"`
}

func (p pageOfComments) items() []Comment {
	values := p.Comments
	if len(values) == 0 {
		values = p.Values
	}
	comments := make([]Comment, 0, len(values))
	for _, item := range values {
		comments = append(comments, item.toComment())
	}
	return comments
}

type commentDocument struct {
	ID      string `json:"id"`
	Created string `json:"created"`
	Updated string `json:"updated"`
	Author  struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Body any `json:"body"`
}

func (d commentDocument) toComment() Comment {
	return Comment{
		ID:      d.ID,
		Author:  d.Author.DisplayName,
		Body:    strings.TrimSpace(adf.ExtractPlainText(d.Body)),
		Created: d.Created,
		Updated: d.Updated,
	}
}

type transitionDocument struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   struct {
		Name string `json:"name"`
	} `json:"to"`
}

func (d transitionDocument) toTransition() Transition {
	return Transition{ID: d.ID, Name: d.Name, ToStatus: d.To.Name}
}

type worklogDocument struct {
	ID        string `json:"id"`
	Started   string `json:"started"`
	TimeSpent string `json:"timeSpent"`
	Author    struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Comment any `json:"comment"`
}

func (d worklogDocument) toWorklog() Worklog {
	return Worklog{
		ID:        d.ID,
		Author:    d.Author.DisplayName,
		Started:   d.Started,
		TimeSpent: d.TimeSpent,
		Comment:   strings.TrimSpace(adf.ExtractPlainText(d.Comment)),
	}
}

// applyIssueFields maps the convenience inputs shared by create and update onto
// the Jira fields payload. Empty values are skipped so they never clear a field.
func applyIssueFields(fields map[string]any, assignee, priority, parent, due string, labels []string) {
	if assignee != "" {
		fields["assignee"] = map[string]any{"accountId": assignee}
	}
	if priority != "" {
		fields["priority"] = refByNameOrID(priority, "name")
	}
	if parent != "" {
		fields["parent"] = refByNameOrID(parent, "key")
	}
	if due != "" {
		fields["duedate"] = due
	}
	if len(labels) > 0 {
		fields["labels"] = labels
	}
}

type userDocument struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
	Active      bool   `json:"active"`
	AccountType string `json:"accountType"`
}

func (d userDocument) toUser() User {
	return User{
		AccountID:   d.AccountID,
		DisplayName: d.DisplayName,
		Email:       d.Email,
		Active:      d.Active,
		AccountType: d.AccountType,
	}
}

type attachmentDocument struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Author   struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Created  string `json:"created"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Content  string `json:"content"`
}

func (d attachmentDocument) toAttachment() Attachment {
	return Attachment{
		ID:       d.ID,
		Filename: d.Filename,
		Author:   d.Author.DisplayName,
		Created:  d.Created,
		Size:     d.Size,
		MimeType: d.MimeType,
		Content:  d.Content,
	}
}

func worklogPayload(input WorklogInput) map[string]any {
	body := map[string]any{}
	if input.TimeSpent != "" {
		body["timeSpent"] = input.TimeSpent
	}
	if input.Started != "" {
		body["started"] = input.Started
	}
	if input.Comment != "" {
		body["comment"] = adf.PlainTextDoc(input.Comment)
	}
	return body
}

func refByNameOrID(value, nameKey string) map[string]any {
	if isDigits(value) {
		return map[string]any{"id": value}
	}
	return map[string]any{nameKey: value}
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func defaultLimit(limit, fallback int) int {
	if limit > 0 {
		return limit
	}
	return fallback
}

func stringField(fields map[string]any, key string) string {
	if value, ok := fields[key].(string); ok {
		return value
	}
	return ""
}

func nestedStringField(fields map[string]any, key, nested string) string {
	value, ok := fields[key].(map[string]any)
	if !ok {
		return ""
	}
	if text, ok := value[nested].(string); ok {
		return text
	}
	return ""
}

func ParseFieldValue(raw string) any {
	trimmed := strings.TrimSpace(raw)
	var out any
	if err := json.Unmarshal([]byte(trimmed), &out); err == nil {
		return out
	}
	return raw
}
