package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetadataCommands(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/myself":
			_, _ = w.Write([]byte(`{"accountId":"x"}`))
		case "/rest/api/3/issue/createmeta/ENG/issuetypes":
			_, _ = w.Write([]byte(`{"issueTypes":[{"id":"10001","name":"Bug","subtask":false}]}`))
		case "/rest/api/3/issue/createmeta/ENG/issuetypes/10001":
			_, _ = w.Write([]byte(`{"results":[{"fieldId":"summary","name":"Summary","required":true,"schema":{"type":"string"}}]}`))
		case "/rest/api/3/issue/ENG-1/editmeta":
			_, _ = w.Write([]byte(`{"fields":{"summary":{"name":"Summary","required":true,"schema":{"type":"string"}}}}`))
		case "/rest/api/3/field":
			_, _ = w.Write([]byte(`[{"id":"customfield_10016","key":"customfield_10016","name":"Story Points","custom":true,"schema":{"type":"number"}}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	if _, err := executeRoot(t, nil, "auth", "init", "--profile", "work", "--site-url", server.URL, "--email", "u@e.com", "--api-token", "t", "--default-project", "ENG", "--json"); err != nil {
		t.Fatal(err)
	}

	out, err := executeRoot(t, nil, "issues", "create-meta", "--profile", "work", "--json")
	if err != nil || !strings.Contains(out, `"name": "Bug"`) {
		t.Fatalf("create-meta issue types: out=%s err=%v", out, err)
	}

	out, err = executeRoot(t, nil, "issues", "create-meta", "--type", "Bug", "--profile", "work", "--json")
	if err != nil || !strings.Contains(out, `"fieldId": "summary"`) || !strings.Contains(out, `"required": true`) {
		t.Fatalf("create-meta fields (resolved by name): out=%s err=%v", out, err)
	}

	out, err = executeRoot(t, nil, "issues", "edit-meta", "ENG-1", "--profile", "work", "--json")
	if err != nil || !strings.Contains(out, `"summary"`) {
		t.Fatalf("edit-meta: out=%s err=%v", out, err)
	}

	out, err = executeRoot(t, nil, "fields", "list", "--profile", "work", "--json")
	if err != nil || !strings.Contains(out, "customfield_10016") {
		t.Fatalf("fields list: out=%s err=%v", out, err)
	}
}
