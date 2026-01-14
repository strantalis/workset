package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	ggit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/workspace"
)

func TestAddRepoClones(t *testing.T) {
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
		Editable:      true,
		Defaults:      defaults,
		Remotes: config.Remotes{
			Base:  config.RemoteConfig{Name: defaults.Remotes.Base, DefaultBranch: defaults.BaseBranch},
			Write: config.RemoteConfig{Name: defaults.Remotes.Write, DefaultBranch: defaults.BaseBranch},
		},
		Git: git.NewGoGitClient(),
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	worktreePath := workspace.RepoWorktreePath(root, defaults.BaseBranch, "demo-repo")
	if _, err := ggit.PlainOpenWithOptions(worktreePath, &ggit.PlainOpenOptions{EnableDotGitCommonDir: true}); err != nil {
		t.Fatalf("expected cloned repo: %v", err)
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
		Editable:      true,
		Defaults:      defaults,
		Remotes: config.Remotes{
			Base:  config.RemoteConfig{Name: defaults.Remotes.Base, DefaultBranch: defaults.BaseBranch},
			Write: config.RemoteConfig{Name: defaults.Remotes.Write, DefaultBranch: defaults.BaseBranch},
		},
		Git: git.NewGoGitClient(),
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	worktreePath := workspace.RepoWorktreePath(root, defaults.BaseBranch, "demo-repo")
	if err := os.WriteFile(filepath.Join(worktreePath, "extra.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	statuses, err := Status(context.Background(), StatusInput{
		WorkspaceRoot: root,
		Defaults:      defaults,
		Git:           git.NewGoGitClient(),
		IncludeAll:    true,
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
