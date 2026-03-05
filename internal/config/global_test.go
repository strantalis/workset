package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestLoadGlobalDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg, err := LoadGlobal(path)
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}

	if cfg.Defaults.BaseBranch != "main" {
		t.Fatalf("expected default base_branch main, got %q", cfg.Defaults.BaseBranch)
	}
	if cfg.Defaults.Remote != "origin" {
		t.Fatalf("expected default remote origin, got %q", cfg.Defaults.Remote)
	}
	if cfg.Defaults.SessionBackend == "" {
		t.Fatalf("expected default session_backend set")
	}
	if cfg.Defaults.SessionNameFormat == "" {
		t.Fatalf("expected default session_name_format set")
	}
	if cfg.Defaults.TerminalIdleTimeout == "" {
		t.Fatalf("expected default terminal_idle_timeout set")
	}
	if cfg.Defaults.TerminalProtocolLog == "" {
		t.Fatalf("expected default terminal_protocol_log set")
	}
	if cfg.Defaults.TerminalDebugOverlay == "" {
		t.Fatalf("expected default terminal_debug_overlay set")
	}
	if cfg.ConfigVersion != CurrentGlobalConfigVersion {
		t.Fatalf("expected default config_version %d, got %d", CurrentGlobalConfigVersion, cfg.ConfigVersion)
	}
}

func TestSaveLoadGlobal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := DefaultConfig()
	cfg.Workspaces["alpha"] = WorkspaceRef{
		Path:           "/tmp/alpha",
		ArchivedAt:     "2024-01-02T03:04:05Z",
		ArchivedReason: "paused",
	}

	if err := SaveGlobal(path, cfg); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}

	loaded, err := LoadGlobal(path)
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}

	if loaded.Workspaces["alpha"].Path != "/tmp/alpha" {
		t.Fatalf("expected workspace path, got %+v", loaded.Workspaces["alpha"])
	}
	if loaded.Workspaces["alpha"].ArchivedAt != "2024-01-02T03:04:05Z" {
		t.Fatalf("expected archived_at preserved, got %+v", loaded.Workspaces["alpha"])
	}
	if loaded.Workspaces["alpha"].ArchivedReason != "paused" {
		t.Fatalf("expected archived_reason preserved, got %+v", loaded.Workspaces["alpha"])
	}
}

func TestLoadGlobalReadsLegacyWorkspacesKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	legacy := `
defaults:
  base_branch: main
workspaces:
  alpha:
    path: /tmp/alpha
`
	if err := os.WriteFile(path, []byte(strings.TrimSpace(legacy)+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, info, err := LoadGlobalWithInfo(path)
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if !info.UsedLegacyWorkspacesKey {
		t.Fatalf("expected legacy workspaces key to be detected")
	}
	if info.ConfigVersionPresent {
		t.Fatalf("expected config_version to be absent for legacy file")
	}
	if info.ConfigVersion != LegacyGlobalConfigVersion {
		t.Fatalf("expected legacy config_version %d, got %d", LegacyGlobalConfigVersion, info.ConfigVersion)
	}
	if cfg.ConfigVersion != LegacyGlobalConfigVersion {
		t.Fatalf("expected loaded config_version %d, got %d", LegacyGlobalConfigVersion, cfg.ConfigVersion)
	}
	if cfg.Workspaces["alpha"].Path != "/tmp/alpha" {
		t.Fatalf("expected legacy workspace migrated into worksets map, got %+v", cfg.Workspaces["alpha"])
	}
}

func TestSaveGlobalWritesWorksetsKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := DefaultConfig()
	cfg.Workspaces["alpha"] = WorkspaceRef{Path: "/tmp/alpha"}

	if err := SaveGlobal(path, cfg); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "config_version: 1") {
		t.Fatalf("expected config_version key, got %q", text)
	}
	if !strings.Contains(text, "worksets:") {
		t.Fatalf("expected worksets key, got %q", text)
	}
	if strings.Contains(text, "\nworkspaces:") {
		t.Fatalf("did not expect legacy workspaces key, got %q", text)
	}
}

