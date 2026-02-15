package sessiond

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentCreateSinglePTY(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty not supported on windows")
	}
	orig := startPTYFunc
	var starts int32
	startPTYFunc = func(cmd *exec.Cmd) (*os.File, error) {
		atomic.AddInt32(&starts, 1)
		return orig(cmd)
	}
	t.Cleanup(func() {
		startPTYFunc = orig
	})

	client, cleanup := startTestServer(t)
	defer cleanup()

	const workers = 8
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := client.Create(ctx, "concurrent-test", "/tmp")
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
	}

	if got := atomic.LoadInt32(&starts); got != 1 {
		t.Fatalf("expected 1 PTY start, got %d", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	list, err := client.List(ctx)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list.Sessions))
	}
	if list.Sessions[0].SessionID != "concurrent-test" {
		t.Fatalf("unexpected session id %q", list.Sessions[0].SessionID)
	}
}

func TestIdleCloseRecreateSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty not supported on windows")
	}
	client, _, cleanup := startTestServerWithOptionsAndServer(t, func(opts *Options) {
		opts.IdleTimeout = 200 * time.Millisecond
	})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, "idle-test", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if !waitForSessionGone(t, client, "idle-test", 5*time.Second) {
		t.Fatalf("expected idle session to be removed")
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	resp, err := client.Create(ctx2, "idle-test", "/tmp")
	if err != nil {
		t.Fatalf("create session after idle: %v", err)
	}
	if resp.Existing {
		t.Fatalf("expected new session after idle close")
	}
}

func TestStopRemovesSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty not supported on windows")
	}
	client, _, cleanup := startTestServerWithOptionsAndServer(t, nil)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, "stop-test", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := client.Stop(stopCtx, "stop-test"); err != nil {
		t.Fatalf("stop session: %v", err)
	}
	if !waitForSessionGone(t, client, "stop-test", 5*time.Second) {
		t.Fatalf("expected stopped session to be removed")
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	resp, err := client.Create(ctx2, "stop-test", "/tmp")
	if err != nil {
		t.Fatalf("create session after stop: %v", err)
	}
	if resp.Existing {
		t.Fatalf("expected new session after stop")
	}
}

func TestAttachFailsWhenSessionNotRunning(t *testing.T) {
	opts := DefaultOptions()
	session := newSession(opts, "not-running", "/tmp")
	server := &Server{
		opts:     opts,
		sessions: map[string]*Session{"not-running": session},
	}

	clientConn, serverConn := net.Pipe()
	defer func() {
		_ = clientConn.Close()
	}()

	attachLine, err := json.Marshal(AttachRequest{
		ProtocolVersion: ProtocolVersion,
		Type:            "attach",
		SessionID:       "not-running",
		StreamID:        "not-running-stream",
	})
	if err != nil {
		t.Fatalf("marshal attach: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = serverConn.Close() }()
		server.handleAttach(serverConn, attachLine)
	}()

	dec := json.NewDecoder(clientConn)
	var first StreamMessage
	if err := dec.Decode(&first); err != nil {
		t.Fatalf("attach response: %v", err)
	}
	if first.Type != "error" {
		t.Fatalf("expected error, got %+v", first)
	}
	if !strings.Contains(first.Error, "not running") {
		t.Fatalf("expected not running error, got %q", first.Error)
	}
	<-done
}

func TestAttachStreamsLiveEvents(t *testing.T) {
	opts := DefaultOptions()
	session := newSession(opts, "running-session", "/tmp")
	session.cmd = &exec.Cmd{}
	server := &Server{
		opts:     opts,
		sessions: map[string]*Session{"running-session": session},
	}

	clientConn, serverConn := net.Pipe()
	defer func() {
		_ = clientConn.Close()
	}()

	attachLine, err := json.Marshal(AttachRequest{
		ProtocolVersion: ProtocolVersion,
		Type:            "attach",
		SessionID:       "running-session",
		StreamID:        "running-stream",
	})
	if err != nil {
		t.Fatalf("marshal attach: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = serverConn.Close() }()
		server.handleAttach(serverConn, attachLine)
	}()

	dec := json.NewDecoder(clientConn)
	var first StreamMessage
	if err := dec.Decode(&first); err != nil {
		t.Fatalf("attach ready response: %v", err)
	}
	if first.Type != "ready" {
		t.Fatalf("expected ready, got %+v", first)
	}

	session.broadcast([]byte("hello"))

	var second StreamMessage
	if err := dec.Decode(&second); err != nil {
		t.Fatalf("attach data response: %v", err)
	}
	if second.Type != "data" {
		t.Fatalf("expected data event, got %+v", second)
	}
	payload, err := base64.StdEncoding.DecodeString(second.DataB64)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if !strings.Contains(string(payload), "hello") {
		t.Fatalf("expected payload to include terminal content, got %q", string(payload))
	}

	_ = clientConn.Close()
	session.closeSubscribers()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("attach handler did not exit")
	}
}

func waitForSessionGone(t *testing.T, client *Client, sessionID string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		list, err := client.List(ctx)
		cancel()
		if err == nil {
			found := false
			for _, session := range list.Sessions {
				if session.SessionID == sessionID {
					found = true
					break
				}
			}
			if !found {
				return true
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func startTestServerWithOptionsAndServer(t *testing.T, mutate func(*Options)) (*Client, *Server, func()) {
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
	return client, server, cleanup
}
