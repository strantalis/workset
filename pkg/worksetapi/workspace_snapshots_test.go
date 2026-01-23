package worksetapi

import (
	"context"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/workspace"
)

func TestListWorkspaceSnapshotsWithoutStatus(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	wsCfg.Repos = append(wsCfg.Repos, config.RepoConfig{
		Name:    "app",
		RepoDir: "app",
	})
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace: %v", err)
	}

	result, err := env.svc.ListWorkspaceSnapshots(context.Background(), WorkspaceSnapshotOptions{})
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(result.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(result.Workspaces))
	}
	if len(result.Workspaces[0].Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(result.Workspaces[0].Repos))
	}
	repo := result.Workspaces[0].Repos[0]
	if repo.StatusKnown {
		t.Fatalf("expected status unknown")
	}
	if repo.Dirty || repo.Missing {
		t.Fatalf("expected empty status")
	}
}

func TestListWorkspaceSnapshotsWithStatus(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	wsCfg.Repos = append(wsCfg.Repos, config.RepoConfig{
		Name:    "app",
		RepoDir: "app",
	})
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace: %v", err)
	}

	repoPath := workspace.RepoWorktreePath(root, "demo", "app")
	env.git.status[repoPath] = git.StatusSummary{Dirty: true}

	result, err := env.svc.ListWorkspaceSnapshots(context.Background(), WorkspaceSnapshotOptions{
		IncludeStatus: true,
	})
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(result.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(result.Workspaces))
	}
	if len(result.Workspaces[0].Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(result.Workspaces[0].Repos))
	}
	repo := result.Workspaces[0].Repos[0]
	if !repo.StatusKnown {
		t.Fatalf("expected status known")
	}
	if !repo.Dirty || repo.Missing {
		t.Fatalf("unexpected status: dirty=%v missing=%v", repo.Dirty, repo.Missing)
	}
}
