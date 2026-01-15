package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/groups"
	"github.com/strantalis/workset/internal/ops"
	"github.com/strantalis/workset/internal/output"
	"github.com/strantalis/workset/internal/workspace"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

func main() {
	root := &cli.Command{
		Name:        "workset",
		Usage:       "Manage multi-repo workspaces with predictable defaults",
		Description: "Workspace commands require -w/--workspace (or defaults.workspace) to target a workspace.",
		Flags: []cli.Flag{
			workspaceFlag(),
			&cli.StringFlag{
				Name:  "config",
				Usage: "Override global config path",
			},
		},
		Commands: []*cli.Command{
			newCommand(),
			initCommand(),
			listCommand(),
			removeWorkspaceCommand(),
			configCommand(),
			groupCommand(),
			repoCommand(),
			statusCommand(),
		},
	}

	args := normalizeArgs(root, os.Args)
	if err := root.Run(context.Background(), args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newCommand() *cli.Command {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:  "path",
			Usage: "Target directory (defaults to ./<name>)",
		},
	}
	flags = append(flags, outputFlags()...)
	return &cli.Command{
		Name:  "new",
		Usage: "Create a new workspace in a new directory",
		Flags: flags,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			name := strings.TrimSpace(cmd.Args().First())
			if name == "" {
				return cli.Exit("workspace name required", 1)
			}

			cfg, cfgPath, err := loadGlobal(cmd.String("config"))
			if err != nil {
				return err
			}

			root := cmd.String("path")
			if root == "" {
				base := cfg.Defaults.WorkspaceRoot
				if base == "" {
					cwd, err := os.Getwd()
					if err != nil {
						return err
					}
					base = cwd
				}
				root = filepath.Join(base, name)
			}
			root, err = filepath.Abs(root)
			if err != nil {
				return err
			}

			if _, err := workspace.Init(root, name, cfg.Defaults); err != nil {
				return err
			}

			warnOutsideWorkspaceRoot(root, cfg.Defaults.WorkspaceRoot)
			info := output.WorkspaceCreated{
				Name:    name,
				Path:    root,
				Workset: workspace.WorksetFile(root),
				Branch:  cfg.Defaults.BaseBranch,
				Next:    fmt.Sprintf("workset repo add -w %s <alias|url>", name),
			}
			mode := outputModeFromContext(cmd)
			if err := printWorkspaceCreated(commandWriter(cmd), info, mode.JSON, mode.Plain); err != nil {
				return err
			}

			registerWorkspace(&cfg, name, root, time.Now())
			return config.SaveGlobal(cfgPath, cfg)
		},
	}
}

func initCommand() *cli.Command {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:  "name",
			Usage: "Workspace name (defaults to directory name)",
		},
	}
	flags = append(flags, outputFlags()...)
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize a workspace in the current directory",
		Flags: flags,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			cfg, cfgPath, err := loadGlobal(cmd.String("config"))
			if err != nil {
				return err
			}

			name := strings.TrimSpace(cmd.String("name"))
			if name == "" {
				name = filepath.Base(cwd)
			}

			exists, err := config.WorkspaceExists(workspace.WorksetFile(cwd))
			if err != nil {
				return err
			}
			if exists {
				return cli.Exit("workset.yaml already exists in this directory", 1)
			}

			if _, err := workspace.Init(cwd, name, cfg.Defaults); err != nil {
				return err
			}
			warnOutsideWorkspaceRoot(cwd, cfg.Defaults.WorkspaceRoot)
			info := output.WorkspaceCreated{
				Name:    name,
				Path:    cwd,
				Workset: workspace.WorksetFile(cwd),
				Branch:  cfg.Defaults.BaseBranch,
				Next:    fmt.Sprintf("workset repo add -w %s <alias|url>", name),
			}
			mode := outputModeFromContext(cmd)
			if err := printWorkspaceCreated(commandWriter(cmd), info, mode.JSON, mode.Plain); err != nil {
				return err
			}
			registerWorkspace(&cfg, name, cwd, time.Now())
			return config.SaveGlobal(cfgPath, cfg)
		},
	}
}