func TestSaveGlobalMigratesTemplateToWorkset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := DefaultConfig()
	cfg.Workspaces["alpha"] = WorkspaceRef{
		Path:     "/tmp/alpha",
		Template: "legacy-template",
	}

	if err := SaveGlobal(path, cfg); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}
	loaded, err := LoadGlobal(path)
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	ref := loaded.Workspaces["alpha"]
	if ref.Workset != "legacy-template" {
		t.Fatalf("expected workset from legacy template, got %q", ref.Workset)
	}
	if ref.Template != "" {
		t.Fatalf("expected legacy template stripped, got %q", ref.Template)
	}
}

func TestSaveGlobalCreatesBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := "defaults:\n  base_branch: legacy\nrepos:\n  demo:\n    url: https://example.com/demo.git\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Defaults.BaseBranch = "main"
	if err := SaveGlobal(path, cfg); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}

	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !strings.Contains(string(backup), "base_branch: legacy") {
		t.Fatalf("expected backup to contain legacy base_branch, got %q", string(backup))
	}
	if !strings.Contains(string(backup), "demo.git") {
		t.Fatalf("expected backup to contain repo alias, got %q", string(backup))
	}
}

func TestUpdateGlobalPreservesFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions not reliable on windows")
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("defaults:\n  base_branch: main\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := UpdateGlobal(path, func(cfg *GlobalConfig, info GlobalConfigLoadInfo) error {
		cfg.Defaults.BaseBranch = "dev"
		return nil
	}); err != nil {
		t.Fatalf("UpdateGlobal: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600, got %04o", info.Mode().Perm())
	}
}

func TestUpdateGlobalPreservesConcurrentUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := SaveGlobal(path, DefaultConfig()); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}

	const iterations = 25
	for i := 0; i < iterations; i++ {
		repoName := fmt.Sprintf("repo-%d", i)
		workspaceName := fmt.Sprintf("ws-%d", i)

		var wg sync.WaitGroup
		wg.Add(2)
		errCh := make(chan error, 2)

		go func() {
			defer wg.Done()
			_, err := UpdateGlobal(path, func(cfg *GlobalConfig, info GlobalConfigLoadInfo) error {
				cfg.Repos[repoName] = RegisteredRepo{
					URL: fmt.Sprintf("https://example.com/%s.git", repoName),
				}
				return nil
			})
			errCh <- err
		}()

		go func() {
			defer wg.Done()
			_, err := UpdateGlobal(path, func(cfg *GlobalConfig, info GlobalConfigLoadInfo) error {
				cfg.Workspaces[workspaceName] = WorkspaceRef{
					Path: filepath.Join("/tmp", workspaceName),
				}
				return nil
			})
			errCh <- err
		}()

		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Fatalf("UpdateGlobal: %v", err)
			}
		}
	}

	loaded, err := LoadGlobal(path)
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	for i := 0; i < iterations; i++ {
		repoName := fmt.Sprintf("repo-%d", i)
		if _, ok := loaded.Repos[repoName]; !ok {
			t.Fatalf("expected repo %s present", repoName)
		}
		workspaceName := fmt.Sprintf("ws-%d", i)
		if _, ok := loaded.Workspaces[workspaceName]; !ok {
			t.Fatalf("expected workspace %s present", workspaceName)
		}
	}
}

func TestLoadGlobalMigratesLegacyConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacyPaths, err := legacyGlobalConfigPaths()
	if err != nil || len(legacyPaths) == 0 {
		t.Fatalf("legacyGlobalConfigPaths: %v", err)
	}
	legacyPath := legacyPaths[0]
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("defaults:\n  base_branch: legacy\n"), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	cfg, err := LoadGlobal("")
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if cfg.Defaults.BaseBranch != "legacy" {
		t.Fatalf("expected migrated base_branch legacy, got %q", cfg.Defaults.BaseBranch)
	}

	newPath := filepath.Join(home, ".workset", "config.yaml")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("expected migrated config at %s: %v", newPath, err)
	}
}

