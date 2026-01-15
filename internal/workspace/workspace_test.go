package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/strantalis/workset/internal/config"
)

func TestInitCreatesWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	ws, err := Init(root, "demo", defaults)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if ws.Config.Name != "demo" {
		t.Fatalf("expected name demo, got %q", ws.Config.Name)
	}

	if _, err := os.Stat(WorksetFile(root)); err != nil {
		t.Fatalf("workset.yaml missing: %v", err)
	}
	if _, err := os.Stat(StatePath(root)); err != nil {
		t.Fatalf("state.json missing: %v", err)
	}
}

func TestLoadRecreatesState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := os.Remove(StatePath(root)); err != nil {
		t.Fatalf("remove state: %v", err)
	}

	ws, err := Load(root, defaults)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ws.State.CurrentBranch != defaults.BaseBranch {
		t.Fatalf("expected base branch %q, got %q", defaults.BaseBranch, ws.State.CurrentBranch)
	}
	if _, err := os.Stat(StatePath(root)); err != nil {
		t.Fatalf("state.json not recreated: %v", err)
	}
}
