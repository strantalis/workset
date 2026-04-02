package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (a *App) startWorkspaceTerminal(workspaceID, terminalID string) error {
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
	root := ""

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
			client := existing.client
			existing.mu.Unlock()
			if client != nil {
				inspectCtx, inspectCancel := context.WithTimeout(ctx, 2*time.Second)
				info, inspectErr := client.Inspect(inspectCtx, sessionID)
				inspectCancel()
				if inspectErr == nil && info.Running {
					return nil
				}
				existing.mu.Lock()
				if root == "" {
					root = existing.path
				}
				existing.client = nil
				existing.mu.Unlock()
			}
			a.terminalMu.Lock()
			if current := a.terminals[sessionID]; current == existing {
				delete(a.terminals, sessionID)
			}
			a.terminalMu.Unlock()
			continue
		}
		a.terminalMu.Unlock()

		if root == "" {
			resolvedRoot, err := a.resolveWorkspaceRoot(ctx, workspaceID)
			if err != nil {
				return err
			}
			resolvedRoot, err = filepath.Abs(resolvedRoot)
			if err != nil {
				return err
			}
			root = resolvedRoot
		}

		session := newTerminalSession(workspaceID, terminalID, root)
		client, err := a.getTerminalServiceClient()
		if err != nil {
			return err
		}
		session.client = client
		a.terminalMu.Lock()
		a.terminals[sessionID] = session
		a.terminalMu.Unlock()

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		resp, createErr := session.client.Create(ctx, sessionID, root)
		_ = resp
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

func (a *App) stopWorkspaceTerminal(workspaceID, terminalID string) error {
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
	a.terminalMu.Unlock()
	if !ok {
		return nil
	}
	session.mu.Lock()
	client := session.client
	session.mu.Unlock()
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := client.Stop(ctx, sessionID)
		cancel()
		if err != nil {
			if !isTransientTerminalCallError(err) {
				return err
			}
			session.mu.Lock()
			session.client = nil
			session.mu.Unlock()
		}
	}
	a.terminalMu.Lock()
	if current := a.terminals[sessionID]; current == session {
		delete(a.terminals, sessionID)
	}
	a.terminalMu.Unlock()
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
		_ = session.CloseWithReason("terminal service reset")
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
