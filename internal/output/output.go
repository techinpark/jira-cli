package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/techinpark/jira-cli/internal/jira"
)

func RenderTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func RenderProjectsTable(w io.Writer, projects []jira.Project) error {
	rows := make([][]string, 0, len(projects))
	for _, item := range projects {
		rows = append(rows, []string{item.Key, item.Name, item.Type, item.Lead, yesNo(item.Archived)})
	}
	return RenderTable(w, []string{"Key", "Name", "Type", "Lead", "Archived"}, rows)
}

func RenderIssuesTable(w io.Writer, issues []jira.Issue) error {
	rows := make([][]string, 0, len(issues))
	for _, item := range issues {
		rows = append(rows, []string{item.Key, item.Summary, item.Status, item.IssueType, item.Assignee})
	}
	return RenderTable(w, []string{"Key", "Summary", "Status", "Type", "Assignee"}, rows)
}

func RenderCommentsTable(w io.Writer, comments []jira.Comment) error {
	rows := make([][]string, 0, len(comments))
	for _, item := range comments {
		rows = append(rows, []string{item.ID, item.Author, item.Created, truncate(item.Body, 80)})
	}
	return RenderTable(w, []string{"ID", "Author", "Created", "Body"}, rows)
}

func RenderTransitionsTable(w io.Writer, transitions []jira.Transition) error {
	rows := make([][]string, 0, len(transitions))
	for _, item := range transitions {
		rows = append(rows, []string{item.ID, item.Name, item.ToStatus})
	}
	return RenderTable(w, []string{"ID", "Name", "To Status"}, rows)
}

func RenderWorklogsTable(w io.Writer, worklogs []jira.Worklog) error {
	rows := make([][]string, 0, len(worklogs))
	for _, item := range worklogs {
		rows = append(rows, []string{item.ID, item.Author, item.Started, item.TimeSpent, truncate(item.Comment, 80)})
	}
	return RenderTable(w, []string{"ID", "Author", "Started", "Time Spent", "Comment"}, rows)
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit-3] + "..."
}