func listCommand() *cli.Command {
	flags := outputFlags()
	return &cli.Command{
		Name:  "ls",
		Usage: "List registered workspaces",
		Flags: flags,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, _, err := loadGlobal(cmd.String("config"))
			if err != nil {
				return err
			}
			mode := outputModeFromContext(cmd)
			styles := output.NewStyles(commandWriter(cmd), mode.Plain)
			if len(cfg.Workspaces) == 0 {
				if mode.JSON {
					return output.WriteJSON(commandWriter(cmd), []any{})
				}
				msg := "no workspaces registered"
				if styles.Enabled {
					msg = styles.Render(styles.Muted, msg)
				}
				if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
					return err
				}
				return nil
			}
			names := make([]string, 0, len(cfg.Workspaces))
			for name := range cfg.Workspaces {
				names = append(names, name)
			}
			sort.Strings(names)
			if mode.JSON {
				type workspaceRow struct {
					Name      string `json:"name"`
					Path      string `json:"path"`
					CreatedAt string `json:"created_at,omitempty"`
					LastUsed  string `json:"last_used,omitempty"`
				}
				rows := make([]workspaceRow, 0, len(names))
				for _, name := range names {
					ref := cfg.Workspaces[name]
					rows = append(rows, workspaceRow{
						Name:      name,
						Path:      ref.Path,
						CreatedAt: ref.CreatedAt,
						LastUsed:  ref.LastUsed,
					})
				}
				return output.WriteJSON(commandWriter(cmd), rows)
			}

			rows := make([][]string, 0, len(names))
			for _, name := range names {
				ref := cfg.Workspaces[name]
				rows = append(rows, []string{name, ref.Path})
			}
			rendered := output.RenderTable(styles, []string{"NAME", "PATH"}, rows)
			_, err = fmt.Fprint(commandWriter(cmd), rendered)
			return err
		},
	}
}

