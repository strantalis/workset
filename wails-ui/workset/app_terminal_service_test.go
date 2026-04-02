package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/strantalis/workset/pkg/terminalservice"
)

func startStubTerminalServiceClientForAppTest(
	t *testing.T,
	handler func(terminalservice.ControlRequest, func()) terminalservice.ControlResponse,
) (*terminalservice.Client, func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets not supported on windows")
	}

	socketPath := filepath.Join("/tmp", fmt.Sprintf("workset-app-terminal-service-%d.sock", time.Now().UnixNano()))
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

	client := terminalservice.NewClient(socketPath)
	return client, func() {
		stop()
		_ = os.Remove(socketPath)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("stub terminal service did not stop")
		}
	}
}

func TestValidateTerminalServiceClientAcceptsMatchingBinary(t *testing.T) {
	executable, hash, err := currentTerminalServiceBinary()
	if err != nil {
		t.Fatalf("current binary: %v", err)
	}

	shutdownCalls := 0
	client, cleanup := startStubTerminalServiceClientForAppTest(
		t,
		func(req terminalservice.ControlRequest, _ func()) terminalservice.ControlResponse {
			switch req.Method {
			case "info":
				return terminalservice.ControlResponse{
					OK: true,
					Result: terminalservice.InfoResponse{
						Executable: executable,
						BinaryHash: hash,
					},
				}
			case "shutdown":
				shutdownCalls++
				return terminalservice.ControlResponse{OK: true}
			default:
				return terminalservice.ControlResponse{OK: false, Error: "unexpected method"}
			}
		},
	)
	defer cleanup()

	app := NewApp()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	info, err := app.validateTerminalServiceClient(ctx, client)
	if err != nil {
		t.Fatalf("validate client: %v", err)
	}
	if info.BinaryHash != hash {
		t.Fatalf("expected binary hash %q, got %q", hash, info.BinaryHash)
	}
	if shutdownCalls != 0 {
		t.Fatalf("expected matching service to stay up, got %d shutdown calls", shutdownCalls)
	}
}

func TestValidateTerminalServiceClientShutsDownMismatchedBinary(t *testing.T) {
	shutdownCalls := 0
	client, cleanup := startStubTerminalServiceClientForAppTest(
		t,
		func(req terminalservice.ControlRequest, stop func()) terminalservice.ControlResponse {
			switch req.Method {
			case "info":
				return terminalservice.ControlResponse{
					OK: true,
					Result: terminalservice.InfoResponse{
						Executable: "/Applications/workset.app/Contents/MacOS/workset",
						BinaryHash: "stale-binary-hash",
					},
				}
			case "shutdown":
				shutdownCalls++
				go stop()
				return terminalservice.ControlResponse{OK: true}
			case "ping":
				return terminalservice.ControlResponse{OK: true}
			default:
				return terminalservice.ControlResponse{OK: false, Error: "unexpected method"}
			}
		},
	)
	defer cleanup()

	app := NewApp()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := app.validateTerminalServiceClient(ctx, client); err == nil {
		t.Fatal("expected mismatched binary to be rejected")
	}
	if shutdownCalls != 1 {
		t.Fatalf("expected one shutdown request, got %d", shutdownCalls)
	}
}
