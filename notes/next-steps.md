# Next Steps (Local)

## CLI Output
- Consider adding `--color` to force styling on non-TTY output (pair with `--plain`).
- Add golden/snapshot tests for key outputs (workspace creation, status, template/repo lists) to prevent regressions.


## Suggested Linear Issue (if/when enabled)
- Title: Output UX + CLI arg parsing follow-ups
- Project: workset
- Labels: improvements, ai-generated
- Body:
  - Add explicit color controls (`--plain` / `--color`) to all output-heavy commands.
  - Add golden/snapshot tests for workspace creation, repo/template lists, and status tables.
