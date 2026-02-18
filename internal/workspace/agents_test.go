package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strantalis/workset/internal/config"
)

func TestFormatGeneratedSection(t *testing.T) {
	section := formatGeneratedSection("hello\nworld")
	if !strings.HasPrefix(section, agentsGeneratedStart+"\n") {
		t.Fatalf("expected section to start with generated marker, got %q", section)
	}
	if !strings.Contains(section, "hello\nworld") {
		t.Fatalf("expected section to include payload, got %q", section)
	}
	if !strings.HasSuffix(section, agentsGeneratedEnd+"\n") {
		t.Fatalf("expected section to end with generated marker, got %q", section)
	}
}

func TestReplaceGeneratedSection(t *testing.T) {
	content := "before\n" + formatGeneratedSection("old") + "after\n"
	section := formatGeneratedSection("new")
	updated, ok := replaceGeneratedSection(content, section)
	if !ok {
		t.Fatal("expected replaceGeneratedSection to succeed")
	}
	if strings.Contains(updated, "old") || !strings.Contains(updated, "new") {
		t.Fatalf("expected section replacement, got %q", updated)
	}
}

func TestReplaceGeneratedSectionMissing(t *testing.T) {
	content := "no markers here"
	section := formatGeneratedSection("new")
	updated, ok := replaceGeneratedSection(content, section)
	if ok {
		t.Fatal("expected replaceGeneratedSection to fail without markers")
	}
	if updated != content {
		t.Fatalf("expected content unchanged, got %q", updated)
	}
}

func TestAppendGeneratedSection(t *testing.T) {
	content := "hello\n"
	section := formatGeneratedSection("gen")
	got := appendGeneratedSection(content, section)
	want := "hello\n\n" + section
	if got != want {
		t.Fatalf("unexpected append output: got %q want %q", got, want)
	}
}

func TestBuildTopLevelTree(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "adir"), 0o755); err != nil {
		t.Fatalf("mkdir adir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	tree, err := buildTopLevelTree(root)
	if err != nil {
		t.Fatalf("buildTopLevelTree: %v", err)
	}
	lines := strings.Split(strings.TrimRight(tree, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), tree)
	}
	if lines[0] != "." || lines[1] != "├── adir/" || lines[2] != "└── b.txt" {
		t.Fatalf("unexpected tree output: %q", tree)
	}
}

func TestBuildAgentsGeneratedBlockNoRepos(t *testing.T) {
	root := t.TempDir()
	block, err := buildAgentsGeneratedBlock(root, config.WorkspaceConfig{}, State{CurrentBranch: "main"})
	if err != nil {
		t.Fatalf("buildAgentsGeneratedBlock: %v", err)
	}
	if !strings.Contains(block, "Workspace Layout (generated)") {
		t.Fatalf("missing layout section: %q", block)
	}
	if !strings.Contains(block, "- (none configured)") {
		t.Fatalf("missing no-repos marker: %q", block)
	}
}

func TestBuildAgentsGeneratedBlockSortedRepos(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(WorktreesPath(root), 0o755); err != nil {
		t.Fatalf("mkdir worktrees: %v", err)
	}
	cfg := config.WorkspaceConfig{
		Repos: []config.RepoConfig{
			{Name: "zeta", RepoDir: "zdir", LocalPath: "/tmp/z"},
			{Name: "alpha"},
		},
	}
	state := State{CurrentBranch: "main"}
	block, err := buildAgentsGeneratedBlock(root, cfg, state)
	if err != nil {
		t.Fatalf("buildAgentsGeneratedBlock: %v", err)
	}
	if strings.Index(block, "name: alpha") > strings.Index(block, "name: zeta") {
		t.Fatalf("expected repos sorted by name, got %q", block)
	}
	if !strings.Contains(block, "repo_dir: alpha") {
		t.Fatalf("expected default repo_dir to use name, got %q", block)
	}
	normalized := strings.ReplaceAll(block, string(os.PathSeparator), "/")
	expectedRel := filepath.ToSlash(filepath.Join("worktrees", WorktreeDirName(state.CurrentBranch), "alpha"))
	if !strings.Contains(normalized, expectedRel) {
		t.Fatalf("expected worktree path %q in %q", expectedRel, normalized)
	}
	if !strings.Contains(block, "local_path: /tmp/z") {
		t.Fatalf("expected local_path to render, got %q", block)
	}
}

func TestUpdateAgentsFileRequiresRoot(t *testing.T) {
	err := UpdateAgentsFile("", config.WorkspaceConfig{}, State{})
	if err == nil {
		t.Fatal("expected error when root is empty")
	}
}

func TestUpdateAgentsFileCreatesFile(t *testing.T) {
	root := t.TempDir()
	state := State{CurrentBranch: "main"}
	if err := UpdateAgentsFile(root, config.WorkspaceConfig{}, state); err != nil {
		t.Fatalf("UpdateAgentsFile: %v", err)
	}
	content, err := os.ReadFile(AgentsFile(root))
	if err != nil {
		t.Fatalf("read agents file: %v", err)
	}
	if !strings.Contains(string(content), agentsGeneratedStart) {
		t.Fatalf("missing generated section: %q", string(content))
	}
	claudeContent, err := os.ReadFile(ClaudeFile(root))
	if err != nil {
		t.Fatalf("read claude file: %v", err)
	}
	if string(claudeContent) != string(content) {
		t.Fatalf("expected CLAUDE.md to mirror AGENTS.md")
	}
}
