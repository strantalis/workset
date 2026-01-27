package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/strantalis/workset/pkg/sessiond"
)

func TestGetTerminalBootstrapSessiond(t *testing.T) {
	client, cleanup := startSessiondServer(t)
	defer cleanup()

	createCtx, createCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer createCancel()
	sessionID := terminalSessionID("bootstrap-app", "test")
	if _, err := client.Create(createCtx, sessionID, "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	sendCtx, sendCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer sendCancel()
	if err := client.Send(sendCtx, sessionID, "printf '\\033[?1000h\\033[?1006h\\033[?1049hREADY\\n'\n"); err != nil {
		t.Fatalf("send output: %v", err)
	}

	if !waitForSessiondSnapshot(t, client, sessionID, "READY", 3*time.Second) {
		t.Fatalf("snapshot did not contain expected output")
	}

	app := NewApp()
	app.sessiondClient = client
	app.ctx = context.Background()

	bootstrap, err := app.GetTerminalBootstrap("bootstrap-app", "test")
	if err != nil {
		t.Fatalf("GetTerminalBootstrap: %v", err)
	}
	if !strings.Contains(bootstrap.Snapshot, "READY") && !strings.Contains(bootstrap.Backlog, "READY") {
		t.Fatalf("expected snapshot or backlog to contain READY, got snapshot=%q backlog=%q",
			bootstrap.Snapshot, bootstrap.Backlog)
	}
	if bootstrap.Source != "sessiond" {
		t.Fatalf("expected source sessiond, got %q", bootstrap.Source)
	}
	if !bootstrap.AltScreen || !bootstrap.Mouse || !bootstrap.MouseSGR {
		t.Fatalf("expected modes to be propagated, got alt=%v mouse=%v sgr=%v",
			bootstrap.AltScreen, bootstrap.Mouse, bootstrap.MouseSGR)
	}
	if bootstrap.SafeToReplay {
		t.Fatalf("expected safeToReplay to be false for alt screen session")
	}
	if bootstrap.MouseEncoding != "sgr" {
		t.Fatalf("expected mouse encoding sgr, got %q", bootstrap.MouseEncoding)
	}
}

func startSessiondServer(t *testing.T) (*sessiond.Client, func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("pty not supported on windows")
	}
	tmp, err := os.MkdirTemp("/tmp", "workset-sessiond-app-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	socketPath := filepath.Join(tmp, "sessiond.sock")
	opts := sessiond.DefaultOptions()
	opts.SocketPath = socketPath
	opts.TranscriptDir = filepath.Join(tmp, "terminal_logs")
	opts.RecordDir = filepath.Join(tmp, "terminal_records")
	opts.StateDir = filepath.Join(tmp, "terminal_state")

	server := sessiond.NewServer(opts)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Listen(ctx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("sessiond listen failed: %v", err)
			}
			t.Fatalf("sessiond stopped before socket was ready")
		default:
		}
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("sessiond socket not ready")
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("PS1", "")

	client := sessiond.NewClient(socketPath)
	cleanup := func() {
		cancel()
		select {
		case <-errCh:
		case <-time.After(2 * time.Second):
		}
		_ = os.RemoveAll(tmp)
	}
	return client, cleanup
}

func waitForSessiondSnapshot(t *testing.T, client *sessiond.Client, sessionID, needle string, timeout time.Duration) bool {
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
