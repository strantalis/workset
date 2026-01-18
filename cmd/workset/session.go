package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/output"
	"github.com/strantalis/workset/internal/workspace"
	"github.com/urfave/cli/v3"
)

type sessionRecord struct {
	Name         string   `json:"name"`
	Backend      string   `json:"backend"`
	Command      []string `json:"command,omitempty"`
	StartedAt    string   `json:"started_at,omitempty"`
	LastAttached string   `json:"last_attached,omitempty"`
	Running      bool     `json:"running"`
}

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
			cfg, cfgPath, err := loadGlobal(cmd.String("config"))
			if err != nil {
				return err
			}

			workspaceArg, commandArgs := parseWorkspaceAndCommand(cmd, &cfg)
			if workspaceArg == "" && cfg.Defaults.Workspace == "" {
				return usageError(ctx, cmd, "workspace required: pass -w <name|path> or set defaults.workspace (example: workset session start -w demo -- zsh)")
			}

			name, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
			if err != nil {
				return err
			}

			ws, err := workspace.Load(root, cfg.Defaults)
			if err != nil {
				return err
			}
			wsName := workspaceName(ws, name, root)
			sessionName := resolveSessionName(cmd.String("name"), cfg.Defaults.SessionNameFormat, wsName)

			backend, err := parseSessionBackend(firstNonEmpty(cmd.String("backend"), cfg.Defaults.SessionBackend))
			if err != nil {
				return err
			}

			runner := execRunner{}
			resolvedBackend, err := resolveSessionBackend(backend, runner)
			if err != nil {
				return err
			}

			env := append(os.Environ(),
				fmt.Sprintf("WORKSET_ROOT=%s", root),
				fmt.Sprintf("WORKSET_CONFIG=%s", workspace.WorksetFile(root)),
			)
			if wsName != "" {
				env = append(env, fmt.Sprintf("WORKSET_WORKSPACE=%s", wsName))
			}

			interactive := cmd.Bool("interactive")
			if interactive && resolvedBackend != sessionBackendExec {
				return fmt.Errorf("--interactive is only supported with the exec backend (use --backend exec)")
			}

			includeAttach := resolvedBackend != sessionBackendExec
			printSessionNotice(cmd, "starting session", wsName, sessionName, resolvedBackend, includeAttach)
			if !cmd.Bool("yes") {
				ok, err := confirmPrompt(os.Stdin, commandWriter(cmd), fmt.Sprintf("start session %s? [y/N] ", sessionName))
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
			}
			if resolvedBackend == sessionBackendExec {
				time.Sleep(750 * time.Millisecond)
			}

			if err := startSession(ctx, runner, resolvedBackend, root, sessionName, commandArgs, env, interactive); err != nil {
				if resolvedBackend == sessionBackendExec {
					return exitWithStatus(err)
				}
				return err
			}

			if resolvedBackend != sessionBackendExec {
				workspace.EnsureSessionState(&ws.State)
				ws.State.Sessions[sessionName] = workspace.SessionState{
					Backend:   string(resolvedBackend),
					Name:      sessionName,
					Command:   commandArgs,
					StartedAt: time.Now().Format(time.RFC3339),
				}
				if err := workspace.SaveState(root, ws.State); err != nil {
					return err
				}
			}

			registerWorkspace(&cfg, wsName, root, time.Now())
			return config.SaveGlobal(cfgPath, cfg)
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
			cfg, _, err := loadGlobal(cmd.String("config"))
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
			cfg, cfgPath, err := loadGlobal(cmd.String("config"))
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

			name, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
			if err != nil {
				return err
			}
			ws, err := workspace.Load(root, cfg.Defaults)
			if err != nil {
				return err
			}
			wsName := workspaceName(ws, name, root)
			explicitName := firstNonEmpty(strings.TrimSpace(cmd.String("name")), strings.TrimSpace(cmd.Args().Get(1)))
			sessionName, sessionState, err := resolveSessionTarget(ws.State, explicitName, cfg.Defaults.SessionNameFormat, wsName)
			if err != nil {
				return err
			}

			backendValue := firstNonEmpty(cmd.String("backend"), cfg.Defaults.SessionBackend)
			backend, err := parseSessionBackend(backendValue)
			if err != nil {
				return err
			}

			if sessionState != nil && sessionState.Backend != "" {
				if parsed, err := parseSessionBackend(sessionState.Backend); err == nil {
					backend = parsed
				}
			}

			runner := execRunner{}
			resolvedBackend, err := resolveSessionBackend(backend, runner)
			if err != nil {
				return err
			}
			if resolvedBackend == sessionBackendExec {
				return fmt.Errorf("attach not supported for backend %q", resolvedBackend)
			}

			printSessionNotice(cmd, "attaching session", wsName, sessionName, resolvedBackend, false)
			if !cmd.Bool("yes") {
				ok, err := confirmPrompt(os.Stdin, commandWriter(cmd), fmt.Sprintf("attach session %s? [y/N] ", sessionName))
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
			}
			time.Sleep(750 * time.Millisecond)
			if err := attachSession(ctx, runner, resolvedBackend, sessionName); err != nil {
				return err
			}

			if sessionState != nil {
				sessionState.LastAttached = time.Now().Format(time.RFC3339)
				workspace.EnsureSessionState(&ws.State)
				ws.State.Sessions[sessionName] = *sessionState
				if err := workspace.SaveState(root, ws.State); err != nil {
					return err
				}
			}

			registerWorkspace(&cfg, wsName, root, time.Now())
			return config.SaveGlobal(cfgPath, cfg)
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
			cfg, _, err := loadGlobal(cmd.String("config"))
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
			cfg, cfgPath, err := loadGlobal(cmd.String("config"))
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

			name, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
			if err != nil {
				return err
			}
			ws, err := workspace.Load(root, cfg.Defaults)
			if err != nil {
				return err
			}
			wsName := workspaceName(ws, name, root)
			explicitName := firstNonEmpty(strings.TrimSpace(cmd.String("name")), strings.TrimSpace(cmd.Args().Get(1)))
			sessionName, sessionState, err := resolveSessionTarget(ws.State, explicitName, cfg.Defaults.SessionNameFormat, wsName)
			if err != nil {
				return err
			}

			backendValue := firstNonEmpty(cmd.String("backend"), cfg.Defaults.SessionBackend)
			backend, err := parseSessionBackend(backendValue)
			if err != nil {
				return err
			}

			if sessionState != nil && sessionState.Backend != "" {
				if parsed, err := parseSessionBackend(sessionState.Backend); err == nil {
					backend = parsed
				}
			}

			runner := execRunner{}
			resolvedBackend, err := resolveSessionBackend(backend, runner)
			if err != nil {
				return err
			}
			if resolvedBackend == sessionBackendExec {
				return fmt.Errorf("stop not supported for backend %q", resolvedBackend)
			}

			printSessionNotice(cmd, "stopping session", wsName, sessionName, resolvedBackend, false)
			if !cmd.Bool("yes") {
				ok, err := confirmPrompt(os.Stdin, commandWriter(cmd), fmt.Sprintf("stop session %s? [y/N] ", sessionName))
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
			}
			if err := stopSession(ctx, runner, resolvedBackend, sessionName); err != nil {
				return err
			}

			if sessionState != nil {
				workspace.EnsureSessionState(&ws.State)
				delete(ws.State.Sessions, sessionName)
				if err := workspace.SaveState(root, ws.State); err != nil {
					return err
				}
			}

			registerWorkspace(&cfg, wsName, root, time.Now())
			if err := config.SaveGlobal(cfgPath, cfg); err != nil {
				return err
			}
			printSessionNotice(cmd, "session stopped", wsName, sessionName, resolvedBackend, false)
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
			cfg, cfgPath, err := loadGlobal(cmd.String("config"))
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

			name, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
			if err != nil {
				return err
			}
			ws, err := workspace.Load(root, cfg.Defaults)
			if err != nil {
				return err
			}
			wsName := workspaceName(ws, name, root)

			mode := outputModeFromContext(cmd)
			styles := output.NewStyles(commandWriter(cmd), mode.Plain)

			workspace.EnsureSessionState(&ws.State)
			if len(ws.State.Sessions) == 0 {
				if mode.JSON {
					return output.WriteJSON(commandWriter(cmd), []sessionRecord{})
				}
				msg := "no sessions recorded"
				if styles.Enabled {
					msg = styles.Render(styles.Muted, msg)
				}
				_, err := fmt.Fprintln(commandWriter(cmd), msg)
				return err
			}

			runner := execRunner{}
			names := make([]string, 0, len(ws.State.Sessions))
			for name := range ws.State.Sessions {
				names = append(names, name)
			}
			sort.Strings(names)

			records := make([]sessionRecord, 0, len(ws.State.Sessions))
			for _, name := range names {
				state := ws.State.Sessions[name]
				backend := sessionBackend(strings.ToLower(strings.TrimSpace(state.Backend)))
				if backend != "" {
					if parsed, err := parseSessionBackend(state.Backend); err == nil {
						backend = parsed
					}
				}
				running := false
				resolvedBackend := backend
				if backend == sessionBackendAuto {
					if resolved, err := resolveSessionBackend(backend, runner); err == nil {
						resolvedBackend = resolved
					}
				}
				if resolvedBackend != "" && resolvedBackend != sessionBackendExec {
					if exists, err := sessionExists(ctx, runner, resolvedBackend, name); err == nil {
						running = exists
					}
				}
				records = append(records, sessionRecord{
					Name:         name,
					Backend:      state.Backend,
					Command:      state.Command,
					StartedAt:    state.StartedAt,
					LastAttached: state.LastAttached,
					Running:      running,
				})
			}

			if mode.JSON {
				return output.WriteJSON(commandWriter(cmd), records)
			}

			rows := make([][]string, 0, len(records))
			for _, record := range records {
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
			if err != nil {
				return err
			}

			registerWorkspace(&cfg, wsName, root, time.Now())
			return config.SaveGlobal(cfgPath, cfg)
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
			cfg, _, err := loadGlobal(cmd.String("config"))
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
			cfg, cfgPath, err := loadGlobal(cmd.String("config"))
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

			name, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
			if err != nil {
				return err
			}
			ws, err := workspace.Load(root, cfg.Defaults)
			if err != nil {
				return err
			}
			wsName := workspaceName(ws, name, root)
			explicitName := firstNonEmpty(strings.TrimSpace(cmd.String("name")), strings.TrimSpace(cmd.Args().Get(1)))
			sessionName, sessionState, err := resolveSessionTarget(ws.State, explicitName, cfg.Defaults.SessionNameFormat, wsName)
			if err != nil {
				return err
			}
			if sessionState == nil {
				return fmt.Errorf("session not recorded: %s", sessionName)
			}

			backend := sessionBackend(strings.ToLower(strings.TrimSpace(sessionState.Backend)))
			if backend != "" {
				if parsed, err := parseSessionBackend(sessionState.Backend); err == nil {
					backend = parsed
				}
			}
			if strings.TrimSpace(cmd.String("backend")) != "" {
				if parsed, err := parseSessionBackend(cmd.String("backend")); err == nil {
					backend = parsed
				}
			}

			runner := execRunner{}
			running := false
			resolvedBackend := backend
			if backend == sessionBackendAuto {
				if resolved, err := resolveSessionBackend(backend, runner); err == nil {
					resolvedBackend = resolved
				}
			}
			if resolvedBackend != "" && resolvedBackend != sessionBackendExec {
				if exists, err := sessionExists(ctx, runner, resolvedBackend, sessionName); err == nil {
					running = exists
				}
			}

			record := sessionRecord{
				Name:         sessionName,
				Backend:      sessionState.Backend,
				Command:      sessionState.Command,
				StartedAt:    sessionState.StartedAt,
				LastAttached: sessionState.LastAttached,
				Running:      running,
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
			if _, err := fmt.Fprint(commandWriter(cmd), rendered); err != nil {
				return err
			}

			registerWorkspace(&cfg, wsName, root, time.Now())
			return config.SaveGlobal(cfgPath, cfg)
		},
	}
}

