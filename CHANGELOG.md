# Changelog

## [0.3.0](https://github.com/strantalis/workset/compare/v0.2.1...v0.3.0) (2026-01-27)


### ⚠ BREAKING CHANGES

* the default config path moved to ~/.workset/config.yaml (legacy ~/.config/workset/config.yaml is migrated on load).
* defaults.parallelism has been removed; parallelism is no longer configurable.
* defaults.remotes.* has been removed; remote names are now derived from local repos or default to origin for URL-based repos.

### Features

* add verbose config diagnostics ([61ccd6a](https://github.com/strantalis/workset/commit/61ccd6a77b3e68cc49a324ddd0a0f0007ae31e1e))
* add wails ui workspace management ([06785eb](https://github.com/strantalis/workset/commit/06785eb22fa78462f54d6b9ae698009ad5190951))
* **api:** introduce worksetapi service layer ([f41526a](https://github.com/strantalis/workset/commit/f41526a337fc3bc46aa158f55bd0893ff6fbfb3e))
* github comment improvements reply,edit,delete,resolve ([7a7d39a](https://github.com/strantalis/workset/commit/7a7d39a9882fff2bfc8f0354351c1b8c76b2c41c))
* **github:** enrich repo diff and checks ([1b601a8](https://github.com/strantalis/workset/commit/1b601a8b70b01527d46778a18846b32097c5b926))
* **hooks:** add repo hooks with trust prompts ([09203c5](https://github.com/strantalis/workset/commit/09203c5d4b05fb2ec44106525fbea5b4db820131))
* **hooks:** add skipped status ([fe29f35](https://github.com/strantalis/workset/commit/fe29f35e5e1d82b16c513c784b293b26c56088b6))
* **hooks:** mark untrusted/disabled as skipped ([4cdc520](https://github.com/strantalis/workset/commit/4cdc52082530a5dd9be130fb18241fa2b0aa4afd))
* improve single repo workspace creation ([9d7fc7a](https://github.com/strantalis/workset/commit/9d7fc7a76c16ca55141f2036dd7f7d77a1025f3a))
* move global config to ~/.workset ([16b954e](https://github.com/strantalis/workset/commit/16b954ee0763d8f0bf56f3d0e7e84ee7090a131b))
* **pr:** add pull request create/status/checks/reviews commands ([59b266a](https://github.com/strantalis/workset/commit/59b266ad496139619f78c43c606700c970597591))
* remove defaults parallelism ([5530186](https://github.com/strantalis/workset/commit/5530186b3a7332eac9665b55604ee1852a934709))
* remove defaults remotes ([a675bad](https://github.com/strantalis/workset/commit/a675bad37c75699959385c897fa41625520b5e4f))
* **repo:** add alias set command ([b7c950b](https://github.com/strantalis/workset/commit/b7c950b1cfd95ffd80f0d245d6bdf1aed919cff5))
* **repo:** add repo remotes update command and defaults ([114a686](https://github.com/strantalis/workset/commit/114a686d2c62d33f19030b3dfc6b047531e48ca1))
* **sessiond:** auto-restart on binary change ([b0faaed](https://github.com/strantalis/workset/commit/b0faaed7a66e45bd80867b1741fef02b67dea6c3))
* **sessiond:** harden restart flow and logging ([991ca11](https://github.com/strantalis/workset/commit/991ca11a4de8a0cc274700666ff7785358d8adea))
* **terminal:** add agent launcher, defaults, and availability checks ([5aec79d](https://github.com/strantalis/workset/commit/5aec79da0b0ceae0f78145f752c063b80ce01c0d))
* **terminal:** add multi-terminal workspace layout ([1237de9](https://github.com/strantalis/workset/commit/1237de9ab36b8af52aa02180068cd65a976130c5))
* **terminal:** migrate to sessiond-backed sessions ([068ae24](https://github.com/strantalis/workset/commit/068ae2446d142195cebdbf5dbcbc66610c6e5b11))
* **ui:** drop agent launcher modal ([9643b36](https://github.com/strantalis/workset/commit/9643b36414e09c4bc40a17670481a6430e1be10a))
* **ui:** surface sessiond restart warnings ([5e6b002](https://github.com/strantalis/workset/commit/5e6b00211bfa64e4147720a9b2a5f1bf86822933))
* **wails-ui:** add pull request tooling and terminal enhancements ([8599847](https://github.com/strantalis/workset/commit/85998476837502d73146ca50be29344b8f46e048))
* **wails:** add destructive delete confirmations ([6e152a3](https://github.com/strantalis/workset/commit/6e152a37f8ae415466c6aa7158e0c60156869491))
* **worksetapi:** run agents via login shell ([4b219ad](https://github.com/strantalis/workset/commit/4b219ade744d5aeb7a247ee88d39976d3c516874))
* **workspace:** add agent guidance file on init ([d95d137](https://github.com/strantalis/workset/commit/d95d1371c1ab73e75e451508f466d760d39893c7))
* **workspaces:** add snapshot API and migrate UI to Svelte 5 ([1635a72](https://github.com/strantalis/workset/commit/1635a72969169f0f462930aaa669623939119ff8))
* **workspace:** stop sessions on delete ([b654283](https://github.com/strantalis/workset/commit/b65428362faed0c224909f04b7f955356dba7ca0))


### Bug Fixes

* add browse button to alias creation ([0bbf4ab](https://github.com/strantalis/workset/commit/0bbf4abe17c5098beb7c7c2e6b54d91dac0256cc))
* **ci:** harden macos notarization ([d2408fe](https://github.com/strantalis/workset/commit/d2408fe03f3ddf7d74e2409ce0f75ab0dcaef9d9))
* **deps:** bump github.com/creack/pty from 1.1.21 to 1.1.24 ([#14](https://github.com/strantalis/workset/issues/14)) ([f6d1878](https://github.com/strantalis/workset/commit/f6d18783a3e216d84f0189a13cb758f7d11408e5))
* **deps:** bump github.com/urfave/cli/v3 from 3.6.1 to 3.6.2 ([#15](https://github.com/strantalis/workset/issues/15)) ([e6c3cab](https://github.com/strantalis/workset/commit/e6c3cab92193be20e669de238e95a3e8dd69196a))
* **git:** map SSH_AUTH_SOCK from IdentityAgent ([90aba50](https://github.com/strantalis/workset/commit/90aba50ebfa592caa3bb6688b486af658f67d5cc))
* **git:** prefer IdentityAgent socket ([1cc9935](https://github.com/strantalis/workset/commit/1cc99356d5015f71d187872e41f07e8677afa345))
* **git:** resolve IdentityAgent via ssh -G ([6b52b67](https://github.com/strantalis/workset/commit/6b52b6743ef92f5378494f04110297ccec3192d8))
* improve pull request creation ([fa793b6](https://github.com/strantalis/workset/commit/fa793b662048c04525dd790f85950cf5e11a580a))
* scroll workspace sidebar properly ([2626796](https://github.com/strantalis/workset/commit/2626796e519210c54b77be24bca74cd19543f3d5))
* **terminal:** restore codex replay after restart ([0f56619](https://github.com/strantalis/workset/commit/0f56619c76e167940fe53771fd12f7b59aa0edc0))
* ui cleanup ([2a52402](https://github.com/strantalis/workset/commit/2a524020e090d5b6c93dca1ccf740830c14ad5f8))
* **ui:** disable auto-capitalization for identifier inputs ([972affa](https://github.com/strantalis/workset/commit/972affa465f706fa1d48d16040aa65001ac795e3))
* **wails:** normalize PATH for gui startup ([54c89e1](https://github.com/strantalis/workset/commit/54c89e1f23fdfdb81eca01193a95875e4b45dc5f))
* **wails:** read PATH from login shell ([b2ab169](https://github.com/strantalis/workset/commit/b2ab16909ad55e69ffffa1804521e8de329df742))
* **workspaces:** register before repo add ([d2e6bd7](https://github.com/strantalis/workset/commit/d2e6bd71459656059ecb4771ba9a655069471640))
* **worktree:** clean up stale worktree metadata ([288553f](https://github.com/strantalis/workset/commit/288553f9b0d1f356d7dc068454d908371c22e09b))

## [0.2.1](https://github.com/strantalis/workset/compare/v0.2.0...v0.2.1) (2026-01-18)


### Miscellaneous Chores

* release 0.2.1 ([f8d7892](https://github.com/strantalis/workset/commit/f8d78925edbddfca83604dfcccf5a0fc76246702))

## [0.2.0](https://github.com/strantalis/workset/compare/v0.1.0...v0.2.0) (2026-01-18)


### Features

* add delete refusal details ([f4998f0](https://github.com/strantalis/workset/commit/f4998f08d3207088809b4d9d6f0bb5cdbcc18185))
* add session management and exec tooling ([#9](https://github.com/strantalis/workset/issues/9)) ([4ada25c](https://github.com/strantalis/workset/commit/4ada25ce463705d111ca03f5e090be348343f84e))
* **release:** add npm trusted publishing ([409891c](https://github.com/strantalis/workset/commit/409891c85a6bab50aa0e6fe3c8f72d6da3956439))

## 0.1.0 (2026-01-18)


### Features

* add shell completion and hints ([d54dec5](https://github.com/strantalis/workset/commit/d54dec58d69f82bb3df38047de5463002978f416))
* bootstrap workset cli ([f3b894a](https://github.com/strantalis/workset/commit/f3b894afd2c1d9cbf0640f27ebe0db76a5062d4a))
* refactor cli and expand workspace setup ([2b43c7c](https://github.com/strantalis/workset/commit/2b43c7cc7f29a2da718c040a5cd3a5bd17ee2784))
* **workset:** revamp repo/worktree model and CLI ([1b5a110](https://github.com/strantalis/workset/commit/1b5a11061cfe36ce406c85c43bddb50fa89c87ef))
