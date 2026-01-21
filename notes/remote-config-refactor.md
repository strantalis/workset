Title: Remote config refactor (single remote per repo alias)
Status: Draft
Owner: Sean
Date: 2026-01-21

Summary
This change removes base/write remotes from workspace config and centralizes remote
and default branch configuration on repo aliases only. The default remote is
origin and the default branch is main. Fork PR flow is treated as an explicit
workflow (future GitHub integration) rather than a persistent config dimension.

Goals
- Single "primary remote" per repo alias (default: origin).
- Single default branch per repo alias (default: main).
- No per-workspace overrides for remotes or branches.
- Strict error when the configured remote is missing (outside alias creation).
- Migration path from workspace remotes to alias fields.

Non-goals
- Modeling "write" remotes or persistent fork preferences.
- Persisting PR base preferences in config.
- Changing git remote behavior beyond selection and validation.

Current state (brief)
- Global config: defaults.base_branch only.
- Workspace config: repos[].remotes.base + repos[].remotes.write.
- Group templates can override remotes/branch per member.
- CLI: workset repo remotes set, group add --base-remote/--write-remote/--base-branch.

Proposed model
- Repo alias becomes the single source of truth for:
  - remote (primary remote; default: origin)
  - default_branch (default: main)
- Workspace repo entries no longer store remotes.
- Fork PR flow is explicit and transient (future CLI).

Configuration changes
Global config (repo aliases):
  repos:
    my-repo:
      url: git@github.com:org/my-repo.git
      remote: origin
      default_branch: main

Global defaults:
  defaults:
    remote: origin
    base_branch: main

Workspace config:
  repos:
    - name: my-repo
      repo_dir: my-repo
      local_path: /Users/sean/src/my-repo
      managed: false
    # no remotes here

Behavior changes
- Add repo:
  - Resolve alias. If alias has no remote/default_branch, seed from defaults.
  - Validate alias.remote exists.
  - Strict error if remote missing (except during alias creation fallback rules).
- Worktree creation:
  - Uses alias.remote + alias.default_branch for start remote/branch.
- Safety/status:
  - Use alias.remote + alias.default_branch only.
- Group templates:
  - No remotes/branch overrides (alias inherits).

Missing remote policy (strict)
During alias creation only:
  - If defaults.remote missing in repo:
    - If exactly one remote exists: warn, set alias.remote to that remote.
    - Else error and require explicit --remote.
All other operations:
  - Error if alias.remote missing or remote does not exist in the repo.

CLI changes
- Add/extend:
  - workset config set defaults.remote <name>
  - workset repo alias add|set --remote <name> [--default-branch <branch>]
  - workset repo alias ls includes REMOTE column
- Remove/deprecate:
  - workset repo remotes set
  - group add --base-remote/--write-remote/--base-branch

Migration plan
- On workspace load:
  - If repos[].remotes exists:
    - Create/merge alias:
      - alias.remote = remotes.base.name
      - alias.default_branch = remotes.base.default_branch
    - If base/write differ, warn once.
  - Strip remotes from workset.yaml and save.
- If alias missing, create alias from:
  - URL for managed repos, or local_path for local repos.

Validation and error handling
- Validate alias.remote exists before worktree or safety operations.
- Provide actionable error:
  - "remote '<name>' not found in repo; set with workset repo alias set <repo> --remote <name>"

Impact areas
- Config types/defaults/migration
- Repo add flow
- Workspace load/save
- Safety checks (remove write remote logic)
- CLI + docs + UI settings panel

Out of scope
- GitHub PR integration and fork workflows (next phase).

Open questions (resolved)
- Strict error on missing remotes outside alias creation: yes.
