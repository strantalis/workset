package sessiond

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

type Client struct {
	socketPath string
	timeout    time.Duration
}

type Stream struct {
	conn net.Conn
	dec  *json.Decoder
	id   string
}

func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		timeout:    2 * time.Second,
	}
}

func (c *Client) Create(ctx context.Context, sessionID, cwd string) (CreateResponse, error) {
	var resp CreateResponse
	err := c.call(ctx, "create", CreateRequest{SessionID: sessionID, Cwd: cwd}, &resp)
	return resp, err
}

func (c *Client) Send(ctx context.Context, sessionID, data string) error {
	return c.call(ctx, "send", SendRequest{SessionID: sessionID, Data: data}, nil)
}

func (c *Client) Resize(ctx context.Context, sessionID string, cols, rows int) error {
	return c.call(ctx, "resize", ResizeRequest{SessionID: sessionID, Cols: cols, Rows: rows}, nil)
}

func (c *Client) Stop(ctx context.Context, sessionID string) error {
	return c.call(ctx, "stop", StopRequest{SessionID: sessionID}, nil)
}

func (c *Client) Backlog(ctx context.Context, sessionID string, since int64) (BacklogResponse, error) {
	var resp BacklogResponse
	err := c.call(ctx, "backlog", BacklogRequest{SessionID: sessionID, Since: since}, &resp)
	return resp, err
}

func (c *Client) Snapshot(ctx context.Context, sessionID string) (SnapshotResponse, error) {
	var resp SnapshotResponse
	err := c.call(ctx, "snapshot", SnapshotRequest{SessionID: sessionID}, &resp)
	return resp, err
}

func (c *Client) Bootstrap(ctx context.Context, sessionID string) (BootstrapResponse, error) {
	var resp BootstrapResponse
	err := c.call(ctx, "bootstrap", BootstrapRequest{SessionID: sessionID}, &resp)
	return resp, err
}

func (c *Client) List(ctx context.Context) (ListResponse, error) {
	var resp ListResponse
	err := c.call(ctx, "list", struct{}{}, &resp)
	return resp, err
}

func (c *Client) Shutdown(ctx context.Context) error {
	return c.ShutdownWithReason(ctx, "unknown", "")
}

func (c *Client) ShutdownWithReason(ctx context.Context, source, reason string) error {
	req := shutdownRequest(source, reason)
	return c.call(ctx, "shutdown", req, nil)
}

func (c *Client) Attach(ctx context.Context, sessionID string, since int64, withBuffer bool, streamID string) (*Stream, StreamMessage, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, StreamMessage{}, err
	}
	if streamID == "" {
		streamID = newStreamID()
	}
	enc := json.NewEncoder(conn)
	if err := enc.Encode(AttachRequest{
		Type:       "attach",
		SessionID:  sessionID,
		StreamID:   streamID,
		Since:      since,
		WithBuffer: withBuffer,
	}); err != nil {
		_ = conn.Close()
		return nil, StreamMessage{}, err
	}
	dec := json.NewDecoder(bufio.NewReader(conn))
	var first StreamMessage
	if err := dec.Decode(&first); err != nil {
		_ = conn.Close()
		return nil, StreamMessage{}, err
	}
	return &Stream{conn: conn, dec: dec, id: streamID}, first, nil
}

func (s *Stream) Next(msg *StreamMessage) error {
	if s == nil || s.dec == nil {
		return errors.New("stream closed")
	}
	return s.dec.Decode(msg)
}

func (s *Stream) ID() string {
	if s == nil {
		return ""
	}
	return s.id
}

func (s *Stream) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (c *Client) Ack(ctx context.Context, sessionID, streamID string, bytes int64) error {
	if bytes <= 0 {
		return nil
	}
	return c.call(ctx, "ack", AckRequest{SessionID: sessionID, StreamID: streamID, Bytes: bytes}, nil)
}

