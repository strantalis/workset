package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/strantalis/workset/pkg/sessiond"
	"github.com/strantalis/workset/pkg/worksetapi"
)

func (a *App) StartWorkspaceTerminal(workspaceID, terminalID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	sessionID := terminalSessionID(workspaceID, terminalID)

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	root, err := a.resolveWorkspaceRoot(ctx, workspaceID)
	if err != nil {
		return err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}

	for {
		a.terminalMu.Lock()
		existing := a.terminals[sessionID]
		if existing != nil {
			a.terminalMu.Unlock()
			waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := existing.waitReady(waitCtx)
			cancel()
			if err != nil {
				return err
			}
			existing.mu.Lock()
			hasSession := existing.client != nil
			streamActive := existing.stream != nil
			existing.mu.Unlock()
			if hasSession {
				if streamActive {
					return nil
				}
				go a.streamTerminal(existing)
				return nil
			}
			a.terminalMu.Lock()
			if current := a.terminals[sessionID]; current == existing {
				delete(a.terminals, sessionID)
			}
			a.terminalMu.Unlock()
			continue
		}

		session := newTerminalSession(workspaceID, terminalID, root)
		var restore terminalModeState
		var hasRestore bool
		if a.restoredModes != nil {
			restore, hasRestore = a.restoredModes[sessionID]
		}
		if hasRestore {
			session.mu.Lock()
			session.altScreen = restore.AltScreen
			session.mouseMask = restore.MouseMask
			session.mouseSGR = restore.MouseSGR
			session.mouseUTF8 = restore.MouseUTF8
			session.mouseURXVT = restore.MouseURXVT
			session.mu.Unlock()
		}
		client, err := a.getSessiondClient()
		if err != nil {
			a.terminalMu.Unlock()
			return err
		}
		session.client = client
		a.terminals[sessionID] = session
		a.terminalMu.Unlock()

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		resp, createErr := session.client.Create(ctx, sessionID, root)
		if createErr == nil && resp.Existing {
			session.resumed = true
		}
		err = createErr
		cancel()
		if err == nil {
			session.mu.Lock()
			session.lastActivity = time.Now()
			session.mu.Unlock()
		}
		session.markReady(err)
		if err != nil {
			a.terminalMu.Lock()
			if current := a.terminals[sessionID]; current == session {
				delete(a.terminals, sessionID)
			}
			a.terminalMu.Unlock()
			a.emitTerminalLifecycle("error", session, err.Error())
			return err
		}
		a.ensureIdleWatcher(session)
		if session.resumed {
			a.emitTerminalLifecycle("started", session, "Session resumed.")
		} else {
			a.emitTerminalLifecycle("started", session, "")
		}
		emitModes := hasRestore
		if emitModes {
			session.mu.Lock()
			altScreen := session.altScreen
			mouseEnabled := session.mouseEnabled()
			mouseSGR := session.mouseSGR
			mouseEncoding := session.mouseEncoding()
			session.mu.Unlock()
			a.emitTerminalModes(session, altScreen, mouseEnabled, mouseSGR, mouseEncoding)
			_ = a.persistTerminalState()
		}
		go a.streamTerminal(session)
		return nil
	}
}

func (a *App) CreateWorkspaceTerminal(workspaceID string) (TerminalCreatePayload, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalCreatePayload{}, fmt.Errorf("workspace id required")
	}
	terminalID := uuid.NewString()
	return TerminalCreatePayload{WorkspaceID: workspaceID, TerminalID: terminalID}, nil
}

func (a *App) WriteWorkspaceTerminal(workspaceID, terminalID, data string) error {
	session, err := a.getTerminal(workspaceID, terminalID)
	if err != nil {
		return err
	}
	return session.Write(data)
}

