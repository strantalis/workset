package terminalservice

import (
	"os/user"
	"reflect"
	"runtime"
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
	t.Setenv("TERM", "")
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
	if got := envValue(env, "TERM"); got != "xterm-256color" {
		t.Fatalf("expected default TERM=xterm-256color when unset, got %q", got)
	}
}

func TestResolveShellCommandUsesPlainShellStartup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell startup semantics do not apply on windows")
	}

	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	tests := []struct {
		name      string
		shell     string
		wantShell string
		wantArgs  []string
	}{
		{name: "zsh", shell: "/bin/zsh", wantShell: "/usr/bin/login", wantArgs: []string{"-fpl", username, "/bin/zsh"}},
		{name: "bash", shell: "/bin/bash", wantShell: "/usr/bin/login", wantArgs: []string{"-fpl", username, "/bin/bash"}},
		{name: "fish", shell: "/opt/homebrew/bin/fish", wantShell: "/usr/bin/login", wantArgs: []string{"-fpl", username, "/opt/homebrew/bin/fish"}},
	}

	if runtime.GOOS != "darwin" {
		// Non-macOS falls back to plain shell launch.
		tests = []struct {
			name      string
			shell     string
			wantShell string
			wantArgs  []string
		}{
			{name: "zsh", shell: "/bin/zsh", wantShell: "/bin/zsh", wantArgs: nil},
			{name: "bash", shell: "/bin/bash", wantShell: "/bin/bash", wantArgs: nil},
			{name: "fish", shell: "/opt/homebrew/bin/fish", wantShell: "/opt/homebrew/bin/fish", wantArgs: nil},
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SHELL", tt.shell)
			gotShell, gotArgs := resolveShellCommand()
			if gotShell != tt.wantShell {
				t.Fatalf("resolveShellCommand shell = %q, want %q", gotShell, tt.wantShell)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("resolveShellCommand args = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}
