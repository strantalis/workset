package ops

import (
	"path/filepath"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/workspace"
)

func TestUpdateRepoRemotes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults
	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	wsConfig, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	wsConfig.Repos = []config.RepoConfig{
		{
			Name: "repo1",
			Remotes: config.Remotes{
				Base:  config.RemoteConfig{Name: "origin", DefaultBranch: "main"},
				Write: config.RemoteConfig{Name: "origin", DefaultBranch: "main"},
			},
		},
	}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsConfig); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}

	if _, err := UpdateRepoRemotes(UpdateRepoRemotesInput{
		WorkspaceRoot:  root,
		Name:           "repo1",
		Defaults:       defaults,
		BaseRemote:     "upstream",
		WriteRemote:    "origin",
		BaseBranch:     "trunk",
		WriteBranch:    "trunk",
		BaseRemoteSet:  true,
		WriteRemoteSet: true,
		BaseBranchSet:  true,
		WriteBranchSet: true,
	}); err != nil {
		t.Fatalf("UpdateRepoRemotes: %v", err)
	}

	updated, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("LoadWorkspace updated: %v", err)
	}
	if len(updated.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(updated.Repos))
	}
	if updated.Repos[0].Remotes.Base.Name != "upstream" {
		t.Fatalf("expected base remote upstream, got %q", updated.Repos[0].Remotes.Base.Name)
	}
	if updated.Repos[0].Remotes.Write.Name != "origin" {
		t.Fatalf("expected write remote origin, got %q", updated.Repos[0].Remotes.Write.Name)
	}
	if updated.Repos[0].Remotes.Base.DefaultBranch != "trunk" {
		t.Fatalf("expected base branch trunk, got %q", updated.Repos[0].Remotes.Base.DefaultBranch)
	}
	if updated.Repos[0].Remotes.Write.DefaultBranch != "trunk" {
		t.Fatalf("expected write branch trunk, got %q", updated.Repos[0].Remotes.Write.DefaultBranch)
	}
}
