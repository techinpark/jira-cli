# Security Policy

## Supported Scope

This project targets Jira Cloud REST API v3.

## Reporting

If you find a security issue, report it privately before opening a public issue.

## Secrets

- Do not commit API tokens
- Prefer environment variables in CI
- Treat `~/.config/jira-cli/config.yaml` as sensitive local configuration

