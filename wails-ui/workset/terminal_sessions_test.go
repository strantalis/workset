package main

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/strantalis/workset/pkg/terminalservice"
)

func startTerminalServiceClientForTerminalSessionsTest(t *testing.T) (*terminalservice.Client, func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("pty not supported on windows")
	}
	t.Setenv("TMPDIR", "/tmp")
	tmp := t.TempDir()
	opts := terminalservice.DefaultOptions()
	opts.SocketPath = filepath.Join("/tmp", fmt.Sprintf("workset-terminal-service-%d.sock", time.Now().UnixNano()))
	opts.TranscriptDir = filepath.Join(tmp, "terminal_logs")
	opts.RecordDir = filepath.Join(tmp, "records")

	server := terminalservice.NewServer(opts)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Listen(ctx)
	}()

	client := terminalservice.NewClient(opts.SocketPath)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		err := client.Ping(pingCtx)
		pingCancel()
		if err == nil {
			return client, func() {
				cancel()
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Fatal("terminal service did not stop")
				}
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	cancel()
	t.Fatal("terminal service did not start")
	return nil, nil
}

func registerTerminalSessionForTerminalSessionsTest(
	app *App,
	workspaceID string,
	terminalID string,
	client *terminalservice.Client,
	lastActivity time.Time,
) *terminalSession {
	session := newTerminalSession(workspaceID, terminalID, "/tmp")
	session.client = client
	session.lastActivity = lastActivity
	session.markReady(nil)
	app.terminals[session.id] = session
	return session
}

func TestWorkspaceTerminalSessionDescriptorReturnsDescriptor(t *testing.T) {
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
		t.Fatalf("create session: %v", err)
	}

	desc, err := app.workspaceTerminalSessionDescriptor(workspaceID, terminalID)
	if err != nil {
		t.Fatalf("get descriptor: %v", err)
	}
	if desc.SessionID != sessionID {
		t.Fatalf("expected session id %q, got %q", sessionID, desc.SessionID)
	}
	if desc.WorkspaceID != workspaceID {
		t.Fatalf("expected workspace id %q, got %q", workspaceID, desc.WorkspaceID)
	}
	if desc.TerminalID != terminalID {
		t.Fatalf("expected terminal id %q, got %q", terminalID, desc.TerminalID)
	}
	if desc.SocketURL == "" {
		t.Fatalf("expected socket URL, got %+v", desc)
	}
	if desc.SocketToken == "" {
		t.Fatalf("expected socket token, got %+v", desc)
	}
}

func TestWorkspaceTerminalSessionDescriptorUsesCachedTerminalServiceInfo(t *testing.T) {
	client, cleanup := startTerminalServiceClientForTerminalSessionsTest(t)
	defer cleanup()

	app := NewApp()
	app.terminalServiceClient = client
	app.terminalServiceReady = true
	app.terminalServiceInfo = &terminalservice.InfoResponse{
		WebSocketURL:   "ws://cached.example/stream",
		WebSocketToken: "cached-token",
	}

	workspaceID := "test-workspace"
	terminalID := "term-1"
	sessionID := terminalSessionID(workspaceID, terminalID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, sessionID, "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	desc, err := app.workspaceTerminalSessionDescriptor(workspaceID, terminalID)
	if err != nil {
		t.Fatalf("get descriptor: %v", err)
	}
	if desc.SocketURL != "ws://cached.example/stream" {
		t.Fatalf("expected cached socket URL, got %q", desc.SocketURL)
	}
	if desc.SocketToken != "cached-token" {
		t.Fatalf("expected cached socket token, got %q", desc.SocketToken)
	}
}

func TestStopWorkspaceTerminalForWindowCleansUp(t *testing.T) {
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
		t.Fatalf("create session: %v", err)
	}

	registerTerminalSessionForTerminalSessionsTest(app, workspaceID, terminalID, client, time.Now())

	if err := app.StopWorkspaceTerminalForWindow(context.Background(), workspaceID, terminalID); err != nil {
		t.Fatalf("stop terminal: %v", err)
	}
	if app.terminals[sessionID] != nil {
		t.Fatal("expected terminal session to be removed after stop")
	}
}

func TestClearTerminalServiceClientResetsDescriptorCache(t *testing.T) {
	app := NewApp()
	app.terminalServiceReady = true
	app.terminalServiceInfo = &terminalservice.InfoResponse{
		WebSocketURL:   "ws://cached.example/stream",
		WebSocketToken: "cached-token",
	}

	app.clearTerminalServiceClient()

	if app.terminalServiceReady {
		t.Fatal("expected descriptor support cache to be cleared")
	}
	if app.terminalServiceInfo != nil {
		t.Fatal("expected cached terminal service info to be cleared")
	}
}
