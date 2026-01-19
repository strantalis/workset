package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/strantalis/workset/internal/output"
	"github.com/strantalis/workset/pkg/worksetapi"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
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
			svc := apiService(cmd)
			result, err := svc.CreateWorkspace(ctx, worksetapi.WorkspaceCreateInput{
				Name:   name,
				Path:   cmd.String("path"),
				Groups: cmd.StringSlice("group"),
				Repos:  cmd.StringSlice("repo"),
			})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			info := output.WorkspaceCreated{
				Name:    result.Workspace.Name,
				Path:    result.Workspace.Path,
				Workset: result.Workspace.Workset,
				Branch:  result.Workspace.Branch,
				Next:    result.Workspace.Next,
			}
			mode := outputModeFromContext(cmd)
			handledHooks := map[string]bool{}
			if !mode.JSON && len(result.PendingHooks) > 0 && term.IsTerminal(int(os.Stdin.Fd())) {
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
						if _, err := svc.RunHooks(ctx, worksetapi.HooksRunInput{
							Workspace: worksetapi.WorkspaceSelector{Value: result.Workspace.Name},
							Repo:      pending.Repo,
							Event:     pending.Event,
							Reason:    "workspace.create",
						}); err != nil {
							return err
						}
						handledHooks[pending.Repo] = true
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
			for _, warning := range result.Warnings {
				_, _ = fmt.Fprintln(os.Stderr, "warning:", warning)
			}
			for _, pending := range result.PendingHooks {
				if handledHooks[pending.Repo] {
					continue
				}
				_, _ = fmt.Fprintf(os.Stderr, "warning: repo %s hooks pending approval; run `workset hooks run -w %s %s` to execute\n", pending.Repo, result.Workspace.Name, pending.Repo)
			}
			return printWorkspaceCreated(commandWriter(cmd), info, mode.JSON, mode.Plain)
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
			svc := apiService(cmd)
			result, err := svc.ListWorkspaces(ctx)
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			mode := outputModeFromContext(cmd)
			styles := output.NewStyles(commandWriter(cmd), mode.Plain)
			if len(result.Workspaces) == 0 {
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
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), result.Workspaces)
			}

			rows := make([][]string, 0, len(result.Workspaces))
			for _, ref := range result.Workspaces {
				rows = append(rows, []string{ref.Name, ref.Path})
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
			deleteRequested := cmd.Bool("delete")
			svc := apiService(cmd)
			input := worksetapi.WorkspaceDeleteInput{
				Selector:     worksetapi.WorkspaceSelector{Value: arg},
				DeleteFiles:  deleteRequested,
				Force:        cmd.Bool("force"),
				Confirmed:    cmd.Bool("yes"),
				FetchRemotes: true,
			}
			result, err := svc.DeleteWorkspace(ctx, input)
			if err != nil {
				if confirm, ok := err.(worksetapi.ConfirmationRequired); ok && deleteRequested && !cmd.Bool("yes") {
					ok, promptErr := confirmPrompt(os.Stdin, commandWriter(cmd), confirm.Message+" [y/N] ")
					if promptErr != nil {
						return promptErr
					}
					if !ok {
						return cli.Exit("aborted", 1)
					}
					input.Confirmed = true
					result, err = svc.DeleteWorkspace(ctx, input)
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
			if deleteRequested {
				msg := fmt.Sprintf("workspace %s deleted", result.Payload.Path)
				if styles.Enabled {
					msg = styles.Render(styles.Success, msg)
				}
				if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
					return err
				}
				return nil
			}
			msg := fmt.Sprintf("removed workspace registration for %s", result.Payload.Path)
			if styles.Enabled {
				msg = styles.Render(styles.Success, msg)
			}
			if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
				return err
			}
			note := fmt.Sprintf("note: files remain on disk; to delete, run: workset rm -w %s --delete", result.Payload.Path)
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

func statusCommand() *cli.Command {
	return &cli.Command{
		Name:      "status",
		Usage:     "Show status for repos in a workspace (requires -w)",
		ArgsUsage: "-w <workspace>",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(true),
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			svc := apiService(cmd)
			result, err := svc.StatusWorkspace(ctx, worksetapi.WorkspaceSelector{Value: cmd.String("workspace")})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)
			mode := outputModeFromContext(cmd)
			if len(result.Statuses) == 0 {
				if mode.JSON {
					return output.WriteJSON(commandWriter(cmd), []worksetapi.RepoStatusJSON{})
				}
				styles := output.NewStyles(commandWriter(cmd), mode.Plain)
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
				return output.WriteJSON(commandWriter(cmd), result.Statuses)
			} else {
				rows := make([]output.StatusRow, 0, len(result.Statuses))
				for _, repo := range result.Statuses {
					detail := repo.Path
					if repo.Error != "" {
						detail = repo.Error
					}
					rows = append(rows, output.StatusRow{
						Name:   repo.Name,
						State:  repo.State,
						Detail: detail,
					})
				}
				styles := output.NewStyles(commandWriter(cmd), mode.Plain)
				if err := output.PrintStatus(commandWriter(cmd), styles, rows); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
