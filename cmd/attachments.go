package cmd

import (
	"context"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/techinpark/jira-cli/internal/output"
)

func newAttachmentsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachments",
		Short: "Manage Jira issue attachments",
	}
	cmd.AddCommand(newAttachmentsListCommand())
	cmd.AddCommand(newAttachmentsDownloadCommand())
	cmd.AddCommand(newAttachmentsDeleteCommand())
	return cmd
}

func newAttachmentsListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <issue-key>",
		Short:   "List attachments for an issue",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			attachments, err := client.ListAttachments(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, attachments)
			}
			headers := []string{"ID", "Filename", "Size", "MimeType", "Created"}
			rows := make([][]string, 0, len(attachments))
			for _, a := range attachments {
				rows = append(rows, []string{a.ID, a.Filename, strconv.FormatInt(a.Size, 10), a.MimeType, a.Created})
			}
			return output.RenderTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	return cmd
}

func newAttachmentsDownloadCommand() *cobra.Command {
	var outputPath string
	cmd := &cobra.Command{
		Use:     "download <attachment-id>",
		Short:   "Download an attachment",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}

			path := outputPath
			if path == "" {
				meta, err := client.AttachmentMeta(context.Background(), args[0])
				if err != nil {
					return err
				}
				path = meta.Filename
			}

			file, err := os.Create(path)
			if err != nil {
				return err
			}
			defer file.Close()

			filename, err := client.DownloadAttachment(context.Background(), args[0], file)
			if err != nil {
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
			return writeJSON(cmd, map[string]any{"id": args[0], "filename": filename, "path": path})
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Destination file path (defaults to the attachment filename)")
	return cmd
}

func newAttachmentsDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <attachment-id>",
		Short:   "Delete an attachment",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			if err := client.DeleteAttachment(context.Background(), args[0]); err != nil {
				return err
			}
			return writeJSON(cmd, map[string]any{"deleted": true, "id": args[0]})
		},
	}
	return cmd
}
