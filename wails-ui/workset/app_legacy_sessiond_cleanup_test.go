package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/strantalis/workset/pkg/terminalservice"
)

func startLegacySessiondStubForTest(
	t *testing.T,
	handler func(terminalservice.ControlRequest, func()) terminalservice.ControlResponse,
) (string, func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets not supported on windows")
	}

	socketPath := filepath.Join("/tmp", fmt.Sprintf("workset-legacy-sessiond-%d.sock", time.Now().UnixNano()))
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	var once sync.Once
	stop := func() {
		once.Do(func() {
			_ = ln.Close()
		})
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				var req terminalservice.ControlRequest
				if err := json.NewDecoder(c).Decode(&req); err != nil {
					return
				}
				resp := handler(req, stop)
				_ = json.NewEncoder(c).Encode(resp)
			}(conn)
		}
	}()

	return socketPath, func() {
		stop()
		_ = os.Remove(socketPath)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("legacy sessiond stub did not stop")
		}
	}
}

func TestRetireLegacySessiondSocketPathLeavesActiveServiceAlone(t *testing.T) {
	socketPath, cleanup := startLegacySessiondStubForTest(
		t,
		func(req terminalservice.ControlRequest, _ func()) terminalservice.ControlResponse {
			switch req.Method {
			case "list":
				return terminalservice.ControlResponse{
					OK: true,
					Result: terminalservice.ListResponse{
						Sessions: []terminalservice.SessionInfo{{SessionID: "active-session", Running: true}},
					},
				}
			case "shutdown":
				return terminalservice.ControlResponse{OK: false, Error: "shutdown should not be called"}
			default:
				return terminalservice.ControlResponse{OK: false, Error: "unexpected method"}
			}
		},
	)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := retireLegacySessiondSocketPath(ctx, socketPath); err != nil {
		t.Fatalf("retire legacy sessiond: %v", err)
	}
	if _, err := os.Stat(socketPath); err != nil {
		t.Fatalf("expected active legacy socket to remain, stat err=%v", err)
	}
}

func TestRetireLegacySessiondSocketPathShutsDownIdleService(t *testing.T) {
	shutdownCalls := 0
	socketPath, cleanup := startLegacySessiondStubForTest(
		t,
		func(req terminalservice.ControlRequest, stop func()) terminalservice.ControlResponse {
			switch req.Method {
			case "list":
				return terminalservice.ControlResponse{
					OK:     true,
					Result: terminalservice.ListResponse{Sessions: nil},
				}
			case "shutdown":
				shutdownCalls++
				go stop()
				return terminalservice.ControlResponse{OK: true}
			default:
				return terminalservice.ControlResponse{OK: false, Error: "unexpected method"}
			}
		},
	)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := retireLegacySessiondSocketPath(ctx, socketPath); err != nil {
		t.Fatalf("retire legacy sessiond: %v", err)
	}
	if shutdownCalls != 1 {
		t.Fatalf("expected one shutdown request, got %d", shutdownCalls)
	}
	if _, err := os.Stat(socketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected idle legacy socket to be removed, stat err=%v", err)
	}
}

func TestRetireLegacySessiondSocketPathRemovesStaleSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets not supported on windows")
	}

	socketPath := filepath.Join("/tmp", fmt.Sprintf("workset-legacy-sessiond-stale-%d.sock", time.Now().UnixNano()))
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_ = ln.Close()

	info, statErr := os.Stat(socketPath)
	if errors.Is(statErr, os.ErrNotExist) {
		t.Skip("platform removed unix socket path when listener closed")
	}
	if statErr != nil {
		t.Fatalf("stat stale socket: %v", statErr)
	}
	if info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("expected socket mode, got %v", info.Mode())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := retireLegacySessiondSocketPath(ctx, socketPath); err != nil {
		t.Fatalf("retire stale sessiond socket: %v", err)
	}
	if _, err := os.Stat(socketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected stale socket to be removed, stat err=%v", err)
	}
}

func TestLegacySessiondSocketLooksStale(t *testing.T) {
	if !legacySessiondSocketLooksStale(syscall.ENOENT) {
		t.Fatal("expected ENOENT to be treated as stale socket")
	}
	if !legacySessiondSocketLooksStale(&net.OpError{Err: syscall.ECONNREFUSED}) {
		t.Fatal("expected ECONNREFUSED to be treated as stale socket")
	}
	if legacySessiondSocketLooksStale(context.DeadlineExceeded) {
		t.Fatal("expected deadline exceeded to keep the socket in place")
	}
}
