package sessiond

import (
	"strings"
	"testing"
)

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

func envHasKey(env []string, key string) bool {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}

func TestBuildSessionEnvScrubsHostTerminalHints(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "42")
	t.Setenv("KITTY_PUBLIC_KEY", "abc")
	t.Setenv("TERM_PROGRAM", "kitty")
	t.Setenv("TERM_PROGRAM_VERSION", "0.42.0")
	t.Setenv("ITERM_SESSION_ID", "iterm")
	t.Setenv("LC_TERMINAL", "iTerm2")
	t.Setenv("LC_TERMINAL_VERSION", "3.5.0")

	env := buildSessionEnv("/bin/zsh", "ws-1", "/tmp/ws")

	if envHasKey(env, "KITTY_WINDOW_ID") {
		t.Fatal("expected KITTY_WINDOW_ID to be removed")
	}
	if envHasKey(env, "KITTY_PUBLIC_KEY") {
		t.Fatal("expected KITTY_PUBLIC_KEY to be removed")
	}
	if envHasKey(env, "ITERM_SESSION_ID") {
		t.Fatal("expected ITERM_SESSION_ID to be removed")
	}
	if envHasKey(env, "LC_TERMINAL") {
		t.Fatal("expected LC_TERMINAL to be removed")
	}
	if envHasKey(env, "LC_TERMINAL_VERSION") {
		t.Fatal("expected LC_TERMINAL_VERSION to be removed")
	}

	if got := envValue(env, "TERM_PROGRAM"); got != "workset" {
		t.Fatalf("expected TERM_PROGRAM=workset, got %q", got)
	}
	if got := envValue(env, "TERM"); got != "xterm-256color" {
		t.Fatalf("expected TERM=xterm-256color, got %q", got)
	}
}

func TestBuildSessionEnvSetsWorksetContext(t *testing.T) {
	env := buildSessionEnv("/bin/bash", "workspace-123", "/tmp/project")

	if got := envValue(env, "SHELL"); got != "/bin/bash" {
		t.Fatalf("expected SHELL=/bin/bash, got %q", got)
	}
	if got := envValue(env, "WORKSET_WORKSPACE"); got != "workspace-123" {
		t.Fatalf("expected WORKSET_WORKSPACE, got %q", got)
	}
	if got := envValue(env, "WORKSET_ROOT"); got != "/tmp/project" {
		t.Fatalf("expected WORKSET_ROOT, got %q", got)
	}
	if got := envValue(env, "COLORTERM"); got != "truecolor" {
		t.Fatalf("expected COLORTERM=truecolor, got %q", got)
	}
}
