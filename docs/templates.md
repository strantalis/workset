# Templates

Templates (aka groups) are reusable repo sets stored in global config.

## Create and apply

```
workset template create platform
workset template add platform repo-alias
workset template apply -w demo platform
```

Template members reference repo aliases from the global config.
