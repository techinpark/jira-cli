package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/techinpark/jira-cli/internal/output"
)

func newUsersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Look up Jira users",
	}
	cmd.AddCommand(newUsersSearchCommand())
	return cmd
}

func newUsersSearchCommand() *cobra.Command {
	var query string
	var limit int

	cmd := &cobra.Command{
		Use:     "search",
		Short:   "Search Jira users by display name or email",
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			users, err := client.SearchUsers(context.Background(), query, limit)
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, users)
			}
			rows := make([][]string, 0, len(users))
			for _, user := range users {
				active := "no"
				if user.Active {
					active = "yes"
				}
				rows = append(rows, []string{user.AccountID, user.DisplayName, user.Email, active})
			}
			return output.RenderTable(cmd.OutOrStdout(), []string{"AccountID", "DisplayName", "Email", "Active"}, rows)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Display name or email to search for")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of users")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}
