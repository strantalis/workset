package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceInitAndConfig(t *testing.T) {
	runner := newRunner(t)
	target := filepath.Join(runner.root, "init-ws")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir init-ws: %v", err)
	}
	if _, err := runner.run("new", "init-ws", "--path", target); err != nil {
		t.Fatalf("workset new --path: %v", err)
	}
	if _, err := runner.run("config", "set", "defaults.base_branch", "trunk"); err != nil {
		t.Fatalf("config set: %v", err)
	}
	repoStore := filepath.Join(runner.root, "repo-store")
	if _, err := runner.run("config", "set", "defaults.repo_store_root", repoStore); err != nil {
		t.Fatalf("config set repo_store_root: %v", err)
	}
	out, err := runner.run("config", "show", "--json")
	if err != nil {
		t.Fatalf("config show: %v", err)
	}
	if !strings.Contains(out, "\"base_branch\": \"trunk\"") {
		t.Fatalf("config show missing base_branch: %s", out)
	}
	if !strings.Contains(out, repoStore) {
		t.Fatalf("config show missing repo_store_root: %s", out)
	}
	out, err = runner.run("ls", "--plain")
	if err != nil {
		t.Fatalf("workset ls: %v", err)
	}
	if !strings.Contains(out, "init-ws") {
		t.Fatalf("workset ls missing init-ws: %s", out)
	}
}

func TestWorkspaceListJSON(t *testing.T) {
	runner := newRunner(t)
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	out, err := runner.run("ls", "--json")
	if err != nil {
		t.Fatalf("workset ls --json: %v", err)
	}
	if !strings.Contains(out, "\"name\": \"demo\"") {
		t.Fatalf("workset ls json missing demo: %s", out)
	}
}

func TestWorkspaceRemoveDelete(t *testing.T) {
	runner := newRunner(t)
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("rm", "-t", "demo"); err != nil {
		t.Fatalf("workset rm: %v", err)
	}

	wsPath := runner.threadRoot("demo")
	if _, err := os.Stat(wsPath); err != nil {
		t.Fatalf("expected workspace to remain: %v", err)
	}

	if _, err := runner.run("rm", "-t", wsPath, "--delete", "--yes"); err != nil {
		t.Fatalf("workset rm --delete: %v", err)
	}
	if _, err := os.Stat(wsPath); !os.IsNotExist(err) {
		t.Fatalf("expected workspace deleted, got err=%v", err)
	}
}

func TestWorkspaceRemoveDirtyWorktree(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "ws-dirty-repo"))

	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-t", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	worktreePath := filepath.Join(runner.threadRoot("demo"), "ws-dirty-repo")
	if err := os.WriteFile(filepath.Join(worktreePath, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty: %v", err)
	}

	if _, err := runner.run("rm", "-t", "demo", "--delete", "--yes"); err == nil {
		t.Fatalf("expected workspace rm to fail when dirty")
	}

	if _, err := runner.run("rm", "-t", "demo", "--delete", "--yes", "--force"); err != nil {
		t.Fatalf("workspace rm --force: %v", err)
	}

	if _, err := os.Stat(runner.threadRoot("demo")); !os.IsNotExist(err) {
		t.Fatalf("expected workspace deleted, got err=%v", err)
	}
}

func TestWorkspaceRemoveSquashMergedBranch(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "squash-merged-repo"))

	if _, err := runner.run("new", "ws-switch"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-t", "ws-switch", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	repoName := filepath.Base(source)
	worktreePath := filepath.Join(runner.threadRoot("ws-switch"), repoName)

	commitFile(t, worktreePath, "", "feature.txt", "feature", "feat: add feature")
	commitFile(t, worktreePath, "", "deps.txt", "dep-one", "chore: adjust deps")
	runGit(t, source, "checkout", "main")
	if err := os.WriteFile(filepath.Join(source, "feature.txt"), []byte("feature"), 0o644); err != nil {
		t.Fatalf("write squash feature: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "deps.txt"), []byte("dep-one"), 0o644); err != nil {
		t.Fatalf("write squash deps: %v", err)
	}
	runGit(t, source, "add", "feature.txt", "deps.txt")
	runGit(t, source, "commit", "-m", "chore: squash merge feature")
	commitFile(t, source, "main", "deps.txt", "dep-two", "chore: tweak after merge")

	if _, err := runner.run("rm", "-t", "ws-switch", "--delete", "--yes"); err != nil {
		t.Fatalf("expected workspace rm after squash merge: %v", err)
	}
}

func TestWorkspaceRemoveMergeCommitMergedBranch(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "merge-merged-repo"))

	if _, err := runner.run("new", "ws-merge"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-t", "ws-merge", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	repoName := filepath.Base(source)
	worktreePath := filepath.Join(runner.threadRoot("ws-merge"), repoName)

	commitFile(t, worktreePath, "", "feature.txt", "feature", "feat: add feature")
	runGit(t, source, "checkout", "main")
	runGit(t, source, "merge", "--no-ff", "-m", "merge ws-merge", "ws-merge")

	if _, err := runner.run("rm", "-t", "ws-merge", "--delete", "--yes"); err != nil {
		t.Fatalf("expected workspace rm after merge commit: %v", err)
	}
}

func TestWorkspaceRemoveRebaseMergedBranch(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "rebase-merged-repo"))

	if _, err := runner.run("new", "ws-rebase"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-t", "ws-rebase", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	repoName := filepath.Base(source)
	worktreePath := filepath.Join(runner.threadRoot("ws-rebase"), repoName)

	commitFile(t, worktreePath, "", "feature.txt", "feature", "feat: add feature")
	commitFile(t, source, "main", "base.txt", "main moved", "chore: main moved")
	runGit(t, source, "cherry-pick", "ws-rebase")

	if _, err := runner.run("rm", "-t", "ws-rebase", "--delete", "--yes"); err != nil {
		t.Fatalf("expected workspace rm after rebase/cherry-pick merge: %v", err)
	}
}
