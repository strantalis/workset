package worksetapi

import (
	"os"
	"testing"
)

func TestParseEnvSnapshot(t *testing.T) {
	snapshot := parseEnvSnapshot("FOO=bar\nEMPTY=\nNOEQUALS\nBAR=baz\n")
	if got := snapshot["FOO"]; got != "bar" {
		t.Fatalf("expected FOO=bar, got %q", got)
	}
	if _, ok := snapshot["NOEQUALS"]; ok {
		t.Fatalf("expected NOEQUALS to be ignored")
	}
	if got := snapshot["EMPTY"]; got != "" {
		t.Fatalf("expected EMPTY to be empty, got %q", got)
	}
	if got := snapshot["BAR"]; got != "baz" {
		t.Fatalf("expected BAR=baz, got %q", got)
	}
}

func TestApplyEnvSnapshotOverridesSSHAuthSock(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "old")
	changed := applyEnvSnapshot(map[string]string{"SSH_AUTH_SOCK": "new"})
	if got := os.Getenv("SSH_AUTH_SOCK"); got != "new" {
		t.Fatalf("expected SSH_AUTH_SOCK=new, got %q", got)
	}
	if !containsKey(changed, "SSH_AUTH_SOCK") {
		t.Fatalf("expected SSH_AUTH_SOCK in changed keys, got %v", changed)
	}
}

func TestApplyEnvSnapshotClearsSSHAuthSockWhenEmpty(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "old")
	changed := applyEnvSnapshot(map[string]string{"SSH_AUTH_SOCK": ""})
	if got := os.Getenv("SSH_AUTH_SOCK"); got != "" {
		t.Fatalf("expected SSH_AUTH_SOCK cleared, got %q", got)
	}
	if !containsKey(changed, "SSH_AUTH_SOCK") {
		t.Fatalf("expected SSH_AUTH_SOCK in changed keys, got %v", changed)
	}
}

func TestApplyEnvSnapshotClearsSSHAuthSockWhenMissing(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "old")
	changed := applyEnvSnapshot(map[string]string{"PATH": "/usr/bin"})
	if got := os.Getenv("SSH_AUTH_SOCK"); got != "" {
		t.Fatalf("expected SSH_AUTH_SOCK cleared, got %q", got)
	}
	if !containsKey(changed, "SSH_AUTH_SOCK") {
		t.Fatalf("expected SSH_AUTH_SOCK in changed keys, got %v", changed)
	}
}

func TestApplyEnvSnapshotKeepsExistingNonOverride(t *testing.T) {
	t.Setenv("WORKSET_TEST_FOO", "bar")
	changed := applyEnvSnapshot(map[string]string{"WORKSET_TEST_FOO": "baz"})
	if got := os.Getenv("WORKSET_TEST_FOO"); got != "bar" {
		t.Fatalf("expected WORKSET_TEST_FOO=bar, got %q", got)
	}
	if containsKey(changed, "WORKSET_TEST_FOO") {
		t.Fatalf("did not expect WORKSET_TEST_FOO in changed keys, got %v", changed)
	}
}

func TestEnvSnapshotDisabledParsing(t *testing.T) {
	t.Setenv("WORKSET_ENV_SNAPSHOT", "0")
	if !envSnapshotDisabled() {
		t.Fatalf("expected snapshot disabled when WORKSET_ENV_SNAPSHOT=0")
	}
	t.Setenv("WORKSET_ENV_SNAPSHOT", "false")
	if !envSnapshotDisabled() {
		t.Fatalf("expected snapshot disabled when WORKSET_ENV_SNAPSHOT=false")
	}
	t.Setenv("WORKSET_ENV_SNAPSHOT", "1")
	if envSnapshotDisabled() {
		t.Fatalf("expected snapshot enabled when WORKSET_ENV_SNAPSHOT=1")
	}
}

func containsKey(values []string, key string) bool {
	for _, value := range values {
		if value == key {
			return true
		}
	}
	return false
}
