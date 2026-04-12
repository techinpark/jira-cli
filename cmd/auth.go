package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/techinpark/jira-cli/internal/config"
	"github.com/techinpark/jira-cli/internal/output"
)

func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Jira authentication profiles",
	}
	cmd.AddCommand(newAuthInitCommand())
	cmd.AddCommand(newAuthListCommand())
	cmd.AddCommand(newAuthUseCommand())
	cmd.AddCommand(newAuthCheckCommand())
	cmd.AddCommand(newAuthRemoveCommand())
	return cmd
}

func newAuthInitCommand() *cobra.Command {
	var profileName string
	var siteURL string
	var email string
	var apiToken string
	var defaultProject string

	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Create or update a local auth profile",
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileName == "" {
				profileName = "default"
			}
			if siteURL == "" {
				value, err := readLine("Site URL")
				if err != nil {
					return err
				}
				siteURL = value
			}
			if email == "" {
				value, err := readLine("Email")
				if err != nil {
					return err
				}
				email = value
			}
			if apiToken == "" {
				value, err := readLine("API token")
				if err != nil {
					return err
				}
				apiToken = value
			}
			if defaultProject == "" {
				value, err := readLine("Default project (optional)")
				if err == nil {
					defaultProject = value
				}
			}

			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			profile := config.Profile{
				SiteURL:        siteURL,
				Email:          email,
				APIToken:       apiToken,
				DefaultProject: defaultProject,
			}
			if err := (config.ResolvedProfile{
				Name:           profileName,
				SiteURL:        profile.SiteURL,
				Email:          profile.Email,
				APIToken:       profile.APIToken,
				DefaultProject: profile.DefaultProject,
			}).ValidateCredentials(); err != nil {
				return err
			}

			cfg.UpsertProfile(profileName, profile)
			cfg.CurrentProfile = profileName
			path, err := config.Save(cfg)
			if err != nil {
				return err
			}

			if outputJSON() {
				return writeJSON(cmd, map[string]any{
					"config_path":     path,
					"profile":         profileName,
					"site_url":        siteURL,
					"email":           email,
					"default_project": defaultProject,
					"current_profile": cfg.CurrentProfile,
					"profiles_count":  len(cfg.Profiles),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved profile %q to %s\n", profileName, path)
			return nil
		},
	}

	cmd.Flags().StringVar(&profileName, "profile", "default", "Profile name")
	cmd.Flags().StringVar(&siteURL, "site-url", "", "Jira site URL")
	cmd.Flags().StringVar(&email, "email", "", "Jira account email")
	cmd.Flags().StringVar(&apiToken, "api-token", "", "Jira API token")
	cmd.Flags().StringVar(&defaultProject, "default-project", "", "Default Jira project key")
	return cmd
}

func newAuthListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List saved auth profiles",
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			type item struct {
				Name           string `json:"name"`
				SiteURL        string `json:"site_url"`
				Email          string `json:"email"`
				DefaultProject string `json:"default_project,omitempty"`
				Current        bool   `json:"current"`
			}
			items := make([]item, 0, len(cfg.Profiles))
			for _, name := range cfg.ProfileNames() {
				profile := cfg.Profiles[name]
				items = append(items, item{
					Name:           name,
					SiteURL:        profile.SiteURL,
					Email:          profile.Email,
					DefaultProject: profile.DefaultProject,
					Current:        cfg.CurrentProfile == name,
				})
			}
			if outputJSON() {
				return writeJSON(cmd, items)
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				current := ""
				if item.Current {
					current = "*"
				}
				rows = append(rows, []string{current, item.Name, item.SiteURL, item.Email, item.DefaultProject})
			}
			return output.RenderTable(cmd.OutOrStdout(), []string{"Current", "Profile", "Site URL", "Email", "Default Project"}, rows)
		},
	}
	return cmd
}

func newAuthUseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "use <profile>",
		Short:   "Switch the active auth profile",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if _, ok := cfg.Profiles[args[0]]; !ok {
				return fmt.Errorf("profile %q not found", args[0])
			}
			cfg.CurrentProfile = args[0]
			if _, err := config.Save(cfg); err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, map[string]any{"current_profile": args[0]})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Current profile: %s\n", args[0])
			return nil
		},
	}
	return cmd
}

func newAuthCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "check",
		Short:   "Validate the active profile against Jira Cloud",
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, profile, err := newJiraClient(context.Background())
			if err != nil {
				return err
			}
			self, err := client.CheckAuth(context.Background())
			if err != nil {
				return err
			}
			result := map[string]any{
				"ok":              true,
				"profile":         profile.Name,
				"site_url":        profile.SiteURL,
				"email":           profile.Email,
				"default_project": profile.DefaultProject,
				"self":            self,
			}
			if outputJSON() {
				return writeJSON(cmd, result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Auth OK for %s (%s)\n", profile.Email, profile.SiteURL)
			return nil
		},
	}
	return cmd
}

func newAuthRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <profile>",
		Short:   "Remove a saved auth profile",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateOutputFlag,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if _, ok := cfg.Profiles[args[0]]; !ok {
				return fmt.Errorf("profile %q not found", args[0])
			}
			cfg.RemoveProfile(args[0])
			if _, err := config.Save(cfg); err != nil {
				return err
			}
			if outputJSON() {
				return writeJSON(cmd, map[string]any{"removed_profile": args[0], "current_profile": cfg.CurrentProfile})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed profile: %s\n", args[0])
			return nil
		},
	}
	return cmd
}

func readLine(label string) (string, error) {
	fmt.Fprintf(os.Stdout, "%s: ", label)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(input), nil
}
