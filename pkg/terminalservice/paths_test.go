package terminalservice

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultSocketPathUsesEnvOverride(t *testing.T) {
	const want = "/tmp/workset-terminal-service-test.sock"
	t.Setenv("WORKSET_TERMINAL_SERVICE_SOCKET", want)

	got, err := DefaultSocketPath()
	if err != nil {
		t.Fatalf("DefaultSocketPath error: %v", err)
	}
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestDefaultSocketPathDefault(t *testing.T) {
	t.Setenv("WORKSET_TERMINAL_SERVICE_SOCKET", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir error: %v", err)
	}
	want := filepath.Join(home, ".workset", "terminal-service.sock")

	got, err := DefaultSocketPath()
	if err != nil {
		t.Fatalf("DefaultSocketPath error: %v", err)
	}
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
