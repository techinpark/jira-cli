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
- `jira raw`

## Safe automation rules

- Reads: safe for autonomous use
- Writes: require explicit issue keys, transition names or IDs, and field values
- Avoid free-form mutation guesses for Jira custom fields
- Use JSON field values when a field expects structured data

## Examples

```bash
jira issues search --jql 'project = ENG ORDER BY created DESC' --json
jira issues update ENG-123 --summary 'Updated title' --json
jira raw GET /rest/api/3/project/search --query query=platform --json
```

