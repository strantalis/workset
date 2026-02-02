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

func TestAttachEmitsBootstrapFirst(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, "attach-bootstrap", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	stream, first, err := client.Attach(ctx, "attach-bootstrap", 0, false, "bootstrap-stream")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if first.Type != "bootstrap" {
		t.Fatalf("expected bootstrap first, got %+v", first)
	}
	if first.InitialCredit != DefaultOptions().StreamInitialCredit {
		t.Fatalf("expected initial credit %d, got %d", DefaultOptions().StreamInitialCredit, first.InitialCredit)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var next StreamMessage
		if err := stream.Next(&next); err != nil {
			t.Fatalf("read after bootstrap: %v", err)
		}
		if next.Type == "bootstrap_done" {
			return
		}
	}
	t.Fatalf("expected bootstrap_done before timeout")
}

func TestAttachBootstrapOrderingWithBacklog(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, "attach-ordering", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	sendCtx, sendCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer sendCancel()
	if err := client.Send(sendCtx, "attach-ordering", "printf 'READY\\n'\n"); err != nil {
		t.Fatalf("send output: %v", err)
	}
	if !waitForSnapshotContains(t, client, "attach-ordering", "READY", 3*time.Second) {
		t.Fatalf("snapshot did not contain expected output")
	}

	stream, first, err := client.Attach(ctx, "attach-ordering", 0, false, "ordering-stream")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if first.Type != "bootstrap" {
		t.Fatalf("expected bootstrap first, got %+v", first)
	}

	foundBootstrapData := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var next StreamMessage
		if err := stream.Next(&next); err != nil {
			t.Fatalf("read during bootstrap: %v", err)
		}
		switch next.Type {
		case "data":
			if next.Source != "bootstrap" {
				t.Fatalf("expected bootstrap source before bootstrap_done, got %+v", next)
			}
			if strings.Contains(next.Data, "READY") {
				foundBootstrapData = true
			}
		case "bootstrap_done":
			if !foundBootstrapData {
				t.Fatalf("expected bootstrap data before bootstrap_done")
			}
			goto live
		}
	}
	t.Fatalf("expected bootstrap_done before timeout")

live:
	ackCtx, ackCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer ackCancel()
	if err := client.Ack(ackCtx, "attach-ordering", "ordering-stream", DefaultOptions().StreamInitialCredit); err != nil {
		t.Fatalf("ack: %v", err)
	}
	postCtx, postCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer postCancel()
	if err := client.Send(postCtx, "attach-ordering", "printf 'LIVE\\n'\n"); err != nil {
		t.Fatalf("send live output: %v", err)
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var msg StreamMessage
		if err := stream.Next(&msg); err != nil {
			t.Fatalf("read live: %v", err)
		}
		if msg.Type == "data" && msg.Source != "bootstrap" {
			if strings.Contains(msg.Data, "LIVE") {
				return
			}
		}
	}
	t.Fatalf("expected live data after bootstrap_done")
}
