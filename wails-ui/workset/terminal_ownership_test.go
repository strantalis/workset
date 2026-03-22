package main

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/strantalis/workset/pkg/sessiond"
)

func startSessiondClientForTerminalOwnershipTest(t *testing.T) (*sessiond.Client, func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("pty not supported on windows")
	}
	t.Setenv("TMPDIR", "/tmp")
	tmp := t.TempDir()
	opts := sessiond.DefaultOptions()
	opts.SocketPath = filepath.Join(tmp, "sessiond.sock")
	opts.TranscriptDir = filepath.Join(tmp, "terminal_logs")
	opts.RecordDir = filepath.Join(tmp, "records")

	server := sessiond.NewServer(opts)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Listen(ctx)
	}()

	client := sessiond.NewClient(opts.SocketPath)
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
					t.Fatal("sessiond server did not stop")
				}
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	cancel()
	t.Fatal("sessiond server did not start")
	return nil, nil
}

func registerTerminalSession(
	app *App,
	workspaceID string,
	terminalID string,
	client *sessiond.Client,
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
	client, cleanup := startSessiondClientForTerminalOwnershipTest(t)
	defer cleanup()

	app := NewApp()
	app.sessiondClient = client

	workspaceID := "test-workspace"
	terminalID := "term-1"
	sessionID := terminalSessionID(workspaceID, terminalID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, sessionID, "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := client.SetOwner(ctx, sessionID, "workspace-test-popout"); err != nil {
		t.Fatalf("set owner: %v", err)
	}

	desc, err := app.workspaceTerminalSessionDescriptor(workspaceID, terminalID, "workspace-test-popout")
	if err != nil {
		t.Fatalf("get descriptor: %v", err)
	}
	if desc.SessionID != sessionID {
		t.Fatalf("expected session id %q, got %q", sessionID, desc.SessionID)
	}
	if desc.Owner != "workspace-test-popout" {
		t.Fatalf("expected owner workspace-test-popout, got %q", desc.Owner)
	}
	if !desc.CanWrite {
		t.Fatalf("expected owner to have write access, got %+v", desc)
	}
	if !desc.Running {
		t.Fatalf("expected running descriptor, got %+v", desc)
	}
	if desc.SocketURL == "" {
		t.Fatalf("expected socket URL, got %+v", desc)
	}
	if desc.SocketToken == "" {
		t.Fatalf("expected socket token, got %+v", desc)
	}
	if desc.Transport != "sessiond-websocket" {
		t.Fatalf("expected transport sessiond-websocket, got %q", desc.Transport)
	}
}

func TestWorkspaceTerminalSessionDescriptorDeniesWriteToNonOwner(t *testing.T) {
	client, cleanup := startSessiondClientForTerminalOwnershipTest(t)
	defer cleanup()

	app := NewApp()
	app.sessiondClient = client

	workspaceID := "test-workspace"
	terminalID := "term-1"
	sessionID := terminalSessionID(workspaceID, terminalID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, sessionID, "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := client.SetOwner(ctx, sessionID, "workspace-test-popout"); err != nil {
		t.Fatalf("set owner: %v", err)
	}

	desc, err := app.workspaceTerminalSessionDescriptor(workspaceID, terminalID, "main")
	if err != nil {
		t.Fatalf("get descriptor: %v", err)
	}
	if desc.CanWrite {
		t.Fatalf("expected non-owner to be read-only, got %+v", desc)
	}
}

