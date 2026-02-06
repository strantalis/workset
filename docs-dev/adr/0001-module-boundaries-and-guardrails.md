# ADR 0001: Module Boundaries and Guardrails

- Status: Accepted
- Date: 2026-02-06
- Owners: @strantalis

## Context

The repository contains multiple source files over 1,000 LOC across Go and Svelte/TypeScript. Large files and high complexity increase review risk and slow maintenance.

## Decision

1. Add CI guardrails for source-file LOC growth with explicit allowlisting.
2. Add PR-scoped complexity checks for Go (`cyclop`, `gocognit`) and frontend (`complexity`, `max-lines`).
3. Define and enforce module boundaries:
   - `cmd/` for entrypoint orchestration.
   - `internal/` for private domain internals.
   - `pkg/` for service/public contracts.
   - `wails-ui/` for desktop/frontend concerns.

## Rationale

- Ratcheting prevents net regressions without forcing disruptive large refactors in one PR.
- PR-scoped complexity checks keep signal high while avoiding immediate legacy noise.
- Explicit boundaries reduce accidental coupling and simplify future decomposition.

## Consequences

- Contributors must update `guardrails.yml` for intentional exceptions.
- Oversized allowlisted files can be touched, but growth is blocked.
- Future refactors should prioritize shrinking allowlisted files and reducing complexity hotspots.
