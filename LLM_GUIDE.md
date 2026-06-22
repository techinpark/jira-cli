# LLM Guide for `jira-cli`

## Recommended usage

- Prefer `--json` on all read commands
- Prefer direct commands over `raw` when a command exists
- Use `raw` for endpoints not wrapped yet
- Use explicit `--profile` when multiple accounts are configured

## Stable command families

- `jira auth`
- `jira projects`
- `jira issues`
- `jira comments`
- `jira transitions`
- `jira worklogs`
- `jira users`
- `jira attachments`
- `jira raw`

## Safe automation rules

- Reads: safe for autonomous use
- Writes: require explicit issue keys, transition names or IDs, and field values
- Avoid free-form mutation guesses for Jira custom fields
- Use JSON field values when a field expects structured data

## Users and assignment

- Jira Cloud identifies users by `accountId`, not email — resolve one with `jira users search --query <email>` or `jira auth whoami` for the current user
- `--assignee`, `--reporter`, and `jira issues assign` accept `me`, an email, or an `accountId`; email and `me` are resolved automatically
- `jira issues assign <key> --assignee <ref>` or `--unassign`
- `jira issues create`/`update` accept convenience flags: `--assignee`, `--labels a,b`, `--priority`, `--parent`, `--due YYYY-MM-DD` (an explicit `--field` still wins)

## Attachments

- Manage the full lifecycle: `attachments list <key>`, `attachments download <id> [--output path]`, `attachments delete <id>`
- Attachments require a local file path; Jira cannot embed them in the create-issue payload
- `jira issues create --attach <path>` creates the issue, then uploads the file(s)
- `jira issues attach <issue-key> --file <path>` uploads to an existing issue
- Both flags are repeatable; at most 60 files per request
- Uploads use `multipart/form-data` with the `X-Atlassian-Token: no-check` header (handled internally)
- Partial failure: if `create --attach` creates the issue but an upload fails, the created issue key is still printed as JSON before a non-zero exit — recover by retrying `jira issues attach <key>`

## Examples

```bash
jira issues search --jql 'project = ENG ORDER BY created DESC' --json
jira issues update ENG-123 --summary 'Updated title' --json
jira issues create --project ENG --type Bug --summary 'Crash' --attach ./crash.log --json
jira issues attach ENG-123 --file ./crash.log --file ./screenshot.png --json
jira raw GET /rest/api/3/project/search --query query=platform --json
```

