package worksetapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/strantalis/workset/internal/config"
)

func TestResolveWorkspaceTarget(t *testing.T) {
	temp := t.TempDir()
	cfg := config.GlobalConfig{
		Defaults: config.Defaults{
			WorkspaceRoot: temp,
		},
		Workspaces: map[string]config.WorkspaceRef{
			"alpha": {Path: filepath.Join(temp, "alpha")},
		},
	}
	if err := os.MkdirAll(filepath.Join(temp, "beta"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	name, path, err := resolveWorkspaceTarget("alpha", &cfg)
	if err != nil {
		t.Fatalf("resolve registered: %v", err)
	}
	if name != "alpha" || path != filepath.Join(temp, "alpha") {
		t.Fatalf("unexpected result: %s %s", name, path)
	}

	name, path, err = resolveWorkspaceTarget("beta", &cfg)
	if err != nil {
		t.Fatalf("resolve relative: %v", err)
	}
	if name != "beta" || path != filepath.Join(temp, "beta") {
		t.Fatalf("unexpected relative: %s %s", name, path)
	}

	abs := filepath.Join(temp, "gamma")
	if err := os.MkdirAll(abs, 0o755); err != nil {
		t.Fatalf("mkdir abs: %v", err)
	}
	name, path, err = resolveWorkspaceTarget(abs, &cfg)
	if err != nil {
		t.Fatalf("resolve abs: %v", err)
	}
	if name != "" || path != abs {
		t.Fatalf("unexpected abs: %s %s", name, path)
	}

	_, _, err = resolveWorkspaceTarget("missing", &cfg)
	_ = requireErrorType[NotFoundError](t, err)
}

func TestResolveWorkspaceSelectorDefault(t *testing.T) {
	cfg := config.GlobalConfig{
		Defaults: config.Defaults{Workspace: "alpha"},
		Workspaces: map[string]config.WorkspaceRef{
			"alpha": {Path: "/tmp/alpha"},
		},
	}
	name, path, err := resolveWorkspaceSelector(&cfg, WorkspaceSelector{})
	if err != nil {
		t.Fatalf("resolve default: %v", err)
	}
	if name != "alpha" || path != "/tmp/alpha" {
		t.Fatalf("unexpected: %s %s", name, path)
	}
}

func TestLooksLikeURLAndPath(t *testing.T) {
	if !looksLikeURL("https://example.com/repo.git") {
		t.Fatalf("expected url detection")
	}
	if !looksLikeURL("git@example.com:org/repo.git") {
		t.Fatalf("expected ssh-style url detection")
	}
	if looksLikeURL("repo") {
		t.Fatalf("unexpected url detection")
	}

	if !looksLikeLocalPath("/tmp/repo") {
		t.Fatalf("expected abs path detection")
	}
	if !looksLikeLocalPath("./repo") {
		t.Fatalf("expected relative path detection")
	}
	if !looksLikeLocalPath("~/repo") {
		t.Fatalf("expected tilde path detection")
	}
	if looksLikeLocalPath("repo") {
		t.Fatalf("unexpected local path detection")
	}
}

func TestResolveLocalPathInput(t *testing.T) {
	_, err := resolveLocalPathInput("")
	_ = requireErrorType[ValidationError](t, err)

	dir := t.TempDir()
	resolved, err := resolveLocalPathInput(dir)
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	expected, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("eval symlinks: %v", err)
	}
	if resolved != expected {
		t.Fatalf("unexpected resolved path: %s", resolved)
	}

	if _, err := resolveLocalPathInput("~"); err != nil {
		t.Fatalf("resolve home: %v", err)
	}
}

func TestWorksetFilePath(t *testing.T) {
	root := t.TempDir()
	path := worksetFilePath(root)
	if filepath.Base(path) != "workset.yaml" {
		t.Fatalf("unexpected workset file: %s", path)
	}
}

func TestRemoveWorkspaceByPath(t *testing.T) {
	cfg := config.GlobalConfig{
		Workspaces: map[string]config.WorkspaceRef{
			"demo": {Path: "/tmp/demo"},
			"alt":  {Path: "/tmp/alt"},
		},
	}
	removeWorkspaceByPath(&cfg, "/tmp/demo")
	if _, ok := cfg.Workspaces["demo"]; ok {
		t.Fatalf("expected demo removed")
	}
	if _, ok := cfg.Workspaces["alt"]; !ok {
		t.Fatalf("expected alt retained")
	}
}

func TestLoadGlobalUsesConfigPath(t *testing.T) {
	env := newTestEnv(t)
	cfg, info, err := env.svc.loadGlobal(context.Background())
	if err != nil {
		t.Fatalf("load global: %v", err)
	}
	if info.Path != env.configPath {
		t.Fatalf("unexpected config path: %s", info.Path)
	}
	if cfg.Defaults.WorkspaceRoot != env.workspaceRoot {
		t.Fatalf("unexpected defaults: %s", cfg.Defaults.WorkspaceRoot)
	}
}
