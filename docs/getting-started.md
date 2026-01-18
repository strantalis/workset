---
description: Install Workset, create a workspace, add repos, and set defaults.
---

# Getting Started

## Install

=== "Homebrew (recommended)"
    ```bash
    brew tap strantalis/homebrew-tap
    brew install workset
    ```

=== "Go"
    ```bash
    go install github.com/strantalis/workset/cmd/workset@v0.1.0
    ```

=== "GitHub Releases"
    ```text
    Download the v0.1.0 release from:
    https://github.com/strantalis/workset/releases/tag/v0.1.0
    ```

!!! tip
    If you use `go install`, ensure `$(go env GOPATH)/bin` is on your PATH.

## Create a workspace

=== "From scratch"
    ```bash
    workset new demo
    workset repo add git@github.com:your/org-repo.git -w demo
    workset status -w demo
    ```

=== "From a template"
    ```bash
    workset group create platform
    workset group add platform repo-alias
    workset group apply -w demo platform
    ```

## Set a default workspace

```bash
workset config set defaults.workspace demo
```

!!! tip
    Once `defaults.workspace` is set, you can omit `-w` for most commands.

## Enable shell completion (optional)

```bash
# bash
workset completion bash > ~/.workset-completion.bash
source ~/.workset-completion.bash

# zsh
workset completion zsh > ~/.workset-completion.zsh
source ~/.workset-completion.zsh
```

For fish or powershell, see the [CLI](cli.md) page for the full set of commands.

## Next steps

- Read the [Concepts](concepts.md) page to understand workspaces, remotes, and templates.
- Review the [Config](config.md) page to customize defaults and repo aliases.
- Use the Command Index to find the right CLI call fast.
