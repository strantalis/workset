# Workspace + Repo Management Plan

## Scope (current pass)
- Add archive support for workspaces in global config and API.
- Expose workspace + repo management methods to the Wails UI.
- Build a management panel to create/archive/delete workspaces and add/remove repos.
- Keep import out of scope for now.

## Plan
- [x] Add archive fields to workspace config + worksetapi JSON types.
- [x] Implement worksetapi archive/unarchive helpers (config updates + timestamps).
- [x] Add list filtering for archived workspaces.
- [x] Add workspace rename support (config + workset.yaml).
- [x] Surface archive status in Wails workspace snapshots and frontend types.
- [x] Add Wails methods for workspace CRUD (create, rename, archive, unarchive, delete).
- [x] Add Wails methods for repo add/remove/remotes update.
- [x] Build a management panel UI (create workspace, rename, archive/unarchive, delete, add/remove repo).
- [x] Wire UI to refresh workspace list + selection state updates.
- [x] Add tests for archive + rename helpers and list behavior.
- [x] Run checks: `go test ./...` (wails-ui/workset), `npm run check`, `npm run build` (frontend).
