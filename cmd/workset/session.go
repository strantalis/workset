package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/strantalis/workset/internal/output"
	"github.com/strantalis/workset/pkg/worksetapi"
	"github.com/urfave/cli/v3"
)

func sessionCommand() *cli.Command {
	return &cli.Command{
		Name:  "session",
		Usage: "Manage workspace sessions",
		Commands: []*cli.Command{
			sessionStartCommand(),
			sessionAttachCommand(),
			sessionStopCommand(),
			sessionShowCommand(),
			sessionListCommand(),
		},
	}
}

func sessionStartCommand() *cli.Command {
	return &cli.Command{
		Name:      "start",
		Usage:     "Start a session in a workspace",
		ArgsUsage: "[<workspace>] [-- <command> [args...]]",
		Description: "If defaults.workspace is set, use `workset session start -- <cmd>` to run without specifying " +
			"a workspace argument.",
		Flags: []cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "backend",
				Usage: "Session backend (auto, tmux, screen, exec)",
			},
			&cli.BoolFlag{
				Name:  "attach",
				Usage: "Attach after starting (tmux/screen only)",
			},
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Skip confirmation prompt",
			},
			&cli.BoolFlag{
				Name:    "interactive",
				Aliases: []string{"pty"},
				Usage:   "Use a PTY when running with the exec backend",
			},
			&cli.StringFlag{
				Name:  "name",
				Usage: "Session name (defaults to session_name_format)",
			},
		},
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "backend") {
				completeSessionBackends(cmd, true)
				return
			}
			if completionFlagRequested(cmd, "name") {
				completeSessionNames(cmd)
				return
			}
			if cmd.NArg() == 0 && strings.TrimSpace(cmd.String("workspace")) == "" {
				completeWorkspaceNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, _, err := loadGlobal(cmd)
			if err != nil {
				return err
			}

			workspaceArg, commandArgs := parseWorkspaceAndCommand(cmd, &cfg)
			if workspaceArg == "" && cfg.Defaults.Workspace == "" {
				return usageError(ctx, cmd, "workspace required: pass -w <name|path> or set defaults.workspace (example: workset session start -w demo -- zsh)")
			}

			svc := apiService(ctx, cmd)
			input := worksetapi.SessionStartInput{
				Workspace:   worksetapi.WorkspaceSelector{Value: workspaceArg},
				Backend:     cmd.String("backend"),
				Attach:      cmd.Bool("attach"),
				Interactive: cmd.Bool("interactive"),
				Name:        cmd.String("name"),
				Command:     commandArgs,
				Confirmed:   cmd.Bool("yes"),
			}
			result, err := svc.StartSession(ctx, input)
			if err != nil {
				var confirm worksetapi.ConfirmationRequired
				if errors.As(err, &confirm) && !cmd.Bool("yes") {
					ok, promptErr := confirmPrompt(os.Stdin, commandWriter(cmd), confirm.Message+" [y/N] ")
					if promptErr != nil {
						return promptErr
					}
					if !ok {
						return nil
					}
					input.Confirmed = true
					result, err = svc.StartSession(ctx, input)
				}
				if err != nil {
					return err
				}
			}
			printConfigInfo(cmd, result)
			if result.Notice.NameNotice != "" {
				_, _ = fmt.Fprintln(commandWriter(cmd), result.Notice.NameNotice)
			}
			printSessionNotice(cmd, result.Notice)
			if result.Notice.AttachNote != "" {
				_, _ = fmt.Fprintln(commandWriter(cmd), result.Notice.AttachNote)
			}
			return nil
		},
	}
}

