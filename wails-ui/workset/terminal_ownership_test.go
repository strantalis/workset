package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/strantalis/workset/pkg/sessiond"
)

func TestAllowWorkspaceTerminalStartMainOwnerAllowsClaim(t *testing.T) {
	app := NewApp()

	if err := app.allowWorkspaceTerminalStart("test-workspace", "workspace-test-popout"); err != nil {
		t.Fatalf("expected popout claim to succeed when main owns workspace: %v", err)
	}
	if got := app.GetWorkspaceTerminalOwner("test-workspace"); got != "workspace-test-popout" {
		t.Fatalf("expected owner workspace-test-popout, got %q", got)
	}
}

func TestAllowWorkspaceTerminalStartAllowsHandoff(t *testing.T) {
	app := NewApp()
	app.claimWorkspaceTerminalOwner("test-workspace", "workspace-test-popout")

	if err := app.allowWorkspaceTerminalStart("test-workspace", "main"); err != nil {
		t.Fatalf("expected ownership handoff to succeed: %v", err)
	}
	if got := app.GetWorkspaceTerminalOwner("test-workspace"); got != "main" {
		t.Fatalf("expected owner main after handoff, got %q", got)
	}
}

func TestAllowWorkspaceTerminalStartAllowsCurrentOwner(t *testing.T) {
	app := NewApp()
	app.claimWorkspaceTerminalOwner("test-workspace", "workspace-test-popout")

	if err := app.allowWorkspaceTerminalStart("test-workspace", "workspace-test-popout"); err != nil {
		t.Fatalf("expected current owner to restart terminal: %v", err)
	}
	if got := app.GetWorkspaceTerminalOwner("test-workspace"); got != "workspace-test-popout" {
		t.Fatalf("expected owner workspace-test-popout, got %q", got)
	}
}

func TestResizeWorkspaceTerminalForWindowNameReturnsMissingTerminal(t *testing.T) {
	app := NewApp()

	if err := app.ResizeWorkspaceTerminalForWindowName(
		context.Background(),
		"test-workspace",
		"missing-terminal",
		80,
		24,
		"main",
	); err == nil {
		t.Fatal("expected missing terminal resize to return an error")
	}
}

func TestResizeWorkspaceTerminalForWindowNameRejectsMissingWorkspaceID(t *testing.T) {
	app := NewApp()

	err := app.ResizeWorkspaceTerminalForWindowName(
		context.Background(),
		"",
		"term-1",
		80,
		24,
		"main",
	)
	if err == nil {
		t.Fatal("expected missing workspace id to fail")
	}
}

func TestResizeWorkspaceTerminalForWindowNameIgnoresTerminalNotStarted(t *testing.T) {
	app := NewApp()
	workspaceID := "test-workspace"
	terminalID := "term-1"
	session := newTerminalSession(workspaceID, terminalID, "/tmp")
	session.markReady(nil)
	app.terminals[terminalSessionID(workspaceID, terminalID)] = session

	err := app.ResizeWorkspaceTerminalForWindowName(
		context.Background(),
		workspaceID,
		terminalID,
		80,
		24,
		"main",
	)
	if err != nil {
		t.Fatalf("expected transient not-started resize to be ignored, got %v", err)
	}
}

func TestRestartTerminalStreamForHandoffSkipsWhenStreamOwnerMatches(t *testing.T) {
	app := NewApp()
	workspaceID := "test-workspace"
	terminalID := "term-1"
	session := newTerminalSession(workspaceID, terminalID, "/tmp")
	stream := &stubTerminalStream{id: "existing"}
	session.markReady(nil)
	session.stream = stream
	session.streamCancel = func() {}
	session.streamOwner = "workspace-test-popout"
	app.terminals[terminalSessionID(workspaceID, terminalID)] = session

	app.restartTerminalStreamForHandoff(workspaceID, terminalID, "main", "workspace-test-popout")

	if session.stream != stream {
		t.Fatal("expected existing stream to remain attached when stream owner matches")
	}
}

