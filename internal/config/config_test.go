package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadAndResolveProfiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg := Config{
		CurrentProfile: "work",
		Profiles: map[string]Profile{
			"work": {
				SiteURL:        "https://work.atlassian.net",
				Email:          "work@example.com",
				APIToken:       "work-token",
				DefaultProject: "WORK",
			},
			"side": {
				SiteURL:        "https://side.atlassian.net",
				Email:          "side@example.com",
				APIToken:       "side-token",
				DefaultProject: "SIDE",
			},
		},
	}
	path, err := Save(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	profile, err := loaded.Resolve(ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Name != "work" || profile.DefaultProject != "WORK" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
}

func TestResolveUsesEnvAndFlags(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("JIRA_PROFILE", "side")
	t.Setenv("JIRA_EMAIL", "env@example.com")

	cfg := Config{
		CurrentProfile: "work",
		Profiles: map[string]Profile{
			"side": {
				SiteURL:  "https://side.atlassian.net",
				Email:    "side@example.com",
				APIToken: "side-token",
			},
		},
	}

	profile, err := cfg.Resolve(ResolveOptions{
		SiteURL:  "https://override.atlassian.net",
		APIToken: "flag-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Name != "side" || profile.SiteURL != "https://override.atlassian.net" || profile.Email != "env@example.com" || profile.APIToken != "flag-token" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
}

func TestValidateCredentialsFailures(t *testing.T) {
	tests := []ResolvedProfile{
		{},
		{SiteURL: "https://jira.example.com"},
		{SiteURL: "https://jira.example.com", Email: "user@example.com"},
		{SiteURL: "not-a-url", Email: "user@example.com", APIToken: "token"},
	}
	for _, item := range tests {
		if err := item.ValidateCredentials(); err == nil {
			t.Fatalf("expected validation error for %+v", item)
		}
	}
}

func TestRemoveProfilePromotesAnotherCurrentProfile(t *testing.T) {
	cfg := Config{
		CurrentProfile: "work",
		Profiles: map[string]Profile{
			"work": {},
			"side": {},
		},
	}
	cfg.RemoveProfile("work")
	if cfg.CurrentProfile != "side" {
		t.Fatalf("expected side to become current, got %q", cfg.CurrentProfile)
	}
}

func TestPathUsesUserConfigDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "config.yaml" || filepath.Base(filepath.Dir(path)) != "jira-cli" {
		t.Fatalf("unexpected path: %s", path)
	}
}
