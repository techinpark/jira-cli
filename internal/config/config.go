package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	CurrentProfile string             `yaml:"current_profile,omitempty" json:"current_profile,omitempty"`
	Profiles       map[string]Profile `yaml:"profiles,omitempty" json:"profiles,omitempty"`
}

type Profile struct {
	SiteURL        string `yaml:"site_url" json:"site_url"`
	Email          string `yaml:"email" json:"email"`
	APIToken       string `yaml:"api_token" json:"api_token"`
	DefaultProject string `yaml:"default_project,omitempty" json:"default_project,omitempty"`
}

type ResolvedProfile struct {
	Name           string `json:"name,omitempty"`
	SiteURL        string `json:"site_url"`
	Email          string `json:"email"`
	APIToken       string `json:"api_token"`
	DefaultProject string `json:"default_project,omitempty"`
}

type ResolveOptions struct {
	ProfileName    string
	SiteURL        string
	Email          string
	APIToken       string
	DefaultProject string
}

func Load() (Config, error) {
	cfg := Config{Profiles: map[string]Profile{}}
	path, err := Path()
	if err != nil {
		return cfg, err
	}

	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return cfg, nil
}

func Save(cfg Config) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "jira-cli", "config.yaml"), nil
}

func (c Config) ProfileNames() []string {
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c Config) Resolve(opts ResolveOptions) (ResolvedProfile, error) {
	selected := strings.TrimSpace(opts.ProfileName)
	if selected == "" {
		selected = strings.TrimSpace(os.Getenv("JIRA_PROFILE"))
	}
	if selected == "" {
		selected = strings.TrimSpace(c.CurrentProfile)
	}

	base := Profile{}
	if selected != "" {
		profile, ok := c.Profiles[selected]
		if !ok {
			return ResolvedProfile{}, fmt.Errorf("profile %q not found", selected)
		}
		base = profile
	}

	if v := os.Getenv("JIRA_SITE_URL"); v != "" {
		base.SiteURL = v
	}
	if v := os.Getenv("JIRA_EMAIL"); v != "" {
		base.Email = v
	}
	if v := os.Getenv("JIRA_API_TOKEN"); v != "" {
		base.APIToken = v
	}
	if v := os.Getenv("JIRA_DEFAULT_PROJECT"); v != "" {
		base.DefaultProject = v
	}

	if opts.SiteURL != "" {
		base.SiteURL = opts.SiteURL
	}
	if opts.Email != "" {
		base.Email = opts.Email
	}
	if opts.APIToken != "" {
		base.APIToken = opts.APIToken
	}
	if opts.DefaultProject != "" {
		base.DefaultProject = opts.DefaultProject
	}

	resolved := ResolvedProfile{
		Name:           selected,
		SiteURL:        strings.TrimRight(strings.TrimSpace(base.SiteURL), "/"),
		Email:          strings.TrimSpace(base.Email),
		APIToken:       strings.TrimSpace(base.APIToken),
		DefaultProject: strings.TrimSpace(base.DefaultProject),
	}
	if err := resolved.ValidateCredentials(); err != nil {
		return ResolvedProfile{}, err
	}
	return resolved, nil
}

func (c *Config) UpsertProfile(name string, profile Profile) {
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	c.Profiles[name] = profile
	if c.CurrentProfile == "" {
		c.CurrentProfile = name
	}
}

func (c *Config) RemoveProfile(name string) {
	delete(c.Profiles, name)
	if c.CurrentProfile == name {
		c.CurrentProfile = ""
		for _, candidate := range c.ProfileNames() {
			c.CurrentProfile = candidate
			break
		}
	}
}

func (r ResolvedProfile) ValidateCredentials() error {
	if r.SiteURL == "" {
		return errors.New("missing Jira site URL: set JIRA_SITE_URL or run `jira auth init --profile <name>`")
	}
	u, err := url.Parse(r.SiteURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid Jira site URL %q", r.SiteURL)
	}
	if r.Email == "" {
		return errors.New("missing Jira email: set JIRA_EMAIL or run `jira auth init --profile <name>`")
	}
	if r.APIToken == "" {
		return errors.New("missing Jira API token: set JIRA_API_TOKEN or run `jira auth init --profile <name>`")
	}
	return nil
}
