# Changelog

## Unreleased

- Metadata discovery for reliable writes: `issues create-meta`, `issues edit-meta`, and `fields list`
- Fix JQL search pagination: send `nextPageToken` to fetch pages past the first; add `issues search --page-token` and `--all`
- `users search` and `auth whoami` for resolving Jira Cloud accountIds
- `issues assign` (with `--unassign`) and user references that accept `me`, an email, or an accountId
- Convenience flags on `issues create`/`update`: `--assignee`, `--labels`, `--priority`, `--parent`, `--due`
- Attachment lifecycle commands: `attachments list`, `attachments download`, `attachments delete`

## v0.2.0

- File attachments: `issues create --attach` and `issues attach` upload files via multipart/form-data

## v0.1.0

- Initial Jira Cloud CLI implementation
- Multi-profile authentication
- Issue, comment, transition, worklog, project, and raw commands
- CI, tests, and release scaffolding

