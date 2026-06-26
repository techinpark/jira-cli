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
- `jira fields`
- `jira links`
- `jira raw`

## Metadata discovery (do this before writing)

- Before `issues create`, run `issues create-meta --project <key>` to list valid issue types, then `issues create-meta --project <key> --type <name>` to learn each field's `required` flag, `schema`, and `allowedValues` (with IDs)
- Before `issues update`, run `issues edit-meta <issue-key>` for the editable fields and their allowed values
- Use `fields list` to map a human field name to its id (e.g. `Story Points` → `customfield_10016`) for `--field`
- This turns writes from guess/422/retry into: fetch metadata → pick valid values → write once

## Search pagination

- `issues search` is token-paginated: `--limit` is the page size, and the JSON `next_page_token` (when present) points to the next page
- Pass `--page-token <token>` to fetch the next page, or `--all` to follow tokens and return every page at once
- `is_last` in the JSON indicates the final page

## Issue links

- `links types` lists valid link type names and their inward/outward labels — call it before `links add` since types are configured per instance
- `links add --outward <key> --inward <key> --type <name>`: the outward issue relates to the inward issue via the type's outward label (e.g. outward "blocks" inward)
- `links list <key>` shows an issue's links with their link IDs; `links delete <link-id>` removes one

## Safe automation rules

- Reads: safe for autonomous use
- Writes: require explicit issue keys, transition names or IDs, and field values
- Do not guess custom fields — resolve them first with `fields list` and `issues create-meta`/`edit-meta` (see Metadata discovery above)
- Use JSON field values when a field expects structured data

## Users and assignment

- Jira Cloud identifies users by `accountId`, not email — resolve one with `jira users search --query <email>` or `jira auth whoami` for the current user
- `--assignee`, `--reporter`, and `jira issues assign` accept `me`, an email, or an `accountId`; email and `me` are resolved automatically
- `jira issues assign <key> --assignee <ref>` or `--unassign`
- `jira issues create`/`update` accept convenience flags: `--assignee`, `--labels a,b`, `--priority`, `--parent`, `--due YYYY-MM-DD` (an explicit `--field` still wins)

## Attachments

- Manage the full lifecycle: `attachments list <key>`, `attachments download <id> [--out path]`, `attachments delete <id>`
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

