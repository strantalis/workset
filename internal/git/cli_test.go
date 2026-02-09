package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsContentMergedSquashMerge(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := initGitRepo(t)
	ensureBranch(t, repo, "main")
	commitFile(t, repo, "file.txt", "one\n", "initial")

	runGit(t, repo, "checkout", "-b", "feature")
	commitFile(t, repo, "file.txt", "two\n", "feature change")

	runGit(t, repo, "checkout", "main")
	runGit(t, repo, "merge", "--squash", "feature")
	runGit(t, repo, "commit", "-m", "squash merge")

	client := NewCLIClient()
	merged, err := client.IsContentMerged(repo, "refs/heads/feature", "refs/heads/main")
	if err != nil {
		t.Fatalf("IsContentMerged: %v", err)
	}
	if !merged {
		t.Fatalf("expected content to be merged after squash merge")
	}
}

func TestIsContentMergedMergeCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := initGitRepo(t)
	ensureBranch(t, repo, "main")
	commitFile(t, repo, "file.txt", "one\n", "initial")

	runGit(t, repo, "checkout", "-b", "feature")
	commitFile(t, repo, "file.txt", "two\n", "feature change")

	runGit(t, repo, "checkout", "main")
	runGit(t, repo, "merge", "--no-ff", "-m", "merge feature", "feature")

	client := NewCLIClient()
	merged, err := client.IsContentMerged(repo, "refs/heads/feature", "refs/heads/main")
	if err != nil {
		t.Fatalf("IsContentMerged: %v", err)
	}
	if !merged {
		t.Fatalf("expected content to be merged after merge commit")
	}
}

func TestIsContentMergedRebaseMerge(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := initGitRepo(t)
	ensureBranch(t, repo, "main")
	commitFile(t, repo, "base.txt", "one\n", "initial")

	runGit(t, repo, "checkout", "-b", "feature")
	commitFile(t, repo, "feature.txt", "feature\n", "feature change")
	featureHead := gitRevParse(t, repo, "refs/heads/feature")

	runGit(t, repo, "checkout", "main")
	commitFile(t, repo, "base.txt", "two\n", "main moved")
	runGit(t, repo, "cherry-pick", featureHead)

	client := NewCLIClient()
	ancestor, err := client.IsAncestor(repo, "refs/heads/feature", "refs/heads/main")
	if err != nil {
		t.Fatalf("IsAncestor: %v", err)
	}
	if ancestor {
		t.Fatalf("expected feature not to be an ancestor after cherry-pick")
	}

	merged, err := client.IsContentMerged(repo, "refs/heads/feature", "refs/heads/main")
	if err != nil {
		t.Fatalf("IsContentMerged: %v", err)
	}
	if !merged {
		t.Fatalf("expected content to be merged after rebase/cherry-pick style merge")
	}
}

func TestIsContentMergedDetectsMissingChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := initGitRepo(t)
	ensureBranch(t, repo, "main")
	commitFile(t, repo, "file.txt", "one\n", "initial")

	runGit(t, repo, "checkout", "-b", "feature")
	commitFile(t, repo, "file.txt", "two\n", "feature change")

	runGit(t, repo, "checkout", "main")

	client := NewCLIClient()
	merged, err := client.IsContentMerged(repo, "refs/heads/feature", "refs/heads/main")
	if err != nil {
		t.Fatalf("IsContentMerged: %v", err)
	}
	if merged {
		t.Fatalf("expected content to be unmerged when changes are missing")
	}
}

func TestWorktreeRemoveRetriesWithForce(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := initGitRepo(t)
	ensureBranch(t, repo, "main")
	commitFile(t, repo, "file.txt", "one\n", "initial")

	worktreePath := filepath.Join(t.TempDir(), "wt")
	runGit(t, repo, "worktree", "add", "-b", "feature", worktreePath)

	if err := os.WriteFile(filepath.Join(worktreePath, "untracked.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write untracked: %v", err)
	}

	client := NewCLIClient()
	worktrees, err := client.WorktreeList(repo)
	if err != nil {
		t.Fatalf("WorktreeList: %v", err)
	}
	if len(worktrees) != 1 {
		t.Fatalf("expected one linked worktree, got %v", worktrees)
	}
	if err := client.WorktreeRemove(WorktreeRemoveOptions{
		RepoPath:     repo,
		WorktreeName: worktrees[0],
		Force:        false,
	}); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
}

func TestUpdateBranchBareRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	source := initGitRepo(t)
	ensureBranch(t, source, "main")

	writeFile(t, source, "file.txt", "one\n")
	runGit(t, source, "add", "file.txt")
	runGit(t, source, "-c", "commit.gpgsign=false", "commit", "-m", "initial")

	bareRoot := t.TempDir()
	bare := filepath.Join(bareRoot, "repo.git")
	runGit(t, bareRoot, "clone", "--bare", source, bare)
	runGit(t, bare, "config", "--replace-all", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")

	writeFile(t, source, "file.txt", "two\n")
	runGit(t, source, "add", "file.txt")
	runGit(t, source, "-c", "commit.gpgsign=false", "commit", "-m", "second")

	client := NewCLIClient()
	if err := client.Fetch(context.Background(), bare, "origin"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if err := client.UpdateBranch(context.Background(), bare, "main", "origin/main"); err != nil {
		t.Fatalf("UpdateBranch: %v", err)
	}

	mainRef := gitRevParse(t, bare, "refs/heads/main")
	originRef := gitRevParse(t, bare, "refs/remotes/origin/main")
	if mainRef != originRef {
		t.Fatalf("expected main to match origin/main, got %s vs %s", mainRef, originRef)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Workset Tests")
	return dir
}

func writeFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func ensureBranch(t *testing.T, dir, branch string) {
	t.Helper()
	if branch == "" {
		t.Fatal("branch required")
	}
	if err := runGitAllowError(dir, "checkout", "-b", branch); err != nil {
		runGit(t, dir, "checkout", branch)
	}
}

func commitFile(t *testing.T, dir, name, contents, message string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, dir, "add", name)
	runGit(t, dir, "commit", "-m", message)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if err := runGitAllowError(dir, args...); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func runGitAllowError(dir string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stdout = &stderr
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}

func gitRevParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", "rev-parse", ref)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git rev-parse %s: %v (%s)", ref, err, stderr.String())
	}
	return string(bytes.TrimSpace(stdout.Bytes()))
}

func TestRunForcesNonInteractiveGitEnv(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-git.sh")
	script := `#!/bin/sh
if [ "$GIT_TERMINAL_PROMPT" != "0" ]; then
  echo "GIT_TERMINAL_PROMPT=$GIT_TERMINAL_PROMPT" >&2
  exit 41
fi
if [ "$GCM_INTERACTIVE" != "Always" ]; then
  echo "GCM_INTERACTIVE=$GCM_INTERACTIVE" >&2
  exit 42
fi
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	t.Setenv("GCM_INTERACTIVE", "Always")

	client := CLIClient{gitPath: scriptPath}
	if _, err := client.run(context.Background(), "", "version"); err != nil {
		t.Fatalf("run: %v", err)
	}
}
