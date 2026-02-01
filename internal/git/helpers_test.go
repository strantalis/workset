package git

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseGitDirFileRelative(t *testing.T) {
	root := t.TempDir()
	worktreeDir := filepath.Join(root, "wt")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	repoGitDir := filepath.Join(root, "repo", ".git", "worktrees", "wt")
	if err := os.MkdirAll(repoGitDir, 0o755); err != nil {
		t.Fatalf("mkdir repo gitdir: %v", err)
	}
	gitFile := filepath.Join(worktreeDir, ".git")
	rel := filepath.Join("..", "repo", ".git", "worktrees", "wt")
	if err := os.WriteFile(gitFile, []byte("gitdir: "+rel), 0o644); err != nil {
		t.Fatalf("write git file: %v", err)
	}
	parsed, ok, err := parseGitDirFile(gitFile)
	if err != nil {
		t.Fatalf("parseGitDirFile: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if parsed != repoGitDir {
		t.Fatalf("expected %s, got %s", repoGitDir, parsed)
	}
}

func TestIsGitDirAndWorktreeAdminDir(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir gitdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0o644); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("[core]"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if !isGitDir(gitDir) {
		t.Fatalf("expected git dir")
	}
	if isWorktreeAdminDir(gitDir) {
		t.Fatalf("expected not worktree admin dir")
	}
	if err := os.WriteFile(filepath.Join(gitDir, "gitdir"), []byte("/tmp/gitdir"), 0o644); err != nil {
		t.Fatalf("write gitdir: %v", err)
	}
	if !isWorktreeAdminDir(gitDir) {
		t.Fatalf("expected worktree admin dir")
	}
}

func TestWorktreeRootFromGitDir(t *testing.T) {
	root := t.TempDir()
	worktreeRoot := filepath.Join(root, "wt")
	adminDir := filepath.Join(root, "repo", ".git", "worktrees", "wt")
	if err := os.MkdirAll(adminDir, 0o755); err != nil {
		t.Fatalf("mkdir admin: %v", err)
	}
	if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adminDir, "gitdir"), []byte("gitdir: "+filepath.Join(worktreeRoot, ".git")), 0o644); err != nil {
		t.Fatalf("write gitdir: %v", err)
	}
	rootPath, ok, err := worktreeRootFromGitDir(adminDir)
	if err != nil {
		t.Fatalf("worktreeRootFromGitDir: %v", err)
	}
	if !ok || rootPath != worktreeRoot {
		t.Fatalf("expected %s, got %s ok=%t", worktreeRoot, rootPath, ok)
	}

	repoRoot := filepath.Join(root, "repo2")
	repoGit := filepath.Join(repoRoot, ".git")
	if err := os.MkdirAll(repoGit, 0o755); err != nil {
		t.Fatalf("mkdir repo git: %v", err)
	}
	rootPath, ok, err = worktreeRootFromGitDir(repoGit)
	if err != nil {
		t.Fatalf("worktreeRootFromGitDir repo: %v", err)
	}
	if !ok || rootPath != repoRoot {
		t.Fatalf("expected %s, got %s ok=%t", repoRoot, rootPath, ok)
	}
}

func TestIsMissingRefAndNotRepo(t *testing.T) {
	if !isMissingRef("fatal: ambiguous argument 'foo': unknown revision") {
		t.Fatalf("expected missing ref")
	}
	if isMissingRef("ok") {
		t.Fatalf("did not expect missing ref")
	}
	if !isNotRepo(gitResult{exitCode: 128, stderr: "not a git repository"}) {
		t.Fatalf("expected not repo")
	}
	if isNotRepo(gitResult{exitCode: 0, stderr: ""}) {
		t.Fatalf("did not expect not repo")
	}
}

func TestSamePathSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior varies on windows")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if !samePath(target, link) {
		t.Fatalf("expected same path for %s and %s", target, link)
	}
	if samePath(target, filepath.Join(root, "other")) {
		t.Fatalf("unexpected same path")
	}
	if samePath(target, strings.ToUpper(target)) && runtime.GOOS != "darwin" {
		t.Fatalf("unexpected case-insensitive match")
	}
}
