package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestAuthAndCommandFlows(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	server := newCommandMockServer()
	defer server.Close()

	out, err := executeRoot(t, nil, "auth", "init", "--profile", "work", "--site-url", server.URL, "--email", "user@example.com", "--api-token", "token", "--default-project", "ENG", "--json")
	if err != nil || !strings.Contains(out, `"profile": "work"`) {
		t.Fatalf("unexpected auth init: out=%s err=%v", out, err)
	}

	tests := []struct {
		args []string
		want string
	}{
		{[]string{"auth", "list", "--json"}, `"name": "work"`},
		{[]string{"auth", "check", "--profile", "work", "--json"}, `"ok": true`},
		{[]string{"projects", "list", "--profile", "work"}, "Engineering"},
		{[]string{"projects", "get", "ENG", "--profile", "work", "--json"}, `"key": "ENG"`},
		{[]string{"issues", "get", "ENG-1", "--profile", "work"}, "Fix bug"},
		{[]string{"issues", "search", "--profile", "work", "--jql", "project = ENG", "--json"}, `"issues"`},
		{[]string{"issues", "create", "--profile", "work", "--type", "Bug", "--summary", "New issue", "--json"}, `"key": "ENG-2"`},
		{[]string{"issues", "update", "ENG-1", "--profile", "work", "--summary", "Renamed"}, `"updated": true`},
		{[]string{"comments", "list", "ENG-1", "--profile", "work"}, "hello"},
		{[]string{"comments", "add", "ENG-1", "--profile", "work", "--body", "new comment"}, `"id": "11"`},
		{[]string{"comments", "update", "ENG-1", "11", "--profile", "work", "--body", "edited"}, `"edited"`},
		{[]string{"comments", "delete", "ENG-1", "11", "--profile", "work"}, `"deleted": true`},
		{[]string{"transitions", "list", "ENG-1", "--profile", "work"}, "Done"},
		{[]string{"transitions", "move", "ENG-1", "--profile", "work", "--transition", "Done"}, `"moved": true`},
		{[]string{"worklogs", "list", "ENG-1", "--profile", "work"}, "logged"},
		{[]string{"worklogs", "add", "ENG-1", "--profile", "work", "--time-spent", "1h"}, `"id": "21"`},
		{[]string{"worklogs", "update", "ENG-1", "21", "--profile", "work", "--time-spent", "2h"}, `"id": "21"`},
		{[]string{"worklogs", "delete", "ENG-1", "21", "--profile", "work"}, `"deleted": true`},
		{[]string{"raw", "GET", "/rest/api/3/custom", "--profile", "work"}, `"ok": true`},
		{[]string{"auth", "remove", "work", "--json"}, `"removed_profile": "work"`},
	}

	for _, tt := range tests {
		out, err := executeRoot(t, nil, tt.args...)
		if err != nil {
			t.Fatalf("args=%v err=%v out=%s", tt.args, err, out)
		}
		if !strings.Contains(out, tt.want) {
			t.Fatalf("args=%v out=%q want=%q", tt.args, out, tt.want)
		}
	}
}

func TestAuthInitPromptAndErrors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	server := newCommandMockServer()
	defer server.Close()

	restore := withStdin(t, server.URL+"\nuser@example.com\ntoken\nENG\n")
	defer restore()
	out, err := executeRoot(t, nil, "auth", "init")
	if err != nil || !strings.Contains(out, "Saved profile") {
		t.Fatalf("unexpected prompt init: out=%s err=%v", out, err)
	}

	if _, err := executeRoot(t, nil, "issues", "search", "--output", "yaml", "--jql", "project = ENG"); err == nil {
		t.Fatal("expected output validation error")
	}
	if _, err := executeRoot(t, nil, "raw", "GET", "/rest/api/3/custom", "--query", "broken"); err == nil {
		t.Fatal("expected raw query error")
	}
}

func executeRoot(t *testing.T, stdin io.Reader, args ...string) (string, error) {
	t.Helper()
	resetOptions()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	if stdin != nil {
		rootCmd.SetIn(stdin)
	}
	err := rootCmd.Execute()
	return buf.String(), err
}

func resetOptions() {
	opts = rootOptions{}
	_ = rootCmd.PersistentFlags().Set("output", "table")
	_ = rootCmd.PersistentFlags().Set("json", "false")
	_ = rootCmd.PersistentFlags().Set("profile", "")
	_ = rootCmd.PersistentFlags().Set("site-url", "")
	_ = rootCmd.PersistentFlags().Set("email", "")
	_ = rootCmd.PersistentFlags().Set("api-token", "")
	_ = rootCmd.PersistentFlags().Set("default-project", "")
}

func withStdin(t *testing.T, input string) func() {
	t.Helper()
	old := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	os.Stdin = r
	return func() {
		os.Stdin = old
		_ = r.Close()
	}
}

func newCommandMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/myself":
			_, _ = w.Write([]byte(`{"accountId":"abc","displayName":"User"}`))
		case r.URL.Path == "/rest/api/3/project/search":
			_, _ = w.Write([]byte(`{"values":[{"id":"100","key":"ENG","name":"Engineering","projectTypeKey":"software","lead":{"displayName":"Tech Lead"}}]}`))
		case r.URL.Path == "/rest/api/3/project/ENG":
			_, _ = w.Write([]byte(`{"id":"100","key":"ENG","name":"Engineering","projectTypeKey":"software","lead":{"displayName":"Tech Lead"}}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"1","key":"ENG-1","fields":{"summary":"Fix bug","status":{"name":"In Progress"},"issuetype":{"name":"Bug"},"project":{"key":"ENG"},"assignee":{"displayName":"Alice"}}}`))
		case r.URL.Path == "/rest/api/3/search/jql":
			_, _ = w.Write([]byte(`{"issues":[{"id":"1","key":"ENG-1","fields":{"summary":"Fix bug","status":{"name":"In Progress"},"issuetype":{"name":"Bug"},"assignee":{"displayName":"Alice"}}}]}`))
		case r.URL.Path == "/rest/api/3/issue" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"id":"2","key":"ENG-2","self":"https://jira/rest/api/3/issue/2"}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1" && (r.Method == http.MethodPut || r.Method == http.MethodDelete):
			w.WriteHeader(http.StatusNoContent)
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
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/rest/api/3/issue/ENG-1/worklog" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"worklogs":[{"id":"20","started":"2026-04-13T09:00:00.000+0900","timeSpent":"1h","author":{"displayName":"Alice"},"comment":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"logged"}]}]}}]}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/worklog" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"id":"21","started":"2026-04-13T09:00:00.000+0900","timeSpent":"1h","author":{"displayName":"Alice"},"comment":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"created"}]}]}}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/worklog/21" && r.Method == http.MethodPut:
			_, _ = w.Write([]byte(`{"id":"21","started":"2026-04-13T09:00:00.000+0900","timeSpent":"2h","author":{"displayName":"Alice"},"comment":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"updated"}]}]}}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/worklog/21" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/rest/api/3/custom":
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
}
