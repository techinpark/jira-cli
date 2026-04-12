package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/techinpark/jira-cli/internal/jira"
	"github.com/techinpark/jira-cli/internal/output"
)

func newTransitionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transitions",
		Short: "Inspect and move issue transitions",
	}
	cmd.AddCommand(newTransitionsListCommand())
	cmd.AddCommand(newTransitionsMoveCommand())
	return cmd
}

func newTransitionsListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <issue-key>",
		Short:   "List available transitions for an issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			transitions, err := client.ListTransitions(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, transitions)
			}
			return output.RenderTransitionsTable(cmd.OutOrStdout(), transitions)
		},
	}
	return cmd
}

func newTransitionsMoveCommand() *cobra.Command {
	var transition string
	var comment string
	var fields []string

	cmd := &cobra.Command{
		Use:     "move <issue-key>",
		Short:   "Transition an issue by transition ID or name",
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
			if err := client.MoveIssue(context.Background(), args[0], jira.MoveIssueInput{
				Transition: transition,
				Comment:    comment,
				Fields:     extraFields,
			}); err != nil {
				return err
			}
			return writeJSON(cmd, map[string]any{"moved": true, "issue": args[0], "transition": transition})
		},
	}
	cmd.Flags().StringVar(&transition, "transition", "", "Transition ID or name")
	cmd.Flags().StringVar(&comment, "comment", "", "Optional comment to add during the transition")
	cmd.Flags().StringArrayVar(&fields, "field", nil, "Additional field in key=value form; JSON values are allowed")
	_ = cmd.MarkFlagRequired("transition")
	return cmd
}
