# CLI

## Commands

```
workset new <name>
workset init [--name <name>]
workset ls
workset config show|set
workset repo alias ls|add|set|rm
workset repo add -w <workspace> <source> [--name] [--repo-dir]
workset repo ls -w <workspace>
workset repo rm -w <workspace> <name> [--delete-worktrees] [--delete-local]
workset status -w <workspace>
workset template ls|show|create|rm|add|remove
workset template from-workspace -w <workspace> <name>
workset template apply -w <workspace> <name>
workset rm -w <name|path> [--delete]
```

Commands that operate on a workspace require an explicit target: pass `-w <workspace>` (name or path) or set `defaults.workspace`. Most flags should appear before positional args; `-w/--workspace`, `--json`, `--plain`, and `--config` are also recognized after args.

## Output modes

```
workset new --json
workset init --plain
workset repo ls -w demo --json
workset status -w demo --json
```