func configCommand() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Manage config",
		Commands: []*cli.Command{
			{
				Name:  "show",
				Usage: "Print the global config",
				Flags: outputFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg, _, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), cfg)
					}
					data, err := yaml.Marshal(cfg)
					if err != nil {
						return err
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					if styles.Enabled {
						if _, err := fmt.Fprintln(commandWriter(cmd), styles.Render(styles.Title, "config")); err != nil {
							return err
						}
					}
					_, err = fmt.Fprintln(commandWriter(cmd), string(data))
					return err
				},
			},
			{
				Name:  "set",
				Usage: "Set a global config value (defaults.* only)",
				Flags: outputFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					key := strings.TrimSpace(cmd.Args().Get(0))
					value := strings.TrimSpace(cmd.Args().Get(1))
					if key == "" || value == "" {
						return cli.Exit("usage: workset config set <key> <value>", 1)
					}
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					if err := setGlobalDefault(&cfg, key, value); err != nil {
						return err
					}
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status": "ok",
							"key":    key,
							"value":  value,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("updated %s = %s", key, value)
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

func groupCommand() *cli.Command {
	return &cli.Command{
		Name:    "template",
		Aliases: []string{"group"},
		Usage:   "Manage repo templates (from-workspace/apply require -w)",
		Commands: []*cli.Command{
			{
				Name:  "ls",
				Usage: "List templates",
				Flags: outputFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg, _, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					names := groups.List(cfg)
					mode := outputModeFromContext(cmd)
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					if len(names) == 0 {
						if mode.JSON {
							return output.WriteJSON(commandWriter(cmd), []any{})
						}
						msg := "no templates defined"
						if styles.Enabled {
							msg = styles.Render(styles.Muted, msg)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
							return err
						}
						return nil
					}
					if mode.JSON {
						type row struct {
							Name        string `json:"name"`
							Description string `json:"description,omitempty"`
							RepoCount   int    `json:"repo_count"`
						}
						rows := make([]row, 0, len(names))
						for _, name := range names {
							group, _ := groups.Get(cfg, name)
							rows = append(rows, row{
								Name:        name,
								Description: group.Description,
								RepoCount:   len(group.Members),
							})
						}
						return output.WriteJSON(commandWriter(cmd), rows)
					}
					rows := make([][]string, 0, len(names))
					for _, name := range names {
						group, _ := groups.Get(cfg, name)
						desc := group.Description
						if desc == "" {
							desc = "-"
						}
						rows = append(rows, []string{name, desc, fmt.Sprintf("%d", len(group.Members))})
					}
					rendered := output.RenderTable(styles, []string{"NAME", "DESCRIPTION", "REPOS"}, rows)
					_, err = fmt.Fprint(commandWriter(cmd), rendered)
					return err
				},
			},
			{
				Name:  "show",
				Usage: "Show a template",
				Flags: outputFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().First())
					if name == "" {
						return cli.Exit("template name required", 1)
					}
					cfg, _, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					group, ok := groups.Get(cfg, name)
					if !ok {
						return cli.Exit("template not found", 1)
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						payload := map[string]any{
							"name":        name,
							"description": group.Description,
							"members":     group.Members,
						}
						return output.WriteJSON(commandWriter(cmd), payload)
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					header := name
					if styles.Enabled {
						header = styles.Render(styles.Title, name)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), header); err != nil {
						return err
					}
					if group.Description != "" {
						desc := group.Description
						if styles.Enabled {
							desc = styles.Render(styles.Muted, desc)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), desc); err != nil {
							return err
						}
					}
					rows := make([][]string, 0, len(group.Members))
					for _, member := range group.Members {
						rows = append(rows, []string{member.Repo})
					}
					if len(rows) == 0 {
						msg := "no repos in template"
						if styles.Enabled {
							msg = styles.Render(styles.Muted, msg)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
							return err
						}
						return nil
					}
					rendered := output.RenderTable(styles, []string{"REPO"}, rows)
					_, err = fmt.Fprint(commandWriter(cmd), rendered)
					return err
				},
			},
			{
				Name:  "create",
				Usage: "Create or update a template",
				Flags: appendOutputFlags([]cli.Flag{
					&cli.StringFlag{
						Name:  "description",
						Usage: "Template description",
					},
				}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().First())
					if name == "" {
						return cli.Exit("template name required", 1)
					}
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					if err := groups.Upsert(&cfg, name, cmd.String("description")); err != nil {
						return err
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
					msg := fmt.Sprintf("template %s saved", name)
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
				Name:  "rm",
				Usage: "Remove a template",
				Flags: outputFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().First())
					if name == "" {
						return cli.Exit("template name required", 1)
					}
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					if err := groups.Delete(&cfg, name); err != nil {
						return err
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
					msg := fmt.Sprintf("template %s removed", name)
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
				Name:  "add",
				Usage: "Add a repo to a template",
				Flags: appendOutputFlags([]cli.Flag{
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
						Usage: "Base branch name",
					},
				}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					groupName := strings.TrimSpace(cmd.Args().Get(0))
					repoName := strings.TrimSpace(cmd.Args().Get(1))
					if groupName == "" || repoName == "" {
						return cli.Exit("template and repo name required", 1)
					}
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					member := config.GroupMember{
						Repo: repoName,
						Remotes: config.Remotes{
							Base: config.RemoteConfig{
								Name:          cmd.String("base-remote"),
								DefaultBranch: cmd.String("base-branch"),
							},
							Write: config.RemoteConfig{
								Name: cmd.String("write-remote"),
							},
						},
					}
					if err := groups.AddMember(&cfg, groupName, member); err != nil {
						return err
					}
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status":   "ok",
							"template": groupName,
							"repo":     repoName,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("added %s to %s", repoName, groupName)
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
				Name:  "remove",
				Usage: "Remove a repo from a template",
				Flags: outputFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					groupName := strings.TrimSpace(cmd.Args().Get(0))
					repoName := strings.TrimSpace(cmd.Args().Get(1))
					if groupName == "" || repoName == "" {
						return cli.Exit("template and repo name required", 1)
					}
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					if err := groups.RemoveMember(&cfg, groupName, repoName); err != nil {
						return err
					}
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status":   "ok",
							"template": groupName,
							"repo":     repoName,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("removed %s from %s", repoName, groupName)
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
				Name:  "from-workspace",
				Usage: "Snapshot a workspace into a template (requires -w)",
				Flags: appendOutputFlags([]cli.Flag{workspaceFlag()}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					groupName := strings.TrimSpace(cmd.Args().First())
					if groupName == "" {
						return cli.Exit("template name required", 1)
					}
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					_, wsConfig, err := resolveWorkspace(cmd, &cfg, cfgPath)
					if err != nil {
						return err
					}
					if err := groups.FromWorkspace(&cfg, groupName, wsConfig); err != nil {
						return err
					}
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status":   "ok",
							"template": groupName,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("template %s updated from workspace", groupName)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					_, err = fmt.Fprintln(commandWriter(cmd), msg)
					return err
				},
			},
			{
				Name:  "apply",
				Usage: "Apply a template to a workspace (requires -w)",
				Flags: appendOutputFlags([]cli.Flag{workspaceFlag()}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					groupName := strings.TrimSpace(cmd.Args().First())
					if groupName == "" {
						return cli.Exit("template name required", 1)
					}
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					wsRoot, wsConfig, err := resolveWorkspace(cmd, &cfg, cfgPath)
					if err != nil {
						return err
					}
					group, ok := groups.Get(cfg, groupName)
					if !ok {
						return cli.Exit("template not found", 1)
					}
					for _, member := range group.Members {
						alias, ok := cfg.Repos[member.Repo]
						if !ok {
							return fmt.Errorf("repo alias %q not found in config", member.Repo)
						}
						baseBranch := cfg.Defaults.BaseBranch
						if member.Remotes.Base.DefaultBranch != "" {
							baseBranch = member.Remotes.Base.DefaultBranch
						} else if alias.DefaultBranch != "" {
							baseBranch = alias.DefaultBranch
						}
						remotes := config.Remotes{
							Base: config.RemoteConfig{
								Name:          member.Remotes.Base.Name,
								DefaultBranch: baseBranch,
							},
							Write: config.RemoteConfig{
								Name:          member.Remotes.Write.Name,
								DefaultBranch: baseBranch,
							},
						}
						if remotes.Base.Name == "" {
							remotes.Base.Name = cfg.Defaults.Remotes.Base
						}
						if remotes.Write.Name == "" {
							remotes.Write.Name = cfg.Defaults.Remotes.Write
						}

						if _, err := ops.AddRepo(ctx, ops.AddRepoInput{
							WorkspaceRoot: wsRoot,
							Name:          member.Repo,
							URL:           alias.URL,
							SourcePath:    alias.Path,
							Defaults:      cfg.Defaults,
							Remotes:       remotes,
							Git:           git.NewGoGitClient(),
						}); err != nil {
							return err
						}
					}
					registerWorkspace(&cfg, wsConfig.Name, wsRoot, time.Now())
					if err := config.SaveGlobal(cfgPath, cfg); err != nil {
						return err
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), map[string]string{
							"status":    "ok",
							"template":  groupName,
							"workspace": wsConfig.Name,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("template %s applied to %s", groupName, wsConfig.Name)
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

func repoCommand() *cli.Command {
	return &cli.Command{
		Name:  "repo",
		Usage: "Manage repos in a workspace (requires -w)",
		Commands: []*cli.Command{
			repoAliasCommand(),
			{
				Name:    "ls",
				Aliases: []string{"list"},
				Usage:   "List repos in a workspace (requires -w)",
				Flags:   appendOutputFlags([]cli.Flag{workspaceFlag()}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
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
				Name:  "add",
				Usage: "Add a repo to the workspace and clone it (requires -w)",
				Flags: appendOutputFlags([]cli.Flag{
					workspaceFlag(),
					&cli.StringFlag{
						Name:  "name",
						Usage: "Override repo name",
					},
					&cli.StringFlag{
						Name:  "repo-dir",
						Usage: "Directory name for the repo within worktrees/<feature>",
					},
				}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
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
						return cli.Exit("repo alias or source required", 1)
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
								Name:          cfg.Defaults.Remotes.Base,
								DefaultBranch: defaultBranch,
							},
							Write: config.RemoteConfig{
								Name:          cfg.Defaults.Remotes.Write,
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
					worktreePath := ""
					if branch != "" && branch != defaultBranch {
						worktreePath = workspace.RepoWorktreePath(wsRoot, branch, repoDir)
					}
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
				Name:    "rm",
				Aliases: []string{"remove"},
				Usage:   "Remove a repo from a workspace",
				Flags: appendOutputFlags([]cli.Flag{
					workspaceFlag(),
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
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().First())
					if name == "" {
						return cli.Exit("usage: workset repo rm -w <workspace> <name>", 1)
					}
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
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

func removeWorkspaceCommand() *cli.Command {
	return &cli.Command{
		Name:  "rm",
		Usage: "Remove a workspace (use --delete to remove files)",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(),
			&cli.BoolFlag{
				Name:  "delete",
				Usage: "Delete the workspace directory",
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
		Action: func(ctx context.Context, cmd *cli.Command) error {
			arg := strings.TrimSpace(cmd.Args().First())
			if arg == "" {
				arg = strings.TrimSpace(cmd.String("workspace"))
			}
			cfg, cfgPath, err := loadGlobal(cmd.String("config"))
			if err != nil {
				return err
			}
			name, root, err := resolveWorkspaceTarget(arg, &cfg)
			if err != nil {
				return err
			}

			deleteRequested := cmd.Bool("delete")
			if deleteRequested {
				workspaceRoot := cfg.Defaults.WorkspaceRoot
				if workspaceRoot != "" {
					absRoot, err := filepath.Abs(workspaceRoot)
					if err == nil {
						absRoot = filepath.Clean(absRoot)
						absTarget := filepath.Clean(root)
						inside := absTarget == absRoot || strings.HasPrefix(absTarget, absRoot+string(os.PathSeparator))
						if !inside && !cmd.Bool("force") {
							return fmt.Errorf("refusing to delete outside defaults.workspace_root (%s); use --force to override", absRoot)
						}
					}
				}

				report, err := ops.CheckWorkspaceSafety(ctx, ops.WorkspaceSafetyInput{
					WorkspaceRoot: root,
					Defaults:      cfg.Defaults,
					Git:           git.NewGoGitClient(),
					FetchRemotes:  true,
				})
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						_, _ = fmt.Fprintln(os.Stderr, "warning: workset.yaml not found; skipping safety checks")
					} else if !cmd.Bool("force") {
						return err
					} else {
						_, _ = fmt.Fprintln(os.Stderr, "warning:", err.Error())
					}
				}

				dirty, unmerged, unpushed, warnings := summarizeWorkspaceSafety(report)
				for _, warning := range warnings {
					_, _ = fmt.Fprintln(os.Stderr, "warning:", warning)
				}
				for _, branch := range unpushed {
					_, _ = fmt.Fprintf(os.Stderr, "warning: branch %s has commits not on write remote\n", branch)
				}

				if !cmd.Bool("force") {
					if len(dirty) > 0 {
						return fmt.Errorf("refusing to delete: dirty worktrees: %s (use --force)", strings.Join(dirty, ", "))
					}
					if len(unmerged) > 0 {
						return fmt.Errorf("refusing to delete: unmerged branches: %s (use --force)", strings.Join(unmerged, ", "))
					}
				}

				if !cmd.Bool("yes") {
					ok, err := confirmPrompt(os.Stdin, commandWriter(cmd), fmt.Sprintf("delete workspace %s? [y/N] ", root))
					if err != nil {
						return err
					}
					if !ok {
						return cli.Exit("aborted", 1)
					}
				}

				if err := os.RemoveAll(root); err != nil {
					return err
				}
			}

			if name != "" {
				delete(cfg.Workspaces, name)
			} else {
				removeWorkspaceByPath(&cfg, root)
			}
			if cfg.Defaults.Workspace == name || cfg.Defaults.Workspace == root {
				cfg.Defaults.Workspace = ""
			}
			if err := config.SaveGlobal(cfgPath, cfg); err != nil {
				return err
			}
			mode := outputModeFromContext(cmd)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), map[string]any{
					"status":        "ok",
					"name":          name,
					"path":          root,
					"deleted_files": deleteRequested,
				})
			}
			styles := output.NewStyles(commandWriter(cmd), mode.Plain)
			if deleteRequested {
				msg := fmt.Sprintf("workspace %s deleted", root)
				if styles.Enabled {
					msg = styles.Render(styles.Success, msg)
				}
				if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
					return err
				}
				return nil
			}
			msg := fmt.Sprintf("removed workspace registration for %s", root)
			if styles.Enabled {
				msg = styles.Render(styles.Success, msg)
			}
			if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
				return err
			}
			note := fmt.Sprintf("note: files remain on disk; to delete, run: workset rm -w %s --delete", root)
			if styles.Enabled {
				note = styles.Render(styles.Muted, note)
			}
			if _, err := fmt.Fprintln(commandWriter(cmd), note); err != nil {
				return err
			}
			return nil
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
					cfg, _, err := loadGlobal(cmd.String("config"))
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
				Name:  "add",
				Usage: "Add or update a repo alias",
				Flags: appendOutputFlags([]cli.Flag{
					&cli.StringFlag{
						Name:  "default-branch",
						Usage: "Default branch name",
					},
				}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().Get(0))
					source := strings.TrimSpace(cmd.Args().Get(1))
					if name == "" || source == "" {
						return cli.Exit("usage: workset repo alias add <name> <source>", 1)
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
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					if cfg.Repos == nil {
						cfg.Repos = map[string]config.RepoAlias{}
					}
					cfg.Repos[name] = config.RepoAlias{
						URL:           url,
						Path:          path,
						DefaultBranch: strings.TrimSpace(cmd.String("default-branch")),
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
				Name:  "set",
				Usage: "Update an existing repo alias",
				Flags: appendOutputFlags([]cli.Flag{
					&cli.StringFlag{
						Name:  "default-branch",
						Usage: "Default branch name",
					},
				}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().Get(0))
					if name == "" {
						return cli.Exit("usage: workset repo alias set [--default-branch <branch>] <name> [source]", 1)
					}
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
					if err != nil {
						return err
					}
					alias, ok := cfg.Repos[name]
					if !ok {
						return cli.Exit("repo alias not found", 1)
					}
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
					}
					if branch := strings.TrimSpace(cmd.String("default-branch")); branch != "" {
						alias.DefaultBranch = branch
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
				Name:  "rm",
				Usage: "Remove a repo alias",
				Flags: outputFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().Get(0))
					if name == "" {
						return cli.Exit("usage: workset repo alias rm <name>", 1)
					}
					cfg, cfgPath, err := loadGlobal(cmd.String("config"))
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

func statusCommand() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show status for repos in a workspace (requires -w)",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(),
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, cfgPath, err := loadGlobal(cmd.String("config"))
			if err != nil {
				return err
			}
			wsRoot, wsConfig, err := resolveWorkspace(cmd, &cfg, cfgPath)
			if err != nil {
				return err
			}

			statuses, err := ops.Status(ctx, ops.StatusInput{
				WorkspaceRoot: wsRoot,
				Defaults:      cfg.Defaults,
				Git:           git.NewGoGitClient(),
			})
			if err != nil {
				return err
			}
			mode := outputModeFromContext(cmd)

			if len(statuses) == 0 {
				if mode.JSON {
					return output.WriteJSON(commandWriter(cmd), []statusJSON{})
				}
				styles := output.NewStyles(commandWriter(cmd), mode.Plain)
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
				payload := make([]statusJSON, 0, len(statuses))
				for _, repo := range statuses {
					state := "clean"
					switch {
					case repo.Missing:
						state = "missing"
					case repo.Dirty:
						state = "dirty"
					case repo.Err != nil:
						state = "error"
					}
					entry := statusJSON{
						Name:    repo.Name,
						Path:    repo.Path,
						State:   state,
						Dirty:   repo.Dirty,
						Missing: repo.Missing,
					}
					if repo.Err != nil {
						entry.Error = repo.Err.Error()
					}
					payload = append(payload, entry)
				}
				if err := output.WriteJSON(commandWriter(cmd), payload); err != nil {
					return err
				}
			} else {
				rows := make([]output.StatusRow, 0, len(statuses))
				for _, repo := range statuses {
					state := "clean"
					switch {
					case repo.Missing:
						state = "missing"
					case repo.Dirty:
						state = "dirty"
					case repo.Err != nil:
						state = "error"
					}
					detail := repo.Path
					if repo.Err != nil {
						detail = repo.Err.Error()
					}
					rows = append(rows, output.StatusRow{
						Name:   repo.Name,
						State:  state,
						Detail: detail,
					})
				}
				styles := output.NewStyles(commandWriter(cmd), mode.Plain)
				if err := output.PrintStatus(commandWriter(cmd), styles, rows); err != nil {
					return err
				}
			}

			registerWorkspace(&cfg, wsConfig.Name, wsRoot, time.Now())
			return config.SaveGlobal(cfgPath, cfg)
		},
	}
}

func loadGlobal(path string) (config.GlobalConfig, string, error) {
	if path == "" {
		var err error
		path, err = config.GlobalConfigPath()
		if err != nil {
			return config.GlobalConfig{}, "", err
		}
	}
	cfg, err := config.LoadGlobal(path)
	if err != nil {
		return config.GlobalConfig{}, "", err
	}
	return cfg, path, nil
}

func resolveWorkspace(cmd *cli.Command, cfg *config.GlobalConfig, cfgPath string) (string, config.WorkspaceConfig, error) {
	arg := strings.TrimSpace(cmd.String("workspace"))
	if arg == "" {
		arg = strings.TrimSpace(workspaceFromArgs(cmd))
	}
	if arg == "" {
		arg = strings.TrimSpace(cfg.Defaults.Workspace)
	}
	if arg == "" {
		return "", config.WorkspaceConfig{}, fmt.Errorf("workspace required: pass -w <name|path> or set defaults.workspace (example: workset repo ls -w <name>)")
	}

	var root string
	if ref, ok := cfg.Workspaces[arg]; ok {
		root = ref.Path
	} else if cfg.Defaults.WorkspaceRoot != "" {
		candidate := filepath.Join(cfg.Defaults.WorkspaceRoot, arg)
		if _, err := os.Stat(candidate); err == nil {
			root = candidate
		}
	} else {
		root = ""
	}
	if root == "" {
		if filepath.IsAbs(arg) {
			root = arg
		} else {
			return "", config.WorkspaceConfig{}, fmt.Errorf("workspace not found: %q (use a registered name, an absolute path, or a path under defaults.workspace_root)", arg)
		}
	}

	wsConfig, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		if os.IsNotExist(err) {
			return "", config.WorkspaceConfig{}, fmt.Errorf("workset.yaml not found at %s\nhint: check -w or register the workspace with workset ls", workspace.WorksetFile(root))
		}
		return "", config.WorkspaceConfig{}, err
	}

	if cfg.Workspaces == nil {
		cfg.Workspaces = map[string]config.WorkspaceRef{}
	}
	ref, exists := cfg.Workspaces[wsConfig.Name]
	if exists && ref.Path != "" && ref.Path != root {
		return "", config.WorkspaceConfig{}, cli.Exit("workspace name already registered to a different path", 1)
	}
	if !exists {
		registerWorkspace(cfg, wsConfig.Name, root, time.Now())
		if err := config.SaveGlobal(cfgPath, *cfg); err != nil {
			return "", config.WorkspaceConfig{}, err
		}
	}

	return root, wsConfig, nil
}

func registerWorkspace(cfg *config.GlobalConfig, name, path string, now time.Time) {
	if cfg.Workspaces == nil {
		cfg.Workspaces = map[string]config.WorkspaceRef{}
	}
	ref := cfg.Workspaces[name]
	if ref.Path == "" {
		ref.Path = path
		if ref.CreatedAt == "" {
			ref.CreatedAt = now.Format(time.RFC3339)
		}
	}
	ref.LastUsed = now.Format(time.RFC3339)
	cfg.Workspaces[name] = ref
}

func looksLikeURL(value string) bool {
	if strings.Contains(value, "://") {
		return true
	}
	if strings.Contains(value, "@") && strings.Contains(value, ":") {
		return true
	}
	return false
}

func looksLikeLocalPath(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "~") || strings.HasPrefix(value, ".") {
		return true
	}
	return filepath.IsAbs(value)
}

func warnOutsideWorkspaceRoot(root, workspaceRoot string) {
	if workspaceRoot == "" {
		return
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return
	}
	absWorkspace, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return
	}
	absRoot = filepath.Clean(absRoot)
	absWorkspace = filepath.Clean(absWorkspace)
	if absRoot == absWorkspace || strings.HasPrefix(absRoot, absWorkspace+string(os.PathSeparator)) {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "warning: workspace created outside defaults.workspace_root (%s)\n", absWorkspace)
}

type statusJSON struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	State   string `json:"state"`
	Dirty   bool   `json:"dirty,omitempty"`
	Missing bool   `json:"missing,omitempty"`
	Error   string `json:"error,omitempty"`
}

func printWorkspaceCreated(w io.Writer, info output.WorkspaceCreated, asJSON bool, plain bool) error {
	if asJSON {
		return output.WriteJSON(w, info)
	}
	styles := output.NewStyles(w, plain)
	return output.PrintWorkspaceCreated(w, info, styles)
}

func commandWriter(cmd *cli.Command) io.Writer {
	if cmd == nil {
		return os.Stdout
	}
	root := cmd.Root()
	if root != nil && root.Writer != nil {
		return root.Writer
	}
	if cmd.Writer != nil {
		return cmd.Writer
	}
	return os.Stdout
}

func setGlobalDefault(cfg *config.GlobalConfig, key, value string) error {
	switch key {
	case "defaults.base_branch":
		cfg.Defaults.BaseBranch = value
	case "defaults.workspace":
		cfg.Defaults.Workspace = value
	case "defaults.workspace_root":
		cfg.Defaults.WorkspaceRoot = value
	case "defaults.repo_store_root":
		cfg.Defaults.RepoStoreRoot = value
	case "defaults.remotes.base":
		cfg.Defaults.Remotes.Base = value
	case "defaults.remotes.write":
		cfg.Defaults.Remotes.Write = value
	case "defaults.parallelism":
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("defaults.parallelism must be a positive integer")
		}
		cfg.Defaults.Parallelism = parsed
	default:
		return fmt.Errorf("unsupported key %q", key)
	}
	return nil
}

func workspaceFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    "workspace",
		Aliases: []string{"w"},
		Usage:   "Workspace name or path",
	}
}

func outputFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:  "json",
			Usage: "Output JSON",
		},
		&cli.BoolFlag{
			Name:  "plain",
			Usage: "Disable styling",
		},
	}
}

func appendOutputFlags(flags []cli.Flag) []cli.Flag {
	return append(flags, outputFlags()...)
}

type outputMode struct {
	JSON  bool
	Plain bool
}

func outputModeFromContext(cmd *cli.Command) outputMode {
	jsonFlag := boolFlagWithArgs(cmd, "json")
	plainFlag := boolFlagWithArgs(cmd, "plain")
	if jsonFlag {
		plainFlag = true
	}
	return outputMode{JSON: jsonFlag, Plain: plainFlag}
}

func boolFlagWithArgs(cmd *cli.Command, name string) bool {
	if cmd.Bool(name) {
		return true
	}
	if value, ok := boolFromArgs(cmd.Args().Slice(), name); ok {
		return value
	}
	return false
}

type flagSpec struct {
	TakesValue bool
}

func normalizeArgs(root *cli.Command, args []string) []string {
	if root == nil || len(args) == 0 {
		return args
	}

	cmd := root
	i := 1
	for i < len(args) {
		token := args[i]
		if token == "--" || strings.HasPrefix(token, "-") {
			break
		}
		next := findSubcommand(cmd, token)
		if next == nil {
			break
		}
		cmd = next
		i++
	}

	prefix := append([]string{}, args[:i]...)
	flags := make([]string, 0)
	rest := make([]string, 0)

	for j := i; j < len(args); j++ {
		token := args[j]
		if token == "--" {
			rest = append(rest, args[j:]...)
			break
		}
		if spec, ok := interspersedFlag(token); ok {
			flags = append(flags, token)
			if spec.TakesValue && !strings.Contains(token, "=") && j+1 < len(args) {
				flags = append(flags, args[j+1])
				j++
			}
			continue
		}
		rest = append(rest, token)
	}

	normalized := append(prefix, flags...)
	normalized = append(normalized, rest...)
	return normalized
}

