package worksetapi

import "testing"

func TestShellEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "''"},
		{name: "simple", input: "codex", want: "'codex'"},
		{name: "spaces", input: "cursor agent", want: "'cursor agent'"},
		{name: "single quote", input: "foo'bar", want: "'foo'\"'\"'bar'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellEscape(tt.input)
			if got != tt.want {
				t.Fatalf("shellEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShellJoinArgs(t *testing.T) {
	args := []string{"codex", "exec", "--output-schema", "/tmp/schema.json", "-"}
	want := "'codex' 'exec' '--output-schema' '/tmp/schema.json' '-'"
	got := shellJoinArgs(args)
	if got != want {
		t.Fatalf("shellJoinArgs(%v) = %q, want %q", args, got, want)
	}
}

func TestShellArgsForMode(t *testing.T) {
	tests := []struct {
		name      string
		shellBase string
		mode      string
		want      []string
	}{
		{name: "bash login", shellBase: "bash", mode: agentShellModeLogin, want: []string{"-lc", "cmd"}},
		{name: "bash interactive", shellBase: "bash", mode: agentShellModeInteractive, want: []string{"-ic", "cmd"}},
		{name: "bash login-interactive", shellBase: "bash", mode: agentShellModeLoginAndI, want: []string{"-lic", "cmd"}},
		{name: "bash plain", shellBase: "bash", mode: agentShellModePlain, want: []string{"-c", "cmd"}},
		{name: "fish login", shellBase: "fish", mode: agentShellModeLogin, want: []string{"-l", "-c", "cmd"}},
		{name: "fish interactive", shellBase: "fish", mode: agentShellModeInteractive, want: []string{"-i", "-c", "cmd"}},
		{name: "fish login-interactive", shellBase: "fish", mode: agentShellModeLoginAndI, want: []string{"-l", "-i", "-c", "cmd"}},
		{name: "fish plain", shellBase: "fish", mode: agentShellModePlain, want: []string{"-c", "cmd"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellArgsForMode(tt.shellBase, "cmd", tt.mode)
			if len(got) != len(tt.want) {
				t.Fatalf("shellArgsForMode(%s, %s) = %v, want %v", tt.shellBase, tt.mode, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("shellArgsForMode(%s, %s) = %v, want %v", tt.shellBase, tt.mode, got, tt.want)
				}
			}
		})
	}
}
