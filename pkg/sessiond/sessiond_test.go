package sessiond

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSessiondInfo(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	info, err := client.Info(ctx)
	cancel()
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Executable == "" {
		t.Fatalf("expected executable path")
	}
	if info.BinaryHash == "" {
		t.Fatalf("expected binary hash")
	}
	if _, err := os.Stat(info.Executable); err != nil {
		t.Fatalf("stat executable: %v", err)
	}
	hash, err := BinaryHash(info.Executable)
	if err != nil {
		t.Fatalf("hash executable: %v", err)
	}
	if hash != info.BinaryHash {
		t.Fatalf("expected hash %s, got %s", hash, info.BinaryHash)
	}
}

func TestClientPingUsesPing(t *testing.T) {
	socketPath, cleanup := startStubServer(t, func(req ControlRequest) ControlResponse {
		switch req.Method {
		case "ping":
			return ControlResponse{OK: true}
		case "list":
			return ControlResponse{OK: false, Error: "list should not be called"}
		default:
			return ControlResponse{OK: false, Error: "unexpected method"}
		}
	})
	defer cleanup()

	client := NewClient(socketPath)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestClientPingFallsBackToList(t *testing.T) {
	socketPath, cleanup := startStubServer(t, func(req ControlRequest) ControlResponse {
		switch req.Method {
		case "ping":
			return ControlResponse{OK: false, Error: "unknown method \"ping\""}
		case "list":
			return ControlResponse{OK: true}
		default:
			return ControlResponse{OK: false, Error: "unexpected method"}
		}
	})
	defer cleanup()

	client := NewClient(socketPath)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping fallback: %v", err)
	}
}

func startStubServer(t *testing.T, handler func(ControlRequest) ControlResponse) (string, func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets not supported on windows")
	}
	t.Setenv("TMPDIR", "/tmp")
	tmp := t.TempDir()
	socketPath := filepath.Join(tmp, "sessiond.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					return
				}
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				dec := json.NewDecoder(c)
				var req ControlRequest
				if err := dec.Decode(&req); err != nil {
					return
				}
				resp := handler(req)
				_ = json.NewEncoder(c).Encode(resp)
			}(conn)
		}
	}()
	cleanup := func() {
		close(done)
		_ = ln.Close()
		_ = os.Remove(socketPath)
	}
	return socketPath, cleanup
}