func TestRestartTerminalStreamForHandoffRestartsWhenStreamOwnerDiffers(t *testing.T) {
	originalAttach := attachSessionStream
	t.Cleanup(func() {
		attachSessionStream = originalAttach
	})

	app := NewApp()
	workspaceID := "test-workspace"
	terminalID := "term-1"
	oldStream := &stubTerminalStream{id: "existing"}
	session := newTerminalSession(workspaceID, terminalID, "/tmp")
	session.markReady(nil)
	session.client = &sessiond.Client{}
	session.stream = oldStream
	session.streamCancel = func() {}
	session.streamOwner = "main"
	app.terminals[terminalSessionID(workspaceID, terminalID)] = session

	var attachCalls atomic.Int32
	attachSessionStream = func(
		_ *sessiond.Client,
		_ context.Context,
		_ string,
		_ int64,
		_ bool,
		_ string,
	) (terminalStream, sessiond.StreamMessage, error) {
		attachCalls.Add(1)
		return &stubTerminalStream{id: "replacement", nextErr: errors.New("done")}, sessiond.StreamMessage{Type: "ready"}, nil
	}

	app.restartTerminalStreamForHandoff(workspaceID, terminalID, "workspace-test-popout", "workspace-test-popout")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if attachCalls.Load() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := attachCalls.Load(); got != 1 {
		t.Fatalf("expected stream replay restart to reattach once, got %d", got)
	}
	if got := atomic.LoadInt32(&oldStream.closeCalls); got != 1 {
		t.Fatalf("expected previous stream to close once, got %d", got)
	}
}

func TestRestartTerminalStreamForHandoffRestartsWhenStreamInactive(t *testing.T) {
	originalAttach := attachSessionStream
	t.Cleanup(func() {
		attachSessionStream = originalAttach
	})

	app := NewApp()
	workspaceID := "test-workspace"
	terminalID := "term-1"
	session := newTerminalSession(workspaceID, terminalID, "/tmp")
	session.markReady(nil)
	session.client = &sessiond.Client{}
	app.terminals[terminalSessionID(workspaceID, terminalID)] = session

	var attachCalls atomic.Int32
	attachSessionStream = func(
		_ *sessiond.Client,
		_ context.Context,
		_ string,
		_ int64,
		_ bool,
		_ string,
	) (terminalStream, sessiond.StreamMessage, error) {
		attachCalls.Add(1)
		return &stubTerminalStream{id: "replacement", nextErr: errors.New("done")}, sessiond.StreamMessage{Type: "ready"}, nil
	}

	app.restartTerminalStreamForHandoff(workspaceID, terminalID, "main", "main")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if attachCalls.Load() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := attachCalls.Load(); got != 1 {
		t.Fatalf("expected inactive stream handoff to start stream once, got %d", got)
	}
}

func TestRestartTerminalStreamForHandoffCancelsInFlightAttachOnOwnerChange(t *testing.T) {
	originalAttach := attachSessionStream
	t.Cleanup(func() {
		attachSessionStream = originalAttach
	})

	app := NewApp()
	workspaceID := "test-workspace"
	terminalID := "term-1"
	session := newTerminalSession(workspaceID, terminalID, "/tmp")
	session.markReady(nil)
	session.client = &sessiond.Client{}
	var cancelCalls atomic.Int32
	session.streamCancel = func() {
		cancelCalls.Add(1)
	}
	session.streamOwner = "main"
	app.terminals[terminalSessionID(workspaceID, terminalID)] = session

	var attachCalls atomic.Int32
	attachSessionStream = func(
		_ *sessiond.Client,
		_ context.Context,
		_ string,
		_ int64,
		_ bool,
		_ string,
	) (terminalStream, sessiond.StreamMessage, error) {
		attachCalls.Add(1)
		return &stubTerminalStream{id: "replacement", nextErr: errors.New("done")}, sessiond.StreamMessage{Type: "ready"}, nil
	}

	app.restartTerminalStreamForHandoff(workspaceID, terminalID, "main", "workspace-test-popout")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if attachCalls.Load() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := cancelCalls.Load(); got != 1 {
		t.Fatalf("expected in-flight attach cancel to be called once, got %d", got)
	}
	if got := attachCalls.Load(); got != 1 {
		t.Fatalf("expected owner handoff to reattach once, got %d", got)
	}
}
