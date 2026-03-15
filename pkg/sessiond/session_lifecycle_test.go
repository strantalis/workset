package sessiond

import (
	"strings"
	"testing"
)

func envHasKey(env []string, key string) bool {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}

func TestBuildSessionEnvPreservesHostTerminalHints(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "42")
	t.Setenv("KITTY_PUBLIC_KEY", "abc")
	t.Setenv("TERM_PROGRAM", "kitty")
	t.Setenv("TERM_PROGRAM_VERSION", "0.42.0")
	t.Setenv("ITERM_SESSION_ID", "iterm")
	t.Setenv("LC_TERMINAL", "iTerm2")
	t.Setenv("LC_TERMINAL_VERSION", "3.5.0")
	t.Setenv("TERM", "xterm-kitty")
	t.Setenv("COLORTERM", "24bit")

	env := buildSessionEnv("/bin/zsh", "ws-1", "/tmp/ws")

	if !envHasKey(env, "KITTY_WINDOW_ID") {
		t.Fatal("expected KITTY_WINDOW_ID to be preserved")
	}
	if !envHasKey(env, "KITTY_PUBLIC_KEY") {
		t.Fatal("expected KITTY_PUBLIC_KEY to be preserved")
	}
	if !envHasKey(env, "ITERM_SESSION_ID") {
		t.Fatal("expected ITERM_SESSION_ID to be preserved")
	}
	if !envHasKey(env, "LC_TERMINAL") {
		t.Fatal("expected LC_TERMINAL to be preserved")
	}
	if !envHasKey(env, "LC_TERMINAL_VERSION") {
		t.Fatal("expected LC_TERMINAL_VERSION to be preserved")
	}

	if got := envValue(env, "TERM_PROGRAM"); got != "kitty" {
		t.Fatalf("expected TERM_PROGRAM=kitty, got %q", got)
	}
	if got := envValue(env, "TERM"); got != "xterm-kitty" {
		t.Fatalf("expected TERM=xterm-kitty, got %q", got)
	}
}

func TestBuildSessionEnvSetsWorksetContext(t *testing.T) {
	t.Setenv("COLORTERM", "24bit")
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
	if got := envValue(env, "COLORTERM"); got != "24bit" {
		t.Fatalf("expected COLORTERM to be preserved, got %q", got)
	}
}
