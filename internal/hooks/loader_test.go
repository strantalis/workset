package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRepoHooksMissing(t *testing.T) {
	root := t.TempDir()
	_, exists, err := LoadRepoHooks(root)
	if err != nil {
		t.Fatalf("load repo hooks: %v", err)
	}
	if exists {
		t.Fatalf("expected hooks file to be missing")
	}
}

func TestLoadRepoHooksValid(t *testing.T) {
	root := t.TempDir()
	hookDir := filepath.Join(root, ".workset")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := []byte("hooks:\n  - id: bootstrap\n    on: [worktree.created]\n    run: [\"npm\", \"ci\"]\n")
	if err := os.WriteFile(filepath.Join(hookDir, "hooks.yaml"), data, 0o644); err != nil {
		t.Fatalf("write hooks: %v", err)
	}
	cfg, exists, err := LoadRepoHooks(root)
	if err != nil {
		t.Fatalf("load repo hooks: %v", err)
	}
	if !exists {
		t.Fatalf("expected hooks file")
	}
	if len(cfg.Hooks) != 1 {
		t.Fatalf("expected 1 hook")
	}
	if cfg.Hooks[0].ID != "bootstrap" {
		t.Fatalf("unexpected hook id: %s", cfg.Hooks[0].ID)
	}
}
