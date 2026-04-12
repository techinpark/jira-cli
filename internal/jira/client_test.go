package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/techinpark/jira-cli/internal/config"
	"github.com/techinpark/jira-cli/internal/httpx"
)

func TestProjectIssueAndSearchFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/project/search":
			_, _ = w.Write([]byte(`{"values":[{"id":"100","key":"ENG","name":"Engineering","projectTypeKey":"software","lead":{"displayName":"Tech Lead"}}]}`))
		case r.URL.Path == "/rest/api/3/project/ENG":
			_, _ = w.Write([]byte(`{"id":"100","key":"ENG","name":"Engineering","projectTypeKey":"software","lead":{"displayName":"Tech Lead"}}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"1","key":"ENG-1","fields":{"summary":"Fix bug","status":{"name":"In Progress"},"issuetype":{"name":"Bug"},"project":{"key":"ENG"},"assignee":{"displayName":"Alice"}}}`))
		case r.URL.Path == "/rest/api/3/search/jql":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["jql"] != "project = ENG" {
				t.Fatalf("unexpected jql: %+v", body)
			}
			_, _ = w.Write([]byte(`{"issues":[{"id":"1","key":"ENG-1","fields":{"summary":"Fix bug","status":{"name":"In Progress"},"issuetype":{"name":"Bug"},"assignee":{"displayName":"Alice"}}}]}`))
		case r.URL.Path == "/rest/api/3/issue" && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			fields := body["fields"].(map[string]any)
			if fields["summary"] != "New issue" {
				t.Fatalf("unexpected create body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"id":"2","key":"ENG-2","self":"https://jira/rest/api/3/issue/2"}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/rest/api/3/issue/ENG-1" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	projects, err := client.ListProjects(context.Background(), ListProjectsOptions{Query: "Eng", Limit: 10})
	if err != nil || len(projects) != 1 || projects[0].Key != "ENG" {
		t.Fatalf("unexpected projects: %+v err=%v", projects, err)
	}
	project, err := client.GetProject(context.Background(), "ENG")
	if err != nil || project.Name != "Engineering" {
		t.Fatalf("unexpected project: %+v err=%v", project, err)
	}
	issue, err := client.GetIssue(context.Background(), "ENG-1", []string{"summary"})
	if err != nil || issue.Status != "In Progress" {
		t.Fatalf("unexpected issue: %+v err=%v", issue, err)
	}
	result, err := client.SearchIssues(context.Background(), SearchOptions{JQL: "project = ENG", Limit: 5})
	if err != nil || len(result.Issues) != 1 {
		t.Fatalf("unexpected search result: %+v err=%v", result, err)
	}
	created, err := client.CreateIssue(context.Background(), CreateIssueInput{
		Project: "ENG", IssueType: "Bug", Summary: "New issue", Description: "hello",
	})
	if err != nil || created.Key != "ENG-2" {
		t.Fatalf("unexpected created issue: %+v err=%v", created, err)
	}
	if err := client.UpdateIssue(context.Background(), "ENG-1", UpdateIssueInput{Summary: "Renamed"}); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteIssue(context.Background(), "ENG-1", true); err != nil {
		t.Fatal(err)
	}
}

func TestCommentTransitionWorklogAndRaw(t *testing.T) {
	var moved bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/myself":
			_, _ = w.Write([]byte(`{"accountId":"abc","displayName":"User"}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/comment" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"comments":[{"id":"10","created":"2026-04-13","author":{"displayName":"Alice"},"body":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}}]}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/comment" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"id":"11","created":"2026-04-13","author":{"displayName":"Bob"},"body":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"new comment"}]}]}}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/comment/11" && r.Method == http.MethodPut:
			_, _ = w.Write([]byte(`{"id":"11","updated":"2026-04-13","author":{"displayName":"Bob"},"body":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"edited"}]}]}}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/comment/11" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/rest/api/3/issue/ENG-1/transitions" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"transitions":[{"id":"31","name":"Done","to":{"name":"Done"}}]}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/transitions" && r.Method == http.MethodPost:
			moved = true
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/rest/api/3/issue/ENG-1/worklog" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"worklogs":[{"id":"20","started":"2026-04-13T09:00:00.000+0900","timeSpent":"1h","author":{"displayName":"Alice"},"comment":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"logged"}]}]}}]}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/worklog" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"id":"21","started":"2026-04-13T09:00:00.000+0900","timeSpent":"2h","author":{"displayName":"Alice"},"comment":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"created"}]}]}}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/worklog/21" && r.Method == http.MethodPut:
			_, _ = w.Write([]byte(`{"id":"21","started":"2026-04-13T09:00:00.000+0900","timeSpent":"3h","author":{"displayName":"Alice"},"comment":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"updated"}]}]}}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/worklog/21" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/rest/api/3/custom":
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	self, err := client.CheckAuth(context.Background())
	if err != nil || self["displayName"] != "User" {
		t.Fatalf("unexpected self: %+v err=%v", self, err)
	}
	comments, err := client.ListComments(context.Background(), "ENG-1")
	if err != nil || comments[0].Body != "hello" {
		t.Fatalf("unexpected comments: %+v err=%v", comments, err)
	}
	comment, err := client.AddComment(context.Background(), "ENG-1", "new comment")
	if err != nil || comment.ID != "11" {
		t.Fatalf("unexpected add comment: %+v err=%v", comment, err)
	}
	comment, err = client.UpdateComment(context.Background(), "ENG-1", "11", "edited")
	if err != nil || comment.Body != "edited" {
		t.Fatalf("unexpected update comment: %+v err=%v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), "ENG-1", "11"); err != nil {
		t.Fatal(err)
	}
	transitions, err := client.ListTransitions(context.Background(), "ENG-1")
	if err != nil || transitions[0].Name != "Done" {
		t.Fatalf("unexpected transitions: %+v err=%v", transitions, err)
	}
	if err := client.MoveIssue(context.Background(), "ENG-1", MoveIssueInput{Transition: "Done", Comment: "done"}); err != nil {
		t.Fatal(err)
	}
	if !moved {
		t.Fatal("expected transition request")
	}
	worklogs, err := client.ListWorklogs(context.Background(), "ENG-1")
	if err != nil || worklogs[0].Comment != "logged" {
		t.Fatalf("unexpected worklogs: %+v err=%v", worklogs, err)
	}
	worklog, err := client.AddWorklog(context.Background(), "ENG-1", WorklogInput{TimeSpent: "2h", Comment: "created"})
	if err != nil || worklog.ID != "21" {
		t.Fatalf("unexpected worklog: %+v err=%v", worklog, err)
	}
	worklog, err = client.UpdateWorklog(context.Background(), "ENG-1", "21", WorklogInput{TimeSpent: "3h", Comment: "updated"})
	if err != nil || worklog.TimeSpent != "3h" {
		t.Fatalf("unexpected updated worklog: %+v err=%v", worklog, err)
	}
	if err := client.DeleteWorklog(context.Background(), "ENG-1", "21"); err != nil {
		t.Fatal(err)
	}
	result, err := client.Raw(context.Background(), http.MethodGet, "/rest/api/3/custom", url.Values{"a": {"1"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.(map[string]any)["ok"] != true {
		t.Fatalf("unexpected raw result: %+v", result)
	}
}

func TestParseHelpers(t *testing.T) {
	if refByNameOrID("123", "name")["id"] != "123" {
		t.Fatal("expected id ref")
	}
	if refByNameOrID("Bug", "name")["name"] != "Bug" {
		t.Fatal("expected name ref")
	}
	if !isDigits("123") || isDigits("QA-1") {
		t.Fatal("unexpected digits check")
	}
	value := ParseFieldValue(`["a","b"]`)
	if list, ok := value.([]any); !ok || len(list) != 2 {
		t.Fatalf("unexpected parsed field value: %#v", value)
	}
	if ParseFieldValue("plain").(string) != "plain" {
		t.Fatal("expected plain string")
	}
	comment := commentDocument{
		Body: map[string]any{
			"type": "doc",
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{"type": "text", "text": "hi"},
					},
				},
			},
		},
	}
	if got := strings.TrimSpace(comment.toComment().Body); got != "hi" {
		t.Fatalf("unexpected extracted comment: %q", got)
	}
}

func newTestClient(siteURL string) *Client {
	return NewClient(httpx.New(httpx.Options{
		Profile: config.ResolvedProfile{
			SiteURL:  siteURL,
			Email:    "user@example.com",
			APIToken: "token",
		},
	}))
}
