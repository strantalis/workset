package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (a *App) StartWorkspaceTerminal(workspaceID, terminalID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return fmt.Errorf("terminal id required")
	}
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
			streamActive := existing.stream != nil || existing.streamCancel != nil
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
			return err
		}
		a.ensureIdleWatcher(session)
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
			logTerminalDebug(TerminalDebugPayload{
				WorkspaceID: workspaceID,
				TerminalID:  terminalID,
				Event:       "status_active_memory",
				Details:     "{}",
			})
			return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: true}
		}
	}
	client, err := a.getSessiondClient()
	if err != nil {
		logTerminalDebug(TerminalDebugPayload{
			WorkspaceID: workspaceID,
			TerminalID:  terminalID,
			Event:       "status_client_error",
			Details:     err.Error(),
		})
		return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: false, Error: err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	list, err := client.List(ctx)
	if err != nil {
		logTerminalDebug(TerminalDebugPayload{
			WorkspaceID: workspaceID,
			TerminalID:  terminalID,
			Event:       "status_list_error",
			Details:     err.Error(),
		})
		return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: false, Error: err.Error()}
	}
	for _, info := range list.Sessions {
		if info.SessionID == sessionID && info.Running {
			logTerminalDebug(TerminalDebugPayload{
				WorkspaceID: workspaceID,
				TerminalID:  terminalID,
				Event:       "status_active_sessiond",
				Details:     "{}",
			})
			return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: true}
		}
	}
	logTerminalDebug(TerminalDebugPayload{
		WorkspaceID: workspaceID,
		TerminalID:  terminalID,
		Event:       "status_inactive",
		Details:     "{}",
	})
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
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return fmt.Errorf("terminal id required")
	}
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
	return err
}

func (a *App) getTerminal(workspaceID, terminalID string) (*terminalSession, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return nil, fmt.Errorf("terminal id required")
	}
	a.terminalMu.Lock()
	sessionID := terminalSessionID(workspaceID, terminalID)
	session := a.terminals[sessionID]
	a.terminalMu.Unlock()
	if session == nil {
		logTerminalDebug(TerminalDebugPayload{
			WorkspaceID: workspaceID,
			TerminalID:  terminalID,
			Event:       "get_terminal_missing",
			Details:     fmt.Sprintf(`{"sessionId":"%s"}`, sessionID),
		})
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

func (a *App) invalidateTerminalSessions(_ string) {
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
	}
}

func (a *App) resolveWorkspaceRoot(ctx context.Context, workspaceID string) (string, error) {
	a.ensureService()
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
