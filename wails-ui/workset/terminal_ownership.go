package main

import (
	"context"
	"fmt"
	"strings"

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
	defer a.popoutMu.Unlock()
	a.terminalOwners[workspaceID] = windowName
}

func (a *App) releaseWorkspaceTerminalOwner(workspaceID, windowName string) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return
	}
	windowName = a.normalizeWindowName(windowName)
	a.popoutMu.Lock()
	defer a.popoutMu.Unlock()
	if current := strings.TrimSpace(a.terminalOwners[workspaceID]); current == "" || current == windowName {
		a.terminalOwners[workspaceID] = a.mainWindowName
	}
}

func (a *App) ensureWorkspaceTerminalOwner(workspaceID, windowName string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace id required")
	}
	windowName = a.normalizeWindowName(windowName)
	owner := a.workspaceTerminalOwner(workspaceID)
	if owner == windowName {
		return nil
	}
	return fmt.Errorf("workspace terminal is owned by window %q (caller %q)", owner, windowName)
}

func (a *App) allowWorkspaceTerminalStart(workspaceID, windowName string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace id required")
	}
	windowName = a.normalizeWindowName(windowName)
	owner := a.workspaceTerminalOwner(workspaceID)
	if owner != windowName && owner != a.mainWindowName {
		return fmt.Errorf("workspace terminal is owned by window %q (caller %q)", owner, windowName)
	}
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
	if err := a.allowWorkspaceTerminalStart(workspaceID, windowName); err != nil {
		return err
	}
	return a.StartWorkspaceTerminal(workspaceID, terminalID)
}

func (a *App) WriteWorkspaceTerminalForWindow(ctx context.Context, workspaceID, terminalID, data string) error {
	return a.WriteWorkspaceTerminalForWindowName(ctx, workspaceID, terminalID, data, "")
}

func (a *App) WriteWorkspaceTerminalForWindowName(ctx context.Context, workspaceID, terminalID, data, windowName string) error {
	windowName = a.resolveCallerWindowName(ctx, windowName)
	if err := a.ensureWorkspaceTerminalOwner(workspaceID, windowName); err != nil {
		return err
	}
	if err := a.WriteWorkspaceTerminal(workspaceID, terminalID, data); err != nil {
		if !isTransientTerminalCallError(err) {
			return err
		}
		// Best-effort recovery for transient session races (HMR, popout handoff).
		if startErr := a.StartWorkspaceTerminal(workspaceID, terminalID); startErr != nil {
			if !isTransientTerminalCallError(startErr) {
				return startErr
			}
			return nil
		}
		if retryErr := a.WriteWorkspaceTerminal(workspaceID, terminalID, data); retryErr != nil && !isTransientTerminalCallError(retryErr) {
			return retryErr
		}
	}
	return nil
}

func (a *App) AckWorkspaceTerminalForWindow(ctx context.Context, workspaceID, terminalID string, bytes int) error {
	return a.AckWorkspaceTerminalForWindowName(ctx, workspaceID, terminalID, bytes, "")
}

func (a *App) AckWorkspaceTerminalForWindowName(ctx context.Context, workspaceID, terminalID string, bytes int, windowName string) error {
	windowName = a.resolveCallerWindowName(ctx, windowName)
	if err := a.ensureWorkspaceTerminalOwner(workspaceID, windowName); err != nil {
		return err
	}
	if err := a.AckWorkspaceTerminal(workspaceID, terminalID, bytes); err != nil && !isTransientTerminalCallError(err) {
		return err
	}
	return nil
}

func (a *App) ResizeWorkspaceTerminalForWindow(ctx context.Context, workspaceID, terminalID string, cols, rows int) error {
	return a.ResizeWorkspaceTerminalForWindowName(ctx, workspaceID, terminalID, cols, rows, "")
}

func (a *App) ResizeWorkspaceTerminalForWindowName(ctx context.Context, workspaceID, terminalID string, cols, rows int, windowName string) error {
	windowName = a.resolveCallerWindowName(ctx, windowName)
	if err := a.ensureWorkspaceTerminalOwner(workspaceID, windowName); err != nil {
		return err
	}
	if err := a.ResizeWorkspaceTerminal(workspaceID, terminalID, cols, rows); err != nil && !isTransientTerminalCallError(err) {
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
