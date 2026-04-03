package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/strantalis/workset/pkg/terminalservice"
)

func ensureLegacySessiondRetired() {
	if runtime.GOOS == "windows" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := retireLegacySessiondSocket(ctx); err != nil {
		warnTerminalServicef("legacy_sessiond_cleanup_failed err=%v", err)
	}
}

func retireLegacySessiondSocket(ctx context.Context) error {
	socketPath, err := legacySessiondSocketPath()
	if err != nil {
		return err
	}
	return retireLegacySessiondSocketPath(ctx, socketPath)
}

func legacySessiondSocketPath() (string, error) {
	if socket := strings.TrimSpace(os.Getenv("WORKSET_SESSIOND_SOCKET")); socket != "" {
		return socket, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "sessiond.sock"), nil
}

func retireLegacySessiondSocketPath(ctx context.Context, socketPath string) error {
	socketPath = strings.TrimSpace(socketPath)
	if socketPath == "" {
		return nil
	}

	info, err := os.Stat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return nil
	}

	var list terminalservice.ListResponse
	if err := legacySessiondControl(ctx, socketPath, "list", struct{}{}, &list); err != nil {
		if !legacySessiondSocketLooksStale(err) {
			return err
		}
		if removeErr := removeLegacySessiondSocket(socketPath); removeErr != nil {
			return removeErr
		}
		debugTerminalServicef("legacy_sessiond_socket_removed path=%s", socketPath)
		return nil
	}

	if len(list.Sessions) > 0 {
		debugTerminalServicef("legacy_sessiond_active path=%s sessions=%d", socketPath, len(list.Sessions))
		return nil
	}

	reason := "legacy sessiond retirement"
	executable, err := os.Executable()
	if err != nil {
		executable = ""
	}
	shutdown := terminalservice.ShutdownRequest{
		Source:     "workset",
		Reason:     reason,
		PID:        os.Getpid(),
		Executable: executable,
	}
	if err := legacySessiondControl(ctx, socketPath, "shutdown", shutdown, nil); err != nil {
		return err
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); errors.Is(err, os.ErrNotExist) {
			debugTerminalServicef("legacy_sessiond_retired path=%s", socketPath)
			return nil
		}
		if err := legacySessiondControl(ctx, socketPath, "list", struct{}{}, nil); err != nil {
			if removeErr := removeLegacySessiondSocket(socketPath); removeErr != nil {
				return removeErr
			}
			debugTerminalServicef("legacy_sessiond_retired path=%s", socketPath)
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	warnTerminalServicef("legacy_sessiond_shutdown_timeout path=%s", socketPath)
	return nil
}

func legacySessiondControl(
	ctx context.Context,
	socketPath string,
	method string,
	params any,
	out any,
) error {
	dialer := net.Dialer{Timeout: 250 * time.Millisecond}
	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	req := terminalservice.ControlRequest{
		ProtocolVersion: terminalservice.ProtocolVersion,
		Method:          method,
	}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return err
		}
		req.Params = raw
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}

	var resp terminalservice.ControlResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return err
	}
	if !resp.OK {
		if resp.Error != "" {
			return errors.New(resp.Error)
		}
		return errors.New("legacy sessiond request failed")
	}

	if out != nil && resp.Result != nil {
		raw, err := json.Marshal(resp.Result)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return err
		}
	}
	return nil
}

func legacySessiondSocketLooksStale(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ENOENT) || errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return true
		}
	}
	return false
}

func removeLegacySessiondSocket(socketPath string) error {
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
