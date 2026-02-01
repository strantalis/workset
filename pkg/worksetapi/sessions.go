package worksetapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/session"
	"github.com/strantalis/workset/internal/workspace"
)

// StartSession starts a new session for a workspace.
func (s *Service) StartSession(ctx context.Context, input SessionStartInput) (SessionStartResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return SessionStartResult{}, err
	}

	workspaceArg := strings.TrimSpace(input.Workspace.Value)
	if workspaceArg == "" {
		workspaceArg = strings.TrimSpace(cfg.Defaults.Workspace)
	}
	if workspaceArg == "" {
		return SessionStartResult{}, ValidationError{Message: "workspace required"}
	}

	name, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
	if err != nil {
		return SessionStartResult{}, err
	}

	ws, err := s.workspaces.Load(ctx, root, cfg.Defaults)
	if err != nil {
		return SessionStartResult{}, err
	}
	wsName := workspaceName(ws, name, root)
	sessionName := resolveSessionName(input.Name, cfg.Defaults.SessionNameFormat, wsName)

	backend, err := session.ParseBackend(firstNonEmpty(input.Backend, cfg.Defaults.SessionBackend))
	if err != nil {
		return SessionStartResult{}, err
	}

	resolvedBackend, err := session.ResolveBackend(backend, s.runner)
	if err != nil {
		return SessionStartResult{}, err
	}

	normalizedName, renamed, err := session.NormalizeNameForBackend(resolvedBackend, sessionName)
	if err != nil {
		return SessionStartResult{}, err
	}
	if renamed {
		sessionName = normalizedName
	}
	if err := ensureSessionNameAvailable(ctx, s.runner, ws.State, sessionName, resolvedBackend); err != nil {
		return SessionStartResult{}, err
	}

	env := append(os.Environ(),
		"WORKSET_ROOT="+root,
		"WORKSET_CONFIG="+workspace.WorksetFile(root),
	)
	if wsName != "" {
		env = append(env, "WORKSET_WORKSPACE="+wsName)
	}

	if input.Interactive && resolvedBackend != session.BackendExec {
		return SessionStartResult{}, ValidationError{Message: "--interactive is only supported with the exec backend (use --backend exec)"}
	}

	attachAllowed, attachNote := allowAttachAfterStart(resolvedBackend, input.Attach)

	theme := session.ResolveTheme(cfg.Defaults)
	themeLabel, themeHint := session.ThemeNotice(theme, resolvedBackend)

	notice := SessionNotice{
		Title:         "starting session",
		Workspace:     wsName,
		Session:       sessionName,
		Backend:       string(resolvedBackend),
		ThemeLabel:    themeLabel,
		ThemeHint:     themeHint,
		DetachHint:    detachHint(resolvedBackend),
		AttachCommand: "",
	}
	if renamed {
		notice.NameNotice = "note: tmux session names use '_' for unsupported characters; using " + sessionName
	}
	if resolvedBackend != session.BackendExec {
		notice.AttachCommand = fmt.Sprintf("workset session attach %s %s", wsName, sessionName)
	}
	if attachNote != "" {
		notice.AttachNote = attachNote
	}

	if !input.Confirmed {
		return SessionStartResult{}, ConfirmationRequired{Message: fmt.Sprintf("start session %s?", sessionName)}
	}

	if resolvedBackend == session.BackendExec {
		time.Sleep(750 * time.Millisecond)
	}

	if err := session.Start(ctx, s.runner, resolvedBackend, root, sessionName, input.Command, env, input.Interactive); err != nil {
		return SessionStartResult{}, err
	}
	if err := session.ApplyTheme(ctx, s.runner, resolvedBackend, sessionName, theme); err != nil {
		if s.logf != nil {
			s.logf("warning: failed to apply session theme: %v", err)
		}
	}

	if resolvedBackend != session.BackendExec {
		workspace.EnsureSessionState(&ws.State)
		ws.State.Sessions[sessionName] = workspace.SessionState{
			Backend:   string(resolvedBackend),
			Name:      sessionName,
			Command:   input.Command,
			StartedAt: s.clock().Format(time.RFC3339),
		}
		if err := s.workspaces.SaveState(ctx, root, ws.State); err != nil {
			return SessionStartResult{}, err
		}
	}

	attached := false
	if attachAllowed {
		if err := session.Attach(ctx, s.runner, resolvedBackend, sessionName); err != nil {
			return SessionStartResult{}, err
		}
		attached = true
		if resolvedBackend != session.BackendExec {
			if markSessionAttached(&ws.State, sessionName, s.clock()) {
				if err := s.workspaces.SaveState(ctx, root, ws.State); err != nil {
					return SessionStartResult{}, err
				}
			}
		}
	}

	registerWorkspace(&cfg, wsName, root, s.clock())
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return SessionStartResult{}, err
	}
	return SessionStartResult{Notice: notice, Attached: attached, Config: info}, nil
}

