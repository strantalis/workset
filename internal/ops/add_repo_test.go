package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	ggit "github.com/go-git/go-git/v6"
	ggitconfig "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/workspace"
)

func TestAddRepoLinksLocal(t *testing.T) {
	source := setupRepo(t)
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, err := AddRepo(context.Background(), AddRepoInput{
		WorkspaceRoot: root,
		Name:          "demo-repo",
		URL:           source,
		Defaults:      defaults,
		Remotes: config.Remotes{
			Base:  config.RemoteConfig{DefaultBranch: defaults.BaseBranch},
			Write: config.RemoteConfig{DefaultBranch: defaults.BaseBranch},
		},
		Git: git.NewGoGitClient(),
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	ws, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if len(ws.Repos) != 1 {
		t.Fatalf("expected repo in workspace")
	}
	expectedPath, err := filepath.EvalSymlinks(source)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if ws.Repos[0].LocalPath != expectedPath {
		t.Fatalf("expected local_path %s, got %s", expectedPath, ws.Repos[0].LocalPath)
	}
}

func TestAddRepoDerivesRemotesFromLocal(t *testing.T) {
	source := setupRepo(t)
	addRemote(t, source, "origin", source)
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, err := AddRepo(context.Background(), AddRepoInput{
		WorkspaceRoot: root,
		Name:          "demo-repo",
		URL:           source,
		Defaults:      defaults,
		Remotes: config.Remotes{
			Base:  config.RemoteConfig{DefaultBranch: defaults.BaseBranch},
			Write: config.RemoteConfig{DefaultBranch: defaults.BaseBranch},
		},
		Git: git.NewGoGitClient(),
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	ws, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if len(ws.Repos) != 1 {
		t.Fatalf("expected repo in workspace")
	}
	if ws.Repos[0].Remotes.Base.Name != "origin" {
		t.Fatalf("expected base remote origin, got %q", ws.Repos[0].Remotes.Base.Name)
	}
	if ws.Repos[0].Remotes.Write.Name != "origin" {
		t.Fatalf("expected write remote origin, got %q", ws.Repos[0].Remotes.Write.Name)
	}
}

func TestStatusDirty(t *testing.T) {
	source := setupRepo(t)
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, err := AddRepo(context.Background(), AddRepoInput{
		WorkspaceRoot: root,
		Name:          "demo-repo",
		URL:           source,
		Defaults:      defaults,
		Remotes: config.Remotes{
			Base:  config.RemoteConfig{DefaultBranch: defaults.BaseBranch},
			Write: config.RemoteConfig{DefaultBranch: defaults.BaseBranch},
		},
		Git: git.NewGoGitClient(),
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	worktreePath := workspace.RepoWorktreePath(root, "demo", "demo-repo")
	if err := os.WriteFile(filepath.Join(worktreePath, "extra.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	statuses, err := Status(context.Background(), StatusInput{
		WorkspaceRoot: root,
		Defaults:      defaults,
		Git:           git.NewGoGitClient(),
	})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if !statuses[0].Dirty {
		t.Fatalf("expected dirty status")
	}
}

func setupRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "source")
	repo, err := ggit.PlainInit(root, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if _, err := worktree.Add("README.md"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	_, err = worktree.Commit("initial", &ggit.CommitOptions{
		Author: &object.Signature{
			Name:  "Tester",
			Email: "tester@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return root
}

func addRemote(t *testing.T, repoPath, name, url string) {
	t.Helper()
	repo, err := ggit.PlainOpen(repoPath)
	if err != nil {
		t.Fatalf("PlainOpen: %v", err)
	}
	if _, err := repo.CreateRemote(&ggitconfig.RemoteConfig{
		Name: name,
		URLs: []string{url},
	}); err != nil && !errors.Is(err, ggit.ErrRemoteExists) {
		t.Fatalf("CreateRemote: %v", err)
	}
}
