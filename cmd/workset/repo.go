package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/internal/output"
	"github.com/strantalis/workset/pkg/worksetapi"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

func repoCommand() *cli.Command {
	return &cli.Command{
		Name:  "repo",
		Usage: "Manage repos in a workspace (requires -w)",
		Commands: []*cli.Command{
			repoRegistryCommand(),
			{
				Name:      "ls",
				Aliases:   []string{"list"},
				Usage:     "List repos in a workspace (requires -w)",
				ArgsUsage: "-w <workspace>",
				Flags:     appendOutputFlags([]cli.Flag{workspaceFlag(true)}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					svc := apiService(ctx, cmd)
					result, err := svc.ListRepos(ctx, worksetapi.WorkspaceSelector{Value: cmd.String("workspace")})
					if err != nil {
						return err
					}
					printConfigInfo(cmd, result)
					mode := outputModeFromContext(cmd)
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)

					if len(result.Repos) == 0 {
						if mode.JSON {
							return output.WriteJSON(commandWriter(cmd), []worksetapi.RepoJSON{})
						}
						msg := "no repos in workspace"
						if styles.Enabled {
							msg = styles.Render(styles.Muted, msg)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
							return err
						}
						return nil
					}

					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), result.Repos)
					}
					tableRows := make([][]string, 0, len(result.Repos))
					for _, row := range result.Repos {
						managed := "no"
						if row.Managed {
							managed = "yes"
						}
						tableRows = append(tableRows, []string{row.Name, row.LocalPath, managed, row.RepoDir, row.Remote, row.DefaultBranch})
					}
					rendered := output.RenderTable(styles, []string{"NAME", "LOCAL_PATH", "MANAGED", "REPO_DIR", "REMOTE", "DEFAULT_BRANCH"}, tableRows)
					if _, err := fmt.Fprint(commandWriter(cmd), rendered); err != nil {
						return err
					}
					return nil
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
						completeRegisteredRepos(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					raw := strings.TrimSpace(cmd.Args().First())
					if raw == "" {
						return usageError(ctx, cmd, "repo alias or source required")
					}
					svc := apiService(ctx, cmd)
					workspaceValue := cmd.String("workspace")
					result, err := svc.AddRepo(ctx, worksetapi.RepoAddInput{
						Workspace: worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
						Source:    raw,
						Name:      strings.TrimSpace(cmd.String("name")),
						NameSet:   cmd.IsSet("name"),
						RepoDir:   cmd.String("repo-dir"),
					})
					if err != nil {
						return err
					}
					printConfigInfo(cmd, result)
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), struct {
							worksetapi.RepoAddResultJSON

							Warnings []string                       `json:"warnings,omitempty"`
							HookRuns []worksetapi.HookExecutionJSON `json:"hook_runs,omitempty"`
						}{
							RepoAddResultJSON: result.Payload,
							Warnings:          result.Warnings,
							HookRuns:          result.HookRuns,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					if len(result.PendingHooks) > 0 && term.IsTerminal(int(os.Stdin.Fd())) {
						for _, pending := range result.PendingHooks {
							if pending.Reason != "" && pending.Reason != "untrusted" {
								continue
							}
							hookList := strings.Join(pending.Hooks, ", ")
							if hookList == "" {
								continue
							}
							prompt := fmt.Sprintf("repo %s defines hooks (%s). Run now? [y/N] ", pending.Repo, hookList)
							ok, promptErr := confirmPrompt(os.Stdin, commandWriter(cmd), prompt)
							if promptErr != nil {
								return promptErr
							}
							if ok {
								runResult, err := svc.RunHooks(ctx, worksetapi.HooksRunInput{
									Workspace: worksetapi.WorkspaceSelector{Value: workspaceValue},
									Repo:      pending.Repo,
									Event:     pending.Event,
									Reason:    "repo.add",
								})
								if err != nil {
									return err
								}
								if err := printHookRunReport(commandWriter(cmd), styles, runResult.Repo, runResult.Event, runResult.Results); err != nil {
									return err
								}
								trustPrompt := fmt.Sprintf("trust hooks for repo %s? [y/N] ", pending.Repo)
								trust, trustErr := confirmPrompt(os.Stdin, commandWriter(cmd), trustPrompt)
								if trustErr != nil {
									return trustErr
								}
								if trust {
									if _, err := svc.TrustRepoHooks(ctx, pending.Repo); err != nil {
										return err
									}
								}
							}
						}
					}
					msg := fmt.Sprintf("added %s to %s", result.Payload.Repo, result.Payload.Workspace)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					localLine := "local: " + result.Payload.LocalPath
					if styles.Enabled {
						localLine = styles.Render(styles.Muted, localLine)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), localLine); err != nil {
						return err
					}
					if result.Payload.Managed {
						note := fmt.Sprintf("note: cloned into repo store (%s)", filepath.Dir(result.Payload.LocalPath))
						if styles.Enabled {
							note = styles.Render(styles.Muted, note)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), note); err != nil {
							return err
						}
					}
					if result.WorktreePath != "" {
						line := "worktree: " + result.WorktreePath
						if styles.Enabled {
							line = styles.Render(styles.Muted, line)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), line); err != nil {
							return err
						}
					}
					for _, warning := range result.Warnings {
						line := "warning: " + warning
						if styles.Enabled {
							line = styles.Render(styles.Warn, line)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), line); err != nil {
							return err
						}
					}
					return printHookExecutionResults(commandWriter(cmd), styles, result.HookRuns)
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
					deleteWorktrees := cmd.Bool("delete-worktrees")
					deleteLocal := cmd.Bool("delete-local")
					svc := apiService(ctx, cmd)
					input := worksetapi.RepoRemoveInput{
						Workspace:       worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
						Name:            name,
						DeleteWorktrees: deleteWorktrees,
						DeleteLocal:     deleteLocal,
						Force:           cmd.Bool("force"),
						Confirmed:       cmd.Bool("yes"),
						FetchRemotes:    true,
					}
					result, err := svc.RemoveRepo(ctx, input)
					if err != nil {
						var confirm worksetapi.ConfirmationRequired
						if errors.As(err, &confirm) && (deleteWorktrees || deleteLocal) && !cmd.Bool("yes") {
							prompt := confirm.Message
							if deleteWorktrees {
								prompt += " and delete worktrees"
							}
							if deleteLocal {
								prompt += " and local repo"
							}
							ok, promptErr := confirmPrompt(os.Stdin, commandWriter(cmd), prompt+"? [y/N] ")
							if promptErr != nil {
								return promptErr
							}
							if !ok {
								return cli.Exit("aborted", 1)
							}
							input.Confirmed = true
							result, err = svc.RemoveRepo(ctx, input)
						}
						if err != nil {
							var unsafeOp worksetapi.UnsafeOperation
							if errors.As(err, &unsafeOp) {
								for _, warning := range unsafeOp.Warnings {
									_, _ = fmt.Fprintln(os.Stderr, "warning:", warning)
								}
							}
							return err
						}
					}

					printConfigInfo(cmd, result)
					for _, warning := range result.Warnings {
						_, _ = fmt.Fprintln(os.Stderr, "warning:", warning)
					}
					for _, branch := range result.Unpushed {
						_, _ = fmt.Fprintf(os.Stderr, "warning: branch %s has commits not on write remote\n", branch)
					}

					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), result.Payload)
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("removed %s from %s", result.Payload.Repo, result.Payload.Workspace)
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