func TestWorkspaceTerminalSessionDescriptorUsesCachedSessiondInfo(t *testing.T) {
	client, cleanup := startSessiondClientForTerminalOwnershipTest(t)
	defer cleanup()

	app := NewApp()
	app.sessiondClient = client
	app.sessiondReady = true
	app.sessiondInfo = &sessiond.InfoResponse{
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

	desc, err := app.workspaceTerminalSessionDescriptor(workspaceID, terminalID, "main")
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

func TestTransferWorkspaceTerminalOwnerUpdatesDaemonLease(t *testing.T) {
	client, cleanup := startSessiondClientForTerminalOwnershipTest(t)
	defer cleanup()

	app := NewApp()
	app.sessiondClient = client

	workspaceID := "test-workspace"
	sessionIDs := []string{
		terminalSessionID(workspaceID, "term-1"),
		terminalSessionID(workspaceID, "term-2"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, sessionID := range sessionIDs {
		if _, err := client.Create(ctx, sessionID, "/tmp"); err != nil {
			t.Fatalf("create session %s: %v", sessionID, err)
		}
	}

	registerTerminalSession(app, workspaceID, "term-1", client, time.Now().Add(-time.Minute))
	registerTerminalSession(app, workspaceID, "term-2", client, time.Now())

	if err := app.transferWorkspaceTerminalOwner(workspaceID, "workspace-test-popout"); err != nil {
		t.Fatalf("transfer workspace owner: %v", err)
	}

	for _, sessionID := range sessionIDs {
		owner, err := client.GetOwner(ctx, sessionID)
		if err != nil {
			t.Fatalf("get owner for %s: %v", sessionID, err)
		}
		if owner.Owner != "workspace-test-popout" {
			t.Fatalf("expected owner workspace-test-popout for %s, got %q", sessionID, owner.Owner)
		}
	}
}

func TestGetWorkspaceTerminalOwnerReflectsDaemonLease(t *testing.T) {
	client, cleanup := startSessiondClientForTerminalOwnershipTest(t)
	defer cleanup()

	app := NewApp()
	app.sessiondClient = client

	workspaceID := "test-workspace"
	olderSessionID := terminalSessionID(workspaceID, "term-1")
	newerSessionID := terminalSessionID(workspaceID, "term-2")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, sessionID := range []string{olderSessionID, newerSessionID} {
		if _, err := client.Create(ctx, sessionID, "/tmp"); err != nil {
			t.Fatalf("create session %s: %v", sessionID, err)
		}
	}
	if err := client.SetOwner(ctx, olderSessionID, "main"); err != nil {
		t.Fatalf("set older owner: %v", err)
	}
	if err := client.SetOwner(ctx, newerSessionID, "workspace-test-popout"); err != nil {
		t.Fatalf("set newer owner: %v", err)
	}

	registerTerminalSession(app, workspaceID, "term-1", client, time.Now().Add(-time.Minute))
	registerTerminalSession(app, workspaceID, "term-2", client, time.Now())

	if got := app.workspaceTerminalOwner(workspaceID); got != "workspace-test-popout" {
		t.Fatalf("expected latest daemon owner workspace-test-popout, got %q", got)
	}
}

func TestGetWorkspaceTerminalOwnerDefaultsToMainWithoutSessions(t *testing.T) {
	app := NewApp()

	if got := app.workspaceTerminalOwner("test-workspace"); got != "main" {
		t.Fatalf("expected main fallback owner, got %q", got)
	}
}

func TestUnregisterWorkspacePopoutIgnoresStaleWindowName(t *testing.T) {
	client, cleanup := startSessiondClientForTerminalOwnershipTest(t)
	defer cleanup()

	app := NewApp()
	app.sessiondClient = client

	workspaceID := "test-workspace"
	terminalID := "term-1"
	sessionID := terminalSessionID(workspaceID, terminalID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, sessionID, "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := client.SetOwner(ctx, sessionID, "workspace-test-popout"); err != nil {
		t.Fatalf("set owner: %v", err)
	}

	registerTerminalSession(app, workspaceID, terminalID, client, time.Now())
	app.popouts[workspaceID] = "workspace-test-popout"

	app.unregisterWorkspacePopout(workspaceID, "stale-window")

	if got := app.popouts[workspaceID]; got != "workspace-test-popout" {
		t.Fatalf("expected popout to remain registered, got %q", got)
	}
	owner, err := client.GetOwner(ctx, sessionID)
	if err != nil {
		t.Fatalf("get owner: %v", err)
	}
	if owner.Owner != "workspace-test-popout" {
		t.Fatalf("expected stale unregister to keep popout owner, got %q", owner.Owner)
	}
}

func TestStopWorkspaceTerminalForWindowRejectsNonOwner(t *testing.T) {
	client, cleanup := startSessiondClientForTerminalOwnershipTest(t)
	defer cleanup()

	app := NewApp()
	app.sessiondClient = client

	workspaceID := "test-workspace"
	terminalID := "term-1"
	sessionID := terminalSessionID(workspaceID, terminalID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, sessionID, "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := client.SetOwner(ctx, sessionID, "workspace-test-popout"); err != nil {
		t.Fatalf("set owner: %v", err)
	}

	registerTerminalSession(app, workspaceID, terminalID, client, time.Now())

	if err := app.StopWorkspaceTerminalForWindow(context.Background(), workspaceID, terminalID); err == nil {
		t.Fatal("expected non-owner stop to fail")
	}
	if app.terminals[sessionID] == nil {
		t.Fatal("expected terminal session to remain registered after rejected stop")
	}
}

func TestStopWorkspaceTerminalForWindowAllowsOwnerAndCleansUp(t *testing.T) {
	client, cleanup := startSessiondClientForTerminalOwnershipTest(t)
	defer cleanup()

	app := NewApp()
	app.sessiondClient = client

	workspaceID := "test-workspace"
	terminalID := "term-1"
	sessionID := terminalSessionID(workspaceID, terminalID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, sessionID, "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := client.SetOwner(ctx, sessionID, "main"); err != nil {
		t.Fatalf("set owner: %v", err)
	}

	registerTerminalSession(app, workspaceID, terminalID, client, time.Now())

	if err := app.StopWorkspaceTerminalForWindow(context.Background(), workspaceID, terminalID); err != nil {
		t.Fatalf("stop terminal for owner: %v", err)
	}
	if app.terminals[sessionID] != nil {
		t.Fatal("expected terminal session to be removed after successful stop")
	}
}

func TestClearSessiondClientResetsDescriptorCache(t *testing.T) {
	app := NewApp()
	app.sessiondReady = true
	app.sessiondInfo = &sessiond.InfoResponse{
		WebSocketURL:   "ws://cached.example/stream",
		WebSocketToken: "cached-token",
	}

	app.clearSessiondClient()

	if app.sessiondReady {
		t.Fatal("expected descriptor support cache to be cleared")
	}
	if app.sessiondInfo != nil {
		t.Fatal("expected cached sessiond info to be cleared")
	}
}
