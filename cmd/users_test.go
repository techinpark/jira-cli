package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUsersAndWhoamiCommands(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/myself":
			_, _ = w.Write([]byte(`{"accountId":"me-1","displayName":"Me","emailAddress":"me@example.com","active":true}`))
		case r.URL.Path == "/rest/api/3/user/search":
			_, _ = w.Write([]byte(`[{"accountId":"a-1","displayName":"Alice","emailAddress":"alice@example.com","active":true}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	if _, err := executeRoot(t, nil, "auth", "init", "--profile", "work", "--site-url", server.URL, "--email", "u@e.com", "--api-token", "t", "--default-project", "ENG", "--json"); err != nil {
		t.Fatal(err)
	}

	out, err := executeRoot(t, nil, "users", "search", "--query", "alice", "--profile", "work", "--json")
	if err != nil {
		t.Fatalf("users search failed: out=%s err=%v", out, err)
	}
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "a-1") {
		t.Fatalf("unexpected users search output: %s", out)
	}

	out, err = executeRoot(t, nil, "auth", "whoami", "--profile", "work", "--json")
	if err != nil {
		t.Fatalf("auth whoami failed: out=%s err=%v", out, err)
	}
	if !strings.Contains(out, "me-1") {
		t.Fatalf("unexpected auth whoami output: %s", out)
	}
}
