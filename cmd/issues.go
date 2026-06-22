package cmd

import (
	"context"
	"fmt"

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

	cmd := &cobra.Command{
		Use:     "search",
		Short:   "Search Jira issues with JQL",
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			result, err := client.SearchIssues(context.Background(), jira.SearchOptions{
				JQL:    jql,
				Fields: fields,
				Limit:  limit,
			})
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, result)
			}
			return output.RenderIssuesTable(cmd.OutOrStdout(), result.Issues)
		},
	}
	cmd.Flags().StringVar(&jql, "jql", "", "JQL expression")
	cmd.Flags().StringSliceVar(&fields, "fields", []string{"summary", "status", "issuetype", "assignee"}, "Fields to return")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of issues")
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
			issue, err := client.CreateIssue(context.Background(), jira.CreateIssueInput{
				Project:     project,
				IssueType:   issueType,
				Summary:     summary,
				Description: description,
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
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("summary")
	return cmd
}

func newIssuesUpdateCommand() *cobra.Command {
	var summary string
	var description string
	var setDescription bool
	var fields []string

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
			if err := client.UpdateIssue(context.Background(), args[0], jira.UpdateIssueInput{
				Summary:     summary,
				Description: descPtr,
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
