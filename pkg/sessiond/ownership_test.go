package sessiond

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSessionOwnerDefaultsEmpty(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, "owner-default", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	owner, err := client.GetOwner(ctx, "owner-default")
	if err != nil {
		t.Fatalf("get owner: %v", err)
	}
	if owner.Owner != "" {
		t.Fatalf("expected empty owner, got %q", owner.Owner)
	}
}

func TestSessionOwnerClaimAndEnforcement(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, "owner-set", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := client.SendWithOwner(ctx, "owner-set", "echo hi\n", "workspace-test-popout"); err != nil {
		t.Fatalf("send with owner: %v", err)
	}
	owner, err := client.GetOwner(ctx, "owner-set")
	if err != nil {
		t.Fatalf("get owner after claim: %v", err)
	}
	if owner.Owner != "workspace-test-popout" {
		t.Fatalf("expected popout owner after claim, got %q", owner.Owner)
	}

	err = client.SendWithOwner(ctx, "owner-set", "echo from-main\n", "main")
	if err == nil {
		t.Fatal("expected write to fail for non-owner")
	}
	if !strings.Contains(err.Error(), "lease held") {
		t.Fatalf("expected lease error, got %v", err)
	}
	owner, err = client.GetOwner(ctx, "owner-set")
	if err != nil {
		t.Fatalf("get owner after rejected write: %v", err)
	}
	if owner.Owner != "workspace-test-popout" {
		t.Fatalf("expected owner to remain unchanged, got %q", owner.Owner)
	}
}

func TestSessionOwnerTransferViaSetOwner(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, "owner-transfer", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := client.SetOwner(ctx, "owner-transfer", "workspace-test-popout"); err != nil {
		t.Fatalf("set owner: %v", err)
	}
	if err := client.SetOwner(ctx, "owner-transfer", "main"); err != nil {
		t.Fatalf("transfer owner: %v", err)
	}
	if err := client.SendWithOwner(ctx, "owner-transfer", "echo after-transfer\n", "main"); err != nil {
		t.Fatalf("send after transfer: %v", err)
	}
	owner, err := client.GetOwner(ctx, "owner-transfer")
	if err != nil {
		t.Fatalf("get owner after transfer: %v", err)
	}
	if owner.Owner != "main" {
		t.Fatalf("expected owner main after transfer, got %q", owner.Owner)
	}
}
