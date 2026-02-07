package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/strantalis/workset/pkg/sessiond"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) getSessiondClient() (*sessiond.Client, error) {
	return a.getSessiondClientInternal(true)
}

func (a *App) getSessiondClientInternal(waitForRestart bool) (*sessiond.Client, error) {
	if waitForRestart && a.sessiondRestart != nil {
		logRestartf("client_wait_for_restart")
		a.sessiondRestart.wait()
	}
	if a.sessiondStart != nil {
		waitCh, leader := a.sessiondStart.begin()
		if !leader {
			logRestartf("client_wait_for_start")
			<-waitCh
		} else {
			logRestartf("client_start_begin")
			defer a.sessiondStart.end()
		}
	}
	a.sessiondMu.Lock()
	client := a.sessiondClient
	a.sessiondMu.Unlock()
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		err := client.Ping(ctx)
		cancel()
		if err == nil {
			logRestartf("client_ready")
			return client, nil
		}
		logRestartf("client_ping_failed err=%v", err)
		a.clearSessiondClient()
	}
	logRestartf("client_ensure_running")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	startOpts := sessiond.StartOptions{}
	svc := a.ensureService()
	if cfg, _, cfgErr := svc.GetConfig(ctx); cfgErr == nil {
		if envTruthy(cfg.Defaults.TerminalProtocolLog) {
			startOpts.ProtocolLogEnabled = true
		}
	}
	client, err := sessiond.EnsureRunningWithOptions(ctx, startOpts)
	if err != nil {
		if strings.Contains(err.Error(), "sessiond did not start") {
			logRestartf("client_ensure_retry err=%v", err)
			ctxRetry, cancelRetry := context.WithTimeout(context.Background(), 8*time.Second)
			client, err = sessiond.EnsureRunningWithOptions(ctxRetry, startOpts)
			cancelRetry()
		}
		if err != nil {
			logRestartf("client_ensure_failed err=%v", err)
			return nil, err
		}
	}
	a.sessiondMu.Lock()
	a.sessiondClient = client
	a.sessiondMu.Unlock()
	logRestartf("client_ready_after_start")
	return client, nil
}

func (a *App) clearSessiondClient() {
	a.sessiondMu.Lock()
	a.sessiondClient = nil
	a.sessiondMu.Unlock()
}

type SessiondStatus struct {
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
	Warning   string `json:"warning,omitempty"`
}

type sessiondStartState struct {
	mu       sync.Mutex
	starting bool
	waitCh   chan struct{}
}

func (s *sessiondStartState) begin() (chan struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.starting {
		return s.waitCh, false
	}
	s.starting = true
	s.waitCh = make(chan struct{})
	return s.waitCh, true
}

func (s *sessiondStartState) end() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.starting {
		return
	}
	close(s.waitCh)
	s.starting = false
	s.waitCh = nil
}

func (s *sessiondStartState) wait() {
	s.mu.Lock()
	ch := s.waitCh
	s.mu.Unlock()
	if ch != nil {
		<-ch
	}
}

type sessiondRestartState struct {
	mu         sync.Mutex
	restarting bool
	waitCh     chan struct{}
}

func (s *sessiondRestartState) begin() (chan struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.restarting {
		return s.waitCh, false
	}
	s.restarting = true
	s.waitCh = make(chan struct{})
	return s.waitCh, true
}

func (s *sessiondRestartState) end() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.restarting {
		return
	}
	close(s.waitCh)
	s.restarting = false
	s.waitCh = nil
}

func (s *sessiondRestartState) wait() {
	s.mu.Lock()
	ch := s.waitCh
	s.mu.Unlock()
	if ch != nil {
		<-ch
	}
}

var restartLogMu sync.Mutex

func logRestartf(format string, args ...any) {
	dir, err := worksetAppDir()
	if err != nil {
		return
	}
	logPath := filepath.Join(dir, "sessiond_restart.log")
	restartLogMu.Lock()
	defer restartLogMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() {
		_ = file.Close()
	}()
	_, _ = fmt.Fprintf(
		file,
		"%s "+format+"\n",
		append([]any{time.Now().Format(time.RFC3339Nano)}, args...)...,
	)
}