func (a *App) AckWorkspaceTerminal(workspaceID, terminalID string, bytes int) error {
	if bytes <= 0 {
		return nil
	}
	session, err := a.getTerminal(workspaceID, terminalID)
	if err != nil {
		return err
	}
	session.mu.Lock()
	client := session.client
	streamID := session.streamID
	session.mu.Unlock()
	if client == nil {
		return fmt.Errorf("terminal not started")
	}
	if streamID == "" {
		return fmt.Errorf("terminal stream not ready")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return client.Ack(ctx, session.id, streamID, int64(bytes))
}

func (a *App) GetTerminalBacklog(workspaceID, terminalID string, since int64) (TerminalBacklogPayload, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalBacklogPayload{}, fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return TerminalBacklogPayload{}, fmt.Errorf("terminal id required")
	}
	sessionID := terminalSessionID(workspaceID, terminalID)
	client, err := a.getSessiondClient()
	if err != nil {
		return TerminalBacklogPayload{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	backlog, err := client.Backlog(ctx, sessionID, since)
	if err != nil {
		return TerminalBacklogPayload{}, err
	}
	return TerminalBacklogPayload{
		WorkspaceID: workspaceID,
		TerminalID:  terminalID,
		Data:        backlog.Data,
		NextOffset:  backlog.NextOffset,
		Truncated:   backlog.Truncated,
		Source:      backlog.Source,
	}, nil
}

func (a *App) GetTerminalSnapshot(workspaceID, terminalID string) (TerminalSnapshotPayload, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalSnapshotPayload{}, fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return TerminalSnapshotPayload{}, fmt.Errorf("terminal id required")
	}
	sessionID := terminalSessionID(workspaceID, terminalID)
	client, err := a.getSessiondClient()
	if err != nil {
		return TerminalSnapshotPayload{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	snap, err := client.Snapshot(ctx, sessionID)
	if err != nil {
		return TerminalSnapshotPayload{}, err
	}
	return TerminalSnapshotPayload{
		WorkspaceID: workspaceID,
		TerminalID:  terminalID,
		Data:        snap.Data,
		Source:      snap.Source,
		Kitty:       snap.Kitty,
	}, nil
}

func (a *App) GetTerminalBootstrap(workspaceID, terminalID string) (TerminalBootstrapPayload, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalBootstrapPayload{}, fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return TerminalBootstrapPayload{}, fmt.Errorf("terminal id required")
	}
	sessionID := terminalSessionID(workspaceID, terminalID)
	client, err := a.getSessiondClient()
	if err != nil {
		return TerminalBootstrapPayload{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	bootstrap, err := client.Bootstrap(ctx, sessionID)
	if err != nil {
		return TerminalBootstrapPayload{}, err
	}
	backlog := sessiond.BacklogResponse{}
	useBacklog := bootstrap.SafeToReplay && bootstrap.Snapshot == ""
	if useBacklog {
		if b, err := client.Backlog(ctx, sessionID, 0); err == nil && b.Data != "" {
			backlog = b
		} else {
			useBacklog = false
		}
	}
	a.terminalMu.Lock()
	session := a.terminals[sessionID]
	a.terminalMu.Unlock()
	if session != nil {
		session.mu.Lock()
		session.altScreen = bootstrap.AltScreen
		if bootstrap.MouseMask != 0 {
			session.mouseMask = bootstrap.MouseMask
		} else if bootstrap.Mouse {
			session.mouseMask = 1
		} else {
			session.mouseMask = 0
		}
		session.mouseSGR = bootstrap.MouseSGR
		session.mouseUTF8 = bootstrap.MouseEncoding == "utf8"
		session.mouseURXVT = bootstrap.MouseEncoding == "urxvt"
		session.mu.Unlock()
		_ = a.persistTerminalState()
	}
	if useBacklog {
		return TerminalBootstrapPayload{
			WorkspaceID:      workspaceID,
			TerminalID:       terminalID,
			Backlog:          backlog.Data,
			BacklogSource:    backlog.Source,
			BacklogTruncated: backlog.Truncated,
			NextOffset:       backlog.NextOffset,
			Source:           "sessiond",
			AltScreen:        bootstrap.AltScreen,
			Mouse:            bootstrap.Mouse,
			MouseSGR:         bootstrap.MouseSGR,
			MouseEncoding:    bootstrap.MouseEncoding,
			SafeToReplay:     bootstrap.SafeToReplay,
			InitialCredit:    bootstrap.InitialCredit,
		}, nil
	}
	return TerminalBootstrapPayload{
		WorkspaceID:      workspaceID,
		TerminalID:       terminalID,
		Snapshot:         bootstrap.Snapshot,
		SnapshotSource:   bootstrap.SnapshotSource,
		Kitty:            bootstrap.Kitty,
		Backlog:          bootstrap.Backlog,
		BacklogSource:    bootstrap.BacklogSource,
		BacklogTruncated: bootstrap.BacklogTruncated,
		NextOffset:       bootstrap.NextOffset,
		Source:           "sessiond",
		AltScreen:        bootstrap.AltScreen,
		Mouse:            bootstrap.Mouse,
		MouseSGR:         bootstrap.MouseSGR,
		MouseEncoding:    bootstrap.MouseEncoding,
		SafeToReplay:     bootstrap.SafeToReplay,
		InitialCredit:    bootstrap.InitialCredit,
	}, nil
}

func (a *App) LogTerminalDebug(payload TerminalDebugPayload) {
	logTerminalDebug(payload)
}

func (a *App) GetWorkspaceTerminalStatus(workspaceID, terminalID string) TerminalStatusPayload {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalStatusPayload{Active: false, Error: "workspace id required"}
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return TerminalStatusPayload{WorkspaceID: workspaceID, Active: false, Error: "terminal id required"}
	}
	sessionID := terminalSessionID(workspaceID, terminalID)
	a.terminalMu.Lock()
	session := a.terminals[sessionID]
	a.terminalMu.Unlock()
	if session != nil {
		session.mu.Lock()
		hasSession := !session.closed && session.client != nil
		session.mu.Unlock()
		if hasSession {
			return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: true}
		}
	}
	client, err := a.getSessiondClient()
	if err != nil {
		return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: false, Error: err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	list, err := client.List(ctx)
	if err != nil {
		return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: false, Error: err.Error()}
	}
	for _, info := range list.Sessions {
		if info.SessionID == sessionID && info.Running {
			return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: true}
		}
	}
	return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: false}
}

