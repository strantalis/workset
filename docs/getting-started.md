---
description: Build Workset, create a workspace, add repos, and set defaults.
---

# Getting Started

## Build from source

```bash
go build ./cmd/workset
```

!!! note
    Workset is still in active development, so the docs assume a local build.

## Create a workspace

=== "From scratch"
    ```bash
    ./workset new demo
    ./workset repo add git@github.com:your/org-repo.git -w demo
    ./workset status -w demo
    ```

=== "From a template"
    ```bash
    ./workset group create platform
    ./workset group add platform repo-alias
    ./workset group apply -w demo platform
    ```

## Set a default workspace

```bash
./workset config set defaults.workspace demo
```

!!! tip
    Once `defaults.workspace` is set, you can omit `-w` for most commands.

## Start a session

```bash
./workset session start demo -- zsh
./workset session attach demo
./workset session stop demo
```

To force a backend:

```bash
./workset session start demo --backend exec --interactive
```

## Run a one-off command

```bash
./workset exec demo -- ls
```

## Enable shell completion (optional)

```bash
# bash
./workset completion bash > ~/.workset-completion.bash
source ~/.workset-completion.bash

# zsh
./workset completion zsh > ~/.workset-completion.zsh
source ~/.workset-completion.zsh
```

For fish or powershell, see the CLI page for the full set of commands.

## Next steps

- Read the Concepts page to understand workspaces, remotes, and templates.
- Review the Config page to customize defaults and repo aliases.
- Use the Command Index to find the right CLI call fast.
