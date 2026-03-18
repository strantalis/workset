---
description: CLI overview, command syntax, and output modes for Workset.
---

# CLI

## Commands

```
workset new <name> [--path <path>] [--workset <name>] [--repo <alias|url|path> ...]
workset ls
workset hooks run -t <thread> <repo> [--event <event>] [--reason <reason>] [--trust]
workset version
workset --version
workset config show|set
workset config recover [--workset-root <path>] [--rebuild-repos] [--dry-run]
workset repo registry ls|add|set|rm
workset repo add -t <thread> <source> [--name] [--repo-dir]
workset repo ls -t <thread>
workset repo rm -t <thread> <name> [--delete-worktrees] [--delete-local]
workset status -t <thread>
workset rm -t <name|path> [--delete]
workset completion <bash|zsh|fish|powershell>
```

Commands that operate on a thread require an explicit target: pass `-t <thread>` (name or path) or set `defaults.thread`. Most flags should appear before positional args; `-t/--thread`, `--path`, `--workset`, `--repo`, `--json`, `--plain`, `--config`, and `--verbose` are also recognized after args.

## GitHub auth

Workset uses your **GitHub CLI** session by default. This works for both the CLI and the desktop app.

```
gh auth login
```

In the desktop app, confirm your connection in **Settings → GitHub**. If you prefer a **personal access token** instead, switch to PAT mode in Settings → GitHub and save a token with repo access. Workset stores it in your OS keychain.

For CLI-only usage (no GUI), set `WORKSET_GITHUB_PAT` in your environment to import a PAT into the keychain:

```
WORKSET_GITHUB_PAT=ghp_... workset <command>
```

If `gh` is not on PATH (e.g., Nix), set the override in `~/.workset/config.yaml`:

```yaml
github:
  cli_path: /Users/you/.nix-profile/bin/gh
```

## Safety checks

`workset rm --delete` and `workset repo rm --delete-*` run safety checks before removing files. Branches are treated as merged when the base branch already contains the same file contents (covers squash merges).

## Shell completion

Generate a completion script for your shell and source it:

```
# bash
workset completion bash > ~/.workset-completion.bash
source ~/.workset-completion.bash

# zsh
workset completion zsh > ~/.workset-completion.zsh
source ~/.workset-completion.zsh

# fish
workset completion fish > ~/.config/fish/completions/workset.fish

# powershell
workset completion powershell > workset.ps1
. ./workset.ps1
```

Completion includes hints for thread names, registered repo names, and repo names within a thread when commands expect those positional args.

## Output modes

```
workset new --json
workset new demo --plain
workset repo ls -t demo --json
workset status -t demo --json
```

## Next steps

- [Command Index](commands.md)
- [Config](config.md)
