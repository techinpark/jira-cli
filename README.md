# Jira CLI for Jira Cloud REST API v3

`jira-cli` is an open source Go CLI for Jira Cloud REST API v3. It helps developers and LLM agents manage Jira issues, comments, transitions, worklogs, projects, and JQL search from the terminal.

## Features

- Easy-to-use Jira Cloud CLI with stable `--json` output
- Multi-account profile support for multiple Jira sites and teams
- Issue-focused workflows: `get`, `search`, `create`, `update`, `delete`
- Comment, transition, and worklog commands
- `raw` escape hatch for uncovered Jira Platform v3 endpoints
- LLM-friendly behavior with predictable machine-readable results

## Scope

- Jira Cloud only
- Compatible with Jira Cloud Platform REST API v3
- The bundled spec source is the Atlassian Platform v3 swagger
- Jira Software Agile board and sprint APIs are out of scope for this repository unless added from a separate spec later

## Install

### Go

```bash
go install github.com/techinpark/jira-cli@latest
```

### Homebrew

```bash
brew tap techinpark/tap
brew install jira-cli
```

## Authentication

`jira-cli` uses Jira Cloud `email + API token` authentication and supports multiple named profiles.

Environment variables:

- `JIRA_SITE_URL`
- `JIRA_EMAIL`
- `JIRA_API_TOKEN`
- `JIRA_PROFILE`
- `JIRA_DEFAULT_PROJECT`

Create a local profile:

```bash
jira auth init --profile work \
  --site-url https://your-team.atlassian.net \
  --email you@example.com \
  --api-token $JIRA_API_TOKEN \
  --default-project ENG
```

Switch profiles:

```bash
jira auth list
jira auth use work
jira auth check --profile work
```

Config file location:

```text
~/.config/jira-cli/config.yaml
```

## Usage

```bash
jira projects list --profile work
jira issues get ENG-123
jira issues search --jql 'project = ENG AND status != Done' --json
jira issues create --project ENG --type Bug --summary 'Crash on launch'
jira comments add ENG-123 --body 'Investigating now'
jira transitions move ENG-123 --transition Done --comment 'Fixed in latest build'
jira worklogs add ENG-123 --time-spent '1h 30m' --comment 'Root cause analysis'
```

Additional fields can be passed as `--field key=value`. JSON values are supported:

```bash
jira issues create \
  --project ENG \
  --type Task \
  --summary 'Prepare release notes' \
  --field labels='["release","docs"]'
```

## Raw API Calls

Use `raw` when the direct command set does not cover an endpoint yet.

```bash
jira raw GET /rest/api/3/project/search --query maxResults=10
jira raw POST /rest/api/3/search/jql --body '{"jql":"project = ENG","maxResults":20}'
```

## LLM Guide

For LLM agents and automation:

- Prefer `--json`
- Prefer direct commands for issue workflows
- Use `jira raw` for uncovered Platform v3 endpoints
- Treat write commands as side-effectful and require explicit inputs
- Use named profiles when multiple Jira sites or accounts are involved

More details: [LLM_GUIDE.md](./LLM_GUIDE.md)

## Security

- API tokens may be stored in the local config for convenience
- Use local file permissions and OS account security
- For CI, prefer environment variables over checked-in config
- Never commit real Jira credentials

See [SECURITY.md](./SECURITY.md).

## Development

```bash
go test ./...
go test ./... -cover
go build ./...
```

## Open Source

- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [CHANGELOG.md](./CHANGELOG.md)
- [LICENSE](./LICENSE)