// AttachSession attaches to a running session.
func (s *Service) AttachSession(ctx context.Context, input SessionAttachInput) (SessionActionResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return SessionActionResult{}, err
	}

	workspaceArg := strings.TrimSpace(input.Workspace.Value)
	if workspaceArg == "" {
		workspaceArg = strings.TrimSpace(cfg.Defaults.Workspace)
	}
	if workspaceArg == "" {
		return SessionActionResult{}, ValidationError{Message: "workspace required"}
	}

	name, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
	if err != nil {
		return SessionActionResult{}, err
	}
	ws, err := s.workspaces.Load(ctx, root, cfg.Defaults)
	if err != nil {
		return SessionActionResult{}, err
	}
	wsName := workspaceName(ws, name, root)

	explicitName := strings.TrimSpace(input.Name)
	sessionName, sessionState, err := resolveSessionTarget(ws.State, explicitName, cfg.Defaults.SessionNameFormat, wsName)
	if err != nil {
		return SessionActionResult{}, err
	}

	backendValue := firstNonEmpty(input.Backend, cfg.Defaults.SessionBackend)
	backend, err := session.ParseBackend(backendValue)
	if err != nil {
		return SessionActionResult{}, err
	}
	if sessionState != nil && sessionState.Backend != "" {
		if parsed, err := session.ParseBackend(sessionState.Backend); err == nil {
			backend = parsed
		}
	}

	resolvedBackend, err := session.ResolveBackend(backend, s.runner)
	if err != nil {
		return SessionActionResult{}, err
	}
	normalizedName, _, err := session.NormalizeNameForBackend(resolvedBackend, sessionName)
	if err != nil {
		return SessionActionResult{}, err
	}
	if normalizedName != sessionName {
		if sessionState != nil {
			workspace.EnsureSessionState(&ws.State)
			state := *sessionState
			state.Name = normalizedName
			delete(ws.State.Sessions, sessionName)
			ws.State.Sessions[normalizedName] = state
			sessionState = &state
		}
		sessionName = normalizedName
	}
	if resolvedBackend == session.BackendExec {
		return SessionActionResult{}, ValidationError{Message: fmt.Sprintf("attach not supported for backend %q", resolvedBackend)}
	}

	notice := SessionNotice{
		Title:      "attaching session",
		Workspace:  wsName,
		Session:    sessionName,
		Backend:    string(resolvedBackend),
		DetachHint: detachHint(resolvedBackend),
	}

	if !input.Confirmed {
		return SessionActionResult{}, ConfirmationRequired{Message: fmt.Sprintf("attach session %s?", sessionName)}
	}

	time.Sleep(750 * time.Millisecond)
	if err := session.ApplyTheme(ctx, s.runner, resolvedBackend, sessionName, session.ResolveTheme(cfg.Defaults)); err != nil {
		if s.logf != nil {
			s.logf("warning: failed to apply session theme: %v", err)
		}
	}
	if err := session.Attach(ctx, s.runner, resolvedBackend, sessionName); err != nil {
		return SessionActionResult{}, err
	}

	if markSessionAttached(&ws.State, sessionName, s.clock()) {
		if err := s.workspaces.SaveState(ctx, root, ws.State); err != nil {
			return SessionActionResult{}, err
		}
	}

	registerWorkspace(&cfg, wsName, root, s.clock())
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return SessionActionResult{}, err
	}

	return SessionActionResult{Notice: notice, Config: info}, nil
}

