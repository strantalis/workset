package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var transientTerminalErrorMarkers = []string{
	"session not found",
	"terminal not started",
	"terminal not found",
	"terminal stream not ready",
}

func isTransientTerminalCallError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range transientTerminalErrorMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (a *App) normalizeWindowName(name string) string {
	candidate := strings.TrimSpace(name)
	if candidate == "" {
		return a.mainWindowName
	}
	return candidate
}

func (a *App) workspaceTerminalSessions(workspaceID string) []*terminalSession {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil
	}
	a.terminalMu.Lock()
	sessions := make([]*terminalSession, 0, len(a.terminals))
	for _, session := range a.terminals {
		if session == nil || session.workspaceID != workspaceID {
			continue
		}
		sessions = append(sessions, session)
	}
	a.terminalMu.Unlock()
	return sessions
}

func (a *App) setTerminalSessionOwner(session *terminalSession, windowName string) error {
	if session == nil {
		return nil
	}
	windowName = a.normalizeWindowName(windowName)
	session.mu.Lock()
	client := session.client
	sessionID := session.id
	workspaceID := session.workspaceID
	terminalID := session.terminalID
	session.mu.Unlock()
	if client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	err := client.SetOwner(ctx, sessionID, windowName)
	cancel()
	if err != nil {
		if isTransientTerminalCallError(err) {
			session.mu.Lock()
			session.client = nil
			session.mu.Unlock()
		}
		logTerminalDebug(TerminalDebugPayload{
			WorkspaceID: workspaceID,
			TerminalID:  terminalID,
			Event:       "owner_sync_sessiond",
			Details: fmt.Sprintf(
				`{"owner":%q,"sessionId":%q,"ok":false,"transient":%t,"error":%q}`,
				windowName,
				sessionID,
				isTransientTerminalCallError(err),
				err.Error(),
			),
		})
		return err
	}
	logTerminalDebug(TerminalDebugPayload{
		WorkspaceID: workspaceID,
		TerminalID:  terminalID,
		Event:       "owner_sync_sessiond",
		Details:     fmt.Sprintf(`{"owner":%q,"sessionId":%q,"ok":true}`, windowName, sessionID),
	})
	return nil
}

func (a *App) transferWorkspaceTerminalOwner(workspaceID, windowName string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace id required")
	}
	windowName = a.normalizeWindowName(windowName)
	sessions := a.workspaceTerminalSessions(workspaceID)
	var firstErr error
	for _, session := range sessions {
		if err := a.setTerminalSessionOwner(session, windowName); err != nil && !isTransientTerminalCallError(err) {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (a *App) bestEffortTransferWorkspaceTerminalOwner(workspaceID, windowName string) {
	if err := a.transferWorkspaceTerminalOwner(workspaceID, windowName); err != nil {
		logTerminalDebug(TerminalDebugPayload{
			WorkspaceID: workspaceID,
			TerminalID:  "__owner__",
			Event:       "owner_transfer_error",
			Details:     fmt.Sprintf(`{"owner":%q,"error":%q}`, a.normalizeWindowName(windowName), err.Error()),
		})
	}
}

func (a *App) GetCurrentWindowName(ctx context.Context) string {
	if ctx != nil {
		if value := ctx.Value(application.WindowKey); value != nil {
			switch win := value.(type) {
			case application.Window:
				if win != nil {
					return a.normalizeWindowName(win.Name())
				}
			case interface{ Name() string }:
				return a.normalizeWindowName(win.Name())
			}
		}
	}
	return a.mainWindowName
}

func (a *App) resolveCallerWindowName(ctx context.Context, windowName string) string {
	candidate := strings.TrimSpace(windowName)
	if candidate != "" {
		return a.normalizeWindowName(candidate)
	}
	return a.GetCurrentWindowName(ctx)
}

func (a *App) GetWorkspaceTerminalOwner(workspaceID string) string {
	session := a.latestTerminalForWorkspace(workspaceID)
	if session == nil {
		return a.mainWindowName
	}
	session.mu.Lock()
	client := session.client
	sessionID := session.id
	session.mu.Unlock()
	if client == nil {
		return a.mainWindowName
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	owner, err := client.GetOwner(ctx, sessionID)
	if err != nil {
		return a.mainWindowName
	}
	if value := strings.TrimSpace(owner.Owner); value != "" {
		return value
	}
	return a.mainWindowName
}

func (a *App) startWorkspaceTerminalForOwner(
	workspaceID,
	terminalID,
	windowName string,
) error {
	if err := a.StartWorkspaceTerminal(workspaceID, terminalID); err != nil {
		return err
	}
	return a.transferWorkspaceTerminalOwner(workspaceID, windowName)
}

func (a *App) StartWorkspaceTerminalSessionForWindow(
	ctx context.Context,
	workspaceID,
	terminalID string,
) (TerminalSessionDescriptor, error) {
	windowName := a.resolveCallerWindowName(ctx, "")
	if err := a.ensureSessiondDescriptorSupport(); err != nil {
		return TerminalSessionDescriptor{}, err
	}
	if err := a.startWorkspaceTerminalForOwner(workspaceID, terminalID, windowName); err != nil {
		return TerminalSessionDescriptor{}, err
	}
	return a.workspaceTerminalSessionDescriptor(workspaceID, terminalID, windowName)
}

func (a *App) workspaceTerminalSessionDescriptor(
	workspaceID,
	terminalID,
	windowName string,
) (TerminalSessionDescriptor, error) {
	if err := a.ensureSessiondDescriptorSupport(); err != nil {
		return TerminalSessionDescriptor{}, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalSessionDescriptor{}, fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return TerminalSessionDescriptor{}, fmt.Errorf("terminal id required")
	}
	windowName = a.normalizeWindowName(windowName)
	sessionID := terminalSessionID(workspaceID, terminalID)

	client, err := a.getSessiondClient()
	if err != nil {
		return TerminalSessionDescriptor{}, err
	}
	inspectCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	info, err := client.Inspect(inspectCtx, sessionID)
	if err != nil {
		return TerminalSessionDescriptor{}, err
	}
	serverInfo, err := client.Info(inspectCtx)
	if err != nil {
		return TerminalSessionDescriptor{}, err
	}
	owner := strings.TrimSpace(info.Owner)
	return TerminalSessionDescriptor{
		WorkspaceID:   workspaceID,
		TerminalID:    terminalID,
		SessionID:     sessionID,
		WindowName:    windowName,
		Owner:         owner,
		CanWrite:      owner == "" || owner == windowName,
		Running:       info.Running,
		CurrentOffset: info.CurrentOffset,
		SocketURL:     strings.TrimSpace(serverInfo.WebSocketURL),
		SocketToken:   strings.TrimSpace(serverInfo.WebSocketToken),
		Transport:     "sessiond-websocket",
	}, nil
}

func (a *App) StopWorkspaceTerminalForWindow(ctx context.Context, workspaceID, terminalID string) error {
	windowName := a.resolveCallerWindowName(ctx, "")
	return a.stopWorkspaceTerminalWithOwner(workspaceID, terminalID, windowName)
}
