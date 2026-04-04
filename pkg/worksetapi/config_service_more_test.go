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
	if cfg.Defaults.AgentModel != "" {
		t.Fatalf("unexpected agent model default: %q", cfg.Defaults.AgentModel)
	}
	if cfg.Defaults.TerminalIdleTimeout == "" {
		t.Fatalf("unexpected terminal idle timeout default: %q", cfg.Defaults.TerminalIdleTimeout)
	}
	if cfg.Defaults.TerminalDebugLog == "" {
		t.Fatalf("unexpected terminal debug log default: %q", cfg.Defaults.TerminalDebugLog)
	}
	if cfg.Defaults.TerminalProtocolLog == "" {
		t.Fatalf("unexpected terminal protocol log default: %q", cfg.Defaults.TerminalProtocolLog)
	}
	if cfg.Defaults.TerminalDebugOverlay == "" {
		t.Fatalf("unexpected terminal debug overlay default: %q", cfg.Defaults.TerminalDebugOverlay)
	}
	if cfg.Defaults.TerminalFontSize == "" {
		t.Fatalf("unexpected terminal font size default: %q", cfg.Defaults.TerminalFontSize)
	}
	if cfg.Defaults.TerminalCursorBlink == "" {
		t.Fatalf("unexpected terminal cursor blink default: %q", cfg.Defaults.TerminalCursorBlink)
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
	_, _, err := env.svc.SetDefault(context.Background(), "defaults.agent", "cursor")
	if err == nil {
		t.Fatalf("expected unsupported agent error")
	}

	_, _, err = env.svc.SetDefault(context.Background(), "defaults.remotes.base", "origin")
	if err == nil {
		t.Fatalf("expected removed key error")
	}

	_, _, err = env.svc.SetDefault(context.Background(), "defaults.parallelism", "4")
	if err == nil {
		t.Fatalf("expected removed key error")
	}

	_, _, err = env.svc.SetDefault(context.Background(), "defaults.terminal_font_size", "abc")
	if err == nil {
		t.Fatalf("expected invalid terminal font size error")
	}

	_, _, err = env.svc.SetDefault(context.Background(), "defaults.terminal_cursor_blink", "maybe")
	if err == nil {
		t.Fatalf("expected invalid terminal cursor blink error")
	}
	_, _, err = env.svc.SetDefault(context.Background(), "defaults.terminal_debug_log", "maybe")
	if err == nil {
		t.Fatalf("expected invalid terminal debug log error")
	}

	_, _, err = env.svc.SetDefault(context.Background(), "defaults.unknown", "value")
	if err == nil {
		t.Fatalf("expected unsupported key error")
	}
}

func TestSetDefaultVariousKeys(t *testing.T) {
	env := newTestEnv(t)
	cases := map[string]string{
		"defaults.remote":                 "origin",
		"defaults.thread":                 "demo",
		"defaults.workset_root":           "/tmp/workset",
		"defaults.repo_store_root":        "/tmp/repos",
		"defaults.agent":                  "codex",
		"defaults.agent_model":            "gpt-4o-mini",
		"defaults.terminal_idle_timeout":  "0",
		"defaults.terminal_debug_log":     "on",
		"defaults.terminal_protocol_log":  "on",
		"defaults.terminal_debug_overlay": "off",
		"defaults.terminal_font_size":     "16",
		"defaults.terminal_cursor_blink":  "off",
	}
	for key, value := range cases {
		if _, _, err := env.svc.SetDefault(context.Background(), key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
	cfg := env.loadConfig()
	if cfg.Defaults.Thread != "demo" {
		t.Fatalf("thread default not set")
	}
	if cfg.Defaults.WorksetRoot != "/tmp/workset" {
		t.Fatalf("workset_root default not set")
	}
	if cfg.Defaults.Agent != "codex" {
		t.Fatalf("agent default not set")
	}
	if cfg.Defaults.AgentModel != "gpt-4o-mini" {
		t.Fatalf("agent model default not set")
	}
	if cfg.Defaults.TerminalIdleTimeout != "0" {
		t.Fatalf("terminal idle timeout default not set")
	}
	if cfg.Defaults.TerminalDebugLog != "on" {
		t.Fatalf("terminal debug log default not set")
	}
	if cfg.Defaults.TerminalProtocolLog != "on" {
		t.Fatalf("terminal protocol log default not set")
	}
	if cfg.Defaults.TerminalDebugOverlay != "off" {
		t.Fatalf("terminal debug overlay default not set")
	}
	if cfg.Defaults.TerminalFontSize != "16" {
		t.Fatalf("terminal font size default not set")
	}
	if cfg.Defaults.TerminalCursorBlink != "off" {
		t.Fatalf("terminal cursor blink default not set")
	}
}
