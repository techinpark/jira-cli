package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/techinpark/jira-cli/internal/jira"
	"github.com/techinpark/jira-cli/internal/output"
)

func newIssuesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issues",
		Short: "Manage Jira issues",
	}
	cmd.AddCommand(newIssuesGetCommand())
	cmd.AddCommand(newIssuesSearchCommand())
	cmd.AddCommand(newIssuesCreateCommand())
	cmd.AddCommand(newIssuesUpdateCommand())
	cmd.AddCommand(newIssuesDeleteCommand())
	cmd.AddCommand(newIssuesAttachCommand())
	cmd.AddCommand(newIssuesAssignCommand())
	cmd.AddCommand(newIssuesCreateMetaCommand())
	cmd.AddCommand(newIssuesEditMetaCommand())
	return cmd
}

func newIssuesGetCommand() *cobra.Command {
	var fields []string
	cmd := &cobra.Command{
		Use:     "get <issue-key>",
		Short:   "Get a Jira issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			issue, err := client.GetIssue(context.Background(), args[0], fields)
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, issue)
			}
			return output.RenderIssuesTable(cmd.OutOrStdout(), []jira.Issue{issue})
		},
	}
	cmd.Flags().StringSliceVar(&fields, "fields", nil, "Fields to request")
	return cmd
}

func newIssuesSearchCommand() *cobra.Command {
	var jql string
	var fields []string
	var limit int
	var pageToken string
	var all bool

	cmd := &cobra.Command{
		Use:     "search",
		Short:   "Search Jira issues with JQL",
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			opts := jira.SearchOptions{
				JQL:       jql,
				Fields:    fields,
				Limit:     limit,
				PageToken: pageToken,
			}
			var result jira.SearchResult
			if all {
				result, err = client.SearchAllIssues(context.Background(), opts, 0)
			} else {
				result, err = client.SearchIssues(context.Background(), opts)
			}
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, result)
			}
			// Table output drops next_page_token, so warn when --all stopped at
			// the page cap with results still pending (JSON exposes the token).
			if all && result.NextPageToken != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: stopped at the page cap with more results remaining; resume with --page-token %s (or use --json)\n", result.NextPageToken)
			}
			return output.RenderIssuesTable(cmd.OutOrStdout(), result.Issues)
		},
	}
	cmd.Flags().StringVar(&jql, "jql", "", "JQL expression")
	cmd.Flags().StringSliceVar(&fields, "fields", []string{"summary", "status", "issuetype", "assignee"}, "Fields to return")
	cmd.Flags().IntVar(&limit, "limit", 50, "Page size: maximum number of issues returned per page")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "nextPageToken from a previous response, to fetch the next page")
	cmd.Flags().BoolVar(&all, "all", false, "Fetch all pages by following nextPageToken")
	_ = cmd.MarkFlagRequired("jql")
	return cmd
}

// createResult embeds the created issue reference and any uploaded attachments.
// Attachments stay omitted when none are requested, preserving the original
// create output shape.
type createResult struct {
	jira.IssueRef
	Attachments []jira.Attachment `json:"attachments,omitempty"`
}

