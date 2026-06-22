package cmd

import "github.com/spf13/cobra"

// newUsersCommand is a stub filled in by the users feature implementation.
func newUsersCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "users",
		Short: "Look up Jira users",
	}
}
