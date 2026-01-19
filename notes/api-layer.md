# API layer refactor tracker

Goal: introduce a public Go package (`pkg/worksetapi`) that exposes a stable CRUD-style API (plus actions for sessions/exec) so CLI and other Go apps can reuse the same logic without shelling out. CLI JSON output must stay stable.

## Scope
- Create `pkg/worksetapi` with exported DTOs mirroring existing CLI JSON.
- Add service layer that owns business logic currently in `cmd/workset/*`.
- Refactor CLI commands to call service layer and render output.
- Keep JSON schemas stable by using DTOs directly.
- Add tests for services + JSON schema stability.

## Progress
- 2026-01-19: Tracking file created; plan approved by Sean.
- 2026-01-19: Implemented `pkg/worksetapi` service layer, refactored CLI to use it, moved session helpers, added tests, fixed lint issues, and ran `gofmt -w ./cmd ./internal ./pkg`, `go test ./...`, `make lint`.
- 2026-01-19: Added non-OS exec runner tests, expanded CRUD edge-case coverage, and regenerated coverage (`pkg/worksetapi` at 80.9%).

## Open decisions
- Package name: `worksetapi` (default).

## Next steps
- Review API shape and DTOs for external consumers.
- Decide whether to expose additional helpers (filters, selectors) or keep in `internal` only.
