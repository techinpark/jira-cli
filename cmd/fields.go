package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/techinpark/jira-cli/internal/output"
)

func newFieldsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fields",
		Short: "Inspect Jira field definitions",
	}
	cmd.AddCommand(newFieldsListCommand())
	return cmd
}

func newFieldsListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all fields, mapping names to their IDs (e.g. customfield_*)",
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			fields, err := client.ListFields(context.Background())
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, fields)
			}
			rows := make([][]string, 0, len(fields))
			for _, field := range fields {
				rows = append(rows, []string{
					mapString(field, "id"),
					mapString(field, "key"),
					mapString(field, "name"),
					mapYesNo(field, "custom"),
					mapSchemaType(field),
				})
			}
			return output.RenderTable(cmd.OutOrStdout(), []string{"ID", "Key", "Name", "Custom", "Type"}, rows)
		},
	}
	return cmd
}
