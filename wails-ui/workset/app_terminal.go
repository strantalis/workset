package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/strantalis/workset/pkg/worksetapi"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type TerminalPayload struct {
	WorkspaceID string `json:"workspaceId"`
	Data        string `json:"data"`
}

type TerminalLifecyclePayload struct {
	WorkspaceID string `json:"workspaceId"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
}

type terminalState struct {
	Sessions []terminalStateEntry `json:"sessions"`
}

type terminalStateEntry struct {
	WorkspaceID string    `json:"workspaceId"`
	LastActive  time.Time `json:"lastActive"`
}

type terminalSession struct {
	id   string
	path string
	cmd  *exec.Cmd
	pty  *os.File
	mu   sync.Mutex

	starting bool
	startErr error
	ready    chan struct{}

	lastActivity time.Time
	idleTimeout  time.Duration
	idleTimer    *time.Timer
	closed       bool
	closeReason  string
}

func newTerminalSession(id, path string) *terminalSession {
	return &terminalSession{
		id:       id,
		path:     path,
		starting: true,
		ready:    make(chan struct{}),
	}
}

func (s *terminalSession) markReady(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startErr = err
	s.starting = false
	if s.ready != nil {
		close(s.ready)
		s.ready = nil
	}
}

func (s *terminalSession) waitReady(ctx context.Context) error {
	s.mu.Lock()
	if !s.starting {
		err := s.startErr
		s.mu.Unlock()
		return err
	}
	ch := s.ready
	s.mu.Unlock()
	select {
	case <-ch:
		s.mu.Lock()
		err := s.startErr
		s.mu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *terminalSession) snapshot() (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.starting || s.closed || s.pty == nil {
		return time.Time{}, false
	}
	return s.lastActivity, true
}

func (s *terminalSession) bumpActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.lastActivity = time.Now()
	if s.idleTimer != nil {
		_ = s.idleTimer.Stop()
		s.idleTimer.Reset(s.idleTimeout)
	}
}

func (s *terminalSession) Write(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pty == nil {
		return fmt.Errorf("terminal not started")
	}
	_, err := s.pty.Write([]byte(data))
	s.lastActivity = time.Now()
	if s.idleTimer != nil {
		_ = s.idleTimer.Stop()
		s.idleTimer.Reset(s.idleTimeout)
	}
	return err
}

func (s *terminalSession) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pty == nil {
		return fmt.Errorf("terminal not started")
	}
	return resizePTY(s.pty, cols, rows)
}

func (s *terminalSession) Close() error {
	return s.CloseWithReason("closed")
}

func (s *terminalSession) CloseWithReason(reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if reason != "" {
		s.closeReason = reason
	}
	if s.idleTimer != nil {
		_ = s.idleTimer.Stop()
		s.idleTimer = nil
	}
	if s.pty != nil {
		_ = s.pty.Close()
		s.pty = nil
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return nil
}

func (a *App) StartWorkspaceTerminal(workspaceID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace id required")
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	root, err := a.resolveWorkspaceRoot(ctx, workspaceID)
	if err != nil {
		return err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}

	for {
		a.terminalMu.Lock()
		existing := a.terminals[workspaceID]
		if existing != nil {
			a.terminalMu.Unlock()
			waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := existing.waitReady(waitCtx)
			cancel()
			if err != nil {
				return err
			}
			existing.mu.Lock()
			hasPTY := existing.pty != nil
			existing.mu.Unlock()
			if hasPTY {
				return nil
			}
			a.terminalMu.Lock()
			if current := a.terminals[workspaceID]; current == existing {
				delete(a.terminals, workspaceID)
			}
			a.terminalMu.Unlock()
			continue
		}

		session := newTerminalSession(workspaceID, root)
		a.terminals[workspaceID] = session
		a.terminalMu.Unlock()

		err := a.startTerminalSession(ctx, session, root, workspaceID)
		session.markReady(err)
		if err != nil {
			a.terminalMu.Lock()
			if current := a.terminals[workspaceID]; current == session {
				delete(a.terminals, workspaceID)
			}
			a.terminalMu.Unlock()
			a.emitTerminalLifecycle("error", workspaceID, err.Error())
			return err
		}
		a.ensureIdleWatcher(session)
		a.emitTerminalLifecycle("started", workspaceID, "")
		_ = a.persistTerminalState()
		go a.streamTerminal(session)
		return nil
	}
}

func (a *App) WriteWorkspaceTerminal(workspaceID, data string) error {
	session, err := a.getTerminal(workspaceID)
	if err != nil {
		return err
	}
	return session.Write(data)
}

func (a *App) ResizeWorkspaceTerminal(workspaceID string, cols, rows int) error {
	session, err := a.getTerminal(workspaceID)
	if err != nil {
		return err
	}
	if cols < 2 {
		cols = 2
	}
	if rows < 1 {
		rows = 1
	}
	return session.Resize(cols, rows)
}

func (a *App) StopWorkspaceTerminal(workspaceID string) error {
	a.terminalMu.Lock()
	session, ok := a.terminals[workspaceID]
	if ok {
		delete(a.terminals, workspaceID)
	}
	a.terminalMu.Unlock()
	if !ok {
		return nil
	}
	err := session.CloseWithReason("closed")
	a.emitTerminalLifecycle("closed", workspaceID, "")
	_ = a.persistTerminalState()
	return err
}

func (a *App) getTerminal(workspaceID string) (*terminalSession, error) {
	a.terminalMu.Lock()
	session := a.terminals[workspaceID]
	a.terminalMu.Unlock()
	if session == nil {
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

func (a *App) streamTerminal(session *terminalSession) {
	buf := make([]byte, 4096)
	const flushThreshold = 8 * 1024
	const maxPending = 256 * 1024
	const maxChunk = 64 * 1024
	flushInterval := 25 * time.Millisecond
	pending := make([]byte, 0, flushThreshold)
	var pendingMu sync.Mutex
	flushTimer := time.NewTimer(flushInterval)
	if !flushTimer.Stop() {
		select {
		case <-flushTimer.C:
		default:
		}
	}
	done := make(chan struct{})

	flushPending := func() {
		pendingMu.Lock()
		if len(pending) == 0 {
			pendingMu.Unlock()
			return
		}
		data := pending
		pending = make([]byte, 0, flushThreshold)
		pendingMu.Unlock()
		for len(data) > 0 {
			chunk := data
			if len(chunk) > maxChunk {
				chunk = data[:maxChunk]
				data = data[maxChunk:]
			} else {
				data = nil
			}
			wruntime.EventsEmit(a.ctx, "terminal:data", TerminalPayload{
				WorkspaceID: session.id,
				Data:        string(chunk),
			})
		}
	}

	go func() {
		for {
			select {
			case <-flushTimer.C:
				flushPending()
			case <-done:
				return
			}
		}
	}()

	defer func() {
		close(done)
		if !flushTimer.Stop() {
			select {
			case <-flushTimer.C:
			default:
			}
		}
		flushPending()
	}()
	for {
		n, err := session.pty.Read(buf)
		if n > 0 {
			session.bumpActivity()
			pendingMu.Lock()
			pending = append(pending, buf[:n]...)
			pendingLen := len(pending)
			pendingMu.Unlock()
			shouldFlush := pendingLen >= flushThreshold
			forcedFlush := pendingLen >= maxPending
			if shouldFlush {
				if !flushTimer.Stop() {
					select {
					case <-flushTimer.C:
					default:
					}
				}
				flushPending()
				if forcedFlush {
					continue
				}
			} else {
				if !flushTimer.Stop() {
					select {
					case <-flushTimer.C:
					default:
					}
				}
				flushTimer.Reset(flushInterval)
			}
		}
		if err != nil {
			a.terminalMu.Lock()
			if current := a.terminals[session.id]; current == session {
				delete(a.terminals, session.id)
			}
			a.terminalMu.Unlock()
			session.mu.Lock()
			reason := session.closeReason
			session.mu.Unlock()
			if reason != "idle" {
				a.emitTerminalLifecycle("closed", session.id, "")
			}
			_ = a.persistTerminalState()
			break
		}
	}
}

func (a *App) startTerminalSession(ctx context.Context, session *terminalSession, root, workspaceID string) error {
	execName, execArgs := resolveShellCommand()
	cmd := exec.CommandContext(ctx, execName, execArgs...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		"TERM=xterm-256color",
		"WORKSET_WORKSPACE="+workspaceID,
		"WORKSET_ROOT="+root,
	)
	cmd.Env = setEnv(cmd.Env, "SHELL", execName)

	ptmx, err := startPTY(cmd)
	if err != nil {
		return err
	}

	session.mu.Lock()
	session.cmd = cmd
	session.pty = ptmx
	session.path = root
	session.lastActivity = time.Now()
	session.mu.Unlock()
	return nil
}

func (a *App) resolveWorkspaceRoot(ctx context.Context, workspaceID string) (string, error) {
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
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

func (a *App) emitTerminalLifecycle(status, workspaceID, message string) {
	wruntime.EventsEmit(a.ctx, "terminal:lifecycle", TerminalLifecyclePayload{
		WorkspaceID: workspaceID,
		Status:      status,
		Message:     message,
	})
}

func (a *App) ensureIdleWatcher(session *terminalSession) {
	session.mu.Lock()
	if session.idleTimer != nil {
		session.mu.Unlock()
		return
	}
	session.idleTimeout = 30 * time.Minute
	session.lastActivity = time.Now()
	session.idleTimer = time.AfterFunc(session.idleTimeout, func() {
		a.handleIdleTimeout(session)
	})
	session.mu.Unlock()
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
	a.emitTerminalLifecycle("idle", session.id, "")
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
			entries = append(entries, terminalStateEntry{
				WorkspaceID: session.id,
				LastActive:  lastActive,
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
	statePath, err := a.terminalStatePath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		return
	}
	var state terminalState
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}
	for _, entry := range state.Sessions {
		_ = a.StartWorkspaceTerminal(entry.WorkspaceID)
	}
}

func resolveShellCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		if shell := os.Getenv("COMSPEC"); shell != "" {
			return shell, nil
		}
		return "cmd.exe", nil
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = lookupUserShell()
	}
	if shell == "" {
		shell = "/bin/sh"
	}

	switch strings.ToLower(filepath.Base(shell)) {
	case "zsh", "bash":
		return shell, []string{"-l", "-i"}
	case "fish":
		return shell, []string{"-l"}
	default:
		return shell, nil
	}
}

func lookupUserShell() string {
	current, err := user.Current()
	if err != nil || current.Username == "" {
		return ""
	}
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		if parts[0] == current.Username {
			return strings.TrimSpace(parts[6])
		}
	}
	return ""
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
