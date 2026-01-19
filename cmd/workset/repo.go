package main

import (
	"context"
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
			repoAliasCommand(),
			{
				Name:      "ls",
				Aliases:   []string{"list"},
				Usage:     "List repos in a workspace (requires -w)",
				ArgsUsage: "-w <workspace>",
				Flags:     appendOutputFlags([]cli.Flag{workspaceFlag(true)}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					svc := apiService(cmd)
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
						tableRows = append(tableRows, []string{row.Name, row.LocalPath, managed, row.RepoDir, row.Base, row.Write})
					}
					rendered := output.RenderTable(styles, []string{"NAME", "LOCAL_PATH", "MANAGED", "REPO_DIR", "BASE", "WRITE"}, tableRows)
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
						completeRepoAliases(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					raw := strings.TrimSpace(cmd.Args().First())
					if raw == "" {
						return usageError(ctx, cmd, "repo alias or source required")
					}
					svc := apiService(cmd)
					workspaceValue := cmd.String("workspace")
					result, err := svc.AddRepo(ctx, worksetapi.RepoAddInput{
						Workspace:     worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
						Source:        raw,
						Name:          strings.TrimSpace(cmd.String("name")),
						NameSet:       cmd.IsSet("name"),
						RepoDir:       cmd.String("repo-dir"),
						UpdateAliases: true,
					})
					if err != nil {
						return err
					}
					printConfigInfo(cmd, result)
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), result.Payload)
					}
					if len(result.PendingHooks) > 0 && term.IsTerminal(int(os.Stdin.Fd())) {
						for _, pending := range result.PendingHooks {
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
								if _, err := svc.RunHooks(ctx, worksetapi.HooksRunInput{
									Workspace: worksetapi.WorkspaceSelector{Value: workspaceValue},
									Repo:      pending.Repo,
									Event:     pending.Event,
									Reason:    "repo.add",
								}); err != nil {
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
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					msg := fmt.Sprintf("added %s to %s", result.Payload.Repo, result.Payload.Workspace)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					localLine := fmt.Sprintf("local: %s", result.Payload.LocalPath)
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
						line := fmt.Sprintf("worktree: %s", result.WorktreePath)
						if styles.Enabled {
							line = styles.Render(styles.Muted, line)
						}
						if _, err := fmt.Fprintln(commandWriter(cmd), line); err != nil {
							return err
						}
					}
					for _, warning := range result.Warnings {
						line := fmt.Sprintf("warning: %s", warning)
						if styles.Enabled {
							line = styles.Render(styles.Warn, line)
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

							baseRemoteSet := cmd.IsSet("base-remote")
							writeRemoteSet := cmd.IsSet("write-remote")
							baseBranchSet := cmd.IsSet("base-branch")
							if !baseRemoteSet && !writeRemoteSet && !baseBranchSet {
								return usageError(ctx, cmd, "at least one remote setting required")
							}

							baseBranch := cmd.String("base-branch")
							svc := apiService(cmd)
							payload, info, err := svc.UpdateRepoRemotes(ctx, worksetapi.RepoRemotesUpdateInput{
								Workspace:      worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
								Name:           name,
								BaseRemote:     cmd.String("base-remote"),
								WriteRemote:    cmd.String("write-remote"),
								BaseBranch:     baseBranch,
								WriteBranch:    baseBranch,
								BaseRemoteSet:  baseRemoteSet,
								WriteRemoteSet: writeRemoteSet,
								BaseBranchSet:  baseBranchSet,
								WriteBranchSet: baseBranchSet,
							})
							if err != nil {
								return err
							}
							if verboseEnabled(cmd) {
								printConfigLoadInfo(cmd, cmd.String("config"), info)
							}

							mode := outputModeFromContext(cmd)
							if mode.JSON {
								return output.WriteJSON(commandWriter(cmd), payload)
							}
							styles := output.NewStyles(commandWriter(cmd), mode.Plain)
							msg := fmt.Sprintf("updated remotes for %s", name)
							if styles.Enabled {
								msg = styles.Render(styles.Success, msg)
							}
							if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
								return err
							}
							if payload.Base != "" {
								line := fmt.Sprintf("base: %s", payload.Base)
								if styles.Enabled {
									line = styles.Render(styles.Muted, line)
								}
								if _, err := fmt.Fprintln(commandWriter(cmd), line); err != nil {
									return err
								}
							}
							if payload.Write != "" {
								line := fmt.Sprintf("write: %s", payload.Write)
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
					deleteWorktrees := cmd.Bool("delete-worktrees")
					deleteLocal := cmd.Bool("delete-local")
					svc := apiService(cmd)
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
						if confirm, ok := err.(worksetapi.ConfirmationRequired); ok && (deleteWorktrees || deleteLocal) && !cmd.Bool("yes") {
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
							if unsafe, ok := err.(worksetapi.UnsafeOperation); ok {
								for _, warning := range unsafe.Warnings {
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
					svc := apiService(cmd)
					result, err := svc.ListAliases(ctx)
					if err != nil {
						return err
					}
					printConfigInfo(cmd, result)
					mode := outputModeFromContext(cmd)
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					if len(result.Aliases) == 0 {
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
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), result.Aliases)
					}

					rows := make([][]string, 0, len(result.Aliases))
					for _, alias := range result.Aliases {
						source := alias.URL
						if alias.Path != "" {
							source = alias.Path
						}
						if source == "" {
							source = "-"
						}
						rows = append(rows, []string{alias.Name, source, alias.DefaultBranch})
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
					svc := apiService(cmd)
					result, info, err := svc.CreateAlias(ctx, worksetapi.AliasUpsertInput{
						Name:          name,
						Source:        source,
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
					source := strings.TrimSpace(cmd.Args().Get(1))
					svc := apiService(cmd)
					result, info, err := svc.UpdateAlias(ctx, worksetapi.AliasUpsertInput{
						Name:             name,
						Source:           source,
						SourceSet:        source != "",
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
					svc := apiService(cmd)
					result, info, err := svc.DeleteAlias(ctx, name)
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