func (a *App) ResizeWorkspaceTerminal(workspaceID, terminalID string, cols, rows int) error {
	session, err := a.getTerminal(workspaceID, terminalID)
	if err != nil {
		return err
	}
	if cols < 2 {
		cols = 2
	}
	if rows < 1 {
		rows = 1
	}
	return session.Resize(cols, rows)
}

func (a *App) StopWorkspaceTerminal(workspaceID, terminalID string) error {
	a.terminalMu.Lock()
	sessionID := terminalSessionID(workspaceID, terminalID)
	session, ok := a.terminals[sessionID]
	if ok {
		delete(a.terminals, sessionID)
	}
	a.terminalMu.Unlock()
	if !ok {
		return nil
	}
	session.mu.Lock()
	client := session.client
	session.mu.Unlock()
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = client.Stop(ctx, sessionID)
		cancel()
	}
	err := session.CloseWithReason("closed")
	a.emitTerminalLifecycle("closed", session, "")
	_ = a.persistTerminalState()
	return err
}

func (a *App) getTerminal(workspaceID, terminalID string) (*terminalSession, error) {
	a.terminalMu.Lock()
	sessionID := terminalSessionID(workspaceID, terminalID)
	session := a.terminals[sessionID]
	a.terminalMu.Unlock()
	if session == nil {
		return nil, fmt.Errorf("terminal not found")
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err := session.waitReady(waitCtx)
	cancel()
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (a *App) latestTerminalForWorkspace(workspaceID string) *terminalSession {
	a.terminalMu.Lock()
	defer a.terminalMu.Unlock()
	var selected *terminalSession
	var latest time.Time
	for _, session := range a.terminals {
		if session.workspaceID != workspaceID {
			continue
		}
		session.mu.Lock()
		last := session.lastActivity
		closed := session.closed
		session.mu.Unlock()
		if closed {
			continue
		}
		if selected == nil || last.After(latest) {
			selected = session
			latest = last
		}
	}
	return selected
}

func (a *App) invalidateTerminalSessions(reason string) {
	a.terminalMu.Lock()
	sessions := make([]*terminalSession, 0, len(a.terminals))
	for _, session := range a.terminals {
		sessions = append(sessions, session)
	}
	a.terminalMu.Unlock()
	for _, session := range sessions {
		_ = session.CloseWithReason("sessiond reset")
		session.mu.Lock()
		session.client = nil
		session.mu.Unlock()
		if reason != "" {
			a.emitTerminalLifecycle("error", session, reason)
		}
	}
}

func (a *App) resolveWorkspaceRoot(ctx context.Context, workspaceID string) (string, error) {
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, err := a.service.ListWorkspaces(ctx)
	if err != nil {
		return "", err
	}
	for _, workspace := range result.Workspaces {
		if workspace.Name == workspaceID {
			return workspace.Path, nil
		}
	}
	return "", fmt.Errorf("workspace not found: %s", workspaceID)
}
