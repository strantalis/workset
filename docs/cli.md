---
description: CLI overview, command syntax, and output modes for Workset.
---

# CLI

## Commands

```
workset new <name> [--path <path>] [--group <name> ...] [--repo <alias> ...]
workset ls
workset exec [<workspace>] [-- <command> [args...]]
workset session start [<workspace>] [-- <command> [args...]] [--yes] [--attach]
workset session attach [<workspace>] [<name>] [--yes]
workset session stop [<workspace>] [<name>] [--yes]
workset session show [<workspace>] [<name>]
workset session ls [<workspace>]
workset version
workset --version
workset config show|set
workset repo alias ls|add|set|rm
workset repo add -w <workspace> <source> [--name] [--repo-dir]
workset repo ls -w <workspace>
workset repo remotes set -w <workspace> <name> [--base-remote] [--write-remote] [--base-branch]
workset repo rm -w <workspace> <name> [--delete-worktrees] [--delete-local]
workset status -w <workspace>
workset group ls|show|create|rm|add|remove
workset group apply -w <workspace> <name>
workset rm -w <name|path> [--delete]
workset completion <bash|zsh|fish|powershell>
```

Commands that operate on a workspace require an explicit target: pass `-w <workspace>` (name or path) or set `defaults.workspace`. Most flags should appear before positional args; `-w/--workspace`, `--path`, `--group`, `--repo`, `--json`, `--plain`, `--config`, and `--verbose` are also recognized after args.

## Safety checks

`workset rm --delete` and `workset repo rm --delete-*` run safety checks before removing files. Branches are treated as merged when the base branch already contains the same file contents (covers squash merges).

## Sessions

`workset session start` starts a persistent session. By default it uses `tmux` if available, falls back to `screen`, and finally runs the command directly (`exec` backend). You can force a backend with `--backend`. Use `--interactive`/`--pty` only with `--backend exec`. Use `--attach` to immediately attach for tmux/screen (ignored for exec). To enable the built-in tmux/screen status line, set `defaults.session_theme` to `workset` in the global config (see [Config](config.md)).

```
workset session start demo -- zsh
workset session start demo --backend exec --interactive
workset session attach demo
workset session attach demo --yes
workset session show demo
workset session stop demo
```

Session metadata is stored locally in `<workspace>/.workset/state.json`.

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

Completion includes hints for workspace names, group names, repo aliases, repo names within a workspace, and recorded session names when commands expect those positional args.

## Output modes

```
workset new --json
workset new demo --plain
workset repo ls -w demo --json
workset status -w demo --json
```

## Next steps

- [Command Index](commands.md)
- [Config](config.md)
