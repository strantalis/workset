package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/strantalis/workset/pkg/terminalservice"
)

func (a *App) getTerminalServiceClient() (*terminalservice.Client, error) {
	return a.getTerminalServiceClientInternal()
}

func (a *App) getTerminalServiceClientInternal() (*terminalservice.Client, error) {
	if a.terminalServiceStart != nil {
		waitCh, leader := a.terminalServiceStart.begin()
		if !leader {
			logTerminalServicef("client_wait_for_start")
			<-waitCh
		} else {
			logTerminalServicef("client_start_begin")
			defer a.terminalServiceStart.end()
		}
	}

	a.terminalServiceMu.Lock()
	client := a.terminalServiceClient
	a.terminalServiceMu.Unlock()
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		err := client.Ping(ctx)
		cancel()
		if err == nil {
			logTerminalServicef("client_ready")
			return client, nil
		}
		logTerminalServicef("client_ping_failed err=%v", err)
		a.stopEmbeddedTerminalService()
	}

	baseCtx, _ := a.serviceContext()
	opts, err := a.terminalServiceOptions()
	if err != nil {
		return nil, err
	}
	if err := a.ensureEmbeddedTerminalServiceStarted(baseCtx, opts); err != nil {
		logTerminalServicef("embedded_start_failed err=%v", err)
		return nil, err
	}

	a.terminalServiceMu.Lock()
	client = a.terminalServiceClient
	a.terminalServiceMu.Unlock()
	if client == nil {
		return nil, errors.New("embedded terminal service unavailable")
	}
	logTerminalServicef("client_ready_after_start")
	return client, nil
}

func (a *App) terminalServiceOptions() (terminalservice.Options, error) {
	opts := terminalservice.DefaultOptions()
	socketPath, err := terminalservice.DefaultSocketPath()
	if err != nil {
		return opts, err
	}
	opts.SocketPath = socketPath

	cfgCtx, svc := a.serviceContext()
	cfg, _, cfgErr := svc.GetConfig(cfgCtx)
	if cfgErr == nil {
		if envTruthy(cfg.Defaults.TerminalProtocolLog) {
			opts.ProtocolLogEnabled = true
		}
		if timeout := strings.TrimSpace(cfg.Defaults.TerminalIdleTimeout); timeout != "" {
			parsed, parseErr := time.ParseDuration(timeout)
			if parseErr != nil {
				logTerminalServicef("idle_timeout_parse_failed value=%q err=%v", timeout, parseErr)
			} else {
				opts.IdleTimeout = parsed
				opts.IdleTimeoutSet = true
			}
		}
	}
	return opts, nil
}

func (a *App) ensureEmbeddedTerminalServiceStarted(ctx context.Context, opts terminalservice.Options) error {
	if opts.SocketPath == "" {
		return errors.New("session socket path required")
	}

	a.terminalServiceMu.Lock()
	if a.terminalServiceClient == nil {
		a.terminalServiceClient = terminalservice.NewClient(opts.SocketPath)
	}
	client := a.terminalServiceClient
	done := a.terminalServiceDone
	if a.terminalServiceServer == nil {
		serverCtx, cancel := context.WithCancel(context.Background())
		server := terminalservice.NewServer(opts)
		server.SetShutdown(cancel)
		done = make(chan struct{})
		a.terminalServiceServer = server
		a.terminalServiceCancel = cancel
		a.terminalServiceDone = done
		go func() {
			defer close(done)
			if err := server.Listen(serverCtx); err != nil {
				logTerminalServicef("embedded_listen_failed socket=%s err=%v", opts.SocketPath, err)
			}
		}()
	}
	a.terminalServiceMu.Unlock()

	deadline := time.Now().Add(5 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}

	var lastErr error
	for time.Now().Before(deadline) {
		pingCtx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		lastErr = client.Ping(pingCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		select {
		case <-done:
			a.stopEmbeddedTerminalService()
			return fmt.Errorf("embedded terminal service stopped before becoming ready: %w", lastErr)
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}

	a.stopEmbeddedTerminalService()
	if lastErr != nil {
		return fmt.Errorf("embedded terminal service did not start: %w", lastErr)
	}
	return errors.New("embedded terminal service did not start")
}

func (a *App) stopEmbeddedTerminalService() {
	a.terminalServiceMu.Lock()
	cancel := a.terminalServiceCancel
	done := a.terminalServiceDone
	a.terminalServiceServer = nil
	a.terminalServiceCancel = nil
	a.terminalServiceDone = nil
	a.terminalServiceClient = nil
	a.terminalServiceInfo = nil
	a.terminalServiceReady = false
	a.terminalServiceMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			logTerminalServicef("embedded_stop_timeout")
		}
	}
}