// StopSession stops a running session.
func (s *Service) StopSession(ctx context.Context, input SessionStopInput) (SessionActionResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return SessionActionResult{}, err
	}

	workspaceArg := strings.TrimSpace(input.Workspace.Value)
	if workspaceArg == "" {
		workspaceArg = strings.TrimSpace(cfg.Defaults.Workspace)
	}
	if workspaceArg == "" {
		return SessionActionResult{}, ValidationError{Message: "workspace required"}
	}

	name, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
	if err != nil {
		return SessionActionResult{}, err
	}
	ws, err := s.workspaces.Load(ctx, root, cfg.Defaults)
	if err != nil {
		return SessionActionResult{}, err
	}
	wsName := workspaceName(ws, name, root)

	explicitName := strings.TrimSpace(input.Name)
	sessionName, sessionState, err := resolveSessionTarget(ws.State, explicitName, cfg.Defaults.SessionNameFormat, wsName)
	if err != nil {
		return SessionActionResult{}, err
	}

	backendValue := firstNonEmpty(input.Backend, cfg.Defaults.SessionBackend)
	backend, err := session.ParseBackend(backendValue)
	if err != nil {
		return SessionActionResult{}, err
	}
	if sessionState != nil && sessionState.Backend != "" {
		if parsed, err := session.ParseBackend(sessionState.Backend); err == nil {
			backend = parsed
		}
	}

	resolvedBackend, err := session.ResolveBackend(backend, s.runner)
	if err != nil {
		return SessionActionResult{}, err
	}
	normalizedName, _, err := session.NormalizeNameForBackend(resolvedBackend, sessionName)
	if err != nil {
		return SessionActionResult{}, err
	}
	if normalizedName != sessionName {
		if sessionState != nil {
			workspace.EnsureSessionState(&ws.State)
			state := *sessionState
			state.Name = normalizedName
			delete(ws.State.Sessions, sessionName)
			ws.State.Sessions[normalizedName] = state
			sessionState = &state
		}
		sessionName = normalizedName
	}
	if resolvedBackend == session.BackendExec {
		return SessionActionResult{}, ValidationError{Message: fmt.Sprintf("stop not supported for backend %q", resolvedBackend)}
	}

	notice := SessionNotice{
		Title:      "stopping session",
		Workspace:  wsName,
		Session:    sessionName,
		Backend:    string(resolvedBackend),
		DetachHint: detachHint(resolvedBackend),
	}

	if !input.Confirmed {
		return SessionActionResult{}, ConfirmationRequired{Message: fmt.Sprintf("stop session %s?", sessionName)}
	}

	if err := session.Stop(ctx, s.runner, resolvedBackend, sessionName); err != nil {
		return SessionActionResult{}, err
	}

	if sessionState != nil {
		workspace.EnsureSessionState(&ws.State)
		delete(ws.State.Sessions, sessionName)
		if err := s.workspaces.SaveState(ctx, root, ws.State); err != nil {
			return SessionActionResult{}, err
		}
	}

	registerWorkspace(&cfg, wsName, root, s.clock())
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return SessionActionResult{}, err
	}

	notice.Title = "session stopped"
	return SessionActionResult{Notice: notice, Config: info}, nil
}

