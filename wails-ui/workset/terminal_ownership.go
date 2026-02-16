package main

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var transientTerminalErrorMarkers = []string{
	"session not found",
	"terminal not started",
	"terminal not found",
	"terminal stream not ready",
}

var terminalInputSeq atomic.Uint64

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

func (a *App) workspaceTerminalOwner(workspaceID string) string {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return a.mainWindowName
	}
	a.popoutMu.Lock()
	defer a.popoutMu.Unlock()
	owner := strings.TrimSpace(a.terminalOwners[workspaceID])
	if owner == "" {
		return a.mainWindowName
	}
	return owner
}

func (a *App) claimWorkspaceTerminalOwner(workspaceID, windowName string) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return
	}
	windowName = a.normalizeWindowName(windowName)
	a.popoutMu.Lock()
	previous := strings.TrimSpace(a.terminalOwners[workspaceID])
	if previous == "" {
		previous = a.mainWindowName
	}
	a.terminalOwners[workspaceID] = windowName
	a.popoutMu.Unlock()
	logTerminalDebug(TerminalDebugPayload{
		WorkspaceID: workspaceID,
		TerminalID:  "__owner__",
		Event:       "owner_claim",
		Details:     fmt.Sprintf(`{"from":%q,"to":%q}`, previous, windowName),
	})
	go a.syncWorkspaceTerminalOwnerToSessiond(workspaceID, windowName)
}

func (a *App) releaseWorkspaceTerminalOwner(workspaceID, windowName string) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return
	}
	windowName = a.normalizeWindowName(windowName)
	a.popoutMu.Lock()
	rawCurrent := strings.TrimSpace(a.terminalOwners[workspaceID])
	current := rawCurrent
	if current == "" {
		current = a.mainWindowName
	}
	released := false
	if rawCurrent == "" || rawCurrent == windowName {
		a.terminalOwners[workspaceID] = a.mainWindowName
		released = true
	}
	a.popoutMu.Unlock()
	if released {
		logTerminalDebug(TerminalDebugPayload{
			WorkspaceID: workspaceID,
			TerminalID:  "__owner__",
			Event:       "owner_release",
			Details:     fmt.Sprintf(`{"from":%q,"to":%q,"caller":%q}`, current, a.mainWindowName, windowName),
		})
		go a.syncWorkspaceTerminalOwnerToSessiond(workspaceID, a.mainWindowName)
	}
}

func (a *App) syncWorkspaceTerminalOwnerToSessiond(workspaceID, windowName string) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return
	}
	windowName = a.normalizeWindowName(windowName)
	a.terminalMu.Lock()
	sessions := make([]*terminalSession, 0, len(a.terminals))
	for _, session := range a.terminals {
		if session == nil || session.workspaceID != workspaceID {
			continue
		}
		sessions = append(sessions, session)
	}
	a.terminalMu.Unlock()
	for _, session := range sessions {
		session.mu.Lock()
		client := session.client
		sessionID := session.id
		session.mu.Unlock()
		if client == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := client.SetOwner(ctx, sessionID, windowName)
		cancel()
		if err == nil {
			logTerminalDebug(TerminalDebugPayload{
				WorkspaceID: workspaceID,
				TerminalID:  session.terminalID,
				Event:       "owner_sync_sessiond",
				Details:     fmt.Sprintf(`{"owner":%q,"sessionId":%q,"ok":true}`, windowName, sessionID),
			})
		}
		if err != nil && !isTransientTerminalCallError(err) {
			logTerminalDebug(TerminalDebugPayload{
				WorkspaceID: workspaceID,
				TerminalID:  session.terminalID,
				Event:       "owner_sync_sessiond",
				Details: fmt.Sprintf(
					`{"owner":%q,"sessionId":%q,"ok":false,"transient":false,"error":%q}`,
					windowName,
					sessionID,
					err.Error(),
				),
			})
			continue
		}
		if err != nil {
			logTerminalDebug(TerminalDebugPayload{
				WorkspaceID: workspaceID,
				TerminalID:  session.terminalID,
				Event:       "owner_sync_sessiond",
				Details: fmt.Sprintf(
					`{"owner":%q,"sessionId":%q,"ok":false,"transient":true,"error":%q}`,
					windowName,
					sessionID,
					err.Error(),
				),
			})
		}
	}
}

