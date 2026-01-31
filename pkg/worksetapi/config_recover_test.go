package worksetapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/workspace"
)

func TestRecoverConfigAddsWorkspaces(t *testing.T) {
	env := newTestEnv(t)
	root := filepath.Join(env.workspaceRoot, "demo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir demo: %v", err)
	}
	wsCfg := config.WorkspaceConfig{Name: "demo"}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace: %v", err)
	}

	result, err := env.svc.RecoverConfig(context.Background(), ConfigRecoverInput{})
	if err != nil {
		t.Fatalf("recover config: %v", err)
	}
	if len(result.Payload.WorkspacesRecovered) != 1 || result.Payload.WorkspacesRecovered[0] != "demo" {
		t.Fatalf("unexpected recovered workspaces: %+v", result.Payload.WorkspacesRecovered)
	}

	cfg := env.loadConfig()
	if _, ok := cfg.Workspaces["demo"]; !ok {
		t.Fatalf("expected recovered workspace to be registered")
	}
}

func TestRecoverConfigUpdatesEmptyWorkspacePath(t *testing.T) {
	env := newTestEnv(t)
	root := filepath.Join(env.workspaceRoot, "demo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir demo: %v", err)
	}
	wsCfg := config.WorkspaceConfig{Name: "demo"}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace: %v", err)
	}

	cfg := env.loadConfig()
	cfg.Workspaces["demo"] = config.WorkspaceRef{}
	env.saveConfig(cfg)

	result, err := env.svc.RecoverConfig(context.Background(), ConfigRecoverInput{})
	if err != nil {
		t.Fatalf("recover config: %v", err)
	}
	if len(result.Payload.WorkspacesRecovered) != 1 || result.Payload.WorkspacesRecovered[0] != "demo" {
		t.Fatalf("unexpected recovered workspaces: %+v", result.Payload.WorkspacesRecovered)
	}
	cfg = env.loadConfig()
	if cfg.Workspaces["demo"].Path == "" {
		t.Fatalf("expected workspace path to be set")
	}
}

func TestRecoverConfigRebuildsRepoAliases(t *testing.T) {
	env := newTestEnv(t)
	root := filepath.Join(env.workspaceRoot, "demo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir demo: %v", err)
	}
	wsCfg := config.WorkspaceConfig{
		Name: "demo",
		Repos: []config.RepoConfig{
			{Name: "repo-a", LocalPath: filepath.Join(env.root, "repos", "repo-a")},
		},
	}
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace: %v", err)
	}

	env.git.remotes[wsCfg.Repos[0].LocalPath] = []string{"origin"}
	env.git.remoteURLs[wsCfg.Repos[0].LocalPath] = map[string][]string{
		"origin": {"https://example.com/repo-a.git"},
	}
	env.git.currentBranch[wsCfg.Repos[0].LocalPath] = "main"
	env.git.currentOK[wsCfg.Repos[0].LocalPath] = true

	_, err := env.svc.RecoverConfig(context.Background(), ConfigRecoverInput{RebuildRepos: true})
	if err != nil {
		t.Fatalf("recover config: %v", err)
	}
	cfg := env.loadConfig()
	alias, ok := cfg.Repos["repo-a"]
	if !ok {
		t.Fatalf("expected repo alias recovered")
	}
	if alias.Path == "" {
		t.Fatalf("expected repo alias path set")
	}
	if alias.URL != "https://example.com/repo-a.git" {
		t.Fatalf("expected repo alias URL, got %q", alias.URL)
	}
	if alias.Remote != "origin" {
		t.Fatalf("expected repo alias remote, got %q", alias.Remote)
	}
	if alias.DefaultBranch != "main" {
		t.Fatalf("expected repo alias default branch, got %q", alias.DefaultBranch)
	}
}
