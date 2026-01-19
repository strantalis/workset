package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/ops"
	"github.com/strantalis/workset/internal/output"
	"github.com/strantalis/workset/internal/workspace"
	"github.com/urfave/cli/v3"
)

func repoCommand() *cli.Command {
	return &cli.Command{
		Name:  "repo",
		Usage: "Manage repos in a workspace (requires -w)",
		Commands: []*cli.Command{
			repoAliasCommand(),
			{
				Name:      "ls",
				Aliases:   []string{"list"},
				Usage:     "List repos in a workspace (requires -w)",
				ArgsUsage: "-w <workspace>",
				Flags:     appendOutputFlags([]cli.Flag{workspaceFlag(true)}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg, cfgPath, err := loadGlobal(cmd)
					if err != nil {
						return err
					}
					wsRoot, wsConfig, err := resolveWorkspace(cmd, &cfg, cfgPath)
					if err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)

					type repoRow struct {
						Name      string `json:"name"`
						LocalPath string `json:"local_path"`
						Managed   bool   `json:"managed"`
						RepoDir   string `json:"repo_dir"`
						Base      string `json:"base"`
						Write     string `json:"write"`
					}

					rows := make([]repoRow, 0, len(wsConfig.Repos))
					for _, repo := range wsConfig.Repos {
						config.ApplyRepoDefaults(&repo, cfg.Defaults)
						base := repo.Remotes.Base.Name
						if repo.Remotes.Base.DefaultBranch != "" {
							base = fmt.Sprintf("%s/%s", base, repo.Remotes.Base.DefaultBranch)
						}
						write := repo.Remotes.Write.Name
						if repo.Remotes.Write.DefaultBranch != "" {
							write = fmt.Sprintf("%s/%s", write, repo.Remotes.Write.DefaultBranch)
						}
						rows = append(rows, repoRow{
							Name:      repo.Name,
							LocalPath: repo.LocalPath,
							Managed:   repo.Managed,
							RepoDir:   repo.RepoDir,
							Base:      base,
							Write:     write,
						})
					}

					if len(rows) == 0 {
						if mode.JSON {
							return output.WriteJSON(commandWriter(cmd), []repoRow{})
						}
						msg := "no repos in workspace"
						if styles.Enabled {
							msg = styles.Render(styles.Muted, msg)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
							return err
						}
						registerWorkspace(&cfg, wsConfig.Name, wsRoot, time.Now())
						return config.SaveGlobal(cfgPath, cfg)
					}

					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), rows)
					}
					tableRows := make([][]string, 0, len(rows))
					for _, row := range rows {
						managed := "no"
						if row.Managed {
							managed = "yes"
						}
						tableRows = append(tableRows, []string{row.Name, row.LocalPath, managed, row.RepoDir, row.Base, row.Write})
					}
					rendered := output.RenderTable(styles, []string{"NAME", "LOCAL_PATH", "MANAGED", "REPO_DIR", "BASE", "WRITE"}, tableRows)
					if _, err := fmt.Fprint(commandWriter(cmd), rendered); err != nil {
						return err
					}

					registerWorkspace(&cfg, wsConfig.Name, wsRoot, time.Now())
					return config.SaveGlobal(cfgPath, cfg)
				},
			},
			{
				Name:      "add",
				Usage:     "Add a repo to the workspace and clone it (requires -w)",
				ArgsUsage: "-w <workspace> <alias|url|path>",
				Flags: appendOutputFlags([]cli.Flag{
					workspaceFlag(true),
					&cli.StringFlag{
						Name:  "name",
						Usage: "Override repo name",
					},
					&cli.StringFlag{
						Name:  "repo-dir",
						Usage: "Directory name for the repo within the workspace",
					},
				}),
				ShellComplete: func(ctx context.Context, cmd *cli.Command) {
					if cmd.NArg() == 0 {
						completeRepoAliases(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg, cfgPath, err := loadGlobal(cmd)
					if err != nil {
						return err
					}
					wsRoot, wsConfig, err := resolveWorkspace(cmd, &cfg, cfgPath)
					if err != nil {
						return err
					}

					raw := strings.TrimSpace(cmd.Args().First())
					name := strings.TrimSpace(cmd.String("name"))
					nameProvided := cmd.IsSet("name")
					sourcePath := ""

					if raw == "" {
						return usageError(ctx, cmd, "repo alias or source required")
					}
					url := ""
					if alias, ok := cfg.Repos[raw]; ok {
						url = alias.URL
						name = raw
						sourcePath = alias.Path
						if sourcePath == "" && looksLikeLocalPath(url) {
							sourcePath = url
							url = ""
							alias.Path = sourcePath
							alias.URL = ""
							cfg.Repos[raw] = alias
						}
					} else if looksLikeURL(raw) {
						url = raw
					} else {
						sourcePath = raw
					}
					if sourcePath == "" && url != "" && looksLikeLocalPath(url) {
						sourcePath = url
						url = ""
					}
					if sourcePath != "" {
						resolved, err := resolveLocalPathInput(sourcePath)
						if err != nil {
							return err
						}
						sourcePath = resolved
						if !nameProvided && name == "" {
							name = filepath.Base(sourcePath)
						}
						if alias, ok := cfg.Repos[name]; ok {
							if alias.Path != sourcePath {
								alias.Path = sourcePath
								alias.URL = ""
								cfg.Repos[name] = alias
							}
						}
					}
					if name == "" {
						name = ops.DeriveRepoNameFromURL(url)
					}
					if nameProvided {
						name = strings.TrimSpace(cmd.String("name"))
					}

					defaultBranch := cfg.Defaults.BaseBranch
					if alias, ok := cfg.Repos[name]; ok && alias.DefaultBranch != "" {
						defaultBranch = alias.DefaultBranch
					}

					input := ops.AddRepoInput{
						WorkspaceRoot: wsRoot,
						Name:          name,
						URL:           url,
						SourcePath:    sourcePath,
						RepoDir:       cmd.String("repo-dir"),
						Defaults:      cfg.Defaults,
						Remotes: config.Remotes{
							Base: config.RemoteConfig{
								DefaultBranch: defaultBranch,
							},
							Write: config.RemoteConfig{
								DefaultBranch: defaultBranch,
							},
						},
						Git: git.NewGoGitClient(),
					}

					if _, err := ops.AddRepo(ctx, input); err != nil {
						return err
					}

					registerWorkspace(&cfg, wsConfig.Name, wsRoot, time.Now())
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					localPath := sourcePath
					managed := false
					if localPath == "" {
						localPath = filepath.Join(cfg.Defaults.RepoStoreRoot, name)
						managed = true
					}
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]any{
							"status":     "ok",
							"workspace":  wsConfig.Name,
							"repo":       name,
							"local_path": localPath,
							"managed":    managed,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					repoDir := input.RepoDir
					if repoDir == "" {
						repoDir = name
					}
					branch := cfg.Defaults.BaseBranch
					if loaded, err := workspace.Load(wsRoot, cfg.Defaults); err == nil {
						branch = loaded.State.CurrentBranch
					}
					if branch == "" {
						branch = defaultBranch
					}
					worktreePath := workspace.RepoWorktreePath(wsRoot, branch, repoDir)
					msg := fmt.Sprintf("added %s to %s", name, wsConfig.Name)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					localLine := fmt.Sprintf("local: %s", localPath)
					if styles.Enabled {
						localLine = styles.Render(styles.Muted, localLine)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), localLine); err != nil {
						return err
					}
					if managed {
						note := fmt.Sprintf("note: cloned into repo store (%s)", cfg.Defaults.RepoStoreRoot)
						if styles.Enabled {
							note = styles.Render(styles.Muted, note)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), note); err != nil {
							return err
						}
					}
					if worktreePath != "" {
						line := fmt.Sprintf("worktree: %s", worktreePath)
						if styles.Enabled {
							line = styles.Render(styles.Muted, line)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), line); err != nil {
							return err
						}
					}
					return nil
				},
			},
			{
				Name:  "remotes",
				Usage: "Update repo remotes in a workspace (requires -w)",
				Commands: []*cli.Command{
					{
						Name:      "set",
						Usage:     "Update remotes for a repo (requires -w)",
						ArgsUsage: "-w <workspace> <name>",
						Flags: appendOutputFlags([]cli.Flag{
							workspaceFlag(true),
							&cli.StringFlag{
								Name:  "base-remote",
								Usage: "Base remote name",
							},
							&cli.StringFlag{
								Name:  "write-remote",
								Usage: "Write remote name",
							},
							&cli.StringFlag{
								Name:  "base-branch",
								Usage: "Base branch name (also updates write branch)",
							},
						}),
						ShellComplete: func(ctx context.Context, cmd *cli.Command) {
							if cmd.NArg() == 0 {
								completeWorkspaceRepoNames(cmd)
							}
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							name := strings.TrimSpace(cmd.Args().First())
							if name == "" {
								return usageError(ctx, cmd, "usage: workset repo remotes set -w <workspace> <name>")
							}
							cfg, cfgPath, err := loadGlobal(cmd)
							if err != nil {
								return err
							}
							wsRoot, wsConfig, err := resolveWorkspace(cmd, &cfg, cfgPath)
							if err != nil {
								return err
							}

							baseRemoteSet := cmd.IsSet("base-remote")
							writeRemoteSet := cmd.IsSet("write-remote")
							baseBranchSet := cmd.IsSet("base-branch")
							if !baseRemoteSet && !writeRemoteSet && !baseBranchSet {
								return usageError(ctx, cmd, "at least one remote setting required")
							}

							baseBranch := cmd.String("base-branch")
							input := ops.UpdateRepoRemotesInput{
								WorkspaceRoot:  wsRoot,
								Name:           name,
								Defaults:       cfg.Defaults,
								BaseRemote:     cmd.String("base-remote"),
								WriteRemote:    cmd.String("write-remote"),
								BaseBranch:     baseBranch,
								WriteBranch:    baseBranch,
								BaseRemoteSet:  baseRemoteSet,
								WriteRemoteSet: writeRemoteSet,
								BaseBranchSet:  baseBranchSet,
								WriteBranchSet: baseBranchSet,
							}

							updated, err := ops.UpdateRepoRemotes(input)
							if err != nil {
								return err
							}

							registerWorkspace(&cfg, wsConfig.Name, wsRoot, time.Now())
							if err := config.SaveGlobal(cfgPath, cfg); err != nil {
								return err
							}

							var updatedRepo config.RepoConfig
							found := false
							for _, repo := range updated.Repos {
								if repo.Name == name {
									updatedRepo = repo
									found = true
									break
								}
							}
							if !found {
								return cli.Exit("repo not found after update", 1)
							}

							base := updatedRepo.Remotes.Base.Name
							if updatedRepo.Remotes.Base.DefaultBranch != "" {
								base = fmt.Sprintf("%s/%s", base, updatedRepo.Remotes.Base.DefaultBranch)
							}
							write := updatedRepo.Remotes.Write.Name
							if updatedRepo.Remotes.Write.DefaultBranch != "" {
								write = fmt.Sprintf("%s/%s", write, updatedRepo.Remotes.Write.DefaultBranch)
							}

							mode := outputModeFromContext(cmd)
							if mode.JSON {
								return output.WriteJSON(commandWriter(cmd), map[string]any{
									"status":    "ok",
									"workspace": wsConfig.Name,
									"repo":      name,
									"base":      base,
									"write":     write,
								})
							}
							styles := output.NewStyles(commandWriter(cmd), mode.Plain)
							msg := fmt.Sprintf("updated remotes for %s", name)
							if styles.Enabled {
								msg = styles.Render(styles.Success, msg)
							}
							if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
								return err
							}
							if base != "" {
								line := fmt.Sprintf("base: %s", base)
								if styles.Enabled {
									line = styles.Render(styles.Muted, line)
								}
								if _, err := fmt.Fprintln(commandWriter(cmd), line); err != nil {
									return err
								}
							}
							if write != "" {
								line := fmt.Sprintf("write: %s", write)
								if styles.Enabled {
									line = styles.Render(styles.Muted, line)
								}
								if _, err := fmt.Fprintln(commandWriter(cmd), line); err != nil {
									return err
								}
							}
							return nil
						},
					},
				},
			},
			{
				Name:      "rm",
				Aliases:   []string{"remove"},
				Usage:     "Remove a repo from a workspace",
				ArgsUsage: "-w <workspace> <name>",
				Flags: appendOutputFlags([]cli.Flag{
					workspaceFlag(true),
					&cli.BoolFlag{
						Name:  "delete-worktrees",
						Usage: "Delete repo worktrees under worktrees/",
					},
					&cli.BoolFlag{
						Name:  "delete-local",
						Usage: "Delete the repo local_path (managed repos only)",
					},
					&cli.BoolFlag{
						Name:  "force",
						Usage: "Bypass safety checks",
					},
					&cli.BoolFlag{
						Name:  "yes",
						Usage: "Skip confirmation",
					},
				}),
				ShellComplete: func(ctx context.Context, cmd *cli.Command) {
					if cmd.NArg() == 0 {
						completeWorkspaceRepoNames(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().First())
					if name == "" {
						return usageError(ctx, cmd, "usage: workset repo rm -w <workspace> <name>")
					}
					cfg, cfgPath, err := loadGlobal(cmd)
					if err != nil {
						return err
					}
					wsRoot, wsConfig, err := resolveWorkspace(cmd, &cfg, cfgPath)
					if err != nil {
						return err
					}
					repoCfg, ok := findRepo(wsConfig, name)
					if !ok {
						return cli.Exit("repo not found in workspace (use `workset repo ls -w <workspace>` to list)", 1)
					}

					report, err := ops.CheckRepoSafety(ctx, ops.RepoSafetyInput{
						WorkspaceRoot: wsRoot,
						Repo:          repoCfg,
						Defaults:      cfg.Defaults,
						Git:           git.NewGoGitClient(),
						FetchRemotes:  true,
					})
					if err != nil {
						return err
					}

					dirty, unmerged, unpushed, warnings := summarizeRepoSafety(report)
					for _, warning := range warnings {
						_, _ = fmt.Fprintln(os.Stderr, "warning:", warning)
					}
					for _, branch := range unpushed {
						_, _ = fmt.Fprintf(os.Stderr, "warning: branch %s has commits not on write remote\n", branch)
					}

					deleteWorktrees := cmd.Bool("delete-worktrees")
					deleteLocal := cmd.Bool("delete-local")

					if (deleteWorktrees || deleteLocal) && !cmd.Bool("force") {
						if len(dirty) > 0 {
							return fmt.Errorf("refusing to delete: dirty worktrees: %s (use --force)", strings.Join(dirty, ", "))
						}
						if len(unmerged) > 0 {
							for _, detail := range unmergedRepoDetails(report) {
								_, _ = fmt.Fprintln(os.Stderr, "detail:", detail)
							}
							return fmt.Errorf("refusing to delete: unmerged branches: %s (use --force)", strings.Join(unmerged, ", "))
						}
						if deleteLocal && !repoCfg.Managed {
							return fmt.Errorf("refusing to delete unmanaged repo at %s (use --force to override)", repoCfg.LocalPath)
						}
					}

					if deleteWorktrees || deleteLocal {
						if !cmd.Bool("yes") {
							prompt := fmt.Sprintf("remove repo %s", name)
							if deleteWorktrees {
								prompt += " and delete worktrees"
							}
							if deleteLocal {
								prompt += " and local repo"
							}
							ok, err := confirmPrompt(os.Stdin, commandWriter(cmd), prompt+"? [y/N] ")
							if err != nil {
								return err
							}
							if !ok {
								return cli.Exit("aborted", 1)
							}
						}
					}

					if _, err := ops.RemoveRepo(ctx, ops.RemoveRepoInput{
						WorkspaceRoot:   wsRoot,
						Name:            name,
						Defaults:        cfg.Defaults,
						Git:             git.NewGoGitClient(),
						DeleteWorktrees: deleteWorktrees,
						DeleteLocal:     deleteLocal,
						Logf: func(format string, args ...any) {
							if verboseEnabled(cmd) {
								_, _ = fmt.Fprintf(commandErrWriter(cmd), format+"\n", args...)
							}
						},
					}); err != nil {
						return err
					}

					registerWorkspace(&cfg, wsConfig.Name, wsRoot, time.Now())
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]any{
							"status":    "ok",
							"workspace": wsConfig.Name,
							"repo":      name,
							"deleted": map[string]bool{
								"worktrees": deleteWorktrees,
								"local":     deleteLocal,
							},
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("removed %s from %s", name, wsConfig.Name)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					if deleteWorktrees || deleteLocal {
						detail := fmt.Sprintf("deleted worktrees: %t, deleted local repo: %t", deleteWorktrees, deleteLocal)
						if styles.Enabled {
							detail = styles.Render(styles.Muted, detail)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), detail); err != nil {
							return err
						}
					}
					return nil
				},
			},
		},
	}
}