func resolveSessionName(explicit, format, workspaceName string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return formatSessionName(format, workspaceName)
}

func formatSessionName(format, workspaceName string) string {
	if strings.TrimSpace(format) == "" {
		format = "workset:{workspace}"
	}
	placeholder := "{workspace}"
	if !strings.Contains(format, placeholder) {
		return format
	}
	return strings.ReplaceAll(format, placeholder, workspaceName)
}

func resolveSessionTarget(state workspace.State, explicit, format, workspaceName string) (string, *workspace.SessionState, error) {
	workspace.EnsureSessionState(&state)
	if strings.TrimSpace(explicit) != "" {
		if entry, ok := state.Sessions[explicit]; ok {
			copied := entry
			return explicit, &copied, nil
		}
		return explicit, nil, nil
	}
	defaultName := formatSessionName(format, workspaceName)
	if entry, ok := state.Sessions[defaultName]; ok {
		copied := entry
		return defaultName, &copied, nil
	}
	if len(state.Sessions) == 1 {
		for name, entry := range state.Sessions {
			copied := entry
			return name, &copied, nil
		}
	}
	if len(state.Sessions) == 0 {
		return "", nil, fmt.Errorf("no sessions recorded; pass --name to attach or stop")
	}
	return "", nil, fmt.Errorf("multiple sessions recorded; pass --name to attach or stop")
}

