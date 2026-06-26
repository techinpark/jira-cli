package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLinksCommands(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	var linkBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/myself":
			_, _ = w.Write([]byte(`{"accountId":"x"}`))
		case r.URL.Path == "/rest/api/3/issueLinkType":
			_, _ = w.Write([]byte(`{"issueLinkTypes":[{"id":"1000","name":"Blocks","inward":"is blocked by","outward":"blocks"}]}`))
		case r.URL.Path == "/rest/api/3/issueLink" && r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&linkBody)
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/rest/api/3/issue/ENG-1" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"1","key":"ENG-1","fields":{"issuelinks":[{"id":"10","type":{"name":"Blocks","inward":"is blocked by","outward":"blocks"},"outwardIssue":{"key":"ENG-2"}}]}}`))
		case r.URL.Path == "/rest/api/3/issueLink/10" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	if _, err := executeRoot(t, nil, "auth", "init", "--profile", "work", "--site-url", server.URL, "--email", "u@e.com", "--api-token", "t", "--default-project", "ENG", "--json"); err != nil {
		t.Fatal(err)
	}

	out, err := executeRoot(t, nil, "links", "types", "--profile", "work", "--json")
	if err != nil || !strings.Contains(out, `"name": "Blocks"`) {
		t.Fatalf("links types: out=%s err=%v", out, err)
	}

	out, err = executeRoot(t, nil, "links", "add", "--outward", "ENG-1", "--inward", "ENG-2", "--type", "Blocks", "--profile", "work")
	if err != nil || !strings.Contains(out, `"linked": true`) {
		t.Fatalf("links add: out=%s err=%v", out, err)
	}
	if linkBody["outwardIssue"].(map[string]any)["key"] != "ENG-1" {
		t.Fatalf("unexpected link body: %+v", linkBody)
	}

	out, err = executeRoot(t, nil, "links", "list", "ENG-1", "--profile", "work", "--json")
	if err != nil || !strings.Contains(out, `"id": "10"`) {
		t.Fatalf("links list: out=%s err=%v", out, err)
	}

	out, err = executeRoot(t, nil, "links", "delete", "10", "--profile", "work")
	if err != nil || !strings.Contains(out, `"deleted": true`) {
		t.Fatalf("links delete: out=%s err=%v", out, err)
	}
}
