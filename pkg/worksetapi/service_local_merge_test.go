package worksetapi

import (
	"context"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/workspace"
)

func TestListWorkspaceSnapshotsIncludesIntegrationMode(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	cfg := env.loadConfig()
	cfg.Repos = map[string]config.RegisteredRepo{
		"repo-a": {
			Path:          env.createLocalRepo("repo-a"),
			Remote:        "origin",
			DefaultBranch: "main",
		},
	}
	env.saveConfig(cfg)

	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	wsCfg.Repos = []config.RepoConfig{{
		Name:      "repo-a",
		RepoDir:   "repo-a",
		LocalPath: cfg.Repos["repo-a"].Path,
	}}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace: %v", err)
	}

	result, err := env.svc.ListWorkspaceSnapshots(context.Background(), WorkspaceSnapshotOptions{})
	if err != nil {
		t.Fatalf("list workspace snapshots: %v", err)
	}
	if len(result.Workspaces) != 1 || len(result.Workspaces[0].Repos) != 1 {
		t.Fatalf("unexpected snapshots: %+v", result.Workspaces)
	}
}

func TestLocalMergeRequiresLocalPath(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	wsCfg.Repos = []config.RepoConfig{{
		Name:    "repo-a",
		RepoDir: "repo-a",
	}}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace: %v", err)
	}

	_, err = env.svc.LocalMerge(context.Background(), LocalMergeInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
	})
	if err == nil {
		t.Fatal("expected local merge to reject missing local_path")
	}
	validationErr := requireErrorType[ValidationError](t, err)
	if validationErr.Message != "local_path required for local merge" {
		t.Fatalf("unexpected validation error: %q", validationErr.Message)
	}
}
