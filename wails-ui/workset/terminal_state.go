package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type terminalRestoreTarget struct {
	workspaceID string
	terminalID  string
}

func (a *App) ensureIdleWatcher(session *terminalSession) {
	session.mu.Lock()
	if session.client == nil {
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
	a.ensureService()
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
	dir, err := worksetAppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ui_sessions.json"), nil
}

func (a *App) restoreTerminalSessions(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	state := a.loadPersistedTerminalState()
	targets := a.collectTerminalRestoreTargets(ctx, state)
	for _, target := range targets {
		_ = a.StartWorkspaceTerminal(target.workspaceID, target.terminalID)
	}
}

func (a *App) loadPersistedTerminalState() terminalState {
	statePath, err := a.terminalStatePath()
	if err != nil {
		return terminalState{}
	}
	data, readErr := os.ReadFile(statePath)
	if readErr != nil {
		return terminalState{}
	}
	var state terminalState
	if jsonErr := json.Unmarshal(data, &state); jsonErr != nil {
		return terminalState{}
	}
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
	return state
}

func (a *App) collectTerminalRestoreTargets(ctx context.Context, state terminalState) []terminalRestoreTarget {
	targets := make([]terminalRestoreTarget, 0)
	seen := make(map[string]struct{})
	add := func(workspaceID, terminalID string) {
		workspaceID = strings.TrimSpace(workspaceID)
		terminalID = strings.TrimSpace(terminalID)
		if workspaceID == "" {
			return
		}
		sessionID := terminalSessionID(workspaceID, terminalID)
		if _, exists := seen[sessionID]; exists {
			return
		}
		seen[sessionID] = struct{}{}
		targets = append(targets, terminalRestoreTarget{
			workspaceID: workspaceID,
			terminalID:  terminalID,
		})
	}

	if store, err := a.loadTerminalLayoutStore(); err == nil {
		for _, target := range a.terminalLayoutRestoreTargets(ctx, store) {
			add(target.workspaceID, target.terminalID)
		}
	}

	if client, err := a.getSessiondClient(); err == nil {
		listCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		list, listErr := client.List(listCtx)
		cancel()
		if listErr == nil {
			for _, info := range list.Sessions {
				if !info.Running {
					continue
				}
				workspaceID, terminalID, _ := parseTerminalSessionID(info.SessionID)
				add(workspaceID, terminalID)
			}
		}
	}

	for _, entry := range state.Sessions {
		add(entry.WorkspaceID, entry.TerminalID)
	}

	return targets
}
