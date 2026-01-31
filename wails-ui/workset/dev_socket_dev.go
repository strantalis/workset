//go:build dev
// +build dev

package main

import (
	"os"
	"path/filepath"
	"strings"
)

func ensureDevSessiondSocket() {
	if strings.TrimSpace(os.Getenv("WORKSET_SESSIOND_SOCKET")) != "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	_ = os.Setenv("WORKSET_SESSIOND_SOCKET", filepath.Join(home, ".workset", "sessiond-dev.sock"))
}
