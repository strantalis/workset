package worksetapi

import (
	"context"
	"testing"
)

func TestGetConfig(t *testing.T) {
	env := newTestEnv(t)
	cfg, info, err := env.svc.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if info.Path != env.configPath {
		t.Fatalf("unexpected config path: %s", info.Path)
	}
	if cfg.Defaults.BaseBranch != "main" {
		t.Fatalf("unexpected defaults: %+v", cfg.Defaults)
	}
	if cfg.Defaults.Remote != "origin" {
		t.Fatalf("unexpected defaults remote: %q", cfg.Defaults.Remote)
	}
	if cfg.Defaults.Agent != "codex" {
		t.Fatalf("unexpected agent default: %q", cfg.Defaults.Agent)
	}
	if cfg.Defaults.TerminalRenderer != "auto" {
		t.Fatalf("unexpected terminal renderer default: %q", cfg.Defaults.TerminalRenderer)
	}
	if cfg.Defaults.TerminalIdleTimeout == "" {
		t.Fatalf("unexpected terminal idle timeout default: %q", cfg.Defaults.TerminalIdleTimeout)
	}
	if cfg.Defaults.TerminalProtocolLog == "" {
		t.Fatalf("unexpected terminal protocol log default: %q", cfg.Defaults.TerminalProtocolLog)
	}
}

func TestSetDefaultUpdatesConfig(t *testing.T) {
	env := newTestEnv(t)
	_, _, err := env.svc.SetDefault(context.Background(), "defaults.base_branch", "develop")
	if err != nil {
		t.Fatalf("set default: %v", err)
	}
	if _, _, err := env.svc.SetDefault(context.Background(), "defaults.remote", "upstream"); err != nil {
		t.Fatalf("set default remote: %v", err)
	}
	cfg := env.loadConfig()
	if cfg.Defaults.BaseBranch != "develop" {
		t.Fatalf("expected base branch updated")
	}
	if cfg.Defaults.Remote != "upstream" {
		t.Fatalf("expected remote updated")
	}
}

func TestSetDefaultErrors(t *testing.T) {
	env := newTestEnv(t)
	_, _, err := env.svc.SetDefault(context.Background(), "defaults.session_backend", "unknown")
	if err == nil {
		t.Fatalf("expected backend error")
	}

	_, _, err = env.svc.SetDefault(context.Background(), "defaults.remotes.base", "origin")
	if err == nil {
		t.Fatalf("expected removed key error")
	}

	_, _, err = env.svc.SetDefault(context.Background(), "defaults.parallelism", "4")
	if err == nil {
		t.Fatalf("expected removed key error")
	}

	_, _, err = env.svc.SetDefault(context.Background(), "defaults.unknown", "value")
	if err == nil {
		t.Fatalf("expected unsupported key error")
	}
}

func TestSetDefaultVariousKeys(t *testing.T) {
	env := newTestEnv(t)
	cases := map[string]string{
		"defaults.remote":                    "origin",
		"defaults.workspace":                 "demo",
		"defaults.workspace_root":            "/tmp/workspaces",
		"defaults.repo_store_root":           "/tmp/repos",
		"defaults.session_name_format":       "ws-{workspace}",
		"defaults.session_theme":             "dark",
		"defaults.session_tmux_status_style": "bold",
		"defaults.session_tmux_status_left":  "left",
		"defaults.session_tmux_status_right": "right",
		"defaults.session_screen_hardstatus": "hard",
		"defaults.session_backend":           "exec",
		"defaults.agent":                     "codex",
		"defaults.terminal_renderer":         "webgl",
		"defaults.terminal_idle_timeout":     "0",
		"defaults.terminal_protocol_log":     "on",
	}
	for key, value := range cases {
		if _, _, err := env.svc.SetDefault(context.Background(), key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
	cfg := env.loadConfig()
	if cfg.Defaults.Workspace != "demo" {
		t.Fatalf("workspace default not set")
	}
	if cfg.Defaults.SessionBackend != "exec" {
		t.Fatalf("session backend not set")
	}
	if cfg.Defaults.Agent != "codex" {
		t.Fatalf("agent default not set")
	}
	if cfg.Defaults.TerminalRenderer != "webgl" {
		t.Fatalf("terminal renderer default not set")
	}
	if cfg.Defaults.TerminalIdleTimeout != "0" {
		t.Fatalf("terminal idle timeout default not set")
	}
	if cfg.Defaults.TerminalProtocolLog != "on" {
		t.Fatalf("terminal protocol log default not set")
	}
}
