//go:build dev
// +build dev

package main

import (
	"os"
	"path/filepath"
	"strings"
)

func ensureDevTerminalServiceSocket() {
	if strings.TrimSpace(os.Getenv("WORKSET_TERMINAL_SERVICE_SOCKET")) != "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	_ = os.Setenv("WORKSET_TERMINAL_SERVICE_SOCKET", filepath.Join(home, ".workset", "terminal-service-dev.sock"))
}
