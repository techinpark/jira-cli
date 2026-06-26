package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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

func TestAddAttachments(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "note.txt")
	second := filepath.Join(dir, "screenshot.png")
	if err := os.WriteFile(first, []byte("log output"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("not really a png"), 0o600); err != nil {
		t.Fatal(err)
	}

	var gotFilenames []string
	var gotMimeTypes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/ENG-1/attachments" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Atlassian-Token") != "no-check" {
			t.Fatalf("missing XSRF bypass header")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		for _, fh := range r.MultipartForm.File["file"] {
			gotFilenames = append(gotFilenames, fh.Filename)
			gotMimeTypes = append(gotMimeTypes, fh.Header.Get("Content-Type"))
		}
		_, _ = w.Write([]byte(`[{"id":"99","filename":"note.txt","author":{"displayName":"Alice"},"created":"2026-04-13T09:00:00.000+0900","size":10,"mimeType":"text/plain","content":"https://jira/rest/api/3/attachment/content/99"},{"id":"100","filename":"screenshot.png","size":16,"mimeType":"image/png"}]`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	attachments, err := client.AddAttachments(context.Background(), "ENG-1", []string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if len(attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(attachments))
	}
	if attachments[0].ID != "99" || attachments[0].Filename != "note.txt" || attachments[0].Author != "Alice" || attachments[0].Size != 10 || attachments[0].MimeType != "text/plain" {
		t.Fatalf("unexpected first attachment: %+v", attachments[0])
	}
	if len(gotFilenames) != 2 || gotFilenames[0] != "note.txt" || gotFilenames[1] != "screenshot.png" {
		t.Fatalf("unexpected uploaded filenames: %+v", gotFilenames)
	}
	if len(gotMimeTypes) != 2 || !strings.HasPrefix(gotMimeTypes[0], "text/plain") || gotMimeTypes[1] != "image/png" {
		t.Fatalf("unexpected part content types: %+v", gotMimeTypes)
	}

	if _, err := client.AddAttachments(context.Background(), "ENG-1", nil); err == nil {
		t.Fatal("expected error for empty attachment list")
	}
	if _, err := client.AddAttachments(context.Background(), "ENG-1", []string{filepath.Join(dir, "missing.txt")}); err == nil {
		t.Fatal("expected error for missing file")
	}
	if _, err := client.AddAttachments(context.Background(), "ENG-1", []string{dir}); err == nil {
		t.Fatal("expected error for directory path")
	}

	tooMany := make([]string, 61)
	for i := range tooMany {
		tooMany[i] = first
	}
	if _, err := client.AddAttachments(context.Background(), "ENG-1", tooMany); err == nil {
		t.Fatal("expected error when exceeding the 60-file limit")
	}
}

func TestSearchPagination(t *testing.T) {
	var requestedTokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		// maxResults must be sent as the per-page size (default 50 here).
		if mr, _ := body["maxResults"].(float64); mr != 50 {
			t.Fatalf("expected maxResults 50 per page, got %v", body["maxResults"])
		}
		token, _ := body["nextPageToken"].(string)
		requestedTokens = append(requestedTokens, token)
		switch token {
		case "":
			// First page: more results follow.
			_, _ = w.Write([]byte(`{"issues":[{"id":"1","key":"ENG-1"}],"nextPageToken":"tok2","isLast":false}`))
		case "tok2":
			// Last page: no nextPageToken, isLast true.
			_, _ = w.Write([]byte(`{"issues":[{"id":"2","key":"ENG-2"}],"isLast":true}`))
		default:
			t.Fatalf("unexpected page token: %q", token)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	ctx := context.Background()

	// A single SearchIssues with a PageToken must SEND it as nextPageToken.
	page2, err := client.SearchIssues(ctx, SearchOptions{JQL: "project = ENG", PageToken: "tok2"})
	if err != nil || len(page2.Issues) != 1 || page2.Issues[0].Key != "ENG-2" || !page2.IsLast {
		t.Fatalf("unexpected page2: %+v err=%v", page2, err)
	}
	if requestedTokens[len(requestedTokens)-1] != "tok2" {
		t.Fatalf("page token was not sent to the server: %+v", requestedTokens)
	}

	// SearchAllIssues must follow nextPageToken and accumulate every page.
	all, err := client.SearchAllIssues(ctx, SearchOptions{JQL: "project = ENG"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Issues) != 2 || all.Issues[0].Key != "ENG-1" || all.Issues[1].Key != "ENG-2" {
		t.Fatalf("expected both pages accumulated, got %+v", all)
	}
	if !all.IsLast || all.NextPageToken != "" {
		t.Fatalf("fully-paginated result should be last with no token: %+v", all)
	}
}

func TestMetadataDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/issue/createmeta/ENG/issuetypes":
			_, _ = w.Write([]byte(`{"issueTypes":[{"id":"10001","name":"Bug","subtask":false},{"id":"10002","name":"Task","subtask":false}]}`))
		case "/rest/api/3/issue/createmeta/ENG/issuetypes/10001":
			// Modern shape: fields under "results".
			_, _ = w.Write([]byte(`{"results":[{"fieldId":"summary","name":"Summary","required":true,"schema":{"type":"string"}}]}`))
		case "/rest/api/3/issue/createmeta/ENG/issuetypes/10002":
			// Legacy shape: fields under "fields" — must still be returned.
			_, _ = w.Write([]byte(`{"fields":[{"fieldId":"description","name":"Description","required":false,"schema":{"type":"string"}}]}`))
		case "/rest/api/3/issue/ENG-1/editmeta":
			_, _ = w.Write([]byte(`{"fields":{"summary":{"name":"Summary","required":true,"schema":{"type":"string"}}}}`))
		case "/rest/api/3/field":
			_, _ = w.Write([]byte(`[{"id":"summary","key":"summary","name":"Summary","custom":false,"schema":{"type":"string"}},{"id":"customfield_10016","key":"customfield_10016","name":"Story Points","custom":true,"schema":{"type":"number"}}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	ctx := context.Background()

	types, err := client.CreateMetaIssueTypes(ctx, "ENG")
	if err != nil || len(types) != 2 || types[0]["id"] != "10001" || types[0]["name"] != "Bug" {
		t.Fatalf("unexpected issue types: %+v err=%v", types, err)
	}

	// Name resolution + numeric passthrough + not-found.
	if id, err := client.ResolveIssueTypeID(ctx, "ENG", "Bug"); err != nil || id != "10001" {
		t.Fatalf("resolve name: %q err=%v", id, err)
	}
	if id, err := client.ResolveIssueTypeID(ctx, "ENG", "10002"); err != nil || id != "10002" {
		t.Fatalf("numeric passthrough: %q err=%v", id, err)
	}
	if _, err := client.ResolveIssueTypeID(ctx, "ENG", "Nope"); err == nil {
		t.Fatal("expected error for unknown issue type")
	}

	// results-shape fields.
	fields, err := client.CreateMetaFields(ctx, "ENG", "10001")
	if err != nil || len(fields) != 1 || fields[0]["fieldId"] != "summary" || fields[0]["required"] != true {
		t.Fatalf("unexpected create-meta fields: %+v err=%v", fields, err)
	}
	// legacy fields-shape fallback.
	legacy, err := client.CreateMetaFields(ctx, "ENG", "10002")
	if err != nil || len(legacy) != 1 || legacy[0]["fieldId"] != "description" {
		t.Fatalf("expected legacy fields fallback: %+v err=%v", legacy, err)
	}

	edit, err := client.EditMeta(ctx, "ENG-1")
	if err != nil || edit["summary"] == nil {
		t.Fatalf("unexpected editmeta: %+v err=%v", edit, err)
	}

	all, err := client.ListFields(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("unexpected fields list: %+v err=%v", all, err)
	}
	foundCustom := false
	for _, f := range all {
		if f["name"] == "Story Points" && f["id"] == "customfield_10016" {
			foundCustom = true
		}
	}
	if !foundCustom {
		t.Fatalf("expected custom field in list: %+v", all)
	}
}

func TestIssueLinks(t *testing.T) {
	var linkBody map[string]any
	var deleted string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/issueLinkType" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"issueLinkTypes":[{"id":"1000","name":"Blocks","inward":"is blocked by","outward":"blocks"}]}`))
		case r.URL.Path == "/rest/api/3/issueLink" && r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&linkBody)
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/rest/api/3/issue/ENG-1" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"1","key":"ENG-1","fields":{"issuelinks":[{"id":"10","type":{"name":"Blocks","inward":"is blocked by","outward":"blocks"},"outwardIssue":{"key":"ENG-2"}}]}}`))
		case r.URL.Path == "/rest/api/3/issueLink/10" && r.Method == http.MethodDelete:
			deleted = "10"
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	ctx := context.Background()

	types, err := client.ListIssueLinkTypes(ctx)
	if err != nil || len(types) != 1 || types[0].Name != "Blocks" || types[0].Outward != "blocks" {
		t.Fatalf("unexpected link types: %+v err=%v", types, err)
	}

	if err := client.LinkIssues(ctx, LinkIssuesInput{Type: "Blocks", Inward: "ENG-2", Outward: "ENG-1", Comment: "see also"}); err != nil {
		t.Fatal(err)
	}
	if linkBody["type"].(map[string]any)["name"] != "Blocks" {
		t.Fatalf("unexpected link type in body: %+v", linkBody)
	}
	if linkBody["inwardIssue"].(map[string]any)["key"] != "ENG-2" || linkBody["outwardIssue"].(map[string]any)["key"] != "ENG-1" {
		t.Fatalf("unexpected link issues in body: %+v", linkBody)
	}
	if _, ok := linkBody["comment"]; !ok {
		t.Fatalf("expected comment in link body: %+v", linkBody)
	}

	links, err := client.ListIssueLinks(ctx, "ENG-1")
	if err != nil || len(links) != 1 || links[0]["id"] != "10" {
		t.Fatalf("unexpected links: %+v err=%v", links, err)
	}

	if err := client.DeleteIssueLink(ctx, "10"); err != nil || deleted != "10" {
		t.Fatalf("delete link failed: deleted=%q err=%v", deleted, err)
	}
}

func TestCreateMetaPaginatesAllPages(t *testing.T) {
	var startAts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/createmeta/ENG/issuetypes" {
			http.NotFound(w, r)
			return
		}
		startAts = append(startAts, r.URL.Query().Get("startAt"))
		switch r.URL.Query().Get("startAt") {
		case "0":
			_, _ = w.Write([]byte(`{"issueTypes":[{"id":"1","name":"A"}],"total":2,"maxResults":200,"startAt":0}`))
		case "1":
			_, _ = w.Write([]byte(`{"issueTypes":[{"id":"2","name":"B"}],"total":2,"maxResults":200,"startAt":1}`))
		default:
			t.Fatalf("unexpected startAt: %q", r.URL.Query().Get("startAt"))
		}
	}))
	defer server.Close()

	types, err := newTestClient(server.URL).CreateMetaIssueTypes(context.Background(), "ENG")
	if err != nil {
		t.Fatal(err)
	}
	if len(types) != 2 || types[0]["id"] != "1" || types[1]["id"] != "2" {
		t.Fatalf("expected both pages, got %+v", types)
	}
	if len(startAts) != 2 || startAts[0] != "0" || startAts[1] != "1" {
		t.Fatalf("expected startAt 0 then 1, got %+v", startAts)
	}
}

