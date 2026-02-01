# Repository Guidelines

## Project Structure & Module Organization
- `cmd/`: Go CLI entrypoints (primary binary is `workset`).
- `internal/`: private Go packages (ops, workspace, hooks, session).
- `pkg/worksetapi/`: public API surface and service layer.
- `wails-ui/workset/`: desktop UI app (Wails + Svelte frontend).
- `docs/` + `mkdocs.yml`: MkDocs content and site config.
- `scripts/`: maintenance scripts; `dist/` contains build artifacts.

## Build, Test, and Development Commands
- `make test`: run Go unit and integration tests (`go test ./...`).
- `make lint`: run `golangci-lint` using `.golangci.yml`.
- `make lint-fmt`: format Go files with `golangci-lint fmt` (gofumpt).
- `make fmt`: format Go code with `gofmt -w ./cmd ./internal`.
- `make check`: `fmt + test + lint` for CI parity.
- `make docs-serve`: serve MkDocs locally (requires `uv` + `requirements.txt`).
- Frontend (from `wails-ui/workset/frontend`):
  - `npm run dev`: Vite dev server.
  - `npm run build`: production build.
  - `npm run check`: `svelte-check` type/diagnostic pass.

## Coding Style & Naming Conventions
- Go: format with `gofumpt` via `make lint-fmt`.
- Keep packages lower-case, exported identifiers `PascalCase`, errors lower-case without trailing punctuation.
- Public surface lives in `pkg/`; keep internal-only logic under `internal/`.

## Testing Guidelines
- Go tests live alongside code as `*_test.go` (see `internal/` and `pkg/`).
- End-to-end coverage lives in `internal/e2e/`.
- Run `go test ./...` or `make test` before opening a PR.

## Commit & Pull Request Guidelines
- Use Conventional Commits (per `commitlint.config.mjs`), e.g.:
  - `feat(workspaces): add snapshot API`
  - `chore(ci): update release pipeline`
- PRs should include: summary, rationale, tests run, and docs updates when behavior changes.
- UI changes in `wails-ui/` should include screenshots or short clips.

## Security & Configuration Tips
- Workspace state lives in `workset.yaml` and `.workset/`; avoid committing local state.
- Default clone root is `~/.workset/repos` (configurable).

## Agent-Specific Instructions
- Favor small, reversible changes; call out production impact early.
- Avoid destructive commands without explicit confirmation.
