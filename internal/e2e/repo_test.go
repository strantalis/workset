package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoAddFromRelativePath(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "test-repo-1"))
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}

	workDir := filepath.Join(runner.root, "run")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir run: %v", err)
	}
	relSource, err := filepath.Rel(workDir, source)
	if err != nil {
		t.Fatalf("rel path: %v", err)
	}
	if _, err := runner.runDir(workDir, "repo", "add", relSource, "-w", "demo"); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	worktreePath := filepath.Join(runner.workspaceRoot(), "demo", "test-repo-1")
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("expected worktree at repo path, got err=%v", err)
	}

	out, err := runner.run("repo", "ls", "-w", "demo")
	if err != nil {
		t.Fatalf("repo ls: %v", err)
	}
	if !strings.Contains(out, "test-repo-1") {
		t.Fatalf("repo ls missing repo: %s", out)
	}

	out, err = runner.run("repo", "ls", "-w", "demo", "--json")
	if err != nil {
		t.Fatalf("repo ls --json: %v", err)
	}
	if !strings.Contains(out, "\"name\": \"test-repo-1\"") {
		t.Fatalf("repo ls --json missing repo: %s", out)
	}
	if !strings.Contains(out, "\"local_path\":") {
		t.Fatalf("repo ls --json missing local_path: %s", out)
	}
}

func TestRepoRemoveDeletesFiles(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "demo-repo"))
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	if _, err := runner.run("repo", "rm", "-w", "demo", "--delete-worktrees", "--yes", "demo-repo"); err != nil {
		t.Fatalf("repo rm: %v", err)
	}

	if _, err := os.Stat(source); err != nil {
		t.Fatalf("expected local repo to remain, got err=%v", err)
	}
}

func TestInterspersedFlags(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "flag-repo"))

	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}

	out, err := runner.run("repo", "add", source, "-w", "demo", "--json")
	if err != nil {
		t.Fatalf("repo add with interspersed flags: %v", err)
	}
	if !strings.Contains(out, "\"status\": \"ok\"") {
		t.Fatalf("repo add json missing status: %s", out)
	}

	if _, err := runner.run("repo", "rm", "--delete-worktrees", "--yes", "flag-repo", "-w", "demo"); err != nil {
		t.Fatalf("repo rm with interspersed -w: %v", err)
	}
}

func TestRepoAddFromURLUsesRepoStore(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "url-repo"))
	store := filepath.Join(runner.root, "repo-store")

	if _, err := runner.run("config", "set", "defaults.repo_store_root", store); err != nil {
		t.Fatalf("config set repo_store_root: %v", err)
	}
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}

	url := fileURL(source)
	if _, err := runner.run("repo", "add", "-w", "demo", url, "--name", "url-repo"); err != nil {
		t.Fatalf("repo add url: %v", err)
	}

	out, err := runner.run("repo", "ls", "-w", "demo", "--json")
	if err != nil {
		t.Fatalf("repo ls --json: %v", err)
	}
	if !strings.Contains(out, "\"managed\": true") {
		t.Fatalf("repo ls missing managed=true: %s", out)
	}
	if !strings.Contains(out, store) {
		t.Fatalf("repo ls missing repo_store_root path: %s", out)
	}

	if _, err := runner.run("repo", "rm", "-w", "demo", "url-repo", "--delete-local", "--yes"); err != nil {
		t.Fatalf("repo rm --delete-local: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store, "url-repo")); !os.IsNotExist(err) {
		t.Fatalf("expected repo store deleted, got err=%v", err)
	}
}

