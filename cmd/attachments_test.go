package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttachmentsCommands(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/myself":
			_, _ = w.Write([]byte(`{"accountId":"x"}`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"1","key":"ENG-1","fields":{"attachment":[{"id":"99","filename":"note.txt","size":4,"mimeType":"text/plain","created":"2026-04-13"}]}}`))
		case r.URL.Path == "/rest/api/3/attachment/99" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"99","filename":"note.txt","size":4,"mimeType":"text/plain"}`))
		case r.URL.Path == "/rest/api/3/attachment/content/99" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte("data"))
		case r.URL.Path == "/rest/api/3/attachment/99" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	if _, err := executeRoot(t, nil, "auth", "init", "--profile", "work", "--site-url", server.URL, "--email", "u@e.com", "--api-token", "t", "--default-project", "ENG", "--json"); err != nil {
		t.Fatal(err)
	}

	out, err := executeRoot(t, nil, "attachments", "list", "ENG-1", "--profile", "work", "--json")
	if err != nil || !strings.Contains(out, "note.txt") {
		t.Fatalf("unexpected list output: out=%s err=%v", out, err)
	}

	// Default destination (no --out) derives a safe filename from metadata into
	// the current directory. Run this before any --out download so the reused
	// cobra command's flag state is still empty.
	t.Chdir(tmp)
	out, err = executeRoot(t, nil, "attachments", "download", "99", "--profile", "work", "--json")
	if err != nil || !strings.Contains(out, `"filename": "note.txt"`) {
		t.Fatalf("default-name download failed: out=%s err=%v", out, err)
	}
	if data, err := os.ReadFile(filepath.Join(tmp, "note.txt")); err != nil || string(data) != "data" {
		t.Fatalf("default download contents: %q err=%v", string(data), err)
	}

	dest := filepath.Join(tmp, "out.txt")
	out, err = executeRoot(t, nil, "attachments", "download", "99", "--profile", "work", "--out", dest, "--json")
	if err != nil {
		t.Fatalf("download failed: out=%s err=%v", out, err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(data) != "data" {
		t.Fatalf("unexpected downloaded contents: %q", string(data))
	}

	out, err = executeRoot(t, nil, "attachments", "delete", "99", "--profile", "work")
	if err != nil || !strings.Contains(out, `"deleted": true`) {
		t.Fatalf("unexpected delete output: out=%s err=%v", out, err)
	}
}
