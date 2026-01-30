package ops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/workspace"
)

func TestAddRepoLinksLocal(t *testing.T) {
	source := setupRepo(t)
	addRemote(t, source, "origin", source)
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, resolvedRemote, warnings, err := AddRepo(context.Background(), AddRepoInput{
		WorkspaceRoot: root,
		Name:          "demo-repo",
		URL:           source,
		Defaults:      defaults,
		Remote:        defaults.Remote,
		DefaultBranch: defaults.BaseBranch,
		AllowFallback: false,
		Git:           git.NewCLIClient(),
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	if resolvedRemote != "origin" {
		t.Fatalf("expected resolved remote origin, got %q", resolvedRemote)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	ws, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if len(ws.Repos) != 1 {
		t.Fatalf("expected repo in workspace")
	}
	expectedPath, err := filepath.EvalSymlinks(source)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if ws.Repos[0].LocalPath != expectedPath {
		t.Fatalf("expected local_path %s, got %s", expectedPath, ws.Repos[0].LocalPath)
	}

	agentsContent, err := os.ReadFile(workspace.AgentsFile(root))
	if err != nil {
		t.Fatalf("agents file missing: %v", err)
	}
	if !strings.Contains(string(agentsContent), "Configured Repos (from workset.yaml)") {
		t.Fatalf("agents file missing configured repos section")
	}
	if !strings.Contains(string(agentsContent), "demo-repo") {
		t.Fatalf("agents file missing repo entry")
	}
}

func TestAddRepoMissingRemoteErrors(t *testing.T) {
	source := setupRepo(t)
	addRemote(t, source, "upstream", source)
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, _, _, err := AddRepo(context.Background(), AddRepoInput{
		WorkspaceRoot: root,
		Name:          "demo-repo",
		URL:           source,
		Defaults:      defaults,
		Remote:        defaults.Remote,
		DefaultBranch: defaults.BaseBranch,
		AllowFallback: false,
		Git:           git.NewCLIClient(),
	})
	if err == nil {
		t.Fatalf("expected missing remote error")
	}
}

func TestStatusDirty(t *testing.T) {
	source := setupRepo(t)
	addRemote(t, source, "origin", source)
	root := filepath.Join(t.TempDir(), "ws")
	defaults := config.DefaultConfig().Defaults

	if _, err := workspace.Init(root, "demo", defaults); err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, _, _, err := AddRepo(context.Background(), AddRepoInput{
		WorkspaceRoot: root,
		Name:          "demo-repo",
		URL:           source,
		Defaults:      defaults,
		Remote:        defaults.Remote,
		DefaultBranch: defaults.BaseBranch,
		AllowFallback: false,
		Git:           git.NewCLIClient(),
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	worktreePath := workspace.RepoWorktreePath(root, "demo", "demo-repo")
	if err := os.WriteFile(filepath.Join(worktreePath, "extra.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	statuses, err := Status(context.Background(), StatusInput{
		WorkspaceRoot:       root,
		Defaults:            defaults,
		RepoDefaultBranches: map[string]string{"demo-repo": defaults.BaseBranch},
		Git:                 git.NewCLIClient(),
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
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.name", "Tester")
	runGit(t, root, "config", "user.email", "tester@example.com")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "initial")
	return root
}

func addRemote(t *testing.T, repoPath, name, url string) {
	t.Helper()
	runGit(t, repoPath, "remote", "add", name, url)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
}
