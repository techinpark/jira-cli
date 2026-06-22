package jira

type Project struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Lead     string `json:"lead,omitempty"`
	Archived bool   `json:"archived,omitempty"`
}

type Issue struct {
	ID         string         `json:"id"`
	Key        string         `json:"key"`
	Summary    string         `json:"summary,omitempty"`
	Status     string         `json:"status,omitempty"`
	IssueType  string         `json:"issue_type,omitempty"`
	ProjectKey string         `json:"project_key,omitempty"`
	Assignee   string         `json:"assignee,omitempty"`
	Reporter   string         `json:"reporter,omitempty"`
	Fields     map[string]any `json:"fields,omitempty"`
}

type IssueRef struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self,omitempty"`
}

type SearchResult struct {
	Issues        []Issue        `json:"issues"`
	NextPageToken string         `json:"next_page_token,omitempty"`
	Names         map[string]any `json:"names,omitempty"`
	Schema        map[string]any `json:"schema,omitempty"`
}

type Comment struct {
	ID      string `json:"id"`
	Author  string `json:"author,omitempty"`
	Body    string `json:"body,omitempty"`
	Created string `json:"created,omitempty"`
	Updated string `json:"updated,omitempty"`
}

type Transition struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ToStatus string `json:"to_status,omitempty"`
}

type Worklog struct {
	ID        string `json:"id"`
	Author    string `json:"author,omitempty"`
	Started   string `json:"started,omitempty"`
	TimeSpent string `json:"time_spent,omitempty"`
	Comment   string `json:"comment,omitempty"`
}

type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Author   string `json:"author,omitempty"`
	Created  string `json:"created,omitempty"`
	Size     int64  `json:"size,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Content  string `json:"content,omitempty"`
}
