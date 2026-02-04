//go:build !dev
// +build !dev

package main

import (
	"os"
	"path/filepath"

	"github.com/strantalis/workset/pkg/worksetapi"
)

func worksetAppDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset"), nil
}

func serviceOptions() worksetapi.Options {
	return worksetapi.Options{}
}

func ensureDevConfig() {}
