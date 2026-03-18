# Workset Hierarchy Hard Cut

Last updated: 2026-03-16

## Outcome

The migration window is closed. Workset now supports only the canonical workset/thread hierarchy and no longer carries runtime compatibility for the legacy config and template model.

## Removed compatibility paths

- `~/.config/workset/config.yaml` auto-import
- top-level `workspaces` config rewrites
- `defaults.workspace_root`
- workspace `template` metadata
- config `groups`
- CLI `workset group ...` commands
- Wails and frontend template/group management APIs

## Canonical contract

- Global config lives at `~/.workset/config.yaml`.
- Worksets are stored under `worksets.<workset>.threads.<thread>`.
- Default generated thread paths live under `<workset_root>/worksets/<workset>/<thread>`.
- Thread creation uses explicit `--workset` plus direct repo selection or registered repos.

## Validation completed in this pass

- Backend config load and save paths accept only the canonical schema.
- Workspace creation and snapshot contracts expose `workset`, not `template`.
- Repo catalog APIs use registered repo naming consistently across backend, Wails, and frontend.
- Settings and create flows removed template/group UI.
- Existing deprecation register entries were marked completed.

## Follow-up expectation

Any old config still using the removed fields must be rewritten manually. There is no in-app or on-load migration path anymore.
