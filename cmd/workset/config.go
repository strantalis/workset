package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/strantalis/workset/internal/output"
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
