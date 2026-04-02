package terminalservice

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type Client struct {
	socketPath string
	timeout    time.Duration
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

func (c *Client) Inspect(ctx context.Context, sessionID string) (InspectResponse, error) {
	var resp InspectResponse
	err := c.call(ctx, "inspect", InspectRequest{SessionID: sessionID}, &resp)
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
	req := ControlRequest{ProtocolVersion: ProtocolVersion, Method: method}
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
		return errors.New("terminal service request failed")
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

func (c *Client) Ping(ctx context.Context) error {
	if err := c.call(ctx, "ping", struct{}{}, nil); err == nil {
		return nil
	} else if strings.Contains(err.Error(), "unknown method") {
		return c.call(ctx, "list", struct{}{}, nil)
	} else {
		return err
	}
}

var streamCounter int64

func newStreamID() string {
	seq := atomic.AddInt64(&streamCounter, 1)
	return fmt.Sprintf("stream-%d-%d", time.Now().UnixNano(), seq)
}

func (c *Client) Info(ctx context.Context) (InfoResponse, error) {
	var resp InfoResponse
	err := c.call(ctx, "info", struct{}{}, &resp)
	return resp, err
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
