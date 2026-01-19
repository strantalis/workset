package config

import (
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
	cfg.Workspaces["alpha"] = WorkspaceRef{Path: "/tmp/alpha"}

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
}