func findSubcommand(cmd *cli.Command, name string) *cli.Command {
	if cmd == nil {
		return nil
	}
	for _, sub := range cmd.Commands {
		if sub.Name == name {
			return sub
		}
		for _, alias := range sub.Aliases {
			if alias == name {
				return sub
			}
		}
	}
	return nil
}

func interspersedFlag(token string) (flagSpec, bool) {
	switch token {
	case "-w", "--workspace":
		return flagSpec{TakesValue: true}, true
	case "--json", "--plain":
		return flagSpec{TakesValue: false}, true
	case "--config":
		return flagSpec{TakesValue: true}, true
	}
	switch {
	case strings.HasPrefix(token, "--workspace="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--config="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--json="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--plain="):
		return flagSpec{TakesValue: false}, true
	}
	return flagSpec{}, false
}

func boolFromArgs(args []string, name string) (bool, bool) {
	long := "--" + name
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == long {
			return true, true
		}
		if strings.HasPrefix(arg, long+"=") {
			value := strings.TrimPrefix(arg, long+"=")
			if value == "" {
				return true, true
			}
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return true, true
			}
			return parsed, true
		}
	}
	return false, false
}

func workspaceFromArgs(cmd *cli.Command) string {
	args := cmd.Args().Slice()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-w" || arg == "--workspace" {
			if i+1 < len(args) {
				return strings.TrimSpace(args[i+1])
			}
			return ""
		}
		if strings.HasPrefix(arg, "--workspace=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--workspace="))
		}
	}
	return ""
}