func repoRegistryCommand() *cli.Command {
	return &cli.Command{
		Name:  "registry",
		Usage: "Manage registered repos",
		Commands: []*cli.Command{
			{
				Name:  "ls",
				Usage: "List registered repos",
				Flags: outputFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					svc := apiService(ctx, cmd)
					result, err := svc.ListRegisteredRepos(ctx)
					if err != nil {
						return err
					}
					printConfigInfo(cmd, result)
					mode := outputModeFromContext(cmd)
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					if len(result.Repos) == 0 {
						if mode.JSON {
							return output.WriteJSON(commandWriter(cmd), []any{})
						}
						msg := "no repos registered"
						if styles.Enabled {
							msg = styles.Render(styles.Muted, msg)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
							return err
						}
						return nil
					}
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), result.Repos)
					}

					rows := make([][]string, 0, len(result.Repos))
					for _, repo := range result.Repos {
						source := repo.URL
						if repo.Path != "" {
							source = repo.Path
						}
						if source == "" {
							source = "-"
						}
						rows = append(rows, []string{repo.Name, source, repo.Remote, repo.DefaultBranch})
					}
					rendered := output.RenderTable(styles, []string{"NAME", "SOURCE", "REMOTE", "DEFAULT_BRANCH"}, rows)
					_, err = fmt.Fprint(commandWriter(cmd), rendered)
					return err
				},
			},
			{
				Name:        "add",
				Usage:       "Register a repo",
				ArgsUsage:   "<name> <source>",
				UsageText:   "workset repo registry add <name> <source> [--remote <name>] [--default-branch <branch>]",
				Description: "Register a repo path or URL. Use `workset repo registry set` to update an existing entry.",
				Flags: appendOutputFlags([]cli.Flag{
					&cli.StringFlag{
						Name:  "remote",
						Usage: "Primary remote name",
					},
					&cli.StringFlag{
						Name:  "default-branch",
						Usage: "Default branch name",
					},
				}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().Get(0))
					source := strings.TrimSpace(cmd.Args().Get(1))
					if name == "" {
						return usageError(ctx, cmd, "repo name required (example: workset repo registry add ask-gill git@github.com:org/repo.git)")
					}
					if source == "" {
						return usageError(ctx, cmd, fmt.Sprintf("source required to register repo %q (path or URL). Example: workset repo registry add --default-branch staging %s git@github.com:org/repo.git", name, name))
					}
					svc := apiService(ctx, cmd)
					result, info, err := svc.RegisterRepo(ctx, worksetapi.RepoRegistryInput{
						Name:          name,
						Source:        source,
						Remote:        strings.TrimSpace(cmd.String("remote")),
						DefaultBranch: strings.TrimSpace(cmd.String("default-branch")),
					})
					if err != nil {
						return err
					}
					if verboseEnabled(cmd) {
						printConfigLoadInfo(cmd, cmd.String("config"), info)
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), result)
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("repo %s registered", name)
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
				Usage:       "Update a registered repo",
				ArgsUsage:   "<name> [source]",
				UsageText:   "workset repo registry set <name> [source] [--remote <name>] [--default-branch <branch>]",
				Description: "Update an existing registered repo. Omit source to keep the current path/URL.",
				Flags: appendOutputFlags([]cli.Flag{
					&cli.StringFlag{
						Name:  "remote",
						Usage: "Primary remote name",
					},
					&cli.StringFlag{
						Name:  "default-branch",
						Usage: "Default branch name",
					},
				}),
				ShellComplete: func(ctx context.Context, cmd *cli.Command) {
					if cmd.NArg() == 0 {
						completeRegisteredRepos(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().Get(0))
					if name == "" {
						return usageError(ctx, cmd, "repo name required (example: workset repo registry set ask-gill --default-branch staging)")
					}
					source := strings.TrimSpace(cmd.Args().Get(1))
					svc := apiService(ctx, cmd)
					result, info, err := svc.UpdateRegisteredRepo(ctx, worksetapi.RepoRegistryInput{
						Name:             name,
						Source:           source,
						SourceSet:        source != "",
						Remote:           strings.TrimSpace(cmd.String("remote")),
						RemoteSet:        cmd.IsSet("remote"),
						DefaultBranch:    strings.TrimSpace(cmd.String("default-branch")),
						DefaultBranchSet: cmd.IsSet("default-branch"),
					})
					if err != nil {
						return err
					}
					if verboseEnabled(cmd) {
						printConfigLoadInfo(cmd, cmd.String("config"), info)
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), result)
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("repo %s updated", name)
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
				Usage:     "Unregister a repo",
				ArgsUsage: "<name>",
				Flags:     outputFlags(),
				ShellComplete: func(ctx context.Context, cmd *cli.Command) {
					if cmd.NArg() == 0 {
						completeRegisteredRepos(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					name := strings.TrimSpace(cmd.Args().Get(0))
					if name == "" {
						return usageError(ctx, cmd, "usage: workset repo registry rm <name>")
					}
					svc := apiService(ctx, cmd)
					result, info, err := svc.UnregisterRepo(ctx, name)
					if err != nil {
						return err
					}
					if verboseEnabled(cmd) {
						printConfigLoadInfo(cmd, cmd.String("config"), info)
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), result)
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("repo %s unregistered", name)
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
