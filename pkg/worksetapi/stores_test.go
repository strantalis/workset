package worksetapi

import (
	"context"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/workspace"
)

func TestFileWorkspaceStoreSaveConfig(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	store := FileWorkspaceStore{}
	cfg := config.WorkspaceConfig{Name: "demo"}
	if err := store.SaveConfig(context.Background(), root, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	loaded, err := store.LoadConfig(context.Background(), root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.Name != "demo" {
		t.Fatalf("unexpected config: %+v", loaded)
	}

	if _, err := store.Load(context.Background(), root, env.loadConfig().Defaults); err != nil {
		t.Fatalf("load workspace: %v", err)
	}

	state, err := store.LoadState(context.Background(), root)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.CurrentBranch == "" {
		t.Fatalf("expected current branch")
	}

	state.CurrentBranch = "demo"
	if err := store.SaveState(context.Background(), root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if _, err := config.LoadWorkspace(workspace.WorksetFile(root)); err != nil {
		t.Fatalf("load workspace file: %v", err)
	}
}
