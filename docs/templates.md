---
description: Templates (groups) let you reuse sets of repo aliases across workspaces.
---

# Templates

Groups (aka templates) are reusable repo sets stored in global config.

## Create and apply

```
workset group create platform
workset group add platform repo-alias
workset group apply -w demo platform
```

Group members reference repo aliases from the global config. The `template` command remains as an alias.

## Next steps

- [Config](config.md)
- [Getting Started](getting-started.md)