func (a *App) ensureWorkspaceTerminalOwner(workspaceID, windowName string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace id required")
	}
	return nil
}

func (a *App) allowWorkspaceTerminalStart(workspaceID, windowName string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace id required")
	}
	windowName = a.normalizeWindowName(windowName)
	// Window ownership is advisory at the app layer. The start caller becomes the
	// current owner while session-level ownership converges into sessiond.
	a.claimWorkspaceTerminalOwner(workspaceID, windowName)
	return nil
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
	return a.workspaceTerminalOwner(workspaceID)
}

func (a *App) StartWorkspaceTerminalForWindow(ctx context.Context, workspaceID, terminalID string) error {
	return a.StartWorkspaceTerminalForWindowName(ctx, workspaceID, terminalID, "")
}

func (a *App) StartWorkspaceTerminalForWindowName(ctx context.Context, workspaceID, terminalID, windowName string) error {
	windowName = a.resolveCallerWindowName(ctx, windowName)
	currentOwner := a.workspaceTerminalOwner(workspaceID)
	logTerminalDebug(TerminalDebugPayload{
		WorkspaceID: workspaceID,
		TerminalID:  terminalID,
		Event:       "owner_start_request",
		Details:     fmt.Sprintf(`{"caller":%q,"currentOwner":%q}`, windowName, currentOwner),
	})
	if err := a.allowWorkspaceTerminalStart(workspaceID, windowName); err != nil {
		return err
	}
	if err := a.StartWorkspaceTerminal(workspaceID, terminalID); err != nil {
		return err
	}
	a.restartTerminalStreamForHandoff(workspaceID, terminalID, currentOwner, windowName)
	go a.syncWorkspaceTerminalOwnerToSessiond(workspaceID, windowName)
	return nil
}

func (a *App) restartTerminalStreamForHandoff(workspaceID, terminalID, previousOwner, nextOwner string) {
	session, err := a.getTerminal(workspaceID, terminalID)
	if err != nil {
		logTerminalDebug(TerminalDebugPayload{
			WorkspaceID: workspaceID,
			TerminalID:  terminalID,
			Event:       "owner_stream_replay_skip",
			Details: fmt.Sprintf(
				`{"from":%q,"to":%q,"reason":"lookup_failed","error":%q}`,
				previousOwner,
				nextOwner,
				err.Error(),
			),
		})
		return
	}
	session.mu.Lock()
	currentStream := session.stream
	currentCancel := session.streamCancel
	currentStreamOwner := strings.TrimSpace(session.streamOwner)
	if currentStreamOwner == "" {
		currentStreamOwner = a.mainWindowName
	}
	if currentStream == nil {
		if currentCancel != nil {
			if currentStreamOwner == nextOwner {
				session.mu.Unlock()
				logTerminalDebug(TerminalDebugPayload{
					WorkspaceID: workspaceID,
					TerminalID:  terminalID,
					Event:       "owner_stream_replay_skip",
					Details: fmt.Sprintf(
						`{"from":%q,"to":%q,"reason":"stream_attach_in_flight_owner_match"}`,
						currentStreamOwner,
						nextOwner,
					),
				})
				return
			}
			session.streamCancel = nil
			session.streamOwner = ""
			session.mu.Unlock()
			currentCancel()
			logTerminalDebug(TerminalDebugPayload{
				WorkspaceID: workspaceID,
				TerminalID:  terminalID,
				Event:       "owner_stream_replay_restart",
				Details: fmt.Sprintf(
					`{"from":%q,"to":%q,"reason":"stream_attach_in_flight"}`,
					currentStreamOwner,
					nextOwner,
				),
			})
			go a.streamTerminal(session)
			return
		}
		session.mu.Unlock()
		logTerminalDebug(TerminalDebugPayload{
			WorkspaceID: workspaceID,
			TerminalID:  terminalID,
			Event:       "owner_stream_replay_restart",
			Details: fmt.Sprintf(
				`{"from":%q,"to":%q,"reason":"stream_inactive"}`,
				currentStreamOwner,
				nextOwner,
			),
		})
		go a.streamTerminal(session)
		return
	}
	if currentStreamOwner == nextOwner {
		session.mu.Unlock()
		logTerminalDebug(TerminalDebugPayload{
			WorkspaceID: workspaceID,
			TerminalID:  terminalID,
			Event:       "owner_stream_replay_skip",
			Details: fmt.Sprintf(
				`{"from":%q,"to":%q,"reason":"stream_owner_match"}`,
				currentStreamOwner,
				nextOwner,
			),
		})
		return
	}
	session.stream = nil
	session.streamCancel = nil
	session.streamOwner = ""
	session.mu.Unlock()
	if currentCancel != nil {
		currentCancel()
	}
	_ = currentStream.Close()
	logTerminalDebug(TerminalDebugPayload{
		WorkspaceID: workspaceID,
		TerminalID:  terminalID,
		Event:       "owner_stream_replay_restart",
		Details: fmt.Sprintf(
			`{"from":%q,"to":%q}`,
			currentStreamOwner,
			nextOwner,
		),
	})
	go a.streamTerminal(session)
}

