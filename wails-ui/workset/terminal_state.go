package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/strantalis/workset/pkg/worksetapi"
)

func (a *App) ensureIdleWatcher(session *terminalSession) {
	session.mu.Lock()
	if session.client != nil {
		session.mu.Unlock()
		return
	}
	if session.idleTimer != nil {
		session.mu.Unlock()
		return
	}
	idleTimeout := a.terminalIdleTimeout()
	if idleTimeout <= 0 {
		session.mu.Unlock()
		return
	}
	session.idleTimeout = idleTimeout
	session.lastActivity = time.Now()
	session.idleTimer = time.AfterFunc(session.idleTimeout, func() {
		a.handleIdleTimeout(session)
	})
	session.mu.Unlock()
}

func (a *App) terminalIdleTimeout() time.Duration {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	cfg, _, err := a.service.GetConfig(ctx)
	if err != nil {
		return 30 * time.Minute
	}
	raw := strings.TrimSpace(cfg.Defaults.TerminalIdleTimeout)
	if raw == "" {
		return 30 * time.Minute
	}
	switch strings.ToLower(raw) {
	case "0", "off", "disabled", "false":
		return 0
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout < 0 {
		return 30 * time.Minute
	}
	return timeout
}

func (a *App) handleIdleTimeout(session *terminalSession) {
	a.terminalMu.Lock()
	current := a.terminals[session.id]
	if current != session {
		a.terminalMu.Unlock()
		return
	}
	delete(a.terminals, session.id)
	a.terminalMu.Unlock()

	_ = session.CloseWithReason("idle")
	a.emitTerminalLifecycle("idle", session, "")
	_ = a.persistTerminalState()
}

func (a *App) persistTerminalState() error {
	statePath, err := a.terminalStatePath()
	if err != nil {
		return err
	}
	state := a.snapshotTerminalState()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0o644)
}

func (a *App) snapshotTerminalState() terminalState {
	a.terminalMu.Lock()
	defer a.terminalMu.Unlock()
	entries := make([]terminalStateEntry, 0, len(a.terminals))
	for _, session := range a.terminals {
		if lastActive, ok := session.snapshot(); ok {
			session.mu.Lock()
			modes := terminalModeState{
				AltScreen:  session.altScreen,
				MouseMask:  session.mouseMask,
				MouseSGR:   session.mouseSGR,
				MouseUTF8:  session.mouseUTF8,
				MouseURXVT: session.mouseURXVT,
			}
			session.mu.Unlock()
			entries = append(entries, terminalStateEntry{
				WorkspaceID: session.workspaceID,
				TerminalID:  session.terminalID,
				LastActive:  lastActive,
				Modes:       &modes,
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastActive.After(entries[j].LastActive)
	})
	return terminalState{Sessions: entries}
}

func (a *App) terminalStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "ui_sessions.json"), nil
}

func (a *App) restoreTerminalSessions(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	statePath, err := a.terminalStatePath()
	var state terminalState
	if err == nil {
		if data, readErr := os.ReadFile(statePath); readErr == nil {
			if jsonErr := json.Unmarshal(data, &state); jsonErr == nil {
				a.terminalMu.Lock()
				if a.restoredModes == nil {
					a.restoredModes = map[string]terminalModeState{}
				}
				for _, entry := range state.Sessions {
					if entry.Modes != nil {
						sessionID := terminalSessionID(entry.WorkspaceID, entry.TerminalID)
						a.restoredModes[sessionID] = *entry.Modes
					}
				}
				a.terminalMu.Unlock()
			}
		}
	}
	if client, err := a.getSessiondClient(); err == nil {
		listCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		list, err := client.List(listCtx)
		cancel()
		if err == nil && len(list.Sessions) > 0 {
			for _, info := range list.Sessions {
				if info.Running {
					workspaceID, terminalID, _ := parseTerminalSessionID(info.SessionID)
					_ = a.StartWorkspaceTerminal(workspaceID, terminalID)
				}
			}
			return
		}
	}
	for _, entry := range state.Sessions {
		_ = a.StartWorkspaceTerminal(entry.WorkspaceID, entry.TerminalID)
	}
}
