package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type WorkspacePopoutPayload struct {
	WorkspaceID string `json:"workspaceId"`
	WindowName  string `json:"windowName"`
	Open        bool   `json:"open"`
}

func sanitizeWindowToken(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "workspace"
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

func (a *App) popoutWindowName(workspaceID string) string {
	return fmt.Sprintf("workspace-%s-popout", sanitizeWindowToken(workspaceID))
}

func (a *App) OpenWorkspacePopout(workspaceID string) (WorkspacePopoutPayload, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return WorkspacePopoutPayload{}, fmt.Errorf("workspace id required")
	}
	if a.runtimeApp == nil {
		return WorkspacePopoutPayload{}, fmt.Errorf("runtime app unavailable")
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := a.resolveWorkspaceRoot(ctx, workspaceID); err != nil {
		return WorkspacePopoutPayload{}, err
	}

	a.popoutMu.Lock()
	existing := strings.TrimSpace(a.popouts[workspaceID])
	a.popoutMu.Unlock()
	if existing != "" {
		if win, ok := a.runtimeApp.Window.Get(existing); ok && win != nil {
			win.Show()
			win.Focus()
			a.claimWorkspaceTerminalOwner(workspaceID, existing)
			return WorkspacePopoutPayload{WorkspaceID: workspaceID, WindowName: existing, Open: true}, nil
		}
	}

	windowName := a.popoutWindowName(workspaceID)
	values := url.Values{}
	values.Set("popout", "1")
	values.Set("workspace", workspaceID)
	values.Set("view", "command-center")
	values.Set("window", windowName)

	window := a.runtimeApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             windowName,
		Title:            fmt.Sprintf("workset - %s", workspaceID),
		Width:            defaultWindowWidth,
		Height:           defaultWindowHeight,
		BackgroundColour: application.NewRGB(8, 16, 24),
		URL:              "/?" + values.Encode(),
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
		},
	})
	window.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		a.unregisterWorkspacePopout(workspaceID, windowName)
	})
	window.Show()
	window.Focus()

	a.popoutMu.Lock()
	a.popouts[workspaceID] = windowName
	a.popoutMu.Unlock()
	a.claimWorkspaceTerminalOwner(workspaceID, windowName)
	emitRuntimeEvent(ctx, EventWorkspacePopoutOpened, WorkspacePopoutPayload{
		WorkspaceID: workspaceID,
		WindowName:  windowName,
		Open:        true,
	})

	return WorkspacePopoutPayload{WorkspaceID: workspaceID, WindowName: windowName, Open: true}, nil
}

func (a *App) unregisterWorkspacePopout(workspaceID, windowName string) {
	a.popoutMu.Lock()
	current := strings.TrimSpace(a.popouts[workspaceID])
	if current == windowName {
		delete(a.popouts, workspaceID)
	}
	a.popoutMu.Unlock()
	a.releaseWorkspaceTerminalOwner(workspaceID, windowName)
	emitRuntimeEvent(a.ctx, EventWorkspacePopoutClosed, WorkspacePopoutPayload{
		WorkspaceID: workspaceID,
		WindowName:  windowName,
		Open:        false,
	})
}

func (a *App) CloseWorkspacePopout(workspaceID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace id required")
	}
	if a.runtimeApp == nil {
		return fmt.Errorf("runtime app unavailable")
	}
	a.popoutMu.Lock()
	windowName := strings.TrimSpace(a.popouts[workspaceID])
	a.popoutMu.Unlock()
	if windowName == "" {
		a.releaseWorkspaceTerminalOwner(workspaceID, "")
		return nil
	}
	if win, ok := a.runtimeApp.Window.Get(windowName); ok && win != nil {
		win.Close()
	}
	a.unregisterWorkspacePopout(workspaceID, windowName)
	return nil
}

func (a *App) ListWorkspacePopouts() []WorkspacePopoutPayload {
	a.popoutMu.Lock()
	defer a.popoutMu.Unlock()
	result := make([]WorkspacePopoutPayload, 0, len(a.popouts))
	for workspaceID, windowName := range a.popouts {
		result = append(result, WorkspacePopoutPayload{
			WorkspaceID: workspaceID,
			WindowName:  windowName,
			Open:        true,
		})
	}
	return result
}
