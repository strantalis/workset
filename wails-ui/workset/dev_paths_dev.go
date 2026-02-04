//go:build dev
// +build dev

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/pkg/worksetapi"
)

func worksetAppDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "dev"), nil
}

func serviceOptions() worksetapi.Options {
	dir, err := worksetAppDir()
	if err != nil {
		return worksetapi.Options{}
	}
	return worksetapi.Options{
		ConfigPath: filepath.Join(dir, "config.yaml"),
	}
}

func ensureDevConfig() {
	dir, err := worksetAppDir()
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	svc := worksetapi.NewService(serviceOptions())
	ctx := context.Background()
	cfg, info, err := svc.GetConfig(ctx)
	if err != nil {
		return
	}

	workspaceRoot := filepath.Join(dir, "workspaces")
	repoStoreRoot := filepath.Join(dir, "repos")
	if !info.Exists {
		_, _, _ = svc.SetDefault(ctx, "defaults.workspace_root", workspaceRoot)
		_, _, _ = svc.SetDefault(ctx, "defaults.repo_store_root", repoStoreRoot)
		return
	}

	if strings.TrimSpace(cfg.Defaults.WorkspaceRoot) == "" {
		_, _, _ = svc.SetDefault(ctx, "defaults.workspace_root", workspaceRoot)
	}
	if strings.TrimSpace(cfg.Defaults.RepoStoreRoot) == "" {
		_, _, _ = svc.SetDefault(ctx, "defaults.repo_store_root", repoStoreRoot)
	}
}
