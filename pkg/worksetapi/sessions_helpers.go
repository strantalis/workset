package worksetapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/strantalis/workset/internal/session"
	"github.com/strantalis/workset/internal/workspace"
)

func resolveSessionName(explicit, format, workspaceName string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return formatSessionName(format, workspaceName)
}

func formatSessionName(format, workspaceName string) string {
	if strings.TrimSpace(format) == "" {
		format = "workset-{workspace}"
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

func detachHint(backend session.Backend) string {
	switch backend {
	case session.BackendTmux:
		return "Ctrl-b d"
	case session.BackendScreen:
		return "Ctrl-a d"
	default:
		return ""
	}
}

func ensureSessionNameAvailable(ctx context.Context, runner session.Runner, state workspace.State, name string, backend session.Backend) error {
	workspace.EnsureSessionState(&state)
	if existing, ok := state.Sessions[name]; ok {
		existingBackend := strings.TrimSpace(existing.Backend)
		if existingBackend != "" {
			if parsed, err := session.ParseBackend(existingBackend); err == nil {
				existingBackend = string(parsed)
			}
		}
		if existingBackend != "" && backend != "" && existingBackend != string(backend) {
			return fmt.Errorf("session name %s already recorded for backend %s; use --name to avoid collisions", name, existingBackend)
		}
		if backend != "" && backend != session.BackendExec {
			running, err := session.Running(ctx, runner, backend, name)
			if err == nil && running {
				return fmt.Errorf("session %s already running; use attach or pass --name to start another", name)
			}
		}
	}
	return nil
}

func allowAttachAfterStart(backend session.Backend, attach bool) (bool, string) {
	if !attach {
		return false, ""
	}
	if backend == session.BackendExec {
		return false, "note: --attach ignored for exec backend"
	}
	return true, ""
}

func markSessionAttached(state *workspace.State, name string, when time.Time) bool {
	if state == nil {
		return false
	}
	workspace.EnsureSessionState(state)
	entry, ok := state.Sessions[name]
	if !ok {
		return false
	}
	entry.LastAttached = when.Format(time.RFC3339)
	state.Sessions[name] = entry
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