func TestLoadGlobalRejectsUnsupportedFutureConfigVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := `
config_version: 999
defaults:
  base_branch: main
`
	if err := os.WriteFile(path, []byte(strings.TrimSpace(raw)+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, _, err := LoadGlobalWithInfo(path); err == nil || !strings.Contains(err.Error(), "unsupported config_version") {
		t.Fatalf("expected unsupported config_version error, got %v", err)
	}
}

func TestSaveGlobalPersistsNestedWorksetThreadsAndDropsCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := DefaultConfig()
	cfg.Repos["platform"] = RegisteredRepo{
		URL:           "git@github.com:org/platform.git",
		Remote:        "origin",
		DefaultBranch: "main",
	}
	cfg.Workspaces["feature-policy-eval"] = WorkspaceRef{
		Path:      "/tmp/worksets/core/feature-policy-eval",
		Workset:   "core",
		CreatedAt: "2026-03-04T00:00:00Z",
	}

	if err := SaveGlobal(path, cfg); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "worksets:\n") {
		t.Fatalf("expected worksets key in serialized config, got %q", text)
	}
	if !strings.Contains(text, "threads:\n") {
		t.Fatalf("expected nested threads map in serialized config, got %q", text)
	}
	if strings.Contains(text, "workset_catalog:") {
		t.Fatalf("did not expect workset_catalog key in serialized config, got %q", text)
	}

	loaded, err := LoadGlobal(path)
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	ref, ok := loaded.Workspaces["feature-policy-eval"]
	if !ok {
		t.Fatalf("expected feature-policy-eval workspace to round-trip")
	}
	if ref.Workset != "core" {
		t.Fatalf("expected workset core, got %q", ref.Workset)
	}
}

func TestCanonicalGlobalForOutputJSONUsesCanonicalKeys(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workspaces["thread-a"] = WorkspaceRef{
		Path:    "/tmp/worksets/core/thread-a",
		Workset: "core",
	}

	data, err := json.Marshal(CanonicalGlobalForOutput(cfg))
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "\"config_version\"") {
		t.Fatalf("expected config_version key in json output, got %q", text)
	}
	if strings.Contains(text, "\"ConfigVersion\"") {
		t.Fatalf("did not expect Go field casing in json output, got %q", text)
	}
	if !strings.Contains(text, "\"worksets\"") {
		t.Fatalf("expected worksets key in json output, got %q", text)
	}
	if !strings.Contains(text, "\"threads\"") {
		t.Fatalf("expected threads key in json output, got %q", text)
	}
}

func TestSaveLoadGlobalPersistsWorksetReposAndRepoOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := DefaultConfig()
	cfg.Workspaces["thread-a"] = WorkspaceRef{
		Path:          "/tmp/worksets/core/thread-a",
		Workset:       "core",
		RepoOverrides: []string{"extra-repo", "extra-repo", " "},
	}
	cfg.WorksetRepos["core"] = []string{"platform", "workset", "platform", ""}

	if err := SaveGlobal(path, cfg); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "repos:") {
		t.Fatalf("expected group repos key in serialized config, got %q", text)
	}
	if !strings.Contains(text, "repo_overrides:") {
		t.Fatalf("expected repo_overrides key in serialized config, got %q", text)
	}

	loaded, err := LoadGlobal(path)
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if got := loaded.WorksetRepos["core"]; len(got) != 2 || got[0] != "platform" || got[1] != "workset" {
		t.Fatalf("expected normalized workset repos [platform workset], got %#v", got)
	}
	if got := loaded.Workspaces["thread-a"].RepoOverrides; len(got) != 1 || got[0] != "extra-repo" {
		t.Fatalf("expected normalized repo_overrides [extra-repo], got %#v", got)
	}
}