// ListSessions lists known sessions for a workspace.
func (s *Service) ListSessions(ctx context.Context, selector WorkspaceSelector) (SessionListResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return SessionListResult{}, err
	}

	workspaceArg := strings.TrimSpace(selector.Value)
	if workspaceArg == "" {
		workspaceArg = strings.TrimSpace(cfg.Defaults.Workspace)
	}
	if workspaceArg == "" {
		return SessionListResult{}, ValidationError{Message: "workspace required"}
	}

	name, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
	if err != nil {
		return SessionListResult{}, err
	}
	ws, err := s.workspaces.Load(ctx, root, cfg.Defaults)
	if err != nil {
		return SessionListResult{}, err
	}
	wsName := workspaceName(ws, name, root)

	workspace.EnsureSessionState(&ws.State)
	if len(ws.State.Sessions) == 0 {
		registerWorkspace(&cfg, wsName, root, s.clock())
		if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
			return SessionListResult{}, err
		}
		return SessionListResult{Sessions: []SessionRecordJSON{}, Config: info}, nil
	}

	names := make([]string, 0, len(ws.State.Sessions))
	for name := range ws.State.Sessions {
		names = append(names, name)
	}
	sort.Strings(names)

	records := make([]SessionRecordJSON, 0, len(ws.State.Sessions))
	for _, sessionName := range names {
		state := ws.State.Sessions[sessionName]
		backend := session.Backend(strings.ToLower(strings.TrimSpace(state.Backend)))
		if backend != "" {
			if parsed, err := session.ParseBackend(state.Backend); err == nil {
				backend = parsed
			}
		}
		running := false
		resolvedBackend := backend
		if backend == session.BackendAuto {
			if resolved, err := session.ResolveBackend(backend, s.runner); err == nil {
				resolvedBackend = resolved
			}
		}
		if resolvedBackend != "" && resolvedBackend != session.BackendExec {
			if exists, err := session.Running(ctx, s.runner, resolvedBackend, sessionName); err == nil {
				running = exists
			} else if err != nil && s.logf != nil {
				s.logf("warning: failed to check session %s: %v", sessionName, err)
			}
		}
		records = append(records, SessionRecordJSON{
			Name:         sessionName,
			Backend:      state.Backend,
			Command:      state.Command,
			StartedAt:    state.StartedAt,
			LastAttached: state.LastAttached,
			Running:      running,
		})
	}

	registerWorkspace(&cfg, wsName, root, s.clock())
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return SessionListResult{}, err
	}

	return SessionListResult{Sessions: records, Config: info}, nil
}

// ShowSession returns a single session record.
func (s *Service) ShowSession(ctx context.Context, input SessionShowInput) (SessionRecordJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return SessionRecordJSON{}, info, err
	}

	workspaceArg := strings.TrimSpace(input.Workspace.Value)
	if workspaceArg == "" {
		workspaceArg = strings.TrimSpace(cfg.Defaults.Workspace)
	}
	if workspaceArg == "" {
		return SessionRecordJSON{}, info, ValidationError{Message: "workspace required"}
	}

	name, root, err := resolveWorkspaceTarget(workspaceArg, &cfg)
	if err != nil {
		return SessionRecordJSON{}, info, err
	}
	ws, err := s.workspaces.Load(ctx, root, cfg.Defaults)
	if err != nil {
		return SessionRecordJSON{}, info, err
	}
	wsName := workspaceName(ws, name, root)

	explicitName := firstNonEmpty(strings.TrimSpace(input.Name))
	sessionName, sessionState, err := resolveSessionTarget(ws.State, explicitName, cfg.Defaults.SessionNameFormat, wsName)
	if err != nil {
		return SessionRecordJSON{}, info, err
	}
	if sessionState == nil {
		return SessionRecordJSON{}, info, NotFoundError{Message: "session not recorded: " + sessionName}
	}

	backend := session.Backend(strings.ToLower(strings.TrimSpace(sessionState.Backend)))
	if backend != "" {
		if parsed, err := session.ParseBackend(sessionState.Backend); err == nil {
			backend = parsed
		}
	}
	if strings.TrimSpace(input.Backend) != "" {
		if parsed, err := session.ParseBackend(input.Backend); err == nil {
			backend = parsed
		}
	}

	running := false
	resolvedBackend := backend
	if backend == session.BackendAuto {
		if resolved, err := session.ResolveBackend(backend, s.runner); err == nil {
			resolvedBackend = resolved
		}
	}
	if resolvedBackend != "" && resolvedBackend != session.BackendExec {
		if exists, err := session.Running(ctx, s.runner, resolvedBackend, sessionName); err == nil {
			running = exists
		} else if err != nil && s.logf != nil {
			s.logf("warning: failed to check session %s: %v", sessionName, err)
		}
	}

	record := SessionRecordJSON{
		Name:         sessionName,
		Backend:      sessionState.Backend,
		Command:      sessionState.Command,
		StartedAt:    sessionState.StartedAt,
		LastAttached: sessionState.LastAttached,
		Running:      running,
	}

	registerWorkspace(&cfg, wsName, root, s.clock())
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return SessionRecordJSON{}, info, err
	}

	return record, info, nil
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
