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
brew install techinpark/tap/jira-cloud-cli
```

Homebrew installs the binary as `jira-cloud-cli` to avoid name collisions with an existing Homebrew `jira-cli` formula.

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
jira issues assign ENG-123 --assignee me
jira comments add ENG-123 --body 'Investigating now'
jira transitions move ENG-123 --transition Done --comment 'Fixed in latest build'
jira worklogs add ENG-123 --time-spent '1h 30m' --comment 'Root cause analysis'
jira users search --query 'jane@example.com'
jira auth whoami
```

### Assignees and convenience fields

User references accept `me`, an email, or an `accountId` (Jira Cloud requires an
`accountId`, which `jira-cli` resolves for you via `users search`):

```bash
jira issues assign ENG-123 --assignee jane@example.com
jira issues assign ENG-123 --unassign
jira issues create --project ENG --type Task --summary 'Release notes' \
  --assignee me --labels release,docs --priority High --due 2026-07-01
jira issues update ENG-123 --assignee me --priority Highest
```

Additional fields can be passed as `--field key=value`. JSON values are supported:

```bash
jira issues create \
  --project ENG \
  --type Task \
  --summary 'Prepare release notes' \
  --field labels='["release","docs"]'
```

## Attachments

Jira Cloud cannot embed file attachments in the create-issue call, so `jira-cli`
creates the issue first and then uploads attachments to it. Use `--attach`
(repeatable) on `issues create` to do both in one command:

```bash
jira issues create \
  --project ENG \
  --type Bug \
  --summary 'Crash on launch' \
  --attach ./crash.log \
  --attach ./screenshot.png
```

Attach files to an existing issue with `issues attach`:

```bash
jira issues attach ENG-123 --file ./crash.log --file ./screenshot.png
```

List, download, and delete attachments:

```bash
jira attachments list ENG-123
jira attachments download 10042 --output ./crash.log
jira attachments delete 10042
```

Both commands return the uploaded attachment metadata as JSON. Jira allows at
most 60 files per request and enforces the site's configured maximum attachment
size.

Because Jira creates the issue before attachments are uploaded, `issues create
--attach` can partially succeed: if an upload fails, the command still prints the
created issue (with its `key`) as JSON and then exits non-zero, so you can retry
with `jira issues attach <key> --file ...`.

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

## Release Automation

Tag pushes matching `v*` run GoReleaser through GitHub Actions.

Required repository secret:

- `TAP_GITHUB_TOKEN`: a GitHub personal access token with permission to create releases in `techinpark/jira-cli` and push formula updates to `techinpark/homebrew-tap`

Example:

```bash
git tag v0.1.1
git push origin v0.1.1
```

## Open Source

- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [CHANGELOG.md](./CHANGELOG.md)
- [LICENSE](./LICENSE)
