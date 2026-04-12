# Contributing

## Setup

```bash
go test ./...
go build ./...
```

## Guidelines

- Keep commands easy to use
- Prefer maintainable hand-written clients over broad generated code
- Preserve stable `--json` output
- Add tests for new command flows and request/response decoding
- Do not commit secrets or site-specific credentials

