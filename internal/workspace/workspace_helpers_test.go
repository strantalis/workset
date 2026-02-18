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

func TestWorkspaceDirName(t *testing.T) {
	name := "fix/ws-test"
	dir := WorkspaceDirName(name)
	if dir != "fix__ws-test" {
		t.Fatalf("unexpected workspace dir name: %q", dir)
	}
}

func TestWorkspaceBranchNamePreservesValid(t *testing.T) {
	name := "fix/ws-test"
	if got := WorkspaceBranchName(name); got != name {
		t.Fatalf("expected branch %q, got %q", name, got)
	}
}

func TestWorkspaceBranchNameSanitizesInvalid(t *testing.T) {
	name := "my ws"
	got := WorkspaceBranchName(name)
	wantPrefix := "my-ws-"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("expected branch prefix %q, got %q", wantPrefix, got)
	}
	if !strings.HasSuffix(got, shortHash(name)) {
		t.Fatalf("expected branch suffix %q, got %q", shortHash(name), got)
	}
	if strings.Contains(got, " ") {
		t.Fatalf("expected sanitized branch without spaces, got %q", got)
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

func TestIsGitSafeBranchName(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{name: "valid simple", input: "feature/one", expect: true},
		{name: "valid underscore and dash", input: "fix/ws_test-123", expect: true},
		{name: "empty", input: "", expect: false},
		{name: "leading slash", input: "/feature", expect: false},
		{name: "double slash", input: "feature//one", expect: false},
		{name: "dot dot", input: "feature..one", expect: false},
		{name: "contains at brace", input: "feature@{1}", expect: false},
		{name: "part starts dot", input: "feature/.hidden", expect: false},
		{name: "part ends lock", input: "feature/topic.lock", expect: false},
		{name: "contains space", input: "feature one", expect: false},
		{name: "contains invalid rune", input: "feature:one", expect: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isGitSafeBranchName(tc.input)
			if got != tc.expect {
				t.Fatalf("isGitSafeBranchName(%q) = %v, want %v", tc.input, got, tc.expect)
			}
		})
	}
}
