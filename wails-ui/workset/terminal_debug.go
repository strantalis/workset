package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	terminalDebugOnce    sync.Once
	terminalDebugEnabled bool
	terminalDebugLog     *os.File
	terminalDebugMu      sync.Mutex
)

func terminalDebugLogPath() (string, error) {
	dir, err := worksetAppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "terminal_debug.log"), nil
}

func ensureConfiguredTerminalDebugLogging(a *App) {
	if strings.TrimSpace(os.Getenv("WORKSET_TERMINAL_DEBUG_LOG")) == "" {
		ctx, svc := a.serviceContext()
		cfg, _, err := svc.GetConfig(ctx)
		if err == nil && envTruthy(cfg.Defaults.TerminalDebugLog) {
			_ = os.Setenv("WORKSET_TERMINAL_DEBUG_LOG", "1")
		}
	}

	if !envTruthy(os.Getenv("WORKSET_TERMINAL_DEBUG_LOG")) {
		return
	}
	if strings.TrimSpace(os.Getenv("WORKSET_TERMINAL_DEBUG_LOG_PATH")) != "" {
		return
	}
	logPath, err := terminalDebugLogPath()
	if err != nil {
		return
	}
	_ = os.Setenv("WORKSET_TERMINAL_DEBUG_LOG_PATH", logPath)
}

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func terminalDebugConfig() bool {
	terminalDebugOnce.Do(func() {
		terminalDebugEnabled = envTruthy(os.Getenv("WORKSET_TERMINAL_DEBUG_LOG"))
		if !terminalDebugEnabled {
			return
		}
		logPath := strings.TrimSpace(os.Getenv("WORKSET_TERMINAL_DEBUG_LOG_PATH"))
		if logPath == "" {
			path, err := terminalDebugLogPath()
			if err != nil {
				terminalDebugEnabled = false
				return
			}
			logPath = path
		}
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			terminalDebugEnabled = false
			return
		}
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			terminalDebugEnabled = false
			return
		}
		terminalDebugLog = file
	})
	return terminalDebugEnabled && terminalDebugLog != nil
}

func logTerminalDebug(payload TerminalDebugPayload) {
	if !terminalDebugConfig() {
		return
	}
	if payload.Event == "" {
		payload.Event = "event"
	}
	details := strings.ReplaceAll(payload.Details, "\n", "\\n")
	terminalDebugMu.Lock()
	defer terminalDebugMu.Unlock()
	_, _ = fmt.Fprintf(
		terminalDebugLog,
		"%s event=%s workspace=%s terminal=%s details=%s\n",
		time.Now().Format(time.RFC3339Nano),
		payload.Event,
		payload.WorkspaceID,
		payload.TerminalID,
		details,
	)
}

func debugTerminalServicef(format string, args ...any) {
	if !terminalDebugConfig() {
		return
	}
	message := strings.ReplaceAll(fmt.Sprintf(format, args...), "\n", "\\n")
	terminalDebugMu.Lock()
	defer terminalDebugMu.Unlock()
	_, _ = fmt.Fprintf(
		terminalDebugLog,
		"%s event=terminal_service details=%s\n",
		time.Now().Format(time.RFC3339Nano),
		message,
	)
}

func warnTerminalServicef(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(os.Stderr, "[terminal-service] %s %s\n", time.Now().Format(time.RFC3339Nano), message)
}
