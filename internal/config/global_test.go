package config

import (
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
