package main

import (
	"context"
	"strings"
	"time"
)

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
}
