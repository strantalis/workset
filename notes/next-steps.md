# Next Steps (Local)

## CLI Output
- Consider adding `--color` to force styling on non-TTY output (pair with `--plain`).
- Add golden/snapshot tests for key outputs (workspace creation, status, template/repo lists) to prevent regressions.

## Tooling
- In sandboxed runs, `golangci-lint` cannot write to the default cache path. Consider setting `GOLANGCI_LINT_CACHE` to a writable dir (e.g. `.cache/golangci-lint` or `$TMPDIR`) in dev/CI scripts.

## Suggested Linear Issue (if/when enabled)
- Title: Output UX + CLI arg parsing follow-ups
- Project: workset
- Labels: improvements, ai-generated
- Body:
  - Add explicit color controls (`--plain` / `--color`) to all output-heavy commands.
  - Add golden/snapshot tests for workspace creation, repo/template lists, and status tables.

- Title: Make golangci-lint cache path writable in constrained environments
- Project: workset
- Labels: improvements, ai-generated
- Body:
  - Set `GOLANGCI_LINT_CACHE` to `.cache/golangci-lint` or `$TMPDIR` to avoid permission errors in sandboxed runs.

- Title: Cache config reads + status errors for workspace snapshots
- Project: workset
- Labels: improvements, ai-generated, performance
- Body:
  - Add read-through caching for global config + workspace config/state (mtime+size or fsnotify).
  - Expose status errors in snapshot payloads so UI can show "error" vs "pending".