func newIssuesCreateCommand() *cobra.Command {
	var project string
	var issueType string
	var summary string
	var description string
	var fields []string
	var attachments []string
	var assignee string
	var labels []string
	var priority string
	var parent string
	var due string

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a Jira issue",
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, profile, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			project, err = requiredProjectArg(project, profile)
			if err != nil {
				return err
			}
			extraFields, err := parseFieldAssignments(fields)
			if err != nil {
				return err
			}
			var assigneeID string
			if assignee != "" {
				assigneeID, err = client.ResolveUserAccountID(context.Background(), assignee)
				if err != nil {
					return err
				}
			}
			issue, err := client.CreateIssue(context.Background(), jira.CreateIssueInput{
				Project:     project,
				IssueType:   issueType,
				Summary:     summary,
				Description: description,
				Assignee:    assigneeID,
				Priority:    priority,
				Parent:      parent,
				Due:         due,
				Labels:      labels,
				Fields:      extraFields,
			})
			if err != nil {
				return err
			}
			result := createResult{IssueRef: issue}
			if len(attachments) > 0 {
				uploaded, err := client.AddAttachments(context.Background(), issue.Key, attachments)
				if err != nil {
					// The issue already exists server-side, so always emit its key
					// to the structured output before failing. This lets a --json
					// consumer recover and retry `issues attach <key>`.
					_ = writeJSON(cmd, result)
					return fmt.Errorf("issue %s created but attachment upload failed: %w", issue.Key, err)
				}
				result.Attachments = uploaded
			}
			return writeJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project key or ID")
	cmd.Flags().StringVar(&issueType, "type", "", "Issue type name or ID")
	cmd.Flags().StringVar(&summary, "summary", "", "Issue summary")
	cmd.Flags().StringVar(&description, "description", "", "Plain-text issue description")
	cmd.Flags().StringArrayVar(&fields, "field", nil, "Additional field in key=value form; JSON values are allowed")
	cmd.Flags().StringArrayVar(&attachments, "attach", nil, "Path to a file to attach after creation (repeatable)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee reference: me, an email, or an accountId")
	cmd.Flags().StringSliceVar(&labels, "labels", nil, "Labels to set (comma-separated)")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority name or ID")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent issue key or ID")
	cmd.Flags().StringVar(&due, "due", "", "Due date (YYYY-MM-DD)")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("summary")
	return cmd
}

func newIssuesUpdateCommand() *cobra.Command {
	var summary string
	var description string
	var setDescription bool
	var fields []string
	var assignee string
	var labels []string
	var priority string
	var parent string
	var due string

	cmd := &cobra.Command{
		Use:     "update <issue-key>",
		Short:   "Update a Jira issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			extraFields, err := parseFieldAssignments(fields)
			if err != nil {
				return err
			}
			var descPtr *string
			if cmd.Flags().Changed("description") {
				setDescription = true
			}
			if setDescription {
				descPtr = &description
			}
			var assigneeID string
			if assignee != "" {
				assigneeID, err = client.ResolveUserAccountID(context.Background(), assignee)
				if err != nil {
					return err
				}
			}
			if err := client.UpdateIssue(context.Background(), args[0], jira.UpdateIssueInput{
				Summary:     summary,
				Description: descPtr,
				Assignee:    assigneeID,
				Priority:    priority,
				Parent:      parent,
				Due:         due,
				Labels:      labels,
				Fields:      extraFields,
			}); err != nil {
				return err
			}
			return writeJSON(cmd, map[string]any{"updated": true, "key": args[0]})
		},
	}
	cmd.Flags().StringVar(&summary, "summary", "", "Updated issue summary")
	cmd.Flags().StringVar(&description, "description", "", "Updated plain-text issue description")
	cmd.Flags().StringArrayVar(&fields, "field", nil, "Additional field in key=value form; JSON values are allowed")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee reference: me, an email, or an accountId")
	cmd.Flags().StringSliceVar(&labels, "labels", nil, "Labels to set (comma-separated)")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority name or ID")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent issue key or ID")
	cmd.Flags().StringVar(&due, "due", "", "Due date (YYYY-MM-DD)")
	return cmd
}

func newIssuesDeleteCommand() *cobra.Command {
	var deleteSubtasks bool

	cmd := &cobra.Command{
		Use:     "delete <issue-key>",
		Short:   "Delete a Jira issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			if err := client.DeleteIssue(context.Background(), args[0], deleteSubtasks); err != nil {
				return err
			}
			return writeJSON(cmd, map[string]any{"deleted": true, "key": args[0]})
		},
	}
	cmd.Flags().BoolVar(&deleteSubtasks, "delete-subtasks", false, "Delete subtasks with the issue")
	return cmd
}