func resolveLocalPathInput(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("local path required")
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func confirmPrompt(r io.Reader, w io.Writer, prompt string) (bool, error) {
	if _, err := fmt.Fprint(w, prompt); err != nil {
		return false, err
	}
	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return false, nil
	}
	line = strings.ToLower(line)
	return line == "y" || line == "yes", nil
}

func findRepo(cfg config.WorkspaceConfig, name string) (config.RepoConfig, bool) {
	for _, repo := range cfg.Repos {
		if repo.Name == name {
			return repo, true
		}
	}
	return config.RepoConfig{}, false
}

func resolveWorkspaceTarget(arg string, cfg *config.GlobalConfig) (string, string, error) {
	target := strings.TrimSpace(arg)
	if target == "" {
		target = strings.TrimSpace(cfg.Defaults.Workspace)
	}
	if target == "" {
		return "", "", fmt.Errorf("workspace required: pass -w <name|path> or set defaults.workspace (example: workset rm -w <name> --delete)")
	}
	if ref, ok := cfg.Workspaces[target]; ok {
		return target, ref.Path, nil
	}
	if !filepath.IsAbs(target) && cfg.Defaults.WorkspaceRoot != "" {
		candidate := filepath.Join(cfg.Defaults.WorkspaceRoot, target)
		if _, err := os.Stat(candidate); err == nil {
			return target, candidate, nil
		}
	}
	if filepath.IsAbs(target) {
		name := workspaceNameByPath(cfg, target)
		return name, target, nil
	}
	return "", "", fmt.Errorf("workspace not found: %q (use a registered name, an absolute path, or a path under defaults.workspace_root)", target)
}

