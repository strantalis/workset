package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/groups"
	"github.com/strantalis/workset/internal/ops"
	"github.com/strantalis/workset/internal/workspace"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "workset",
		Usage: "Manage multi-repo workspaces with predictable defaults",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "workspace",
				Aliases: []string{"w"},
				Usage:   "Workspace name or path",
			},
			&cli.StringFlag{
				Name:  "config",
				Usage: "Override global config path",
			},
		},
		Commands: []*cli.Command{
			newCommand(),
			initCommand(),
			listCommand(),
			groupCommand(),
			repoCommand(),
			statusCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newCommand() *cli.Command {
	return &cli.Command{
		Name:  "new",
		Usage: "Create a new workspace in a new directory",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "Target directory (defaults to ./<name>)",
			},
		},
		Action: func(c *cli.Context) error {
			name := strings.TrimSpace(c.Args().First())
			if name == "" {
				return cli.Exit("workspace name required", 1)
			}

			cfg, cfgPath, err := loadGlobal(c.String("config"))
			if err != nil {
				return err
			}

			root := c.String("path")
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				root = filepath.Join(cwd, name)
			}
			root, err = filepath.Abs(root)
			if err != nil {
				return err
			}

			if _, err := workspace.Init(root, name, cfg.Defaults); err != nil {
				return err
			}

			registerWorkspace(&cfg, name, root, time.Now())
			return config.SaveGlobal(cfgPath, cfg)
		},
	}
}

func initCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize a workspace in the current directory",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "name",
				Usage: "Workspace name (defaults to directory name)",
			},
		},
		Action: func(c *cli.Context) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			cfg, cfgPath, err := loadGlobal(c.String("config"))
			if err != nil {
				return err
			}

			name := strings.TrimSpace(c.String("name"))
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
			registerWorkspace(&cfg, name, cwd, time.Now())
			return config.SaveGlobal(cfgPath, cfg)
		},
	}
}