func sessionAttachCommand() *cli.Command {
	return &cli.Command{
		Name:      "attach",
		Usage:     "Attach to a running session",
		ArgsUsage: "[<workspace>] [<name>]",
		Flags: []cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "backend",
				Usage: "Session backend (auto, tmux, screen)",
			},
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Skip confirmation prompt",
			},
			&cli.StringFlag{
				Name:  "name",
				Usage: "Session name (defaults to session_name_format)",
			},
		},
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "backend") {
				completeSessionBackends(cmd, false)
				return
			}
			if completionFlagRequested(cmd, "name") {
				completeSessionNames(cmd)
				return
			}
			cfg, _, err := loadGlobal(cmd)
			if err != nil {
				return
			}
			if cmd.NArg() == 0 {
				if strings.TrimSpace(cmd.String("workspace")) != "" || strings.TrimSpace(cfg.Defaults.Workspace) != "" {
					completeSessionNames(cmd)
				} else {
					completeWorkspaceNames(cmd)
				}
				return
			}
			if cmd.NArg() == 1 {
				completeSessionNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, _, err := loadGlobal(cmd)
			if err != nil {
				return err
			}
			workspaceArg := strings.TrimSpace(cmd.Args().Get(0))
			if workspaceArg == "" {
				workspaceArg = strings.TrimSpace(cmd.String("workspace"))
			}
			if workspaceArg == "" {
				workspaceArg = strings.TrimSpace(cfg.Defaults.Workspace)
			}
			if workspaceArg == "" {
				return usageError(ctx, cmd, "workspace required: pass -w <name|path> or set defaults.workspace")
			}

			svc := apiService(ctx, cmd)
			input := worksetapi.SessionAttachInput{
				Workspace: worksetapi.WorkspaceSelector{Value: workspaceArg},
				Backend:   cmd.String("backend"),
				Name:      firstNonEmpty(strings.TrimSpace(cmd.String("name")), strings.TrimSpace(cmd.Args().Get(1))),
				Confirmed: cmd.Bool("yes"),
			}
			result, err := svc.AttachSession(ctx, input)
			if err != nil {
				var confirm worksetapi.ConfirmationRequired
				if errors.As(err, &confirm) && !cmd.Bool("yes") {
					ok, promptErr := confirmPrompt(os.Stdin, commandWriter(cmd), confirm.Message+" [y/N] ")
					if promptErr != nil {
						return promptErr
					}
					if !ok {
						return nil
					}
					input.Confirmed = true
					result, err = svc.AttachSession(ctx, input)
				}
				if err != nil {
					return err
				}
			}
			printConfigInfo(cmd, result)
			printSessionNotice(cmd, result.Notice)
			return nil
		},
	}
}

func sessionStopCommand() *cli.Command {
	return &cli.Command{
		Name:      "stop",
		Usage:     "Stop a running session",
		ArgsUsage: "[<workspace>] [<name>]",
		Flags: []cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "backend",
				Usage: "Session backend (auto, tmux, screen)",
			},
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Skip confirmation prompt",
			},
			&cli.StringFlag{
				Name:  "name",
				Usage: "Session name (defaults to session_name_format)",
			},
		},
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "backend") {
				completeSessionBackends(cmd, false)
				return
			}
			if completionFlagRequested(cmd, "name") {
				completeSessionNames(cmd)
				return
			}
			cfg, _, err := loadGlobal(cmd)
			if err != nil {
				return
			}
			if cmd.NArg() == 0 {
				if strings.TrimSpace(cmd.String("workspace")) != "" || strings.TrimSpace(cfg.Defaults.Workspace) != "" {
					completeSessionNames(cmd)
				} else {
					completeWorkspaceNames(cmd)
				}
				return
			}
			if cmd.NArg() == 1 {
				completeSessionNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, _, err := loadGlobal(cmd)
			if err != nil {
				return err
			}
			workspaceArg := strings.TrimSpace(cmd.Args().Get(0))
			if workspaceArg == "" {
				workspaceArg = strings.TrimSpace(cmd.String("workspace"))
			}
			if workspaceArg == "" {
				workspaceArg = strings.TrimSpace(cfg.Defaults.Workspace)
			}
			if workspaceArg == "" {
				return usageError(ctx, cmd, "workspace required: pass -w <name|path> or set defaults.workspace")
			}

			svc := apiService(ctx, cmd)
			input := worksetapi.SessionStopInput{
				Workspace: worksetapi.WorkspaceSelector{Value: workspaceArg},
				Backend:   cmd.String("backend"),
				Name:      firstNonEmpty(strings.TrimSpace(cmd.String("name")), strings.TrimSpace(cmd.Args().Get(1))),
				Confirmed: cmd.Bool("yes"),
			}
			result, err := svc.StopSession(ctx, input)
			if err != nil {
				var confirm worksetapi.ConfirmationRequired
				if errors.As(err, &confirm) && !cmd.Bool("yes") {
					ok, promptErr := confirmPrompt(os.Stdin, commandWriter(cmd), confirm.Message+" [y/N] ")
					if promptErr != nil {
						return promptErr
					}
					if !ok {
						return nil
					}
					input.Confirmed = true
					result, err = svc.StopSession(ctx, input)
				}
				if err != nil {
					return err
				}
			}
			printConfigInfo(cmd, result)
			printSessionNotice(cmd, result.Notice)
			return nil
		},
	}
}

