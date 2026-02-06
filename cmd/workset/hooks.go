package main

import (
	"context"
	"strings"

	"github.com/strantalis/workset/internal/hooks"
	"github.com/strantalis/workset/internal/output"
	"github.com/strantalis/workset/pkg/worksetapi"
	"github.com/urfave/cli/v3"
)

func hooksCommand() *cli.Command {
	return &cli.Command{
		Name:  "hooks",
		Usage: "Run repo hooks",
		Commands: []*cli.Command{
			{
				Name:      "run",
				Usage:     "Run repo hooks for an event (requires -w)",
				ArgsUsage: "-w <workspace> <repo>",
				Flags: appendOutputFlags([]cli.Flag{
					workspaceFlag(true),
					&cli.StringFlag{
						Name:  "event",
						Usage: "Hook event name",
						Value: string(hooks.EventWorktreeCreated),
					},
					&cli.StringFlag{
						Name:  "reason",
						Usage: "Hook reason (for logs/env)",
					},
					&cli.BoolFlag{
						Name:  "trust",
						Usage: "Add this repo to the trusted hooks list",
					},
				}),
				ShellComplete: func(ctx context.Context, cmd *cli.Command) {
					if cmd.NArg() == 0 {
						completeWorkspaceRepoNames(cmd)
					}
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					repo := strings.TrimSpace(cmd.Args().First())
					if repo == "" {
						return usageError(ctx, cmd, "usage: workset hooks run -w <workspace> <repo>")
					}
					svc := apiService(ctx, cmd)
					result, err := svc.RunHooks(ctx, worksetapi.HooksRunInput{
						Workspace: worksetapi.WorkspaceSelector{Value: cmd.String("workspace")},
						Repo:      repo,
						Event:     cmd.String("event"),
						Reason:    cmd.String("reason"),
						TrustRepo: cmd.Bool("trust"),
					})
					if err != nil {
						return err
					}
					printConfigInfo(cmd, result)
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), struct {
							Event   string                   `json:"event"`
							Repo    string                   `json:"repo"`
							Results []worksetapi.HookRunJSON `json:"results"`
						}{
							Event:   result.Event,
							Repo:    result.Repo,
							Results: result.Results,
						})
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					return printHookRunReport(commandWriter(cmd), styles, result.Repo, result.Event, result.Results)
				},
			},
		},
	}
}