func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	if err := applyDeadline(ctx, conn); err != nil {
		_ = conn.Close()
		return err
	}
	defer func() {
		_ = conn.Close()
	}()
	enc := json.NewEncoder(conn)
	req := ControlRequest{Method: method}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return err
		}
		req.Params = raw
	}
	if err := enc.Encode(req); err != nil {
		return err
	}
	dec := json.NewDecoder(bufio.NewReader(conn))
	var resp ControlResponse
	if err := dec.Decode(&resp); err != nil {
		return err
	}
	if !resp.OK {
		if resp.Error != "" {
			return errors.New(resp.Error)
		}
		return errors.New("sessiond request failed")
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

func (c *Client) dial(ctx context.Context) (net.Conn, error) {
	dialer := net.Dialer{Timeout: c.timeout}
	return dialer.DialContext(ctx, "unix", c.socketPath)
}

func applyDeadline(ctx context.Context, conn net.Conn) error {
	if conn == nil {
		return nil
	}
	if deadline, ok := ctx.Deadline(); ok {
		return conn.SetDeadline(deadline)
	}
	return nil
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

var streamCounter int64

func newStreamID() string {
	seq := atomic.AddInt64(&streamCounter, 1)
	return fmt.Sprintf("stream-%d-%d", time.Now().UnixNano(), seq)
}

func EnsureRunning(ctx context.Context) (*Client, error) {
	return EnsureRunningWithOptions(ctx, StartOptions{})
}

type StartOptions struct {
	ProtocolLogEnabled bool
	ProtocolLogDir     string
}

func EnsureRunningWithOptions(ctx context.Context, opts StartOptions) (*Client, error) {
	socketPath, err := DefaultSocketPath()
	if err != nil {
		return nil, err
	}
	client := NewClient(socketPath)
	if err := client.Ping(ctx); err == nil {
		return client, nil
	} else if isTimeoutErr(err) {
		// Treat an unresponsive daemon as stale so we can replace it.
		_ = os.Remove(socketPath)
	}
	if err := removeStaleSocket(socketPath); err != nil {
		return nil, err
	}
	if err := startDaemon(ctx, socketPath, opts); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	for time.Now().Before(deadline) {
		if err := client.Ping(ctx); err == nil {
			return client, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, errors.New("sessiond did not start")
}

func (c *Client) Ping(ctx context.Context) error {
	if err := c.call(ctx, "ping", struct{}{}, nil); err == nil {
		return nil
	} else if strings.Contains(err.Error(), "unknown method") {
		return c.call(ctx, "list", struct{}{}, nil)
	} else {
		return err
	}
}

func (c *Client) Info(ctx context.Context) (InfoResponse, error) {
	var resp InfoResponse
	err := c.call(ctx, "info", struct{}{}, &resp)
	return resp, err
}

func removeStaleSocket(path string) error {
	conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	_ = os.Remove(path)
	return nil
}

func startDaemon(_ context.Context, socketPath string, opts StartOptions) error {
	bin, err := FindSessiondBinary()
	if err != nil {
		return errors.New("workset-sessiond not found in PATH")
	}
	logPath, err := daemonLogPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	args := []string{"--socket", socketPath}
	if opts.ProtocolLogEnabled {
		args = append(args, "--verbose")
		if strings.TrimSpace(opts.ProtocolLogDir) != "" {
			args = append(args, "--protocol-log-dir", opts.ProtocolLogDir)
		}
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), "WORKSET_SESSIOND_LOG="+logPath)
	if attr := daemonSysProcAttr(); attr != nil {
		// Detach from the parent process group so "wails dev" shutdown
		// doesn't tear down the session daemon.
		cmd.SysProcAttr = attr
	}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()
	return nil
}

func shutdownRequest(source, reason string) ShutdownRequest {
	req := ShutdownRequest{
		Source: strings.TrimSpace(source),
		Reason: strings.TrimSpace(reason),
		PID:    os.Getpid(),
	}
	if req.Source == "" {
		req.Source = "unknown"
	}
	if exe, err := os.Executable(); err == nil {
		req.Executable = exe
	}
	return req
}

func FindSessiondBinary() (string, error) {
	if env := os.Getenv("WORKSET_SESSIOND_PATH"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env, nil
		}
	}
	if bin, err := exec.LookPath("workset-sessiond"); err == nil {
		return bin, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeDir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(exeDir, "workset-sessiond"),
	}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates,
			filepath.Join(exeDir, "..", "Resources", "workset-sessiond"),
			filepath.Join(exeDir, "..", "MacOS", "workset-sessiond"),
		)
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("workset-sessiond not found")
}

func daemonLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "sessiond.log"), nil
}

func WaitForShutdown(ctx context.Context, socketPath string) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, err := net.DialTimeout("unix", socketPath, 200*time.Millisecond)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			_ = os.Remove(socketPath)
			return nil
		}
		_ = conn.Close()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
