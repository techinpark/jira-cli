package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/techinpark/jira-cli/internal/output"
)

func newCommentsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comments",
		Short: "Manage issue comments",
	}
	cmd.AddCommand(newCommentsListCommand())
	cmd.AddCommand(newCommentsAddCommand())
	cmd.AddCommand(newCommentsUpdateCommand())
	cmd.AddCommand(newCommentsDeleteCommand())
	return cmd
}

func newCommentsListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <issue-key>",
		Short:   "List comments on an issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			comments, err := client.ListComments(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, comments)
			}
			return output.RenderCommentsTable(cmd.OutOrStdout(), comments)
		},
	}
	return cmd
}

func newCommentsAddCommand() *cobra.Command {
	var body string
	cmd := &cobra.Command{
		Use:     "add <issue-key>",
		Short:   "Add a comment to an issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			comment, err := client.AddComment(context.Background(), args[0], body)
			if err != nil {
				return err
			}
			return writeJSON(cmd, comment)
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "Plain-text comment body")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

func newCommentsUpdateCommand() *cobra.Command {
	var body string
	cmd := &cobra.Command{
		Use:     "update <issue-key> <comment-id>",
		Short:   "Update an issue comment",
		Args:    cobra.ExactArgs(2),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			comment, err := client.UpdateComment(context.Background(), args[0], args[1], body)
			if err != nil {
				return err
			}
			return writeJSON(cmd, comment)
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "Plain-text comment body")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

func newCommentsDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <issue-key> <comment-id>",
		Short:   "Delete an issue comment",
		Args:    cobra.ExactArgs(2),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			if err := client.DeleteComment(context.Background(), args[0], args[1]); err != nil {
				return err
			}
			return writeJSON(cmd, map[string]any{"deleted": true, "issue": args[0], "comment_id": args[1]})
		},
	}
	return cmd
}