func (a *App) WriteWorkspaceTerminalForWindow(ctx context.Context, workspaceID, terminalID, data string) error {
	return a.WriteWorkspaceTerminalForWindowName(ctx, workspaceID, terminalID, data, "")
}

func (a *App) WriteWorkspaceTerminalForWindowName(ctx context.Context, workspaceID, terminalID, data, windowName string) error {
	windowName = a.resolveCallerWindowName(ctx, windowName)
	currentOwner := a.workspaceTerminalOwner(workspaceID)
	if currentOwner != windowName {
		logTerminalDebug(TerminalDebugPayload{
			WorkspaceID: workspaceID,
			TerminalID:  terminalID,
			Event:       "owner_write_mismatch",
			Details:     fmt.Sprintf(`{"caller":%q,"currentOwner":%q}`, windowName, currentOwner),
		})
	}
	if err := a.ensureWorkspaceTerminalOwner(workspaceID, windowName); err != nil {
		return err
	}
	inputSeq := terminalInputSeq.Add(1)
	logTerminalDebug(TerminalDebugPayload{
		WorkspaceID: workspaceID,
		TerminalID:  terminalID,
		Event:       "app_input_write",
		Details: fmt.Sprintf(
			`{"seq":%d,"owner":%q,"summary":%q}`,
			inputSeq,
			windowName,
			summarizeTerminalBytes([]byte(data), 48),
		),
	})
	session, err := a.getTerminal(workspaceID, terminalID)
	if err != nil {
		return err
	}
	return session.WriteAsOwner(data, windowName)
}

func (a *App) ResizeWorkspaceTerminalForWindow(ctx context.Context, workspaceID, terminalID string, cols, rows int) error {
	return a.ResizeWorkspaceTerminalForWindowName(ctx, workspaceID, terminalID, cols, rows, "")
}

func (a *App) ResizeWorkspaceTerminalForWindowName(ctx context.Context, workspaceID, terminalID string, cols, rows int, windowName string) error {
	windowName = a.resolveCallerWindowName(ctx, windowName)
	currentOwner := a.workspaceTerminalOwner(workspaceID)
	if currentOwner != windowName {
		logTerminalDebug(TerminalDebugPayload{
			WorkspaceID: workspaceID,
			TerminalID:  terminalID,
			Event:       "owner_resize_mismatch",
			Details: fmt.Sprintf(
				`{"caller":%q,"currentOwner":%q,"cols":%d,"rows":%d}`,
				windowName,
				currentOwner,
				cols,
				rows,
			),
		})
	}
	if err := a.ensureWorkspaceTerminalOwner(workspaceID, windowName); err != nil {
		return err
	}
	if err := a.ResizeWorkspaceTerminal(workspaceID, terminalID, cols, rows); err != nil {
		// Resize calls race attach/start during window handoff. Treat "terminal not started"
		// as transient so frontend fit retries can recover silently.
		if strings.Contains(strings.ToLower(err.Error()), "terminal not started") {
			return nil
		}
		return err
	}
	return nil
}

func (a *App) StopWorkspaceTerminalForWindow(ctx context.Context, workspaceID, terminalID string) error {
	return a.StopWorkspaceTerminalForWindowName(ctx, workspaceID, terminalID, "")
}

func (a *App) StopWorkspaceTerminalForWindowName(ctx context.Context, workspaceID, terminalID, windowName string) error {
	windowName = a.resolveCallerWindowName(ctx, windowName)
	if err := a.ensureWorkspaceTerminalOwner(workspaceID, windowName); err != nil {
		return err
	}
	return a.StopWorkspaceTerminal(workspaceID, terminalID)
}
