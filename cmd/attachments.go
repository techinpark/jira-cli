package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	var outPath string
	cmd := &cobra.Command{
		Use:     "download <attachment-id>",
		Short:   "Download an attachment",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			client, _, err := newJiraClient(ctx)
			if err != nil {
				return err
			}

			path := outPath
			if path == "" {
				meta, err := client.AttachmentMeta(ctx, args[0])
				if err != nil {
					return err
				}
				// Filename comes from the server; never let it escape the cwd.
				name := filepath.Base(meta.Filename)
				if name == "." || name == ".." || name == string(os.PathSeparator) || strings.TrimSpace(name) == "" {
					return fmt.Errorf("attachment has an unsafe filename %q; pass --out to choose a destination", meta.Filename)
				}
				path = name
			}

			// Download atomically: stream into a temp file in the destination
			// directory and rename on success, so a failed download never
			// clobbers an existing file or leaves a truncated one behind.
			tmp, err := os.CreateTemp(filepath.Dir(path), ".jira-download-*")
			if err != nil {
				return err
			}
			tmpName := tmp.Name()
			defer os.Remove(tmpName)

			if err := client.DownloadAttachmentContent(ctx, args[0], tmp); err != nil {
				tmp.Close()
				return err
			}
			if err := tmp.Close(); err != nil {
				return err
			}
			if err := os.Rename(tmpName, path); err != nil {
				return err
			}
			return writeJSON(cmd, map[string]any{"id": args[0], "filename": filepath.Base(path), "path": path})
		},
	}
	cmd.Flags().StringVarP(&outPath, "out", "o", "", "Destination file path (defaults to the attachment filename)")
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