func repoAliasCommand() *cli.Command {
	return &cli.Command{
		Name:  "alias",
		Usage: "Manage repo aliases in config",
		Commands: []*cli.Command{
			{
				Name:  "ls",
				Usage: "List repo aliases",
				Flags: outputFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg, _, err := loadGlobal(cmd)
					if err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					if len(cfg.Repos) == 0 {
						if mode.JSON {
							return output.WriteJSON(commandWriter(cmd), []any{})
						}
						msg := "no repo aliases defined"
						if styles.Enabled {
							msg = styles.Render(styles.Muted, msg)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
							return err
						}
						return nil
					}
					names := make([]string, 0, len(cfg.Repos))
					for name := range cfg.Repos {
						names = append(names, name)
					}
					sort.Strings(names)
					if mode.JSON {
						type row struct {
							Name          string `json:"name"`
							URL           string `json:"url,omitempty"`
							Path          string `json:"path,omitempty"`
							DefaultBranch string `json:"default_branch,omitempty"`
						}
						rows := make([]row, 0, len(names))
						for _, name := range names {
							alias := cfg.Repos[name]
							rows = append(rows, row{
								Name:          name,
								URL:           alias.URL,
								Path:          alias.Path,
								DefaultBranch: alias.DefaultBranch,
							})
						}
						return output.WriteJSON(commandWriter(cmd), rows)
					}

					rows := make([][]string, 0, len(names))
					for _, name := range names {
						alias := cfg.Repos[name]
						source := alias.URL
						if alias.Path != "" {
							source = alias.Path
						}
						if source == "" {
							source = "-"
						}
						rows = append(rows, []string{name, source, alias.DefaultBranch})
					}
					rendered := output.RenderTable(styles, []string{"NAME", "SOURCE", "DEFAULT_BRANCH"}, rows)
					_, err = fmt.Fprint(commandWriter(cmd), rendered)
					return err
				},
			},
			{
				Name:        "add",
				Usage:       "Create a repo alias",
				ArgsUsage:   "<name> <source>",
				UsageText:   "workset repo alias add <name> <source> [--default-branch <branch>]",
				Description: "Create a new alias for a repo path or URL. Use `workset repo alias set` to update an existing alias.",
				Flags: appendOutputFlags([]cli.Flag{
					&cli.StringFlag{
						Name:  "default-branch",
						Usage: "Default branch name",
					},
				}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().Get(0))
					source := strings.TrimSpace(cmd.Args().Get(1))
					if name == "" {
						return usageError(ctx, cmd, "alias name required (example: workset repo alias add ask-gill git@github.com:org/repo.git)")
					}
					if source == "" {
						return usageError(ctx, cmd, fmt.Sprintf("source required to create alias %q (path or URL). Example: workset repo alias add --default-branch staging %s git@github.com:org/repo.git", name, name))
					}
					cfg, cfgPath, err := loadGlobal(cmd)
					if err != nil {
						return err
					}
					if cfg.Repos != nil {
						if _, ok := cfg.Repos[name]; ok {
							return cli.Exit(fmt.Sprintf("repo alias %q already exists; use 'workset repo alias set' to update it", name), 1)
						}
					}
					url := ""
					path := ""
					if looksLikeURL(source) {
						url = source
					} else {
						resolved, err := resolveLocalPathInput(source)
						if err != nil {
							return err
						}
						path = resolved
					}
					if cfg.Repos == nil {
						cfg.Repos = map[string]config.RepoAlias{}
					}
					defaultBranch := strings.TrimSpace(cmd.String("default-branch"))
					if defaultBranch == "" {
						defaultBranch = cfg.Defaults.BaseBranch
					}
					cfg.Repos[name] = config.RepoAlias{
						URL:           url,
						Path:          path,
						DefaultBranch: defaultBranch,
					}
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status": "ok",
							"name":   name,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("alias %s saved", name)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:        "set",
				Usage:       "Update a repo alias",
				ArgsUsage:   "<name> [source]",
				UsageText:   "workset repo alias set <name> [source] [--default-branch <branch>]",
				Description: "Update an existing alias. Omit source to keep the current path/URL.",
				Flags: appendOutputFlags([]cli.Flag{
					&cli.StringFlag{
						Name:  "default-branch",
						Usage: "Default branch name",
					},
				}),
				ShellComplete: func(ctx context.Context, cmd *cli.Command) {
					if cmd.NArg() == 0 {
						completeRepoAliases(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().Get(0))
					if name == "" {
						return usageError(ctx, cmd, "alias name required (example: workset repo alias set ask-gill --default-branch staging)")
					}
					cfg, cfgPath, err := loadGlobal(cmd)
					if err != nil {
						return err
					}
					alias, ok := cfg.Repos[name]
					if !ok {
						return cli.Exit(fmt.Sprintf("repo alias %q not found; use 'workset repo alias add' to create it", name), 1)
					}
					updated := false
					source := strings.TrimSpace(cmd.Args().Get(1))
					if source != "" {
						if looksLikeURL(source) {
							alias.URL = source
							alias.Path = ""
						} else {
							resolved, err := resolveLocalPathInput(source)
							if err != nil {
								return err
							}
							alias.Path = resolved
							alias.URL = ""
						}
						updated = true
					}
					if cmd.IsSet("default-branch") {
						defaultBranch := strings.TrimSpace(cmd.String("default-branch"))
						if defaultBranch == "" {
							return cli.Exit("default branch cannot be empty", 1)
						}
						alias.DefaultBranch = defaultBranch
						updated = true
					}
					if !updated {
						return usageError(ctx, cmd, "no updates specified (provide a new source or --default-branch)")
					}
					cfg.Repos[name] = alias
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status": "ok",
							"name":   name,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("alias %s updated", name)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:      "rm",
				Usage:     "Remove a repo alias",
				ArgsUsage: "<name>",
				Flags:     outputFlags(),
				ShellComplete: func(ctx context.Context, cmd *cli.Command) {
					if cmd.NArg() == 0 {
						completeRepoAliases(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().Get(0))
					if name == "" {
						return usageError(ctx, cmd, "usage: workset repo alias rm <name>")
					}
					cfg, cfgPath, err := loadGlobal(cmd)
					if err != nil {
						return err
					}
					if _, ok := cfg.Repos[name]; !ok {
						return cli.Exit("repo alias not found", 1)
					}
					delete(cfg.Repos, name)
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status": "ok",
							"name":   name,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("alias %s removed", name)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					return nil
				},
			},
		},
	}
}
