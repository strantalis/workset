package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/strantalis/workset/internal/output"
	"github.com/strantalis/workset/pkg/worksetapi"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

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
					svc := apiService(cmd)
					cfg, info, err := svc.GetConfig(ctx)
					if err != nil {
						return err
					}
					if verboseEnabled(cmd) {
						printConfigLoadInfo(cmd, cmd.String("config"), info)
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
				Name:  "recover",
				Usage: "Recover workspace registrations from workset.yaml files",
				Flags: appendOutputFlags([]cli.Flag{
					&cli.StringFlag{
						Name:  "workspace-root",
						Usage: "Workspace root to scan (defaults to defaults.workspace_root)",
					},
					&cli.BoolFlag{
						Name:  "rebuild-repos",
						Value: true,
						Usage: "Rebuild repo aliases with local paths from workset.yaml",
					},
					&cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Preview changes without writing config",
					},
				}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					svc := apiService(cmd)
					result, err := svc.RecoverConfig(ctx, worksetapi.ConfigRecoverInput{
						WorkspaceRoot: cmd.String("workspace-root"),
						RebuildRepos:  cmd.Bool("rebuild-repos"),
						DryRun:        cmd.Bool("dry-run"),
					})
					if err != nil {
						return err
					}
					if verboseEnabled(cmd) {
						printConfigLoadInfo(cmd, cmd.String("config"), result.Config)
					}
					mode := outputModeFromContext(cmd)
					if mode.JSON {
						return output.WriteJSON(commandWriter(cmd), result.Payload)
					}
					styles := output.NewStyles(commandWriter(cmd), mode.Plain)
					status := "recovered"
					if result.Payload.DryRun {
						status = "recovery preview"
					}
					msg := fmt.Sprintf("%s config from %s", status, result.Payload.WorkspaceRoot)
					if styles.Enabled {
						msg = styles.Render(styles.Success, msg)
					}
					if _, err := fmt.Fprintln(commandWriter(cmd), msg); err != nil {
						return err
					}
					if len(result.Payload.WorkspacesRecovered) > 0 {
						_, _ = fmt.Fprintf(commandWriter(cmd), "workspaces: %s\n", strings.Join(result.Payload.WorkspacesRecovered, ", "))
					} else {
						_, _ = fmt.Fprintln(commandWriter(cmd), "workspaces: none")
					}
					if len(result.Payload.ReposRecovered) > 0 {
						_, _ = fmt.Fprintf(commandWriter(cmd), "repos: %s\n", strings.Join(result.Payload.ReposRecovered, ", "))
					}
					for _, conflict := range result.Payload.Conflicts {
						_, _ = fmt.Fprintln(os.Stderr, "warning:", conflict)
					}
					for _, warning := range result.Payload.Warnings {
						_, _ = fmt.Fprintln(os.Stderr, "warning:", warning)
					}
					return nil
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
					svc := apiService(cmd)
					result, info, err := svc.SetDefault(ctx, key, value)
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
