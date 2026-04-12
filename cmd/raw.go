package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newRawCommand() *cobra.Command {
	var body string
	var bodyFile string
	var query []string

	cmd := &cobra.Command{
		Use:     "raw <method> <path>",
		Short:   "Call any Jira Cloud Platform REST API endpoint directly",
		Args:    cobra.ExactArgs(2),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			values := url.Values{}
			for _, item := range query {
				key, value, ok := strings.Cut(item, "=")
				if !ok {
					return fmt.Errorf("invalid --query value %q, expected key=value", item)
				}
				values.Add(key, value)
			}
			requestBody, err := rawBody(body, bodyFile)
			if err != nil {
				return err
			}
			result, err := client.Raw(context.Background(), strings.ToUpper(args[0]), args[1], values, requestBody)
			if err != nil {
				return err
			}
			return writeJSON(cmd, result)
		},
	}

	cmd.Flags().StringVar(&body, "body", "", "Inline JSON request body")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Path to a JSON request body file")
	cmd.Flags().StringArrayVar(&query, "query", nil, "Query parameter in key=value form")
	return cmd
}

func rawBody(inline, filePath string) (any, error) {
	if inline == "" && filePath == "" {
		return nil, nil
	}
	var data []byte
	var err error
	if filePath != "" {
		data, err = os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
	} else {
		data = []byte(inline)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %w", err)
	}
	return out, nil
}
