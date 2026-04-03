# Workset Hierarchy

This document describes the current Workset hierarchy contract after the hard cut away from legacy workspace and template compatibility paths.

## Canonical contract

- Global config lives at `~/.workset/config.yaml`.
- Worksets are stored under `worksets.<workset>.threads.<thread>`.
- Default generated thread paths live under `<workset_root>/worksets/<workset>/<thread>`.
- Thread creation uses explicit `--workset` plus direct repo selection or registered repos.

## Removed compatibility paths

These legacy paths are no longer supported at runtime:

- `~/.config/workset/config.yaml` auto-import
- top-level `workspaces` config rewrites
- `defaults.workspace_root`
- workspace `template` metadata
- config `groups`
- CLI `workset group ...` commands
- Wails and frontend template and group management APIs

## Current expectations

- Config load and save paths accept only the canonical schema.
- Workspace creation and snapshot contracts expose `workset`, not `template`.
- Repo catalog APIs use registered repo naming consistently across backend, Wails, and frontend.
- Any remaining old config using the removed fields must be rewritten manually. Workset no longer performs on-load migration for those fields.