func workspaceName(ws workspace.Workspace, fallback, root string) string {
	if ws.Config.Name != "" {
		return ws.Config.Name
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return filepath.Base(root)
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

func printSessionNotice(cmd *cli.Command, title, workspaceName, sessionName string, backend sessionBackend, includeAttach bool) {
	w := commandWriter(cmd)
	styles := output.NewStyles(w, false)
	header := title
	if styles.Enabled {
		header = styles.Render(styles.Title, header)
	}
	_, _ = fmt.Fprintln(w, header)
	if workspaceName != "" {
		_, _ = fmt.Fprintf(w, "  workspace: %s\n", workspaceName)
	}
	_, _ = fmt.Fprintf(w, "  session:   %s\n", sessionName)
	_, _ = fmt.Fprintf(w, "  backend:   %s\n", backend)
	if includeAttach {
		_, _ = fmt.Fprintf(w, "  attach:    workset session attach %s %s\n", workspaceName, sessionName)
	}
	if hint := detachHint(backend); hint != "" {
		_, _ = fmt.Fprintf(w, "  detach:    %s\n", hint)
	}
}

func detachHint(backend sessionBackend) string {
	switch backend {
	case sessionBackendTmux:
		return "Ctrl-b d"
	case sessionBackendScreen:
		return "Ctrl-a d"
	default:
		return ""
	}
}
