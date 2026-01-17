---
description: CLI overview, command syntax, and output modes for Workset.
---

# CLI

## Commands

```
workset new <name> [--path <path>] [--group <name> ...] [--repo <alias> ...]
workset ls
workset version
workset --version
workset config show|set
workset repo alias ls|add|rm
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

Commands that operate on a workspace require an explicit target: pass `-w <workspace>` (name or path) or set `defaults.workspace`. Most flags should appear before positional args; `-w/--workspace`, `--path`, `--group`, `--repo`, `--json`, `--plain`, and `--config` are also recognized after args.

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

Completion includes hints for workspace names, group names, repo aliases, and repo names within a workspace when commands expect those positional args.

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
