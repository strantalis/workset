# Contributing to Workset

This document covers developer-facing contribution rules that complement user docs.

## Local Development

Run from repository root unless noted.

### Core checks

Before opening a PR:

```bash
make test
make lint
cd wails-ui/workset/frontend && npm run check && npm run build
```

### Useful commands

- Go tests only: `go test ./...`
- Frontend tests only: `cd wails-ui/workset/frontend && npm run test`
- Frontend format check: `cd wails-ui/workset/frontend && npm run format:check`

## Code Organization Expectations

Keep responsibilities clear across packages:

- `cmd/`: CLI entrypoints and orchestration.
- `internal/`: private repository internals and domain logic.
- `pkg/`: stable service/public contracts.
- `wails-ui/`: desktop runtime and frontend concerns.

Prefer small, reversible changes and explicit error handling. If a change crosses boundaries, call out rationale in the PR.

## Testing and PR Expectations

- Add or update tests for behavior changes.
- Include edge/error-path coverage for high-risk logic.
- Update docs when user-visible behavior changes.
- In PR description, include:
  - What changed and why.
  - Commands/checks you ran.
  - Any known follow-ups.

## Code Health Guardrails

CI enforces baseline code-health policies to prevent long-term drift:

- LOC/file-size ratchet policy (`guardrails.yml`).
- Go complexity checks (`.golangci-complexity.yml`).
- Frontend complexity checks on changed files.

Guardrails are intended to prevent regressions. If an exception is necessary, document it explicitly in the PR and update policy files intentionally.