func workspaceNameByPath(cfg *config.GlobalConfig, path string) string {
	clean := filepath.Clean(path)
	for name, ref := range cfg.Workspaces {
		if filepath.Clean(ref.Path) == clean {
			return name
		}
	}
	return ""
}

func removeWorkspaceByPath(cfg *config.GlobalConfig, path string) {
	clean := filepath.Clean(path)
	for name, ref := range cfg.Workspaces {
		if filepath.Clean(ref.Path) == clean {
			delete(cfg.Workspaces, name)
		}
	}
}

func summarizeRepoSafety(report ops.RepoSafetyReport) (dirty []string, unmerged []string, unpushed []string, warnings []string) {
	for _, branch := range report.Branches {
		if branch.StatusErr != "" {
			warnings = append(warnings, fmt.Sprintf("%s: status failed (%s)", branch.Branch, branch.StatusErr))
		}
		if branch.FetchBaseErr != "" {
			warnings = append(warnings, fmt.Sprintf("%s: base fetch failed (%s)", branch.Branch, branch.FetchBaseErr))
		}
		if branch.FetchWriteErr != "" {
			warnings = append(warnings, fmt.Sprintf("%s: write fetch failed (%s)", branch.Branch, branch.FetchWriteErr))
		}
		if branch.UnmergedErr != "" {
			warnings = append(warnings, fmt.Sprintf("%s: unmerged check failed (%s)", branch.Branch, branch.UnmergedErr))
		}
		if branch.UnpushedErr != "" {
			warnings = append(warnings, fmt.Sprintf("%s: unpushed check failed (%s)", branch.Branch, branch.UnpushedErr))
		}
		if branch.Dirty {
			dirty = append(dirty, branch.Branch)
		}
		if branch.Unmerged {
			unmerged = append(unmerged, branch.Branch)
		}
		if branch.Unpushed {
			unpushed = append(unpushed, branch.Branch)
		}
	}
	return dirty, unmerged, unpushed, warnings
}

func summarizeWorkspaceSafety(report ops.WorkspaceSafetyReport) (dirty []string, unmerged []string, unpushed []string, warnings []string) {
	for _, repo := range report.Repos {
		repoDirty, repoUnmerged, repoUnpushed, repoWarnings := summarizeRepoSafety(repo)
		for _, branch := range repoDirty {
			dirty = append(dirty, fmt.Sprintf("%s:%s", repo.RepoName, branch))
		}
		for _, branch := range repoUnmerged {
			unmerged = append(unmerged, fmt.Sprintf("%s:%s", repo.RepoName, branch))
		}
		for _, branch := range repoUnpushed {
			unpushed = append(unpushed, fmt.Sprintf("%s:%s", repo.RepoName, branch))
		}
		for _, warning := range repoWarnings {
			warnings = append(warnings, fmt.Sprintf("%s: %s", repo.RepoName, warning))
		}
	}
	return dirty, unmerged, unpushed, warnings
}
