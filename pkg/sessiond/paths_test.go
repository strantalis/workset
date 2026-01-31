package sessiond

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultSocketPathUsesEnvOverride(t *testing.T) {
	const want = "/tmp/workset-sessiond-test.sock"
	t.Setenv("WORKSET_SESSIOND_SOCKET", want)

	got, err := DefaultSocketPath()
	if err != nil {
		t.Fatalf("DefaultSocketPath error: %v", err)
	}
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestDefaultSocketPathDefault(t *testing.T) {
	t.Setenv("WORKSET_SESSIOND_SOCKET", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir error: %v", err)
	}
	want := filepath.Join(home, ".workset", "sessiond.sock")

	got, err := DefaultSocketPath()
	if err != nil {
		t.Fatalf("DefaultSocketPath error: %v", err)
	}
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
