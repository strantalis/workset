package workspace

import (
	"os"
	"strings"
	"testing"
)

func TestWorktreeDirNameAndBranchNameRoundTrip(t *testing.T) {
	branch := "feature/one"
	dir := WorktreeDirName(branch)
	if dir != "feature__one" {
		t.Fatalf("unexpected dir name: %q", dir)
	}
	got := BranchNameFromDir(dir)
	if got != branch {
		t.Fatalf("unexpected branch name: got %q want %q", got, branch)
	}
	if BranchNameFromDir("") != "" {
		t.Fatalf("expected empty branch name for empty dir")
	}
}

func TestWorktreeName(t *testing.T) {
	name := WorktreeName("feature/one")
	if !strings.HasPrefix(name, "feature-one-") {
		t.Fatalf("unexpected worktree name: %q", name)
	}
	if len(name) != len("feature-one-")+8 {
		t.Fatalf("expected worktree name with 8-char hash, got %q", name)
	}
	empty := WorktreeName("")
	if !strings.HasPrefix(empty, "branch-") {
		t.Fatalf("expected branch- prefix for empty name, got %q", empty)
	}
}

func TestUseBranchDirs(t *testing.T) {
	root := t.TempDir()
	if UseBranchDirs(root) {
		t.Fatal("expected UseBranchDirs to be false without worktrees dir")
	}
	if err := os.MkdirAll(WorktreesPath(root), 0o755); err != nil {
		t.Fatalf("mkdir worktrees: %v", err)
	}
	if !UseBranchDirs(root) {
		t.Fatal("expected UseBranchDirs to be true with worktrees dir")
	}
}

func TestWriteAndReadBranchMeta(t *testing.T) {
	root := t.TempDir()
	branch := "feature/one"
	dir := WorktreeBranchPath(root, branch)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir branch dir: %v", err)
	}
	if err := WriteBranchMeta(root, branch); err != nil {
		t.Fatalf("WriteBranchMeta: %v", err)
	}
	name, ok, err := ReadBranchMeta(dir)
	if err != nil {
		t.Fatalf("ReadBranchMeta: %v", err)
	}
	if !ok || name != branch {
		t.Fatalf("unexpected branch meta: ok=%v name=%q", ok, name)
	}
}

func TestReadBranchMetaMissing(t *testing.T) {
	dir := t.TempDir()
	name, ok, err := ReadBranchMeta(dir)
	if err != nil {
		t.Fatalf("ReadBranchMeta: %v", err)
	}
	if ok || name != "" {
		t.Fatalf("expected missing meta to return ok=false, got ok=%v name=%q", ok, name)
	}
}

func TestSanitizeWorktreeName(t *testing.T) {
	got := sanitizeWorktreeName("feat/one  two***")
	if got != "feat-one-two" {
		t.Fatalf("unexpected sanitized name: %q", got)
	}
}

func TestShortHash(t *testing.T) {
	if shortHash("demo") != "89e495e7" {
		t.Fatalf("unexpected short hash for demo: %q", shortHash("demo"))
	}
}
