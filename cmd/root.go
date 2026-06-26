package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/techinpark/jira-cli/internal/config"
	"github.com/techinpark/jira-cli/internal/httpx"
	"github.com/techinpark/jira-cli/internal/jira"
)

type rootOptions struct {
	output         string
	jsonOutput     bool
	profile        string
	siteURL        string
	email          string
	apiToken       string
	defaultProject string
}

var opts rootOptions

var rootCmd = &cobra.Command{
	Use:           "jira",
	Short:         "Jira CLI for Jira Cloud REST API v3",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&opts.output, "output", "table", "Output format: table or json")
	rootCmd.PersistentFlags().BoolVar(&opts.jsonOutput, "json", false, "Shortcut for --output json")
	rootCmd.PersistentFlags().StringVar(&opts.profile, "profile", "", "Jira profile name")
	rootCmd.PersistentFlags().StringVar(&opts.siteURL, "site-url", "", "Jira site URL, e.g. https://example.atlassian.net")
	rootCmd.PersistentFlags().StringVar(&opts.email, "email", "", "Jira account email")
	rootCmd.PersistentFlags().StringVar(&opts.apiToken, "api-token", "", "Jira API token")
	rootCmd.PersistentFlags().StringVar(&opts.defaultProject, "default-project", "", "Default Jira project key")

	rootCmd.AddCommand(newAuthCommand())
	rootCmd.AddCommand(newProjectsCommand())
	rootCmd.AddCommand(newIssuesCommand())
	rootCmd.AddCommand(newCommentsCommand())
	rootCmd.AddCommand(newTransitionsCommand())
	rootCmd.AddCommand(newWorklogsCommand())
	rootCmd.AddCommand(newUsersCommand())
	rootCmd.AddCommand(newAttachmentsCommand())
	rootCmd.AddCommand(newFieldsCommand())
	rootCmd.AddCommand(newRawCommand())
}

func outputJSON() bool {
	return opts.jsonOutput || opts.output == "json"
}

func validateOutputFlag(cmd *cobra.Command, _ []string) error {
	if opts.output != "table" && opts.output != "json" {
		return fmt.Errorf("invalid --output %q: must be table or json", opts.output)
	}
	return nil
}

func loadConfig() (config.Config, error) {
	return config.Load()
}

func resolveProfile(cfg config.Config) (config.ResolvedProfile, error) {
	return cfg.Resolve(config.ResolveOptions{
		ProfileName:    opts.profile,
		SiteURL:        opts.siteURL,
		Email:          opts.email,
		APIToken:       opts.apiToken,
		DefaultProject: opts.defaultProject,
	})
}

func newJiraClient(ctx context.Context) (*jira.Client, config.ResolvedProfile, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, config.ResolvedProfile{}, err
	}
	profile, err := resolveProfile(cfg)
	if err != nil {
		return nil, config.ResolvedProfile{}, err
	}
	client := jira.NewClient(httpx.New(httpx.Options{
		Profile:    profile,
		Timeout:    45 * time.Second,
		MaxRetries: 2,
	}))
	return client, profile, nil
}

func requiredProjectArg(project string, profile config.ResolvedProfile) (string, error) {
	if strings.TrimSpace(project) != "" {
		return project, nil
	}
	if profile.DefaultProject != "" {
		return profile.DefaultProject, nil
	}
	return "", errors.New("missing project key: provide --project or configure default_project on the active profile")
}

func parseFieldAssignments(values []string) (map[string]any, error) {
	fields := map[string]any{}
	for _, item := range values {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --field value %q, expected key=value", item)
		}
		fields[key] = jira.ParseFieldValue(value)
	}
	return fields, nil
}

func writeJSON(cmd *cobra.Command, value any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

// mapString reads a string field from a decoded JSON object.
func mapString(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

// mapYesNo renders a boolean JSON field as yes/no for table output.
func mapYesNo(m map[string]any, key string) string {
	if b, ok := m[key].(bool); ok && b {
		return "yes"
	}
	return "no"
}

// mapSchemaType reads the nested schema.type of a Jira field metadata object.
func mapSchemaType(m map[string]any) string {
	if schema, ok := m["schema"].(map[string]any); ok {
		if t, ok := schema["type"].(string); ok {
			return t
		}
	}
	return ""
}
