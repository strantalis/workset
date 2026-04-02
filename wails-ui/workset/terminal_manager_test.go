package main

import (
	"context"
	"testing"
	"time"
)

func TestStartWorkspaceTerminalRecreatesRemoteClosedSession(t *testing.T) {
	client, cleanup := startTerminalServiceClientForTerminalSessionsTest(t)
	defer cleanup()

	app := NewApp()
	app.terminalServiceClient = client

	workspaceID := "test-workspace"
	terminalID := "term-1"
	sessionID := terminalSessionID(workspaceID, terminalID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, sessionID, "/tmp"); err != nil {
		t.Fatalf("create initial session: %v", err)
	}

	stale := newTerminalSession(workspaceID, terminalID, "/tmp")
	stale.client = client
	stale.markReady(nil)
	app.terminals[sessionID] = stale

	if err := client.Stop(ctx, sessionID); err != nil {
		t.Fatalf("stop remote session: %v", err)
	}

	if err := app.startWorkspaceTerminal(workspaceID, terminalID); err != nil {
		t.Fatalf("restart terminal after remote close: %v", err)
	}

	inspect, err := client.Inspect(ctx, sessionID)
	if err != nil {
		t.Fatalf("inspect recreated session: %v", err)
	}
	if !inspect.Running {
		t.Fatalf("expected recreated session to be running, got %+v", inspect)
	}
}
