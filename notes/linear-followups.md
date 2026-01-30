# Linear Follow-ups (local note)

Linear access is not configured in this environment. Suggested issue(s):

- Title: Document GitHub CLI + PAT auth flow
  Labels: label_improvements, label_ai_generated, docs
  Body:
  - Document default GitHub CLI auth (run `gh auth login`) and fallback to PAT in the UI.
  - Call out GitHub.com-only limitation and how we plan to expand to Enterprise/SSO later.
  - Mention the auth-required modal behavior, how to disconnect, and PAT storage in the OS keychain.

- Title: Consolidate GitHub auth UI into a shared component
  Labels: label_improvements, label_ai_generated, frontend
  Body:
  - Extract a reusable GitHub auth form for the modal + settings panel to avoid drift.
  - Ensure mode toggle, CLI instructions, PAT input, and status messaging stay in sync.

- Title: Track GitHub Enterprise host support
  Labels: label_ideas, label_ai_generated, backend
  Body:
  - Revisit github.com-only restriction and add enterprise host support (custom API base URL).
  - Define UX for selecting host and validating auth mode on non-github.com.
