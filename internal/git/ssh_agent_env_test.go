package git

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestApplySSHAuthSockOverridesDifferentSocket(t *testing.T) {
	dir := tempSocketDir(t)
	current := makeSocket(t, dir, "current.sock")
	target := makeSocket(t, dir, "target.sock")

	next, ok := applySSHAuthSock(current, target)
	if !ok {
		t.Fatalf("expected override")
	}
	if next != target {
		t.Fatalf("expected %s, got %s", target, next)
	}
}

func TestApplySSHAuthSockSkipsSameSocket(t *testing.T) {
	dir := tempSocketDir(t)
	target := makeSocket(t, dir, "agent.sock")

	if next, ok := applySSHAuthSock(target, target); ok || next != "" {
		t.Fatalf("expected no change for same socket")
	}
}

func TestApplySSHAuthSockUsesIdentityAgentPath(t *testing.T) {
	dir := tempSocketDir(t)
	current := makeSocket(t, dir, "current.sock")
	identity := filepath.Join(dir, "not-a-socket")
	if err := os.WriteFile(identity, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if next, ok := applySSHAuthSock(current, identity); !ok || next != identity {
		t.Fatalf("expected override to identity agent path")
	}
}

func TestApplySSHAuthSockTrimsQuotes(t *testing.T) {
	dir := tempSocketDir(t)
	target := makeSocket(t, dir, "quoted.sock")

	next, ok := applySSHAuthSock("", "\""+target+"\"")
	if !ok {
		t.Fatalf("expected override with quoted path")
	}
	if next != target {
		t.Fatalf("expected %s, got %s", target, next)
	}
}

func makeSocket(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(path)
	})
	return path
}

func tempSocketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "workset-ssh-")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}