func (a *App) clearTerminalServiceClient() {
	a.terminalServiceMu.Lock()
	a.terminalServiceClient = nil
	a.terminalServiceInfo = nil
	a.terminalServiceReady = false
	a.terminalServiceMu.Unlock()
}

func (a *App) hasTerminalServiceDescriptorSupport() bool {
	a.terminalServiceMu.Lock()
	defer a.terminalServiceMu.Unlock()
	return a.terminalServiceReady
}

func (a *App) markTerminalServiceDescriptorSupportReady() {
	a.terminalServiceMu.Lock()
	a.terminalServiceReady = true
	a.terminalServiceMu.Unlock()
}

func (a *App) cachedTerminalServiceInfo() (terminalservice.InfoResponse, bool) {
	a.terminalServiceMu.Lock()
	defer a.terminalServiceMu.Unlock()
	if a.terminalServiceInfo == nil {
		return terminalservice.InfoResponse{}, false
	}
	return *a.terminalServiceInfo, true
}

func (a *App) setCachedTerminalServiceInfo(info terminalservice.InfoResponse) {
	info = terminalservice.InfoResponse{
		Executable:     strings.TrimSpace(info.Executable),
		BinaryHash:     strings.TrimSpace(info.BinaryHash),
		WebSocketURL:   strings.TrimSpace(info.WebSocketURL),
		WebSocketToken: strings.TrimSpace(info.WebSocketToken),
	}
	a.terminalServiceMu.Lock()
	a.terminalServiceInfo = &info
	a.terminalServiceMu.Unlock()
}

func (a *App) getTerminalServiceInfo(ctx context.Context) (terminalservice.InfoResponse, error) {
	if cached, ok := a.cachedTerminalServiceInfo(); ok {
		return cached, nil
	}
	client, err := a.getTerminalServiceClient()
	if err != nil {
		return terminalservice.InfoResponse{}, err
	}
	info, err := client.Info(ctx)
	if err != nil {
		return terminalservice.InfoResponse{}, err
	}
	a.setCachedTerminalServiceInfo(info)
	return info, nil
}

type terminalServiceStartState struct {
	mu       sync.Mutex
	starting bool
	waitCh   chan struct{}
}

func (s *terminalServiceStartState) begin() (chan struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.starting {
		return s.waitCh, false
	}
	s.starting = true
	s.waitCh = make(chan struct{})
	return s.waitCh, true
}

func (s *terminalServiceStartState) end() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.starting {
		return
	}
	close(s.waitCh)
	s.starting = false
	s.waitCh = nil
}

func (s *terminalServiceStartState) wait() {
	s.mu.Lock()
	ch := s.waitCh
	s.mu.Unlock()
	if ch != nil {
		<-ch
	}
}

func logTerminalServicef(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("[terminal-service] %s %s\n", time.Now().Format(time.RFC3339Nano), message)
}

type TerminalServiceStatus struct {
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

func (a *App) GetTerminalServiceStatus() TerminalServiceStatus {
	client, err := a.getTerminalServiceClientInternal()
	if err != nil {
		return TerminalServiceStatus{Available: false, Error: err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		return TerminalServiceStatus{Available: false, Error: err.Error()}
	}
	return TerminalServiceStatus{Available: true}
}

func isUnknownTerminalServiceMethodError(err error, method string) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if method != "" && strings.Contains(message, strings.ToLower(fmt.Sprintf("unknown method %q", method))) {
		return true
	}
	return strings.Contains(message, "unknown method")
}

func (a *App) ensureTerminalServiceDescriptorSupport() error {
	if a.hasTerminalServiceDescriptorSupport() {
		return nil
	}
	client, err := a.getTerminalServiceClient()
	if err != nil {
		return err
	}

	probeInspect := func(c *terminalservice.Client) error {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_, err := c.Inspect(ctx, "__descriptor_probe__")
		if err == nil {
			return nil
		}
		if isUnknownTerminalServiceMethodError(err, "inspect") {
			return err
		}
		if strings.Contains(strings.ToLower(err.Error()), "session not found") {
			return nil
		}
		return err
	}

	probeInfo := func(c *terminalservice.Client) error {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		info, err := c.Info(ctx)
		if err == nil {
			a.setCachedTerminalServiceInfo(info)
			return nil
		}
		return err
	}

	if err := probeInspect(client); err != nil {
		return err
	}
	if err := probeInfo(client); err != nil {
		return err
	}

	a.markTerminalServiceDescriptorSupportReady()
	return nil
}

var terminalServiceOnce sync.Once

func ensureTerminalServiceStarted(a *App) {
	terminalServiceOnce.Do(func() {
		go func() {
			_, _ = a.getTerminalServiceClient()
		}()
	})
}
