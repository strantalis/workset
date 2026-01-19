package main

import (
	"context"
	"errors"
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

func newCommand() *cli.Command {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:  "path",
			Usage: "Target directory (defaults to ./<name>)",
		},
		&cli.StringSliceFlag{
			Name:  "group",
			Usage: "Group to apply (repeatable)",
			Config: cli.StringConfig{
				TrimSpace: true,
			},
		},
		&cli.StringSliceFlag{
			Name:  "repo",
			Usage: "Repo alias to add (repeatable)",
			Config: cli.StringConfig{
				TrimSpace: true,
			},
		},
	}
	flags = append(flags, outputFlags()...)
	return &cli.Command{
		Name:      "new",
		Usage:     "Create a new workspace in a new directory",
		ArgsUsage: "<name>",
		Flags:     flags,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			name := strings.TrimSpace(cmd.Args().First())
			if name == "" {
				return usageError(ctx, cmd, "workspace name required")
			}

			cfg, cfgPath, err := loadGlobal(cmd)
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

			ws, err := workspace.Init(root, name, cfg.Defaults)
			if err != nil {
				return err
			}

			repoPlans, err := buildNewWorkspaceRepoPlans(cfg, cmd.StringSlice("group"), cmd.StringSlice("repo"))
			if err != nil {
				return err
			}
			for _, plan := range repoPlans {
				if _, err := ops.AddRepo(ctx, ops.AddRepoInput{
					WorkspaceRoot: ws.Root,
					Name:          plan.Name,
					URL:           plan.URL,
					SourcePath:    plan.SourcePath,
					Defaults:      cfg.Defaults,
					Remotes:       plan.Remotes,
					Git:           git.NewGoGitClient(),
				}); err != nil {
					return err
				}
			}

			warnOutsideWorkspaceRoot(root, cfg.Defaults.WorkspaceRoot)
			info := output.WorkspaceCreated{
				Name:    name,
				Path:    root,
				Workset: workspace.WorksetFile(root),
				Branch:  ws.State.CurrentBranch,
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

func versionCommand() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Print the workset version",
		Flags: outputFlags(),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			mode := outputModeFromContext(cmd)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), map[string]string{
					"version": version,
				})
			}
			_, err := fmt.Fprintln(commandWriter(cmd), version)
			return err
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
			cfg, _, err := loadGlobal(cmd)
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
				type row struct {
					Name      string `json:"name"`
					Path      string `json:"path"`
					CreatedAt string `json:"created_at,omitempty"`
					LastUsed  string `json:"last_used,omitempty"`
				}
				rows := make([]row, 0, len(names))
				for _, name := range names {
					ref := cfg.Workspaces[name]
					rows = append(rows, row{
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

func removeWorkspaceCommand() *cli.Command {
	return &cli.Command{
		Name:  "rm",
		Usage: "Remove a workspace (use --delete to remove files and stop sessions)",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(false),
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
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if cmd.NArg() == 0 {
				completeWorkspaceNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			arg := strings.TrimSpace(cmd.Args().First())
			if arg == "" {
				arg = strings.TrimSpace(cmd.String("workspace"))
			}
			cfg, cfgPath, err := loadGlobal(cmd)
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
						for _, detail := range unmergedWorkspaceDetails(report) {
							_, _ = fmt.Fprintln(os.Stderr, "detail:", detail)
						}
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

				if err := stopWorkspaceSessions(ctx, cmd, root, cmd.Bool("force")); err != nil {
					return err
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

func stopWorkspaceSessions(ctx context.Context, cmd *cli.Command, root string, force bool) error {
	state, err := workspace.LoadState(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if force {
			_, _ = fmt.Fprintf(commandErrWriter(cmd), "warning: failed to read workspace session state: %v\n", err)
			return nil
		}
		return err
	}
	if len(state.Sessions) == 0 {
		return nil
	}
	runner := execRunner{}
	for name, entry := range state.Sessions {
		sessionName := name
		if strings.TrimSpace(entry.Name) != "" {
			sessionName = entry.Name
		}
		backendValue := strings.TrimSpace(entry.Backend)
		if backendValue == "" {
			if force {
				_, _ = fmt.Fprintf(commandErrWriter(cmd), "warning: session %s missing backend; skipping\n", sessionName)
				continue
			}
			return fmt.Errorf("session %s missing backend; use --force to skip", sessionName)
		}
		backend, err := parseSessionBackend(backendValue)
		if err != nil {
			if force {
				_, _ = fmt.Fprintf(commandErrWriter(cmd), "warning: session %s has invalid backend %q: %v\n", sessionName, backendValue, err)
				continue
			}
			return err
		}
		if backend == sessionBackendAuto || backend == sessionBackendExec {
			if force {
				_, _ = fmt.Fprintf(commandErrWriter(cmd), "warning: session %s uses unsupported backend %q; skipping\n", sessionName, backend)
				continue
			}
			return fmt.Errorf("session %s uses unsupported backend %q; use --force to skip", sessionName, backend)
		}
		if err := runner.LookPath(string(backend)); err != nil {
			if force {
				_, _ = fmt.Fprintf(commandErrWriter(cmd), "warning: %s not available to stop session %s: %v\n", backend, sessionName, err)
				continue
			}
			return fmt.Errorf("%s not available to stop session %s", backend, sessionName)
		}
		exists, err := sessionExists(ctx, runner, backend, sessionName)
		if err != nil {
			if force {
				_, _ = fmt.Fprintf(commandErrWriter(cmd), "warning: failed to check session %s: %v\n", sessionName, err)
				continue
			}
			return err
		}
		if !exists {
			continue
		}
		if err := stopSession(ctx, runner, backend, sessionName); err != nil {
			if force {
				_, _ = fmt.Fprintf(commandErrWriter(cmd), "warning: failed to stop session %s: %v\n", sessionName, err)
				continue
			}
			return err
		}
	}
	return nil
}

type statusJSON struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	State   string `json:"state"`
	Dirty   bool   `json:"dirty,omitempty"`
	Missing bool   `json:"missing,omitempty"`
	Error   string `json:"error,omitempty"`
}

func statusCommand() *cli.Command {
	return &cli.Command{
		Name:      "status",
		Usage:     "Show status for repos in a workspace (requires -w)",
		ArgsUsage: "-w <workspace>",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(true),
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, cfgPath, err := loadGlobal(cmd)
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