func listCommand() *cli.Command {
	return &cli.Command{
		Name:  "ls",
		Usage: "List registered workspaces",
		Action: func(c *cli.Context) error {
			cfg, _, err := loadGlobal(c.String("config"))
			if err != nil {
				return err
			}
			if len(cfg.Workspaces) == 0 {
				if _, err := fmt.Fprintln(c.App.Writer, "no workspaces registered"); err != nil {
					return err
				}
				return nil
			}
			names := make([]string, 0, len(cfg.Workspaces))
			for name := range cfg.Workspaces {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				ref := cfg.Workspaces[name]
				if _, err := fmt.Fprintf(c.App.Writer, "%s\t%s\n", name, ref.Path); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func groupCommand() *cli.Command {
	return &cli.Command{
		Name:    "template",
		Aliases: []string{"group"},
		Usage:   "Manage repo templates in global config",
		Subcommands: []*cli.Command{
			{
				Name:  "ls",
				Usage: "List templates",
				Action: func(c *cli.Context) error {
					cfg, _, err := loadGlobal(c.String("config"))
					if err != nil {
						return err
					}
					names := groups.List(cfg)
					if len(names) == 0 {
						if _, err := fmt.Fprintln(c.App.Writer, "no templates defined"); err != nil {
							return err
						}
						return nil
					}
					for _, name := range names {
						if _, err := fmt.Fprintln(c.App.Writer, name); err != nil {
							return err
						}
					}
					return nil
				},
			},
			{
				Name:  "show",
				Usage: "Show a template",
				Action: func(c *cli.Context) error {
					name := strings.TrimSpace(c.Args().First())
					if name == "" {
						return cli.Exit("template name required", 1)
					}
					cfg, _, err := loadGlobal(c.String("config"))
					if err != nil {
						return err
					}
					group, ok := groups.Get(cfg, name)
					if !ok {
						return cli.Exit("template not found", 1)
					}
					if group.Description != "" {
						if _, err := fmt.Fprintf(c.App.Writer, "%s\t%s\n", name, group.Description); err != nil {
							return err
						}
					} else {
						if _, err := fmt.Fprintf(c.App.Writer, "%s\n", name); err != nil {
							return err
						}
					}
					for _, member := range group.Members {
						flag := "context"
						if member.Editable {
							flag = "editable"
						}
						if _, err := fmt.Fprintf(c.App.Writer, "  %s\t%s\n", member.Repo, flag); err != nil {
							return err
						}
					}
					return nil
				},
			},
			{
				Name:  "create",
				Usage: "Create or update a template",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "description",
						Usage: "Template description",
					},
				},
				Action: func(c *cli.Context) error {
					name := strings.TrimSpace(c.Args().First())
					if name == "" {
						return cli.Exit("template name required", 1)
					}
					cfg, cfgPath, err := loadGlobal(c.String("config"))
					if err != nil {
						return err
					}
					if err := groups.Upsert(&cfg, name, c.String("description")); err != nil {
						return err
					}
					return config.SaveGlobal(cfgPath, cfg)
				},
			},
			{
				Name:  "rm",
				Usage: "Remove a template",
				Action: func(c *cli.Context) error {
					name := strings.TrimSpace(c.Args().First())
					if name == "" {
						return cli.Exit("template name required", 1)
					}
					cfg, cfgPath, err := loadGlobal(c.String("config"))
					if err != nil {
						return err
					}
					if err := groups.Delete(&cfg, name); err != nil {
						return err
					}
					return config.SaveGlobal(cfgPath, cfg)
				},
			},
			{
				Name:  "add",
				Usage: "Add a repo to a template",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "context",
						Usage: "Mark repo as context-only (not editable by default)",
					},
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
				},
				Action: func(c *cli.Context) error {
					groupName := strings.TrimSpace(c.Args().Get(0))
					repoName := strings.TrimSpace(c.Args().Get(1))
					if groupName == "" || repoName == "" {
						return cli.Exit("template and repo name required", 1)
					}
					cfg, cfgPath, err := loadGlobal(c.String("config"))
					if err != nil {
						return err
					}
					member := config.GroupMember{
						Repo:     repoName,
						Editable: !c.Bool("context"),
						Remotes: config.Remotes{
							Base: config.RemoteConfig{
								Name:          c.String("base-remote"),
								DefaultBranch: c.String("base-branch"),
							},
							Write: config.RemoteConfig{
								Name: c.String("write-remote"),
							},
						},
					}
					if err := groups.AddMember(&cfg, groupName, member); err != nil {
						return err
					}
					return config.SaveGlobal(cfgPath, cfg)
				},
			},
			{
				Name:  "remove",
				Usage: "Remove a repo from a template",
				Action: func(c *cli.Context) error {
					groupName := strings.TrimSpace(c.Args().Get(0))
					repoName := strings.TrimSpace(c.Args().Get(1))
					if groupName == "" || repoName == "" {
						return cli.Exit("template and repo name required", 1)
					}
					cfg, cfgPath, err := loadGlobal(c.String("config"))
					if err != nil {
						return err
					}
					if err := groups.RemoveMember(&cfg, groupName, repoName); err != nil {
						return err
					}
					return config.SaveGlobal(cfgPath, cfg)
				},
			},
			{
				Name:  "from-workspace",
				Usage: "Snapshot the current workspace into a template",
				Action: func(c *cli.Context) error {
					groupName := strings.TrimSpace(c.Args().First())
					if groupName == "" {
						return cli.Exit("template name required", 1)
					}
					cfg, cfgPath, err := loadGlobal(c.String("config"))
					if err != nil {
						return err
					}
					_, wsConfig, err := resolveWorkspace(c, &cfg, cfgPath)
					if err != nil {
						return err
					}
					if err := groups.FromWorkspace(&cfg, groupName, wsConfig); err != nil {
						return err
					}
					return config.SaveGlobal(cfgPath, cfg)
				},
			},
			{
				Name:  "apply",
				Usage: "Apply a template to a workspace",
				Action: func(c *cli.Context) error {
					groupName := strings.TrimSpace(c.Args().First())
					if groupName == "" {
						return cli.Exit("template name required", 1)
					}
					cfg, cfgPath, err := loadGlobal(c.String("config"))
					if err != nil {
						return err
					}
					wsRoot, wsConfig, err := resolveWorkspace(c, &cfg, cfgPath)
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

						if _, err := ops.AddRepo(c.Context, ops.AddRepoInput{
							WorkspaceRoot: wsRoot,
							Name:          member.Repo,
							URL:           alias.URL,
							Editable:      member.Editable,
							Defaults:      cfg.Defaults,
							Remotes:       remotes,
							Git:           git.NewGoGitClient(),
						}); err != nil {
							return err
						}
					}
					registerWorkspace(&cfg, wsConfig.Name, wsRoot, time.Now())
					return config.SaveGlobal(cfgPath, cfg)
				},
			},
		},
	}
}

