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

func TestWorkspaceNewWithGroupAndRepo(t *testing.T) {
	runner := newRunner(t)
	repoA := setupRepo(t, filepath.Join(runner.root, "src", "new-group-repo-a"))
	repoB := setupRepo(t, filepath.Join(runner.root, "src", "new-group-repo-b"))

	if _, err := runner.run("repo", "registry", "add", "new-repo-a", repoA); err != nil {
		t.Fatalf("registry add repo-a: %v", err)
	}
	if _, err := runner.run("repo", "registry", "add", "new-repo-b", repoB); err != nil {
		t.Fatalf("registry add repo-b: %v", err)
	}
	if _, err := runner.run("group", "create", "new-group"); err != nil {
		t.Fatalf("group create: %v", err)
	}
	if _, err := runner.run("group", "add", "new-group", "new-repo-a"); err != nil {
		t.Fatalf("group add: %v", err)
	}

	if _, err := runner.run("new", "demo", "--group", "new-group", "--repo", "new-repo-b"); err != nil {
		t.Fatalf("workset new with group+repo: %v", err)
	}
	out, err := runner.run("repo", "ls", "-w", "demo", "--plain")
	if err != nil {
		t.Fatalf("repo ls: %v", err)
	}
	if !strings.Contains(out, "new-repo-a") || !strings.Contains(out, "new-repo-b") {
		t.Fatalf("repo ls missing group/repos: %s", out)
	}
}

func TestWorkspaceNewDuplicateRepoAcrossGroupAndRepo(t *testing.T) {
	runner := newRunner(t)
	repoA := setupRepo(t, filepath.Join(runner.root, "src", "conflict-repo-a"))

	if _, err := runner.run("repo", "registry", "add", "conflict-repo-a", repoA); err != nil {
		t.Fatalf("registry add repo-a: %v", err)
	}
	if _, err := runner.run("group", "create", "conflict-group"); err != nil {
		t.Fatalf("group create: %v", err)
	}
	if _, err := runner.run("group", "add", "conflict-group", "conflict-repo-a"); err != nil {
		t.Fatalf("group add: %v", err)
	}

	if _, err := runner.run("new", "demo", "--group", "conflict-group", "--repo", "conflict-repo-a"); err != nil {
		t.Fatalf("workset new with duplicate repo: %v", err)
	}

	out, err := runner.run("repo", "ls", "-w", "demo", "--plain")
	if err != nil {
		t.Fatalf("repo ls: %v", err)
	}
	if !strings.Contains(out, "conflict-repo-a") {
		t.Fatalf("repo ls missing repo: %s", out)
	}
}

func TestWorkspaceRemoveDelete(t *testing.T) {
	runner := newRunner(t)
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("rm", "-w", "demo"); err != nil {
		t.Fatalf("workset rm: %v", err)
	}

	wsPath := filepath.Join(runner.workspaceRoot(), "demo")
	if _, err := os.Stat(wsPath); err != nil {
		t.Fatalf("expected workspace to remain: %v", err)
	}

	if _, err := runner.run("rm", "-w", "demo", "--delete", "--yes"); err != nil {
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
	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	worktreePath := filepath.Join(runner.workspaceRoot(), "demo", "ws-dirty-repo")
	if err := os.WriteFile(filepath.Join(worktreePath, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty: %v", err)
	}

	if _, err := runner.run("rm", "-w", "demo", "--delete", "--yes"); err == nil {
		t.Fatalf("expected workspace rm to fail when dirty")
	}

	if _, err := runner.run("rm", "-w", "demo", "--delete", "--yes", "--force"); err != nil {
		t.Fatalf("workspace rm --force: %v", err)
	}

	if _, err := os.Stat(filepath.Join(runner.workspaceRoot(), "demo")); !os.IsNotExist(err) {
		t.Fatalf("expected workspace deleted, got err=%v", err)
	}
}

func TestWorkspaceRemoveSquashMergedBranch(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "squash-merged-repo"))

	if _, err := runner.run("new", "ws-switch"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "ws-switch", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	repoName := filepath.Base(source)
	worktreePath := filepath.Join(runner.workspaceRoot(), "ws-switch", repoName)

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

	if _, err := runner.run("rm", "-w", "ws-switch", "--delete", "--yes"); err != nil {
		t.Fatalf("expected workspace rm after squash merge: %v", err)
	}
}

func TestWorkspaceRemoveMergeCommitMergedBranch(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "merge-merged-repo"))

	if _, err := runner.run("new", "ws-merge"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "ws-merge", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	repoName := filepath.Base(source)
	worktreePath := filepath.Join(runner.workspaceRoot(), "ws-merge", repoName)

	commitFile(t, worktreePath, "", "feature.txt", "feature", "feat: add feature")
	runGit(t, source, "checkout", "main")
	runGit(t, source, "merge", "--no-ff", "-m", "merge ws-merge", "ws-merge")

	if _, err := runner.run("rm", "-w", "ws-merge", "--delete", "--yes"); err != nil {
		t.Fatalf("expected workspace rm after merge commit: %v", err)
	}
}

func TestWorkspaceRemoveRebaseMergedBranch(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "rebase-merged-repo"))

	if _, err := runner.run("new", "ws-rebase"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "ws-rebase", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	repoName := filepath.Base(source)
	worktreePath := filepath.Join(runner.workspaceRoot(), "ws-rebase", repoName)

	commitFile(t, worktreePath, "", "feature.txt", "feature", "feat: add feature")
	commitFile(t, source, "main", "base.txt", "main moved", "chore: main moved")
	runGit(t, source, "cherry-pick", "ws-rebase")

	if _, err := runner.run("rm", "-w", "ws-rebase", "--delete", "--yes"); err != nil {
		t.Fatalf("expected workspace rm after rebase/cherry-pick merge: %v", err)
	}
}
