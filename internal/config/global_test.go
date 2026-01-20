package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGlobalDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg, err := LoadGlobal(path)
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}

	if cfg.Defaults.BaseBranch != "main" {
		t.Fatalf("expected default base_branch main, got %q", cfg.Defaults.BaseBranch)
	}
	if cfg.Defaults.SessionBackend == "" {
		t.Fatalf("expected default session_backend set")
	}
	if cfg.Defaults.SessionNameFormat == "" {
		t.Fatalf("expected default session_name_format set")
	}
}

func TestSaveLoadGlobal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := DefaultConfig()
	cfg.Workspaces["alpha"] = WorkspaceRef{
		Path:           "/tmp/alpha",
		ArchivedAt:     "2024-01-02T03:04:05Z",
		ArchivedReason: "paused",
	}

	if err := SaveGlobal(path, cfg); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}

	loaded, err := LoadGlobal(path)
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}

	if loaded.Workspaces["alpha"].Path != "/tmp/alpha" {
		t.Fatalf("expected workspace path, got %+v", loaded.Workspaces["alpha"])
	}
	if loaded.Workspaces["alpha"].ArchivedAt != "2024-01-02T03:04:05Z" {
		t.Fatalf("expected archived_at preserved, got %+v", loaded.Workspaces["alpha"])
	}
	if loaded.Workspaces["alpha"].ArchivedReason != "paused" {
		t.Fatalf("expected archived_reason preserved, got %+v", loaded.Workspaces["alpha"])
	}
}

func TestLoadGlobalMigratesLegacyConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacyPaths, err := legacyGlobalConfigPaths()
	if err != nil || len(legacyPaths) == 0 {
		t.Fatalf("legacyGlobalConfigPaths: %v", err)
	}
	legacyPath := legacyPaths[0]
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("defaults:\n  base_branch: legacy\n"), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	cfg, err := LoadGlobal("")
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if cfg.Defaults.BaseBranch != "legacy" {
		t.Fatalf("expected migrated base_branch legacy, got %q", cfg.Defaults.BaseBranch)
	}

	newPath := filepath.Join(home, ".workset", "config.yaml")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("expected migrated config at %s: %v", newPath, err)
	}
}
