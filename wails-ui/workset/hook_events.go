package main

import (
	"strings"

	"github.com/strantalis/workset/pkg/worksetapi"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var hookEventsEmit = wruntime.EventsEmit

type HookProgressPayload struct {
	Operation string `json:"operation,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Repo      string `json:"repo"`
	Event     string `json:"event"`
	HookID    string `json:"hookId"`
	Phase     string `json:"phase"`
	Status    string `json:"status,omitempty"`
	LogPath   string `json:"logPath,omitempty"`
	Error     string `json:"error,omitempty"`
}

type appHookObserver struct {
	app *App
}

func (o appHookObserver) OnHookProgress(progress worksetapi.HookProgress) {
	if o.app == nil || o.app.ctx == nil {
		return
	}
	payload := HookProgressPayload{
		Operation: hookOperation(progress.Reason),
		Reason:    progress.Reason,
		Workspace: progress.Workspace,
		Repo:      progress.Repo,
		Event:     progress.Event,
		HookID:    progress.HookID,
		Phase:     progress.Phase,
		Status:    string(progress.Status),
		LogPath:   progress.LogPath,
	}
	if progress.Error != "" {
		payload.Error = progress.Error
	}
	hookEventsEmit(o.app.ctx, "hooks:progress", payload)
}

func hookOperation(reason string) string {
	reason = strings.TrimSpace(reason)
	switch {
	case strings.HasPrefix(reason, "workspace.create"):
		return "workspace.create"
	case strings.HasPrefix(reason, "repo.add"):
		return "repo.add"
	case reason == "":
		return "hooks.run"
	default:
		return reason
	}
}
