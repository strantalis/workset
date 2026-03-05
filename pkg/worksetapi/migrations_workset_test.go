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
	if loaded.ConfigVersion != config.CurrentGlobalConfigVersion {
		t.Fatalf("expected config_version %d after migration, got %d", config.CurrentGlobalConfigVersion, loaded.ConfigVersion)
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
	if loaded.ConfigVersion != config.CurrentGlobalConfigVersion {
		t.Fatalf("expected config_version %d after migration, got %d", config.CurrentGlobalConfigVersion, loaded.ConfigVersion)
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
	if _, _, err := env.svc.GetConfig(context.Background()); err != nil {
		t.Fatalf("get config: %v", err)
	}
	data, err := os.ReadFile(env.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "worksets:") {
		t.Fatalf("expected migrated worksets key, got %q", text)
	}
	if !strings.Contains(text, "config_version: 1") {
		t.Fatalf("expected config_version key after migration, got %q", text)
	}
	if strings.Contains(text, "\nworkspaces:") {
		t.Fatalf("did not expect legacy workspaces key after migration, got %q", text)
	}
	if strings.Contains(text, "workset_catalog:") {
		t.Fatalf("did not expect workset_catalog key after migration, got %q", text)
	}
	if strings.Contains(text, "\ngroups:") {
		t.Fatalf("did not expect legacy groups key after migration, got %q", text)
	}

	if _, _, err := env.svc.GetConfig(context.Background()); err != nil {
		t.Fatalf("get config second pass: %v", err)
	}
	dataAgain, err := os.ReadFile(env.configPath)
	if err != nil {
		t.Fatalf("read config second pass: %v", err)
	}
	if string(dataAgain) != text {
		t.Fatalf("expected config migration to be idempotent on second load")
	}
}

func TestLoadGlobalRewritesFlatWorksetsShapeToNestedThreads(t *testing.T) {
	env := newTestEnv(t)
	flat := `
config_version: 1
defaults:
  remote: origin
  base_branch: main
  workspace_root: ` + env.workspaceRoot + `
  repo_store_root: ` + env.repoRoot + `
worksets:
  demo:
    path: ` + env.workspaceRoot + `/demo
    workset: core
`
	if err := os.MkdirAll(filepath.Join(env.workspaceRoot, "demo"), 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.WriteFile(env.configPath, []byte(strings.TrimSpace(flat)+"\n"), 0o644); err != nil {
		t.Fatalf("write flat config: %v", err)
	}

	loaded, _, err := env.svc.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	ref := loaded.Workspaces["demo"]
	if ref.Workset != "core" {
		t.Fatalf("expected workset core, got %q", ref.Workset)
	}

	data, err := os.ReadFile(env.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "threads:") {
		t.Fatalf("expected nested threads shape after rewrite, got %q", text)
	}
	if strings.Contains(text, "workset_catalog:") {
		t.Fatalf("did not expect workset_catalog key after rewrite, got %q", text)
	}
}

func TestLoadGlobalRewritesLegacyWorksetCatalogKey(t *testing.T) {
	env := newTestEnv(t)
	legacy := `
config_version: 1
defaults:
  remote: origin
  base_branch: main
  workspace_root: ` + env.workspaceRoot + `
  repo_store_root: ` + env.repoRoot + `
workset_catalog:
  core:
    repos:
      - platform
worksets:
  core:
    threads:
      demo:
        path: ` + env.workspaceRoot + `/demo
        workset: core
`
	if err := os.MkdirAll(filepath.Join(env.workspaceRoot, "demo"), 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.WriteFile(env.configPath, []byte(strings.TrimSpace(legacy)+"\n"), 0o644); err != nil {
		t.Fatalf("write legacy catalog config: %v", err)
	}

	if _, _, err := env.svc.GetConfig(context.Background()); err != nil {
		t.Fatalf("get config: %v", err)
	}

	data, err := os.ReadFile(env.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "workset_catalog:") {
		t.Fatalf("did not expect workset_catalog key after rewrite, got %q", text)
	}
	if !strings.Contains(text, "threads:") {
		t.Fatalf("expected canonical nested threads shape after rewrite, got %q", text)
	}
}

func TestLoadGlobalMigratesWorksetReposAndThreadOverrides(t *testing.T) {
	env := newTestEnv(t)
	alphaRoot := env.createWorkspace(context.Background(), "alpha")
	betaRoot := env.createWorkspace(context.Background(), "beta")

	alphaCfg, err := config.LoadWorkspace(workspace.WorksetFile(alphaRoot))
	if err != nil {
		t.Fatalf("load alpha workspace config: %v", err)
	}
	alphaCfg.Repos = []config.RepoConfig{
		{Name: "platform", RepoDir: "platform"},
		{Name: "workset", RepoDir: "workset"},
	}
	if err := config.SaveWorkspace(workspace.WorksetFile(alphaRoot), alphaCfg); err != nil {
		t.Fatalf("save alpha workspace config: %v", err)
	}

	betaCfg, err := config.LoadWorkspace(workspace.WorksetFile(betaRoot))
	if err != nil {
		t.Fatalf("load beta workspace config: %v", err)
	}
	betaCfg.Repos = []config.RepoConfig{
		{Name: "platform", RepoDir: "platform"},
		{Name: "workset", RepoDir: "workset"},
		{Name: "gh-action-tests", RepoDir: "gh-action-tests"},
	}
	if err := config.SaveWorkspace(workspace.WorksetFile(betaRoot), betaCfg); err != nil {
		t.Fatalf("save beta workspace config: %v", err)
	}

	cfg := env.loadConfig()
	refAlpha := cfg.Workspaces["alpha"]
	refAlpha.Workset = "platform + workset"
	refAlpha.Template = ""
	cfg.Workspaces["alpha"] = refAlpha
	refBeta := cfg.Workspaces["beta"]
	refBeta.Workset = "platform + workset"
	refBeta.Template = ""
	cfg.Workspaces["beta"] = refBeta
	cfg.WorksetRepos = map[string][]string{}
	env.saveConfig(cfg)

	loaded, _, err := env.svc.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("get config: %v", err)
	}

	base := loaded.WorksetRepos["platform + workset"]
	if len(base) != 2 || base[0] != "platform" || base[1] != "workset" {
		t.Fatalf("expected base repos [platform workset], got %#v", base)
	}
	if got := loaded.Workspaces["alpha"].RepoOverrides; len(got) != 0 {
		t.Fatalf("expected no overrides for alpha, got %#v", got)
	}
	betaOverrides := loaded.Workspaces["beta"].RepoOverrides
	if len(betaOverrides) != 1 || betaOverrides[0] != "gh-action-tests" {
		t.Fatalf("expected beta override [gh-action-tests], got %#v", betaOverrides)
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
		if migration.TargetVersion != config.CurrentGlobalConfigVersion {
			t.Fatalf(
				"expected migration %q target_version=%d, got %d",
				migration.ID,
				config.CurrentGlobalConfigVersion,
				migration.TargetVersion,
			)
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