func repoCommand() *cli.Command {
	return &cli.Command{
		Name:  "repo",
		Usage: "Manage repos in a workspace",
		Subcommands: []*cli.Command{
			{
				Name:  "add",
				Usage: "Add a repo to the workspace and clone it",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "url",
						Usage: "Repo URL (overrides alias or positional arg)",
					},
					&cli.StringFlag{
						Name:  "name",
						Usage: "Repo name (required when using --url)",
					},
					&cli.BoolFlag{
						Name:  "context",
						Usage: "Mark repo as context-only (not editable by default)",
					},
					&cli.StringFlag{
						Name:  "repo-dir",
						Usage: "Directory name for the repo within branches/<branch>",
					},
				},
				Action: func(c *cli.Context) error {
					cfg, cfgPath, err := loadGlobal(c.String("config"))
					if err != nil {
						return err
					}
					wsRoot, wsConfig, err := resolveWorkspace(c, &cfg, cfgPath)
					if err != nil {
						return err
					}

					raw := strings.TrimSpace(c.Args().First())
					url := strings.TrimSpace(c.String("url"))
					name := strings.TrimSpace(c.String("name"))

					if url == "" {
						if raw == "" {
							return cli.Exit("repo alias or url required", 1)
						}
						if alias, ok := cfg.Repos[raw]; ok {
							url = alias.URL
							name = raw
						} else if looksLikeURL(raw) {
							url = raw
						} else {
							return cli.Exit("unknown repo alias; use --url", 1)
						}
					}
					if name == "" {
						name = ops.DeriveRepoNameFromURL(url)
					}

					editable := !c.Bool("context")

					defaultBranch := cfg.Defaults.BaseBranch
					if alias, ok := cfg.Repos[name]; ok && alias.DefaultBranch != "" {
						defaultBranch = alias.DefaultBranch
					}

					input := ops.AddRepoInput{
						WorkspaceRoot: wsRoot,
						Name:          name,
						URL:           url,
						Editable:      editable,
						RepoDir:       c.String("repo-dir"),
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

					if _, err := ops.AddRepo(c.Context, input); err != nil {
						return err
					}

					registerWorkspace(&cfg, wsConfig.Name, wsRoot, time.Now())
					return config.SaveGlobal(cfgPath, cfg)
				},
			},
		},
	}
}

func statusCommand() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show status for repos in the workspace",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "all",
				Usage: "Include context repos",
			},
		},
		Action: func(c *cli.Context) error {
			cfg, cfgPath, err := loadGlobal(c.String("config"))
			if err != nil {
				return err
			}
			wsRoot, wsConfig, err := resolveWorkspace(c, &cfg, cfgPath)
			if err != nil {
				return err
			}

			statuses, err := ops.Status(c.Context, ops.StatusInput{
				WorkspaceRoot: wsRoot,
				Defaults:      cfg.Defaults,
				Git:           git.NewGoGitClient(),
				IncludeAll:    c.Bool("all"),
			})
			if err != nil {
				return err
			}

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
				line := fmt.Sprintf("%s\t%s\t%s", repo.Name, state, repo.Path)
				if repo.Err != nil {
					line = fmt.Sprintf("%s\t%s\t%s", repo.Name, state, repo.Err.Error())
				}
				if _, err := fmt.Fprintln(c.App.Writer, line); err != nil {
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

func resolveWorkspace(c *cli.Context, cfg *config.GlobalConfig, cfgPath string) (string, config.WorkspaceConfig, error) {
	arg := strings.TrimSpace(c.String("workspace"))
	var root string
	if arg != "" {
		if ref, ok := cfg.Workspaces[arg]; ok {
			root = ref.Path
		} else {
			abs, err := filepath.Abs(arg)
			if err != nil {
				return "", config.WorkspaceConfig{}, err
			}
			root = abs
		}
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return "", config.WorkspaceConfig{}, err
		}
		root, err = workspace.FindRoot(cwd)
		if err != nil {
			return "", config.WorkspaceConfig{}, err
		}
	}

	wsConfig, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
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
