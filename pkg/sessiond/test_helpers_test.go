package sessiond

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func startTestServer(t *testing.T) (*Client, func()) {
	return startTestServerWithOptions(t, nil)
}

func startTestServerWithOptions(t *testing.T, mutate func(*Options)) (*Client, func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("pty not supported on windows")
	}
	t.Setenv("TMPDIR", "/tmp")
	tmp := t.TempDir()
	socketPath := filepath.Join(tmp, "sessiond.sock")
	opts := DefaultOptions()
	opts.SocketPath = socketPath
	opts.TranscriptDir = filepath.Join(tmp, "terminal_logs")
	opts.RecordDir = filepath.Join(tmp, "terminal_records")
	if mutate != nil {
		mutate(&opts)
	}

	server := NewServer(opts)
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

	client := NewClient(socketPath)
	cleanup := func() {
		cancel()
		select {
		case <-errCh:
		case <-time.After(2 * time.Second):
		}
	}
	return client, cleanup
}
