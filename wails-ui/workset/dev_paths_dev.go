//go:build dev
// +build dev

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/pkg/worksetapi"
	"gopkg.in/yaml.v3"
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

	worksetRoot := dir
	repoStoreRoot := filepath.Join(dir, "repos")
	if !info.Exists {
		_, _, _ = svc.SetDefault(ctx, "defaults.workset_root", worksetRoot)
		_, _, _ = svc.SetDefault(ctx, "defaults.repo_store_root", repoStoreRoot)
		return
	}

	// Force dev roots to the dev sandbox path and persist keys if they are only inferred in memory.
	if strings.TrimSpace(cfg.Defaults.WorksetRoot) != worksetRoot || !defaultsKeyExists(info.Path, "workset_root") {
		_, _, _ = svc.SetDefault(ctx, "defaults.workset_root", worksetRoot)
	}
	if strings.TrimSpace(cfg.Defaults.RepoStoreRoot) != repoStoreRoot || !defaultsKeyExists(info.Path, "repo_store_root") {
		_, _, _ = svc.SetDefault(ctx, "defaults.repo_store_root", repoStoreRoot)
	}
}

func defaultsKeyExists(path, key string) bool {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(key) == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	raw := map[string]any{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false
	}
	defaultsRaw, ok := raw["defaults"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = defaultsRaw[key]
	return ok
}
