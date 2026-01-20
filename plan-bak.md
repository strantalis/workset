• Sean — here’s a detailed, buildable plan for workspace/repo management with archive tracking (option 2), no import.

  Phase 0 — Scope lock

  - In‑scope: workspace create/rename/archive/delete, repo add/remove/remotes, open folder.
  - Out‑of‑scope: import existing workspace, PR creation, checks, review comments (tracked separately).

  Phase 1 — Data model + config

  - Extend internal/config.WorkspaceRef with archive metadata:
      - ArchivedAt (RFC3339 string), ArchivedReason (optional).
  - Update config load/save + JSON/YAML tags.
  - Update worksetapi types (WorkspaceRefJSON) to include archive fields.
  - Update list API to include archived, plus optional includeArchived filter.
  - Tests: config serialization round‑trip; list behavior with archive flag.

  Phase 2 — Workset API endpoints

  - Add ArchiveWorkspace (sets archive fields; keeps config entry).
  - Add UnarchiveWorkspace (clears archive fields).
  - Update DeleteWorkspace to remove entry only if “Delete” is confirmed; refuse if archived? (decide rule).
  - Add RenameWorkspace (update config key + workset.yaml name, adjust worktree paths if needed).
  - Tests: archive/unarchive behavior, rename conflicts, delete on archived.

  Phase 3 — Repo management API

  - AddRepo wrapper: use existing Service.AddRepo path with hooks surfaced.
  - RemoveRepo wrapper: delete worktree/local repo per flags.
  - UpdateRepoRemotes wrapper: for base/write changes.
  - OpenRepoPath (optional): platform opener; non‑critical.

  - Expose endpoints:
      - ListWorkspaces(includeArchived bool)
      - CreateWorkspace(input)
      - RenameWorkspace(input)
      - ArchiveWorkspace(name, reason?)
      - UnarchiveWorkspace(name)
  - Map errors into structured JSON (message + type + warnings + pendingHooks).
  - Tests: basic unit tests for parsing, error wrapping.

  Phase 5 — UI flows

  - Workspace sidebar:
      - Top actions: Create, Archive, Delete.
      - Archived section (collapsed by default).
  - Workspace dialog:
      - Create: name + optional path + repo/group selections.
      - Archive: reason (optional) + confirmation.
      - Delete: checkbox “delete files on disk” + force + confirmation.
  - Repo actions:
      - Add repo (alias/URL/local path).
      - Remove repo (delete worktree/local).
      - Update remotes.
      - Open in Finder.

  Phase 6 — UX states + safety

  - Empty state for no workspaces: “Create workspace.”
  - Warnings for dirty/unmerged when deleting; surface hook prompts.
  - For archive: treat as non‑destructive but still confirm.

  Phase 7 — Test coverage

  - Go:
      - Config migration/round‑trip tests for archive fields.
      - API tests for archive/unarchive/rename/delete.
      - Repo add/remove/remotes tests (existing patterns).
  - Frontend:
      - State transitions for archive/unarchive.
      - Error handling for delete safety warnings.
      - Form validation (name required, conflicts).

  Phase 8 — Verification

  - Manual checklist:
      - Create workspace → appears in active list.
      - Archive → moves to Archived section.
      - Unarchive → returns to active list.
      - Delete (no files) → removed from config.