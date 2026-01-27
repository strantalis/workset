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
