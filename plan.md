# Wails CI/CD Plan (Release-Please + GoReleaser Compatible)

## Context summary
- Repo uses **release-please** to create GitHub Releases on `main`.
- GoReleaser already handles CLI builds and release attachments.
- Wails app lives in `wails-ui/workset` with its own `go.mod` and `wails.json`.
- You want a **separate** Wails CI/CD pipeline that:
  - Targets **macOS** as the primary supported platform.
  - Includes **Windows best-effort** (allowed to fail) until validated.
  - Attaches artifacts to the **same GitHub Release** that release-please creates.
  - Does **not** attempt to integrate Wails into GoReleaser.

## Goals
1. Build Wails desktop artifacts on release for macOS, attach to the GitHub Release created by release-please.
2. Build Windows artifacts in CI **best-effort** (non-blocking) and attach if successful.
3. Keep CLI release pipeline (GoReleaser) unchanged and independent.
4. Avoid new signing complexity for macOS/Windows in the first iteration; leave hooks for future signing.

## Non-goals (for first iteration)
- macOS notarization and signing (no `gon` config present).
- Windows code signing (unknown capability and app compatibility).
- Packaging beyond Wails defaults (no DMG/installer packaging unless already configured).

## Design choices (explicit)
- **Trigger**: GitHub Actions `release` event, `types: [published]` so it works with release-please without guessing tag patterns.
- **Artifact attachment**: Use `softprops/action-gh-release` to upload Wails build artifacts directly to the existing release.
- **Platform strategy**:
  - `macos-latest` required, blocks job failure.
  - `windows-latest` `continue-on-error: true` to be best-effort.
- **Tooling**:
  - Use Go + Node setup per platform.
  - Install Wails via `go install github.com/wailsapp/wails/v2/cmd/wails@latest`.
  - `wails build` invoked from `wails-ui/workset`.
- **Versioning**:
  - Let release-please tag and release; Wails binary name stays `workset` (from `wails.json`).
  - Add the release tag to artifact filenames for clarity (via rename step).

## Files to add/edit
- **New**: `.github/workflows/wails-release.yml`
- **Optional** (if needed by workflow): `wails-ui/workset/build/darwin` or `wails-ui/workset/build/windows` remain unchanged.

## Workflow structure (detailed)
### Job: `wails-macos`
- `runs-on: macos-latest`
- Steps:
  1. Checkout repository (`actions/checkout` **pinned by SHA**).
  2. Setup Go (`actions/setup-go` **pinned by SHA**), using `wails-ui/workset/go.mod`.
  3. Setup Node (`actions/setup-node` **pinned by SHA**), use `node-version: 24` for consistency with CLI release pipeline.
  4. Install Wails CLI.
  5. Build Wails app from `wails-ui/workset`.
     - `wails build` uses `wails.json` (frontend install/build commands).
  6. Rename artifacts in `wails-ui/workset/build/bin` to include release tag.
     - e.g. `workset-macos-${{ github.ref_name }}` or `workset-${{ github.ref_name }}-macos`.
  7. Upload release assets to the existing GitHub Release.
     - Use `softprops/action-gh-release` **pinned by SHA** with `files: wails-ui/workset/build/bin/*`.

### Job: `wails-windows` (best-effort)
- `runs-on: windows-latest`
- `continue-on-error: true`
- Steps mirror macOS with Windows-specific paths.
- No signing or packaging in this iteration.
- Attach assets to the same release if build succeeds.

### Permissions
- `contents: write` required for attaching release assets.
- No other permissions needed.

## Artifact naming and expectations
- Default Wails output folder: `wails-ui/workset/build/bin`.
- Rename step to avoid ambiguous filenames across OS:
  - macOS: `workset-${TAG}-macos` (or `.app` if Wails outputs an app bundle)
  - Windows: `workset-${TAG}-windows.exe`
- The workflow will upload any files in `build/bin` after rename.

## CI edge cases and handling
- **Release tag name**:
  - Use `github.ref_name` to pick the release tag for filenames.
- **Windows build failures**:
  - Allowed; does not fail workflow.
- **Node + Wails install**:
  - If the frontend build is slow or fails due to old dependencies, we can pin Node to a compatible version later.

## Steps to implement
1. Add `.github/workflows/wails-release.yml` with the release trigger and two jobs.
2. Ensure Go setup uses the Wails module path (`wails-ui/workset/go.mod`).
3. Validate build command, expected output directory, and asset naming with a local dry run.
4. Add asset upload to GitHub Release.

## Checks to run (local or CI)
- `cd wails-ui/workset && wails build` (local smoke test).
- Ensure the workflow passes on a release event (or `workflow_dispatch` to test).

## Follow-up items (optional, post-implementation)
- Add macOS notarization (needs `gon` config and certificates).
- Add Windows signing (needs PFX cert + signtool config).
- Add packaging (DMG/installer) if required.

---

If you confirm this plan, I’ll implement it and then run the repo checks (lint/test) that are relevant. If full checks are too heavy locally, I’ll tell you exactly what I couldn’t run.
