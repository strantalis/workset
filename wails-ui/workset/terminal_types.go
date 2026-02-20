package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/strantalis/workset/pkg/sessiond"
)

type TerminalPayload struct {
	WorkspaceID string `json:"workspaceId"`
	TerminalID  string `json:"terminalId"`
	WindowName  string `json:"windowName,omitempty"`
	DataB64     string `json:"dataB64,omitempty"`
	Bytes       int    `json:"bytes"`
	Seq         int64  `json:"seq,omitempty"`
}

type TerminalStatusPayload struct {
	WorkspaceID string `json:"workspaceId"`
	TerminalID  string `json:"terminalId,omitempty"`
	Active      bool   `json:"active"`
	Error       string `json:"error,omitempty"`
}

type TerminalDebugPayload struct {
	WorkspaceID string `json:"workspaceId"`
	TerminalID  string `json:"terminalId"`
	Event       string `json:"event"`
	Details     string `json:"details,omitempty"`
}

type TerminalCreatePayload struct {
	WorkspaceID string `json:"workspaceId"`
	TerminalID  string `json:"terminalId"`
}

type terminalStream interface {
	Next(*sessiond.StreamMessage) error
	ID() string
	Close() error
}

type terminalSession struct {
	id          string
	workspaceID string
	terminalID  string
	path        string
	mu          sync.Mutex

	client       *sessiond.Client
	stream       terminalStream
	streamCancel context.CancelFunc
	streamOwner  string

	starting bool
	startErr error
	ready    chan struct{}

	lastActivity time.Time
	idleTimeout  time.Duration
	idleTimer    *time.Timer
	closed       bool
	closeReason  string
	resumed      bool
}

const terminalSessionSeparator = "::"

func terminalSessionID(workspaceID, terminalID string) string {
	workspaceID = strings.TrimSpace(workspaceID)
	terminalID = strings.TrimSpace(terminalID)
	return workspaceID + terminalSessionSeparator + terminalID
}

func newTerminalSession(workspaceID, terminalID, path string) *terminalSession {
	return &terminalSession{
		id:          terminalSessionID(workspaceID, terminalID),
		workspaceID: workspaceID,
		terminalID:  terminalID,
		path:        path,
		starting:    true,
		ready:       make(chan struct{}),
	}
}

func (s *terminalSession) markReady(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startErr = err
	s.starting = false
	if s.ready != nil {
		close(s.ready)
		s.ready = nil
	}
}

func (s *terminalSession) waitReady(ctx context.Context) error {
	s.mu.Lock()
	if !s.starting {
		err := s.startErr
		s.mu.Unlock()
		return err
	}
	ch := s.ready
	s.mu.Unlock()
	select {
	case <-ch:
		s.mu.Lock()
		err := s.startErr
		s.mu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *terminalSession) bumpActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.lastActivity = time.Now()
	if s.idleTimer != nil {
		_ = s.idleTimer.Stop()
		s.idleTimer.Reset(s.idleTimeout)
	}
}

func (s *terminalSession) Write(data string) error {
	return s.WriteAsOwner(data, "")
}

func (s *terminalSession) WriteAsOwner(data, owner string) error {
	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := client.SendWithOwner(ctx, s.id, data, owner)
		if err == nil {
			s.bumpActivity()
		}
		if err != nil && strings.Contains(err.Error(), "session not found") {
			s.mu.Lock()
			s.client = nil
			s.mu.Unlock()
		}
		return err
	}
	return fmt.Errorf("terminal not started")
}

func (s *terminalSession) Resize(cols, rows int) error {
	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := client.Resize(ctx, s.id, cols, rows)
		if err != nil && strings.Contains(err.Error(), "session not found") {
			s.mu.Lock()
			s.client = nil
			s.mu.Unlock()
		}
		return err
	}
	return fmt.Errorf("terminal not started")
}

func (s *terminalSession) Close() error {
	return s.CloseWithReason("closed")
}

func (s *terminalSession) releaseStream(stream terminalStream) (releasedCurrent bool) {
	if stream != nil {
		_ = stream.Close()
	}
	s.mu.Lock()
	if s.stream == stream {
		s.stream = nil
		s.streamCancel = nil
		s.streamOwner = ""
		releasedCurrent = true
	}
	s.mu.Unlock()
	return releasedCurrent
}

func (s *terminalSession) CloseWithReason(reason string) error {
	s.mu.Lock()
	if s.streamCancel != nil {
		s.streamCancel()
		s.streamCancel = nil
	}
	if s.stream != nil {
		_ = s.stream.Close()
		s.stream = nil
	}
	s.streamOwner = ""
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if reason != "" {
		s.closeReason = reason
	}
	if s.idleTimer != nil {
		_ = s.idleTimer.Stop()
		s.idleTimer = nil
	}
	return nil
}
