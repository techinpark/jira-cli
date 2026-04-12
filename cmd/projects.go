package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/techinpark/jira-cli/internal/jira"
	"github.com/techinpark/jira-cli/internal/output"
)

func newProjectsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Inspect Jira projects",
	}
	cmd.AddCommand(newProjectsListCommand())
	cmd.AddCommand(newProjectsGetCommand())
	return cmd
}

func newProjectsListCommand() *cobra.Command {
	var query string
	var limit int

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List Jira projects",
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			projects, err := client.ListProjects(context.Background(), jira.ListProjectsOptions{Query: query, Limit: limit})
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, projects)
			}
			return output.RenderProjectsTable(cmd.OutOrStdout(), projects)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Filter projects by query text")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of projects")
	return cmd
}

func newProjectsGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <project-key>",
		Short:   "Get a Jira project",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			project, err := client.GetProject(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, project)
			}
			return output.RenderProjectsTable(cmd.OutOrStdout(), []jira.Project{project})
		},
	}
	return cmd
}
