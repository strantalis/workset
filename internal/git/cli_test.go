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

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Workset Tests")
	return dir
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
