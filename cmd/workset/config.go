package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/strantalis/workset/internal/config"
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
	case "defaults.session_backend":
		backend, err := parseSessionBackend(value)
		if err != nil {
			return err
		}
		cfg.Defaults.SessionBackend = string(backend)
	case "defaults.session_name_format":
		cfg.Defaults.SessionNameFormat = value
	case "defaults.session_theme":
		cfg.Defaults.SessionTheme = value
	case "defaults.session_tmux_status_style":
		cfg.Defaults.SessionTmuxStyle = value
	case "defaults.session_tmux_status_left":
		cfg.Defaults.SessionTmuxLeft = value
	case "defaults.session_tmux_status_right":
		cfg.Defaults.SessionTmuxRight = value
	case "defaults.session_screen_hardstatus":
		cfg.Defaults.SessionScreenHard = value
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
