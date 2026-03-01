package worksetapi

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/workspace"
)

func TestLoadGlobalMigratesWorkspaceWorksetFromWorkspaceConfig(t *testing.T) {
	env := newTestEnv(t)
	root := env.createWorkspace(context.Background(), "demo")

	wsCfg, err := config.LoadWorkspace(workspace.WorksetFile(root))
	if err != nil {
		t.Fatalf("load workspace config: %v", err)
	}
	wsCfg.Repos = append(wsCfg.Repos, config.RepoConfig{Name: "repo-a", RepoDir: "repo-a"})
	if err := config.SaveWorkspace(workspace.WorksetFile(root), wsCfg); err != nil {
		t.Fatalf("save workspace config: %v", err)
	}

	cfg := env.loadConfig()
	ref := cfg.Workspaces["demo"]
	ref.Workset = ""
	ref.Template = ""
	cfg.Workspaces["demo"] = ref
	env.saveConfig(cfg)

	loaded, _, err := env.svc.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if loaded.Workspaces["demo"].Workset != "repo-a" {
		t.Fatalf("expected migrated workset repo-a, got %q", loaded.Workspaces["demo"].Workset)
	}

	cfg = env.loadConfig()
	if cfg.Workspaces["demo"].Workset != "repo-a" {
		t.Fatalf("expected persisted migrated workset repo-a, got %q", cfg.Workspaces["demo"].Workset)
	}
}

func TestLoadGlobalMigratesWorkspaceWorksetFromTemplate(t *testing.T) {
	env := newTestEnv(t)
	_ = env.createWorkspace(context.Background(), "demo")

	cfg := env.loadConfig()
	ref := cfg.Workspaces["demo"]
	ref.Workset = ""
	ref.Template = "Manual Template"
	cfg.Workspaces["demo"] = ref
	env.saveConfig(cfg)

	loaded, _, err := env.svc.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if loaded.Workspaces["demo"].Workset != "Manual Template" {
		t.Fatalf("expected migrated workset from template, got %q", loaded.Workspaces["demo"].Workset)
	}

	cfg = env.loadConfig()
	if cfg.Workspaces["demo"].Workset != "Manual Template" {
		t.Fatalf("expected persisted migrated workset from template, got %q", cfg.Workspaces["demo"].Workset)
	}
}

func TestLoadGlobalPersistsLegacyWorkspacesKeyAsWorksets(t *testing.T) {
	env := newTestEnv(t)
	legacy := `
defaults:
  remote: origin
  base_branch: main
  workspace_root: ` + env.workspaceRoot + `
  repo_store_root: ` + env.repoRoot + `
workspaces:
  demo:
    path: ` + env.workspaceRoot + `/demo
    workset: demo
groups:
  legacy-template:
    description: ""
    members:
      - repo: repo-a
`
	if err := os.MkdirAll(filepath.Join(env.workspaceRoot, "demo"), 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.WriteFile(env.configPath, []byte(strings.TrimSpace(legacy)+"\n"), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}
	loaded, _, err := env.svc.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if len(loaded.WorksetCatalog) != 1 {
		t.Fatalf("expected 1 workset catalog entry, got %d", len(loaded.WorksetCatalog))
	}
	entry := loaded.WorksetCatalog["demo"]
	if len(entry.Threads) != 1 || entry.Threads[0] != "demo" {
		t.Fatalf("expected catalog threads [demo], got %+v", entry.Threads)
	}
	data, err := os.ReadFile(env.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "worksets:") {
		t.Fatalf("expected migrated worksets key, got %q", text)
	}
	if strings.Contains(text, "\nworkspaces:") {
		t.Fatalf("did not expect legacy workspaces key after migration, got %q", text)
	}
	if !strings.Contains(text, "workset_catalog:") {
		t.Fatalf("expected workset_catalog key after migration, got %q", text)
	}
	if strings.Contains(text, "\ngroups:") {
		t.Fatalf("did not expect legacy groups key after migration, got %q", text)
	}

	loadedAgain, _, err := env.svc.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("get config second pass: %v", err)
	}
	if len(loadedAgain.WorksetCatalog) != len(loaded.WorksetCatalog) {
		t.Fatalf(
			"expected idempotent workset catalog on second migration run, first=%d second=%d",
			len(loaded.WorksetCatalog),
			len(loadedAgain.WorksetCatalog),
		)
	}
	dataAgain, err := os.ReadFile(env.configPath)
	if err != nil {
		t.Fatalf("read config second pass: %v", err)
	}
	if string(dataAgain) != text {
		t.Fatalf("expected config migration to be idempotent on second load")
	}
}

func TestGlobalConfigMigrationPlanHasOrderedRemovalMetadata(t *testing.T) {
	env := newTestEnv(t)
	plan := env.svc.globalConfigMigrationPlan(config.GlobalConfigLoadInfo{
		UsedLegacyWorkspacesKey: true,
	})
	if len(plan) == 0 {
		t.Fatalf("expected at least one global config migration")
	}
	ids := make([]string, 0, len(plan))
	for _, migration := range plan {
		ids = append(ids, migration.ID)
		if strings.TrimSpace(migration.Summary) == "" {
			t.Fatalf("expected migration %q to include summary", migration.ID)
		}
		if strings.TrimSpace(migration.RemoveAfter) == "" {
			t.Fatalf("expected migration %q to include remove_after guidance", migration.ID)
		}
	}
	expected := []string{
		migrationIDWorkspacesToWorksets,
		migrationIDGroupRemotesToAliases,
	}
	if !slices.Equal(ids, expected) {
		t.Fatalf("unexpected migration order, got %v want %v", ids, expected)
	}
}