func (a *App) GetSessiondStatus() SessiondStatus {
	client, err := a.getSessiondClientInternal(false)
	if err != nil {
		return SessiondStatus{Available: false, Error: err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		return SessiondStatus{Available: false, Error: err.Error()}
	}
	return SessiondStatus{Available: true}
}

func (a *App) RestartSessiond() SessiondStatus {
	return a.restartSessiond("manual")
}

func (a *App) RestartSessiondWithReason(reason string) SessiondStatus {
	return a.restartSessiond(reason)
}

func (a *App) restartSessiond(reason string) SessiondStatus {
	defer func() {
		if r := recover(); r != nil {
			logRestartf("restart_panic value=%v", r)
		}
	}()
	if a.sessiondRestart != nil {
		waitCh, leader := a.sessiondRestart.begin()
		if !leader {
			logRestartf("restart_wait_existing")
			<-waitCh
			return SessiondStatus{Available: true}
		}
		logRestartf("restart_begin")
		defer a.sessiondRestart.end()
	}

	client, err := a.getSessiondClientInternal(false)
	if err == nil && client != nil {
		pid := os.Getpid()
		exe, exeErr := os.Executable()
		if exeErr != nil {
			exe = "unknown"
		}
		logRestartf("restart_shutdown_send source=app reason=%s pid=%d exe=%s", reason, pid, exe)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		shutdownErr := client.ShutdownWithReason(ctx, "app", reason)
		cancel()
		logRestartf("restart_shutdown_done err=%v", shutdownErr)
		a.clearSessiondClient()
	}
	logRestartf("restart_socket_path_begin")
	forceSocketRemove := false
	if socketPath, pathErr := sessiond.DefaultSocketPath(); pathErr == nil {
		logRestartf("restart_wait socket=%s", socketPath)
		ctxWait, cancelWait := context.WithTimeout(context.Background(), 5*time.Second)
		waitErr := sessiond.WaitForShutdown(ctxWait, socketPath)
		cancelWait()
		if waitErr != nil {
			logRestartf("restart_wait_failed err=%v", waitErr)
			logRestartf("restart_force_socket_remove socket=%s", socketPath)
			forceSocketRemove = true
		} else {
			logRestartf("restart_wait_done")
		}
		_ = os.Remove(socketPath)
	} else {
		logRestartf("restart_socket_path_failed err=%v", pathErr)
	}
	logRestartf("restart_invalidate_sessions")
	a.invalidateTerminalSessions("Session daemon restarted.")
	client, err = a.getSessiondClientInternal(false)
	if err != nil {
		logRestartf("restart_client_failed err=%v", err)
		status := SessiondStatus{Available: false, Error: err.Error()}
		if forceSocketRemove {
			status.Warning = "Session daemon did not shut down cleanly; socket was force removed."
		}
		return status
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		logRestartf("restart_ping_failed err=%v", err)
		status := SessiondStatus{Available: false, Error: err.Error()}
		if forceSocketRemove {
			status.Warning = "Session daemon did not shut down cleanly; socket was force removed."
		}
		return status
	}
	if a.ctx != nil {
		status := SessiondStatus{Available: true}
		if forceSocketRemove {
			status.Warning = "Session daemon did not shut down cleanly; socket was force removed."
		}
		wruntime.EventsEmit(a.ctx, EventSessiondRestarted, status)
	}
	logRestartf("restart_done")
	status := SessiondStatus{Available: true}
	if forceSocketRemove {
		status.Warning = "Session daemon did not shut down cleanly; socket was force removed."
	}
	return status
}

var sessiondOnce sync.Once

func ensureSessiondStarted(a *App) {
	sessiondOnce.Do(func() {
		go func() {
			_, _ = a.getSessiondClient()
		}()
	})
}

var sessiondUpgradeOnce sync.Once

func ensureSessiondUpToDate(a *App) {
	sessiondUpgradeOnce.Do(func() {
		go a.checkSessiondUpgrade()
	})
}

func (a *App) checkSessiondUpgrade() {
	expectedPath, err := sessiond.FindSessiondBinary()
	if err != nil {
		logRestartf("upgrade_expected_binary_missing err=%v", err)
		return
	}
	expectedHash, err := sessiond.BinaryHash(expectedPath)
	if err != nil {
		logRestartf("upgrade_expected_hash_failed path=%s err=%v", expectedPath, err)
		return
	}

	client, err := a.getSessiondClientInternal(false)
	if err != nil {
		logRestartf("upgrade_client_failed err=%v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	info, err := client.Info(ctx)
	cancel()
	if err != nil {
		if strings.Contains(err.Error(), "unknown method") {
			logRestartf("upgrade_info_unsupported err=%v", err)
			status := a.restartSessiond("upgrade")
			if !status.Available || status.Error != "" {
				logRestartf("upgrade_restart_failed err=%s warning=%s", status.Error, status.Warning)
			}
		} else {
			logRestartf("upgrade_info_failed err=%v", err)
		}
		return
	}
	if info.BinaryHash == "" {
		logRestartf("upgrade_info_empty expected=%s", expectedHash)
		return
	}
	if info.BinaryHash != expectedHash {
		logRestartf("upgrade_mismatch expected=%s got=%s exe=%s", expectedHash, info.BinaryHash, info.Executable)
		status := a.restartSessiond("upgrade")
		if !status.Available || status.Error != "" {
			logRestartf("upgrade_restart_failed err=%s warning=%s", status.Error, status.Warning)
		}
		return
	}
	logRestartf("upgrade_match hash=%s", expectedHash)
}
