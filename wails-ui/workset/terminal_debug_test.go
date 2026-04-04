package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/strantalis/workset/pkg/worksetapi"
)

func resetAppTerminalDebugLogForTest(t *testing.T) {
	t.Helper()
	if terminalDebugLog != nil {
		_ = terminalDebugLog.Close()
	}
	terminalDebugOnce = sync.Once{}
	terminalDebugEnabled = false
	terminalDebugLog = nil
}

func TestDebugTerminalServicefWritesOnlyWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "terminal-debug.log")

	t.Setenv("WORKSET_TERMINAL_DEBUG_LOG", "1")
	t.Setenv("WORKSET_TERMINAL_DEBUG_LOG_PATH", logPath)
	resetAppTerminalDebugLogForTest(t)
	t.Cleanup(func() { resetAppTerminalDebugLogForTest(t) })

	debugTerminalServicef("client_ready")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read debug log: %v", err)
	}
	if !strings.Contains(string(data), "event=terminal_service details=client_ready") {
		t.Fatalf("expected terminal service debug line, got %q", string(data))
	}
}

func TestDebugTerminalServicefDoesNothingWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "terminal-debug.log")

	t.Setenv("WORKSET_TERMINAL_DEBUG_LOG", "0")
	t.Setenv("WORKSET_TERMINAL_DEBUG_LOG_PATH", logPath)
	resetAppTerminalDebugLogForTest(t)
	t.Cleanup(func() { resetAppTerminalDebugLogForTest(t) })

	debugTerminalServicef("client_ready")

	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("expected no debug log file, got err=%v", err)
	}
}

func TestEnsureConfiguredTerminalDebugLoggingUsesConfigDefault(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")

	t.Setenv("HOME", root)
	t.Setenv("WORKSET_TERMINAL_DEBUG_LOG", "")
	t.Setenv("WORKSET_TERMINAL_DEBUG_LOG_PATH", "")

	app := NewApp()
	app.service = worksetapi.NewService(worksetapi.Options{ConfigPath: configPath})
	if _, err := app.SetDefaultSetting("defaults.terminal_debug_log", "on"); err != nil {
		t.Fatalf("set terminal debug log default: %v", err)
	}

	ensureConfiguredTerminalDebugLogging(app)

	if got := os.Getenv("WORKSET_TERMINAL_DEBUG_LOG"); got != "1" {
		t.Fatalf("expected WORKSET_TERMINAL_DEBUG_LOG=1, got %q", got)
	}
	wantPath := filepath.Join(root, ".workset", "terminal_debug.log")
	if got := os.Getenv("WORKSET_TERMINAL_DEBUG_LOG_PATH"); got != wantPath {
		t.Fatalf("expected WORKSET_TERMINAL_DEBUG_LOG_PATH=%q, got %q", wantPath, got)
	}
}
