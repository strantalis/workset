package terminalservice

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func resetTerminalDebugLogForTest(t *testing.T) {
	t.Helper()
	if terminalDebugLog != nil {
		_ = terminalDebugLog.Close()
	}
	terminalDebugOnce = sync.Once{}
	terminalDebugEnabled = false
	terminalDebugLog = nil
}

func TestDebugServerfWritesOnlyWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "terminal-debug.log")

	t.Setenv("WORKSET_TERMINAL_DEBUG_LOG", "1")
	t.Setenv("WORKSET_TERMINAL_DEBUG_LOG_PATH", logPath)
	resetTerminalDebugLogForTest(t)
	t.Cleanup(func() { resetTerminalDebugLogForTest(t) })

	debugServerf("ws_subscribe session=%s", "session-1")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read debug log: %v", err)
	}
	if !strings.Contains(string(data), "server ws_subscribe session=session-1") {
		t.Fatalf("expected server debug line, got %q", string(data))
	}
}

func TestDebugServerfDoesNothingWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "terminal-debug.log")

	t.Setenv("WORKSET_TERMINAL_DEBUG_LOG", "0")
	t.Setenv("WORKSET_TERMINAL_DEBUG_LOG_PATH", logPath)
	resetTerminalDebugLogForTest(t)
	t.Cleanup(func() { resetTerminalDebugLogForTest(t) })

	debugServerf("ws_subscribe session=%s", "session-1")

	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("expected no debug log file, got err=%v", err)
	}
}
