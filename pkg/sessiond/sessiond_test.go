package sessiond

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSessiondSnapshotAndBacklog(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()
	createCtx, createCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer createCancel()
	if _, err := client.Create(createCtx, "snap-test", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	sendCtx, sendCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer sendCancel()
	if err := client.Send(sendCtx, "snap-test", "printf 'READY\\n'\n"); err != nil {
		t.Fatalf("send output: %v", err)
	}

	if !waitForSnapshotContains(t, client, "snap-test", "READY", 3*time.Second) {
		t.Fatalf("snapshot did not contain expected output")
	}

	backlogCtx, backlogCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer backlogCancel()
	backlog, err := client.Backlog(backlogCtx, "snap-test", 0)
	if err != nil {
		t.Fatalf("backlog: %v", err)
	}
	if !strings.Contains(backlog.Data, "READY") {
		t.Fatalf("expected backlog to contain READY, got %q", backlog.Data)
	}
}

func TestSessiondMouseEncoding(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	createCtx, createCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer createCancel()
	if _, err := client.Create(createCtx, "mouse-test", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	sendCtx, sendCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer sendCancel()
	if err := client.Send(sendCtx, "mouse-test", "printf '\\033[?1000h\\033[?1006hREADY\\n'\n"); err != nil {
		t.Fatalf("send output: %v", err)
	}

	if !waitForMouseEncoding(t, client, "mouse-test", "sgr", 3*time.Second) {
		t.Fatalf("mouse encoding did not update")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	snap, err := client.Snapshot(ctx, "mouse-test")
	cancel()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.SafeToReplay {
		t.Fatalf("expected safeToReplay false when mouse mode is active")
	}
}

func waitForSnapshotContains(t *testing.T, client *Client, sessionID, needle string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		snap, err := client.Snapshot(ctx, sessionID)
		cancel()
		if err == nil && strings.Contains(snap.Data, needle) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func waitForMouseEncoding(t *testing.T, client *Client, sessionID, encoding string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		snap, err := client.Snapshot(ctx, sessionID)
		cancel()
		if err == nil && snap.MouseEncoding == encoding {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