func TestRepoRegistryLocalAndURLFlow(t *testing.T) {
	runner := newRunner(t)
	localRepo := setupRepo(t, filepath.Join(runner.root, "src", "registry-local"))
	urlRepo := setupRepo(t, filepath.Join(runner.root, "src", "registry-url"))
	store := filepath.Join(runner.root, "repo-store")

	if _, err := runner.run("config", "set", "defaults.repo_store_root", store); err != nil {
		t.Fatalf("config set repo_store_root: %v", err)
	}
	if _, err := runner.run("repo", "registry", "add", "local-reg", localRepo); err != nil {
		t.Fatalf("registry add local: %v", err)
	}
	if _, err := runner.run("repo", "registry", "add", "url-reg", fileURL(urlRepo)); err != nil {
		t.Fatalf("registry add url: %v", err)
	}
	out, err := runner.run("repo", "registry", "ls", "--json")
	if err != nil {
		t.Fatalf("registry ls: %v", err)
	}
	if !strings.Contains(out, "\"name\": \"local-reg\"") || !strings.Contains(out, "\"name\": \"url-reg\"") {
		t.Fatalf("registry ls missing entries: %s", out)
	}
	if !strings.Contains(out, "\"default_branch\": \"main\"") {
		t.Fatalf("registry ls missing default_branch: %s", out)
	}

	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", "local-reg"); err != nil {
		t.Fatalf("repo add local registered repo: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", "url-reg"); err != nil {
		t.Fatalf("repo add url registered repo: %v", err)
	}

	out, err = runner.run("repo", "ls", "-w", "demo", "--json")
	if err != nil {
		t.Fatalf("repo ls: %v", err)
	}
	if !strings.Contains(out, localRepo) {
		t.Fatalf("repo ls missing local path: %s", out)
	}
	if !strings.Contains(out, filepath.Join(store, "url-reg")) {
		t.Fatalf("repo ls missing repo store path: %s", out)
	}

	if _, err := runner.run("repo", "registry", "set", "--default-branch", "main", "url-reg", fileURL(urlRepo)); err != nil {
		t.Fatalf("registry set default branch: %v", err)
	}
	if _, err := runner.run("repo", "registry", "rm", "local-reg"); err != nil {
		t.Fatalf("registry rm local: %v", err)
	}
	if _, err := runner.run("repo", "registry", "rm", "url-reg"); err != nil {
		t.Fatalf("registry rm url: %v", err)
	}
}

func TestRepoAddWithRepoDirCreatesWorktree(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "dir-repo"))

	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", source, "--repo-dir", "custom-dir"); err != nil {
		t.Fatalf("repo add --repo-dir: %v", err)
	}

	worktreePath := filepath.Join(runner.workspaceRoot(), "demo", "custom-dir")
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("expected worktree at custom dir: %v", err)
	}
}

func TestRepoRegistryDefaults(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "remotes-repo"))

	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "registry", "add", "remotes-repo", source, "--remote", "origin", "--default-branch", "trunk"); err != nil {
		t.Fatalf("repo registry add: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", "remotes-repo"); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	out, err := runner.run("repo", "ls", "-w", "demo", "--json")
	if err != nil {
		t.Fatalf("repo ls: %v", err)
	}
	if !strings.Contains(out, "\"remote\": \"origin\"") {
		t.Fatalf("repo ls missing remote: %s", out)
	}
	if !strings.Contains(out, "\"default_branch\": \"trunk\"") {
		t.Fatalf("repo ls missing default branch: %s", out)
	}
}

func TestRepoRemoveWorktreesSafety(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "dirty-repo"))

	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}

	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	worktreePath := filepath.Join(runner.workspaceRoot(), "demo", "dirty-repo")
	if err := os.WriteFile(filepath.Join(worktreePath, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty: %v", err)
	}

	if _, err := runner.run("repo", "rm", "-w", "demo", "--delete-worktrees", "--yes", "dirty-repo"); err == nil {
		t.Fatalf("expected repo rm to fail when dirty")
	}

	if _, err := runner.run("repo", "rm", "-w", "demo", "--delete-worktrees", "--yes", "--force", "dirty-repo"); err != nil {
		t.Fatalf("repo rm --force: %v", err)
	}

	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree deleted, got err=%v", err)
	}
}

func TestRepoListPlainOutput(t *testing.T) {
	runner := newRunner(t)
	source := setupRepo(t, filepath.Join(runner.root, "src", "repo-list-plain"))
	if _, err := runner.run("new", "demo"); err != nil {
		t.Fatalf("workset new: %v", err)
	}
	if _, err := runner.run("repo", "add", "-w", "demo", source); err != nil {
		t.Fatalf("repo add: %v", err)
	}
	out, err := runner.run("repo", "ls", "-w", "demo", "--plain")
	if err != nil {
		t.Fatalf("repo ls --plain: %v", err)
	}
	if !strings.Contains(out, "repo-list-plain") {
		t.Fatalf("repo ls plain missing repo: %s", out)
	}
}
