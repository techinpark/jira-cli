package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/techinpark/jira-cli/internal/jira"
	"github.com/techinpark/jira-cli/internal/output"
)

func newWorklogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worklogs",
		Short: "Manage issue worklogs",
	}
	cmd.AddCommand(newWorklogsListCommand())
	cmd.AddCommand(newWorklogsAddCommand())
	cmd.AddCommand(newWorklogsUpdateCommand())
	cmd.AddCommand(newWorklogsDeleteCommand())
	return cmd
}

func newWorklogsListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <issue-key>",
		Short:   "List worklogs for an issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			worklogs, err := client.ListWorklogs(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, worklogs)
			}
			return output.RenderWorklogsTable(cmd.OutOrStdout(), worklogs)
		},
	}
	return cmd
}

func newWorklogsAddCommand() *cobra.Command {
	var timeSpent string
	var started string
	var comment string
	cmd := &cobra.Command{
		Use:     "add <issue-key>",
		Short:   "Add a worklog to an issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			worklog, err := client.AddWorklog(context.Background(), args[0], jira.WorklogInput{
				TimeSpent: timeSpent,
				Started:   started,
				Comment:   comment,
			})
			if err != nil {
				return err
			}
			return writeJSON(cmd, worklog)
		},
	}
	cmd.Flags().StringVar(&timeSpent, "time-spent", "", "Time spent, e.g. 1h 30m")
	cmd.Flags().StringVar(&started, "started", "", "Started timestamp, e.g. 2026-04-13T09:00:00.000+0900")
	cmd.Flags().StringVar(&comment, "comment", "", "Plain-text worklog comment")
	_ = cmd.MarkFlagRequired("time-spent")
	return cmd
}

func newWorklogsUpdateCommand() *cobra.Command {
	var timeSpent string
	var started string
	var comment string
	cmd := &cobra.Command{
		Use:     "update <issue-key> <worklog-id>",
		Short:   "Update an issue worklog",
		Args:    cobra.ExactArgs(2),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			worklog, err := client.UpdateWorklog(context.Background(), args[0], args[1], jira.WorklogInput{
				TimeSpent: timeSpent,
				Started:   started,
				Comment:   comment,
			})
			if err != nil {
				return err
			}
			return writeJSON(cmd, worklog)
		},
	}
	cmd.Flags().StringVar(&timeSpent, "time-spent", "", "Time spent, e.g. 1h 30m")
	cmd.Flags().StringVar(&started, "started", "", "Started timestamp, e.g. 2026-04-13T09:00:00.000+0900")
	cmd.Flags().StringVar(&comment, "comment", "", "Plain-text worklog comment")
	return cmd
}

func newWorklogsDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <issue-key> <worklog-id>",
		Short:   "Delete an issue worklog",
		Args:    cobra.ExactArgs(2),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			if err := client.DeleteWorklog(context.Background(), args[0], args[1]); err != nil {
				return err
			}
			return writeJSON(cmd, map[string]any{"deleted": true, "issue": args[0], "worklog_id": args[1]})
		},
	}
	return cmd
}
