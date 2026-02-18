package worksetapi

import (
	"context"
	"testing"

	"github.com/strantalis/workset/internal/config"
)

func TestReorderWorkspacesUpdatesPinOrder(t *testing.T) {
	env := newTestEnv(t)
	cfg := env.loadConfig()
	cfg.Workspaces = map[string]config.WorkspaceRef{
		"alpha": {Path: "/tmp/alpha", Pinned: true, PinOrder: 0},
		"beta":  {Path: "/tmp/beta", Pinned: true, PinOrder: 1},
	}
	env.saveConfig(cfg)

	result, _, err := env.svc.ReorderWorkspaces(context.Background(), map[string]int{
		"alpha": 2,
		"beta":  0,
	})
	if err != nil {
		t.Fatalf("reorder workspaces: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 reordered workspaces, got %d", len(result))
	}

	updated := env.loadConfig()
	if updated.Workspaces["alpha"].PinOrder != 2 {
		t.Fatalf("expected alpha pin order 2, got %d", updated.Workspaces["alpha"].PinOrder)
	}
	if updated.Workspaces["beta"].PinOrder != 0 {
		t.Fatalf("expected beta pin order 0, got %d", updated.Workspaces["beta"].PinOrder)
	}
}

func TestReorderWorkspacesFailsWhenWorkspaceMissing(t *testing.T) {
	env := newTestEnv(t)
	cfg := env.loadConfig()
	cfg.Workspaces = map[string]config.WorkspaceRef{
		"alpha": {Path: "/tmp/alpha", Pinned: true, PinOrder: 0},
	}
	env.saveConfig(cfg)

	_, _, err := env.svc.ReorderWorkspaces(context.Background(), map[string]int{
		"alpha": 1,
		"beta":  0,
	})
	notFound := requireErrorType[NotFoundError](t, err)
	if notFound.Message == "" {
		t.Fatalf("expected not found message for unknown workspace")
	}

	updated := env.loadConfig()
	if updated.Workspaces["alpha"].PinOrder != 0 {
		t.Fatalf("expected alpha pin order unchanged on failure, got %d", updated.Workspaces["alpha"].PinOrder)
	}
}