func sessionListCommand() *cli.Command {
	return &cli.Command{
		Name:      "ls",
		Usage:     "List sessions for a workspace",
		ArgsUsage: "[<workspace>]",
		Flags:     appendOutputFlags([]cli.Flag{workspaceFlag(false)}),
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			completeWorkspaceNames(cmd)
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			workspaceArg := strings.TrimSpace(cmd.Args().Get(0))
			if workspaceArg == "" {
				workspaceArg = strings.TrimSpace(cmd.String("workspace"))
			}
			svc := apiService(ctx, cmd)
			result, err := svc.ListSessions(ctx, worksetapi.WorkspaceSelector{Value: workspaceArg})
			if err != nil {
				return err
			}
			printConfigInfo(cmd, result)

			mode := outputModeFromContext(cmd)
			styles := output.NewStyles(commandWriter(cmd), mode.Plain)
			if len(result.Sessions) == 0 {
				if mode.JSON {
					return output.WriteJSON(commandWriter(cmd), []worksetapi.SessionRecordJSON{})
				}
				msg := "no sessions recorded"
				if styles.Enabled {
					msg = styles.Render(styles.Muted, msg)
				}
				_, err := fmt.Fprintln(commandWriter(cmd), msg)
				return err
			}

			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), result.Sessions)
			}

			rows := make([][]string, 0, len(result.Sessions))
			for _, record := range result.Sessions {
				status := statusLabel(record.Running)
				rows = append(rows, []string{
					record.Name,
					record.Backend,
					status,
					record.StartedAt,
				})
			}
			rendered := output.RenderTable(styles, []string{"NAME", "BACKEND", "STATUS", "STARTED"}, rows)
			_, err = fmt.Fprint(commandWriter(cmd), rendered)
			return err
		},
	}
}

func sessionShowCommand() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Usage:     "Show details for a session",
		ArgsUsage: "[<workspace>] [<name>]",
		Flags: appendOutputFlags([]cli.Flag{
			workspaceFlag(false),
			&cli.StringFlag{
				Name:  "name",
				Usage: "Session name (defaults to session_name_format)",
			},
			&cli.StringFlag{
				Name:  "backend",
				Usage: "Session backend override (auto, tmux, screen)",
			},
		}),
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if completionFlagRequested(cmd, "backend") {
				completeSessionBackends(cmd, false)
				return
			}
			if completionFlagRequested(cmd, "name") {
				completeSessionNames(cmd)
				return
			}
			cfg, _, err := loadGlobal(cmd)
			if err != nil {
				return
			}
			if cmd.NArg() == 0 {
				if strings.TrimSpace(cmd.String("workspace")) != "" || strings.TrimSpace(cfg.Defaults.Workspace) != "" {
					completeSessionNames(cmd)
				} else {
					completeWorkspaceNames(cmd)
				}
				return
			}
			if cmd.NArg() == 1 {
				completeSessionNames(cmd)
			}
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			workspaceArg := strings.TrimSpace(cmd.Args().Get(0))
			if workspaceArg == "" {
				workspaceArg = strings.TrimSpace(cmd.String("workspace"))
			}
			svc := apiService(ctx, cmd)
			record, info, err := svc.ShowSession(ctx, worksetapi.SessionShowInput{
				Workspace: worksetapi.WorkspaceSelector{Value: workspaceArg},
				Name:      firstNonEmpty(strings.TrimSpace(cmd.String("name")), strings.TrimSpace(cmd.Args().Get(1))),
				Backend:   cmd.String("backend"),
			})
			if err != nil {
				return err
			}
			if verboseEnabled(cmd) {
				printConfigLoadInfo(cmd, cmd.String("config"), info)
			}

			mode := outputModeFromContext(cmd)
			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), record)
			}

			styles := output.NewStyles(commandWriter(cmd), mode.Plain)
			rows := [][]string{{
				record.Name,
				record.Backend,
				statusLabel(record.Running),
				record.StartedAt,
			}}
			rendered := output.RenderTable(styles, []string{"NAME", "BACKEND", "STATUS", "STARTED"}, rows)
			_, err = fmt.Fprint(commandWriter(cmd), rendered)
			return err
		},
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func statusLabel(running bool) string {
	if running {
		return "running"
	}
	return "stopped"
}

func printSessionNotice(cmd *cli.Command, notice worksetapi.SessionNotice) {
	w := commandWriter(cmd)
	styles := output.NewStyles(w, false)
	header := notice.Title
	if styles.Enabled {
		header = styles.Render(styles.Title, header)
	}
	_, _ = fmt.Fprintln(w, header)
	if notice.Workspace != "" {
		_, _ = fmt.Fprintf(w, "  workspace: %s\n", notice.Workspace)
	}
	if notice.Session != "" {
		_, _ = fmt.Fprintf(w, "  session:   %s\n", notice.Session)
	}
	if notice.Backend != "" {
		_, _ = fmt.Fprintf(w, "  backend:   %s\n", notice.Backend)
	}
	if notice.ThemeLabel != "" {
		_, _ = fmt.Fprintf(w, "  theme:     %s\n", notice.ThemeLabel)
	} else if notice.ThemeHint != "" {
		_, _ = fmt.Fprintf(w, "  theme:     disabled\n")
		_, _ = fmt.Fprintf(w, "  tip:       %s\n", notice.ThemeHint)
	}
	if notice.AttachCommand != "" {
		_, _ = fmt.Fprintf(w, "  attach:    %s\n", notice.AttachCommand)
	}
	if notice.DetachHint != "" {
		_, _ = fmt.Fprintf(w, "  detach:    %s\n", notice.DetachHint)
	}
}
