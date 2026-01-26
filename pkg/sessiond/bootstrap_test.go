package sessiond

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSessiondBootstrapSnapshot(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	createCtx, createCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer createCancel()
	if _, err := client.Create(createCtx, "bootstrap-test", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	sendCtx, sendCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer sendCancel()
	if err := client.Send(sendCtx, "bootstrap-test", "printf 'READY\\n'\n"); err != nil {
		t.Fatalf("send output: %v", err)
	}

	if !waitForSnapshotContains(t, client, "bootstrap-test", "READY", 3*time.Second) {
		t.Fatalf("snapshot did not contain expected output")
	}

	bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer bootstrapCancel()
	bootstrap, err := client.Bootstrap(bootstrapCtx, "bootstrap-test")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if !strings.Contains(bootstrap.Snapshot, "READY") {
		t.Fatalf("expected snapshot to contain READY, got %q", bootstrap.Snapshot)
	}
	if bootstrap.Backlog != "" {
		t.Fatalf("expected backlog to be empty when snapshot is present, got %q", bootstrap.Backlog)
	}
}
