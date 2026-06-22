package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// resetSubcommandFlags clears the sticky per-invocation flag state (value and
// Changed) on a subcommand of rootCmd. rootCmd and its subcommands are built
// once in init(), so flag state leaks between executeRoot calls; this restores
// each flag to its default so successive invocations are independent.
func resetSubcommandFlags(t *testing.T, path ...string) {
	t.Helper()
	cmd := rootCmd
	for _, name := range path {
		var next *cobra.Command
		for _, c := range cmd.Commands() {
			if c.Name() == name {
				next = c
				break
			}
		}
		if next == nil {
			t.Fatalf("subcommand %q not found", name)
		}
		cmd = next
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Slice/array flags append on Set, so Set(DefValue) would inject the
		// literal default (e.g. "[]") as an element. Clear them via Replace.
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	})
}

// newAssignMockServer records the most recent assignee PUT body and the most
// recent POST /issue body so tests can assert on what the cmd layer sent.
func newAssignMockServer(t *testing.T) (*httptest.Server, func() string, func() string) {
	t.Helper()
	var (
		mu         sync.Mutex
		assignBody string
		createBody string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/myself":
			_, _ = w.Write([]byte(`{"accountId":"me-1","displayName":"Me"}`))
		case r.URL.Path == "/rest/api/3/user/search":
			_, _ = w.Write([]byte(`[{"accountId":"a-1","displayName":"Alice","emailAddress":"alice@example.com"}]`))
		case r.URL.Path == "/rest/api/3/issue/ENG-1/assignee" && r.Method == http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			assignBody = string(body)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/rest/api/3/issue" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			createBody = string(body)
			mu.Unlock()
			_, _ = w.Write([]byte(`{"id":"2","key":"ENG-2","self":"https://jira/rest/api/3/issue/2"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	lastAssign := func() string {
		mu.Lock()
		defer mu.Unlock()
		return assignBody
	}
	lastCreate := func() string {
		mu.Lock()
		defer mu.Unlock()
		return createBody
	}
	return server, lastAssign, lastCreate
}

func TestIssuesAssignCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	server, lastAssign, _ := newAssignMockServer(t)
	defer server.Close()

	if _, err := executeRoot(t, nil, "auth", "init", "--profile", "work", "--site-url", server.URL, "--email", "user@example.com", "--api-token", "token", "--default-project", "ENG", "--json"); err != nil {
		t.Fatal(err)
	}

	resetSubcommandFlags(t, "issues", "assign")
	out, err := executeRoot(t, nil, "issues", "assign", "ENG-1", "--assignee", "alice@example.com", "--profile", "work", "--json")
	if err != nil {
		t.Fatalf("assign by email failed: out=%s err=%v", out, err)
	}
	if body := lastAssign(); !strings.Contains(body, `"accountId":"a-1"`) {
		t.Fatalf("expected resolved accountId in assignee body, got: %s", body)
	}
	if !strings.Contains(out, `"assignee": "a-1"`) || !strings.Contains(out, `"key": "ENG-1"`) {
		t.Fatalf("unexpected assign output: %s", out)
	}

	resetSubcommandFlags(t, "issues", "assign")
	out, err = executeRoot(t, nil, "issues", "assign", "ENG-1", "--unassign", "--profile", "work", "--json")
	if err != nil {
		t.Fatalf("unassign failed: out=%s err=%v", out, err)
	}
	if body := lastAssign(); !strings.Contains(body, `"accountId":null`) {
		t.Fatalf("expected null accountId in unassign body, got: %s", body)
	}
	if !strings.Contains(out, `"unassigned": true`) {
		t.Fatalf("unexpected unassign output: %s", out)
	}
}

func TestIssuesAssignFlagValidation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	server, _, _ := newAssignMockServer(t)
	defer server.Close()

	if _, err := executeRoot(t, nil, "auth", "init", "--profile", "work", "--site-url", server.URL, "--email", "user@example.com", "--api-token", "token", "--default-project", "ENG", "--json"); err != nil {
		t.Fatal(err)
	}

	resetSubcommandFlags(t, "issues", "assign")
	if _, err := executeRoot(t, nil, "issues", "assign", "ENG-1", "--profile", "work", "--json"); err == nil {
		t.Fatal("expected error when neither --assignee nor --unassign is set")
	}
	resetSubcommandFlags(t, "issues", "assign")
	if _, err := executeRoot(t, nil, "issues", "assign", "ENG-1", "--assignee", "me", "--unassign", "--profile", "work", "--json"); err == nil {
		t.Fatal("expected error when both --assignee and --unassign are set")
	}
}

func TestIssuesCreateConvenienceFlags(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	server, _, lastCreate := newAssignMockServer(t)
	defer server.Close()

	if _, err := executeRoot(t, nil, "auth", "init", "--profile", "work", "--site-url", server.URL, "--email", "user@example.com", "--api-token", "token", "--default-project", "ENG", "--json"); err != nil {
		t.Fatal(err)
	}

	resetSubcommandFlags(t, "issues", "create")
	// Restore defaults afterwards so the scalar flags set here (priority, due,
	// assignee) do not leak into other tests that reuse the shared create command.
	defer resetSubcommandFlags(t, "issues", "create")
	out, err := executeRoot(t, nil, "issues", "create", "--type", "Bug", "--summary", "x", "--assignee", "me", "--labels", "a,b", "--priority", "High", "--due", "2026-07-01", "--profile", "work", "--json")
	if err != nil {
		t.Fatalf("create with convenience flags failed: out=%s err=%v", out, err)
	}
	body := lastCreate()
	for _, want := range []string{
		`"accountId":"me-1"`,
		`"a"`,
		`"b"`,
		`"name":"High"`,
		`"duedate":"2026-07-01"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected create body to contain %s, got: %s", want, body)
		}
	}
	if !strings.Contains(out, `"key": "ENG-2"`) {
		t.Fatalf("unexpected create output: %s", out)
	}
}
