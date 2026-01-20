package worksetapi

import (
	"context"
	"testing"
)

func TestArchiveWorkspaceMarksConfig(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	env.createWorkspace(ctx, "alpha")

	result, _, err := env.svc.ArchiveWorkspace(ctx, WorkspaceSelector{Value: "alpha"}, "done for now")
	if err != nil {
		t.Fatalf("archive workspace: %v", err)
	}
	if !result.Archived {
		t.Fatalf("expected archived true")
	}

	cfg := env.loadConfig()
	ref := cfg.Workspaces["alpha"]
	if ref.ArchivedAt == "" {
		t.Fatalf("expected archived_at set")
	}
	if ref.ArchivedReason != "done for now" {
		t.Fatalf("unexpected archived_reason: %q", ref.ArchivedReason)
	}
}

func TestUnarchiveWorkspaceClearsFlags(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	env.createWorkspace(ctx, "alpha")

	if _, _, err := env.svc.ArchiveWorkspace(ctx, WorkspaceSelector{Value: "alpha"}, "paused"); err != nil {
		t.Fatalf("archive workspace: %v", err)
	}
	result, _, err := env.svc.UnarchiveWorkspace(ctx, WorkspaceSelector{Value: "alpha"})
	if err != nil {
		t.Fatalf("unarchive workspace: %v", err)
	}
	if result.Archived {
		t.Fatalf("expected archived false")
	}

	cfg := env.loadConfig()
	ref := cfg.Workspaces["alpha"]
	if ref.ArchivedAt != "" || ref.ArchivedReason != "" {
		t.Fatalf("expected archive fields cleared")
	}
}
