package e2e

import (
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	runner := newRunner(t)

	out, err := runner.run("version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if strings.TrimSpace(out) != "dev" {
		t.Fatalf("unexpected version output: %q", out)
	}

	out, err = runner.run("--version")
	if err != nil {
		t.Fatalf("--version: %v", err)
	}
	if !strings.Contains(out, "dev") {
		t.Fatalf("unexpected --version output: %q", out)
	}
}

func TestShellCompletionCommand(t *testing.T) {
	runner := newRunner(t)
	out, err := runner.run("completion", "bash")
	if err != nil {
		t.Fatalf("completion bash: %v", err)
	}
	if !strings.Contains(out, "workset") {
		t.Fatalf("completion output missing workset: %s", out)
	}
}
