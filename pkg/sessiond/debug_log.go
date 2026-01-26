package sessiond

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

func terminalDebugConfig() bool {
	terminalDebugOnce.Do(func() {
		terminalDebugEnabled = envTruthy(os.Getenv("WORKSET_TERMINAL_DEBUG_LOG"))
		if !terminalDebugEnabled {
			return
		}
		logPath := strings.TrimSpace(os.Getenv("WORKSET_TERMINAL_DEBUG_LOG_PATH"))
		if logPath == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				terminalDebugEnabled = false
				return
			}
			logPath = filepath.Join(home, ".workset", "terminal_debug.log")
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

func debugLogf(format string, args ...any) {
	if !terminalDebugConfig() {
		return
	}
	terminalDebugMu.Lock()
	defer terminalDebugMu.Unlock()
	_, _ = fmt.Fprintf(
		terminalDebugLog,
		"%s "+format+"\n",
		append([]any{time.Now().Format(time.RFC3339Nano)}, args...)...,
	)
}
