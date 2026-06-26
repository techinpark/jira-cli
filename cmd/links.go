package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/techinpark/jira-cli/internal/jira"
	"github.com/techinpark/jira-cli/internal/output"
)

func newLinksCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "links",
		Short: "Manage issue links",
	}
	cmd.AddCommand(newLinksTypesCommand())
	cmd.AddCommand(newLinksAddCommand())
	cmd.AddCommand(newLinksListCommand())
	cmd.AddCommand(newLinksDeleteCommand())
	return cmd
}

func newLinksTypesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "types",
		Short:   "List available issue link types",
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			types, err := client.ListIssueLinkTypes(context.Background())
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, types)
			}
			rows := make([][]string, 0, len(types))
			for _, t := range types {
				rows = append(rows, []string{t.ID, t.Name, t.Inward, t.Outward})
			}
			return output.RenderTable(cmd.OutOrStdout(), []string{"ID", "Name", "Inward", "Outward"}, rows)
		},
	}
	return cmd
}

func newLinksAddCommand() *cobra.Command {
	var inward, outward, linkType, comment string

	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Link two issues (outward <type> inward, e.g. outward blocks inward)",
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			if err := client.LinkIssues(context.Background(), jira.LinkIssuesInput{
				Type:    linkType,
				Inward:  inward,
				Outward: outward,
				Comment: comment,
			}); err != nil {
				return err
			}
			return writeJSON(cmd, map[string]any{
				"linked":        true,
				"type":          linkType,
				"inward_issue":  inward,
				"outward_issue": outward,
			})
		},
	}
	cmd.Flags().StringVar(&outward, "outward", "", "Outward issue key or ID (the side described by the link type's outward label)")
	cmd.Flags().StringVar(&inward, "inward", "", "Inward issue key or ID (the side described by the link type's inward label)")
	cmd.Flags().StringVar(&linkType, "type", "", "Link type name or ID (see 'links types')")
	cmd.Flags().StringVar(&comment, "comment", "", "Optional comment to add with the link")
	_ = cmd.MarkFlagRequired("outward")
	_ = cmd.MarkFlagRequired("inward")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

func newLinksListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <issue-key>",
		Short:   "List the links on an issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			links, err := client.ListIssueLinks(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, links)
			}
			rows := make([][]string, 0, len(links))
			for _, link := range links {
				linkType, _ := link["type"].(map[string]any)
				direction, other := "", ""
				if iw, ok := link["inwardIssue"].(map[string]any); ok {
					direction = mapString(linkType, "inward")
					other = mapString(iw, "key")
				} else if ow, ok := link["outwardIssue"].(map[string]any); ok {
					direction = mapString(linkType, "outward")
					other = mapString(ow, "key")
				}
				rows = append(rows, []string{mapString(link, "id"), mapString(linkType, "name"), direction, other})
			}
			return output.RenderTable(cmd.OutOrStdout(), []string{"LinkID", "Type", "Direction", "Issue"}, rows)
		},
	}
	return cmd
}

func newLinksDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <link-id>",
		Short:   "Delete an issue link",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			if err := client.DeleteIssueLink(context.Background(), args[0]); err != nil {
				return err
			}
			return writeJSON(cmd, map[string]any{"deleted": true, "id": args[0]})
		},
	}
	return cmd
}