func newIssuesAttachCommand() *cobra.Command {
	var files []string

	cmd := &cobra.Command{
		Use:     "attach <issue-key>",
		Short:   "Upload file attachments to an existing Jira issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			attachments, err := client.AddAttachments(context.Background(), args[0], files)
			if err != nil {
				return err
			}
			return writeJSON(cmd, map[string]any{"key": args[0], "attachments": attachments})
		},
	}
	cmd.Flags().StringArrayVar(&files, "file", nil, "Path to a file to attach (repeatable)")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newIssuesAssignCommand() *cobra.Command {
	var assignee string
	var unassign bool

	cmd := &cobra.Command{
		Use:     "assign <issue-key>",
		Short:   "Assign or unassign a Jira issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			wantAssign := cmd.Flags().Changed("assignee")
			wantUnassign := cmd.Flags().Changed("unassign")
			if wantAssign == wantUnassign {
				return fmt.Errorf("specify exactly one of --assignee or --unassign")
			}
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			key := args[0]
			if wantUnassign {
				if err := client.AssignIssue(context.Background(), key, ""); err != nil {
					return err
				}
				return writeJSON(cmd, map[string]any{"key": key, "assignee": nil, "unassigned": true})
			}
			accountID, err := client.ResolveUserAccountID(context.Background(), assignee)
			if err != nil {
				return err
			}
			if err := client.AssignIssue(context.Background(), key, accountID); err != nil {
				return err
			}
			return writeJSON(cmd, map[string]any{"key": key, "assignee": accountID, "unassigned": false})
		},
	}
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee reference: me, an email, or an accountId")
	cmd.Flags().BoolVar(&unassign, "unassign", false, "Remove the current assignee")
	return cmd
}

func newIssuesCreateMetaCommand() *cobra.Command {
	var project string
	var issueType string

	cmd := &cobra.Command{
		Use:     "create-meta",
		Short:   "Show creatable issue types (or, with --type, createable fields) for a project",
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, profile, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			project, err = requiredProjectArg(project, profile)
			if err != nil {
				return err
			}
			ctx := context.Background()

			if issueType == "" {
				types, err := client.CreateMetaIssueTypes(ctx, project)
				if err != nil {
					return err
				}
				if outputJSON() {
					return writeJSON(cmd, types)
				}
				rows := make([][]string, 0, len(types))
				for _, item := range types {
					rows = append(rows, []string{mapString(item, "id"), mapString(item, "name"), mapYesNo(item, "subtask")})
				}
				return output.RenderTable(cmd.OutOrStdout(), []string{"ID", "Name", "Subtask"}, rows)
			}

			typeID, err := client.ResolveIssueTypeID(ctx, project, issueType)
			if err != nil {
				return err
			}
			fields, err := client.CreateMetaFields(ctx, project, typeID)
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, fields)
			}
			rows := make([][]string, 0, len(fields))
			for _, field := range fields {
				rows = append(rows, []string{mapString(field, "fieldId"), mapString(field, "name"), mapYesNo(field, "required"), mapSchemaType(field), mapArrayLen(field, "allowedValues")})
			}
			return output.RenderTable(cmd.OutOrStdout(), []string{"FieldID", "Name", "Required", "Type", "Allowed"}, rows)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project key or ID")
	cmd.Flags().StringVar(&issueType, "type", "", "Issue type name or ID; omit to list the project's issue types")
	return cmd
}

func newIssuesEditMetaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edit-meta <issue-key>",
		Short:   "Show editable fields and allowed values for an issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			fields, err := client.EditMeta(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, fields)
			}
			ids := make([]string, 0, len(fields))
			for id := range fields {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			rows := make([][]string, 0, len(fields))
			for _, id := range ids {
				meta, _ := fields[id].(map[string]any)
				rows = append(rows, []string{id, mapString(meta, "name"), mapYesNo(meta, "required"), mapSchemaType(meta), mapArrayLen(meta, "allowedValues")})
			}
			return output.RenderTable(cmd.OutOrStdout(), []string{"FieldID", "Name", "Required", "Type", "Allowed"}, rows)
		},
	}
	return cmd
}
