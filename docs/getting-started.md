---
description: Install Workset, create threads, register repos, and set defaults.
---

# Getting Started

## Install

=== "Homebrew (recommended)"
    ```bash
    brew tap strantalis/homebrew-tap
    brew install workset
    ```

    Upgrade:

    ```bash
    brew update
    brew upgrade --cask workset
    ```

=== "Go"
    ```bash
    go install github.com/strantalis/workset/cmd/workset@latest
    ```

=== "npm"
    ```bash
    npm install -g @strantalis/workset@latest
    ```

=== "GitHub Releases"
    ```text
    Download the latest release from:
    https://github.com/strantalis/workset/releases/latest
    ```

!!! tip
    If you use `go install`, ensure `$(go env GOPATH)/bin` is on your PATH.

## Create a thread

=== "From scratch"
    ```bash
    workset new demo
    workset repo add git@github.com:your/org-repo.git -t demo
    workset status -t demo
    ```

=== "With registered repos and an explicit workset"
    ```bash
    workset repo registry add platform git@github.com:your/platform.git
    workset repo registry add api git@github.com:your/api.git
    workset new auth-spike --workset platform-core --repo platform --repo api
    ```

    This creates a thread at `~/.workset/worksets/platform-core/auth-spike` by default.

## Set a default thread

```bash
workset config set defaults.thread demo
```

!!! tip
    Once `defaults.thread` is set, you can omit `-t` for most commands.

## GitHub authentication

See the [CLI page](cli.md#github-auth) for setup instructions covering both CLI and desktop app.

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

- Read the [Concepts](concepts.md) page to understand worksets, threads, and registered repos.
- Review the [Config](config.md) page to customize defaults and registered repos.
- Use the Command Index to find the right CLI call fast.
- Explore the [Desktop App](desktop-app.md) if you prefer a GUI.