func TestSearchAllAbortsOnStuckToken(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		// Always return the same token with isLast=false (a misbehaving server).
		_, _ = w.Write([]byte(`{"issues":[{"id":"1","key":"ENG-1"}],"nextPageToken":"stuck","isLast":false}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.SearchAllIssues(context.Background(), SearchOptions{JQL: "project = ENG"}, 0)
	if err == nil {
		t.Fatal("expected error when the pagination token does not advance")
	}
	if calls > 3 {
		t.Fatalf("expected to abort quickly, made %d calls", calls)
	}
}

func TestUserAndAttachmentDataLayer(t *testing.T) {
	var assigneeBody string
	var createBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/myself":
			_, _ = w.Write([]byte(`{"accountId":"me-123","displayName":"Me","emailAddress":"me@example.com","active":true,"accountType":"atlassian"}`))
		case r.URL.Path == "/rest/api/3/user/search":
			if r.URL.Query().Get("query") == "alice@example.com" {
				_, _ = w.Write([]byte(`[{"accountId":"a-1","displayName":"Alice","emailAddress":"alice@example.com","active":true}]`))
				return
			}
			_, _ = w.Write([]byte(`[]`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/assignee" && r.Method == http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			assigneeBody = string(b)
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/rest/api/3/issue" && r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&createBody)
			_, _ = w.Write([]byte(`{"id":"2","key":"ENG-2"}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"1","key":"ENG-1","fields":{"attachment":[{"id":"99","filename":"note.txt","size":4,"mimeType":"text/plain"}]}}`))
		case r.URL.Path == "/rest/api/3/attachment/99" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"99","filename":"note.txt","size":4,"mimeType":"text/plain"}`))
		case r.URL.Path == "/rest/api/3/attachment/content/99" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`file`))
		case r.URL.Path == "/rest/api/3/attachment/99" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	ctx := context.Background()

	me, err := client.Myself(ctx)
	if err != nil || me.AccountID != "me-123" || me.Email != "me@example.com" {
		t.Fatalf("unexpected myself: %+v err=%v", me, err)
	}

	// ResolveUserAccountID: me / email / raw accountId.
	if id, err := client.ResolveUserAccountID(ctx, "me"); err != nil || id != "me-123" {
		t.Fatalf("resolve me: %q err=%v", id, err)
	}
	if id, err := client.ResolveUserAccountID(ctx, "alice@example.com"); err != nil || id != "a-1" {
		t.Fatalf("resolve email: %q err=%v", id, err)
	}
	if id, err := client.ResolveUserAccountID(ctx, "raw-account-id"); err != nil || id != "raw-account-id" {
		t.Fatalf("resolve raw: %q err=%v", id, err)
	}
	if _, err := client.ResolveUserAccountID(ctx, "ghost@example.com"); err == nil {
		t.Fatal("expected error for unknown email")
	}

	// AssignIssue: set then unassign.
	if err := client.AssignIssue(ctx, "ENG-1", "a-1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(assigneeBody, `"accountId":"a-1"`) {
		t.Fatalf("unexpected assignee body: %s", assigneeBody)
	}
	if err := client.AssignIssue(ctx, "ENG-1", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(assigneeBody, `"accountId":null`) {
		t.Fatalf("expected null assignee on unassign: %s", assigneeBody)
	}

	// Attachment lifecycle.
	atts, err := client.ListAttachments(ctx, "ENG-1")
	if err != nil || len(atts) != 1 || atts[0].ID != "99" || atts[0].Filename != "note.txt" {
		t.Fatalf("unexpected attachments: %+v err=%v", atts, err)
	}
	var buf strings.Builder
	if err := client.DownloadAttachmentContent(ctx, "99", &buf); err != nil || buf.String() != "file" {
		t.Fatalf("unexpected download: body=%q err=%v", buf.String(), err)
	}
	if err := client.DeleteAttachment(ctx, "99"); err != nil {
		t.Fatal(err)
	}

	// Convenience fields flow through create.
	if _, err := client.CreateIssue(ctx, CreateIssueInput{
		Project: "ENG", IssueType: "Bug", Summary: "x",
		Assignee: "a-1", Priority: "High", Parent: "ENG-1", Due: "2026-07-01", Labels: []string{"a", "b"},
	}); err != nil {
		t.Fatal(err)
	}
	cf := createBody["fields"].(map[string]any)
	if cf["assignee"].(map[string]any)["accountId"] != "a-1" {
		t.Fatalf("missing assignee in create: %+v", cf)
	}
	if cf["priority"].(map[string]any)["name"] != "High" || cf["parent"].(map[string]any)["key"] != "ENG-1" || cf["duedate"] != "2026-07-01" {
		t.Fatalf("unexpected convenience fields: %+v", cf)
	}
	if labels, ok := cf["labels"].([]any); !ok || len(labels) != 2 {
		t.Fatalf("unexpected labels: %+v", cf["labels"])
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
