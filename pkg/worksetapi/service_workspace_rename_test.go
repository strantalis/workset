package worksetapi

import (
	"context"
	"testing"
)

func TestRenameWorkspaceUpdatesConfigAndWorkset(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	root := env.createWorkspace(ctx, "alpha")

	result, err := env.svc.RenameWorkspace(ctx, WorkspaceRenameInput{
		Selector: WorkspaceSelector{Value: "alpha"},
		NewName:  "beta",
	})
	if err != nil {
		t.Fatalf("rename workspace: %v", err)
	}
	if result.Name != "beta" {
		t.Fatalf("unexpected rename result: %+v", result)
	}

	cfg := env.loadConfig()
	if _, ok := cfg.Workspaces["alpha"]; ok {
		t.Fatalf("expected old name removed")
	}
	if cfg.Workspaces["beta"].Path != root {
		t.Fatalf("expected new name path %s, got %+v", root, cfg.Workspaces["beta"])
	}

	wsConfig, err := env.svc.workspaces.LoadConfig(ctx, root)
	if err != nil {
		t.Fatalf("load workspace config: %v", err)
	}
	if wsConfig.Name != "beta" {
		t.Fatalf("expected workset.yaml name updated, got %q", wsConfig.Name)
	}
}

func TestRenameWorkspaceConflicts(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	env.createWorkspace(ctx, "alpha")
	env.createWorkspace(ctx, "beta")

	_, err := env.svc.RenameWorkspace(ctx, WorkspaceRenameInput{
		Selector: WorkspaceSelector{Value: "alpha"},
		NewName:  "beta",
	})
	_ = requireErrorType[ConflictError](t, err)
}
