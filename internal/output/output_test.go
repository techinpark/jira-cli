package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/techinpark/jira-cli/internal/jira"
)

func TestRenderTables(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderProjectsTable(&buf, []jira.Project{{Key: "ENG", Name: "Engineering", Type: "software", Lead: "Alice"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Engineering") {
		t.Fatalf("unexpected project table: %s", buf.String())
	}
	buf.Reset()
	if err := RenderIssuesTable(&buf, []jira.Issue{{Key: "ENG-1", Summary: "Fix bug"}}); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := RenderCommentsTable(&buf, []jira.Comment{{ID: "1", Body: strings.Repeat("a", 120)}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "...") {
		t.Fatalf("expected truncation: %s", buf.String())
	}
}
