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
	if ws.State.CurrentBranch != "demo" {
		t.Fatalf("expected branch demo, got %q", ws.State.CurrentBranch)
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
	if ws.State.CurrentBranch != "demo" {
		t.Fatalf("expected branch %q, got %q", "demo", ws.State.CurrentBranch)
	}
	if _, err := os.Stat(StatePath(root)); err != nil {
		t.Fatalf("state.json not recreated: %v", err)
	}
}

func TestLoadNormalizesBranchToWorkspaceName(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := os.WriteFile(StatePath(root), []byte("{\"current_branch\": \"main\"}"), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	ws, err := Load(root, defaults)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ws.State.CurrentBranch != "demo" {
		t.Fatalf("expected branch %q, got %q", "demo", ws.State.CurrentBranch)
	}
}

func TestSaveStatePersistsSessions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	ws, err := Init(root, "demo", defaults)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	ws.State.Sessions = map[string]SessionState{
		"workset:demo": {
			Backend:   "tmux",
			StartedAt: "2026-01-01T00:00:00Z",
		},
	}
	if err := SaveState(root, ws.State); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := Load(root, defaults)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := loaded.State.Sessions["workset:demo"]; !ok {
		t.Fatalf("expected session to be persisted")
	}
}
