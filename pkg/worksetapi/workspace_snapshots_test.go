package worksetapi

import (
	"context"
	"os"
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
	if repo.TrackedPullRequest != nil {
		t.Fatalf("expected no tracked pull request")
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
	state, err := workspace.LoadState(root)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	state.PullRequests = map[string]workspace.PullRequestState{
		"app": {
			Repo:       "app",
			Number:     123,
			URL:        "https://github.com/example/app/pull/123",
			Title:      "Use API-backed workspace snapshot data",
			State:      "open",
			Draft:      false,
			BaseRepo:   "example/app",
			BaseBranch: "main",
			HeadRepo:   "example/app",
			HeadBranch: "feature/snapshot",
			UpdatedAt:  "2026-02-09T00:00:00Z",
		},
	}
	if err := workspace.SaveState(root, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

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
	if repo.TrackedPullRequest == nil {
		t.Fatalf("expected tracked pull request")
	}
	if repo.TrackedPullRequest.Number != 123 {
		t.Fatalf("expected tracked pull request number 123, got %d", repo.TrackedPullRequest.Number)
	}
	if repo.TrackedPullRequest.State != "open" {
		t.Fatalf("expected tracked pull request state open, got %q", repo.TrackedPullRequest.State)
	}
}

func TestListWorkspaceSnapshotsIncludesWorkspaceWhenConfigMissing(t *testing.T) {
	env := newTestEnv(t)
	alphaRoot := env.createWorkspace(context.Background(), "alpha")
	betaRoot := env.createWorkspace(context.Background(), "beta")

	if err := os.Remove(workspace.WorksetFile(betaRoot)); err != nil {
		t.Fatalf("remove beta workset: %v", err)
	}

	result, err := env.svc.ListWorkspaceSnapshots(context.Background(), WorkspaceSnapshotOptions{})
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(result.Workspaces) != 2 {
		t.Fatalf("expected 2 workspaces with missing config tolerated, got %d", len(result.Workspaces))
	}
	if result.Workspaces[0].Name != "alpha" {
		t.Fatalf("expected alpha workspace first, got %q", result.Workspaces[0].Name)
	}
	if result.Workspaces[0].Path != alphaRoot {
		t.Fatalf("expected alpha path %q, got %q", alphaRoot, result.Workspaces[0].Path)
	}
	if result.Workspaces[1].Name != "beta" {
		t.Fatalf("expected beta workspace second, got %q", result.Workspaces[1].Name)
	}
	if len(result.Workspaces[1].Repos) != 0 {
		t.Fatalf("expected beta workspace to have no repos when config missing")
	}
}

func TestListWorkspaceSnapshotsIncludesWorkspaceWhenStateCorrupt(t *testing.T) {
	env := newTestEnv(t)
	alphaRoot := env.createWorkspace(context.Background(), "alpha")
	betaRoot := env.createWorkspace(context.Background(), "beta")

	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(betaRoot))
	if err != nil {
		t.Fatalf("load beta workspace config: %v", err)
	}
	wsCfg.Repos = append(wsCfg.Repos, config.RepoConfig{
		Name:    "app",
		RepoDir: "app",
	})
	if err := config.SaveWorkspace(workspace.WorksetFile(betaRoot), wsCfg); err != nil {
		t.Fatalf("save beta workspace config: %v", err)
	}

	if err := os.WriteFile(workspace.StatePath(betaRoot), []byte("{invalid-json"), 0o644); err != nil {
		t.Fatalf("corrupt beta state: %v", err)
	}

	result, err := env.svc.ListWorkspaceSnapshots(context.Background(), WorkspaceSnapshotOptions{
		IncludeStatus: true,
	})
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(result.Workspaces) != 2 {
		t.Fatalf("expected 2 workspaces with corrupt state tolerated, got %d", len(result.Workspaces))
	}
	if result.Workspaces[0].Name != "alpha" {
		t.Fatalf("expected alpha workspace first, got %q", result.Workspaces[0].Name)
	}
	if result.Workspaces[0].Path != alphaRoot {
		t.Fatalf("expected alpha path %q, got %q", alphaRoot, result.Workspaces[0].Path)
	}
	if result.Workspaces[1].Name != "beta" {
		t.Fatalf("expected beta workspace second, got %q", result.Workspaces[1].Name)
	}
	if len(result.Workspaces[1].Repos) != 1 {
		t.Fatalf("expected beta repos to load from config despite corrupt state")
	}
	if result.Workspaces[1].Repos[0].TrackedPullRequest != nil {
		t.Fatalf("expected no tracked pull request when state is corrupt")
	}
}
