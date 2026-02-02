package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/strantalis/workset/pkg/kitty"
	"github.com/strantalis/workset/pkg/sessiond"
)

type TerminalPayload struct {
	WorkspaceID string `json:"workspaceId"`
	TerminalID  string `json:"terminalId"`
	Data        string `json:"data"`
	Bytes       int    `json:"bytes"`
}

type TerminalLifecyclePayload struct {
	WorkspaceID string `json:"workspaceId"`
	TerminalID  string `json:"terminalId"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
}

type TerminalKittyPayload struct {
	WorkspaceID string      `json:"workspaceId"`
	TerminalID  string      `json:"terminalId"`
	Event       kitty.Event `json:"event"`
}

type TerminalBacklogPayload struct {
	WorkspaceID string `json:"workspaceId"`
	TerminalID  string `json:"terminalId"`
	Data        string `json:"data"`
	NextOffset  int64  `json:"nextOffset"`
	Truncated   bool   `json:"truncated"`
	Source      string `json:"source,omitempty"`
}

type TerminalSnapshotPayload struct {
	WorkspaceID string          `json:"workspaceId"`
	TerminalID  string          `json:"terminalId"`
	Data        string          `json:"data"`
	Source      string          `json:"source,omitempty"`
	Kitty       *kitty.Snapshot `json:"kitty,omitempty"`
}

type TerminalBootstrapPayload struct {
	WorkspaceID      string          `json:"workspaceId"`
	TerminalID       string          `json:"terminalId"`
	Snapshot         string          `json:"snapshot,omitempty"`
	SnapshotSource   string          `json:"snapshotSource,omitempty"`
	Kitty            *kitty.Snapshot `json:"kitty,omitempty"`
	Backlog          string          `json:"backlog,omitempty"`
	BacklogSource    string          `json:"backlogSource,omitempty"`
	BacklogTruncated bool            `json:"backlogTruncated,omitempty"`
	NextOffset       int64           `json:"nextOffset,omitempty"`
	Source           string          `json:"source,omitempty"`
	AltScreen        bool            `json:"altScreen,omitempty"`
	Mouse            bool            `json:"mouse,omitempty"`
	MouseSGR         bool            `json:"mouseSGR,omitempty"`
	MouseEncoding    string          `json:"mouseEncoding,omitempty"`
	SafeToReplay     bool            `json:"safeToReplay,omitempty"`
	InitialCredit    int64           `json:"initialCredit,omitempty"`
}

type TerminalBootstrapDonePayload struct {
	WorkspaceID string `json:"workspaceId"`
	TerminalID  string `json:"terminalId"`
}

type TerminalStatusPayload struct {
	WorkspaceID string `json:"workspaceId"`
	TerminalID  string `json:"terminalId,omitempty"`
	Active      bool   `json:"active"`
	Error       string `json:"error,omitempty"`
}

type TerminalModesPayload struct {
	WorkspaceID   string `json:"workspaceId"`
	TerminalID    string `json:"terminalId"`
	AltScreen     bool   `json:"altScreen"`
	Mouse         bool   `json:"mouse"`
	MouseSGR      bool   `json:"mouseSGR"`
	MouseEncoding string `json:"mouseEncoding"`
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

type terminalState struct {
	Sessions []terminalStateEntry `json:"sessions"`
}

type terminalStateEntry struct {
	WorkspaceID string             `json:"workspaceId"`
	TerminalID  string             `json:"terminalId,omitempty"`
	LastActive  time.Time          `json:"lastActive"`
	Modes       *terminalModeState `json:"modes,omitempty"`
}

type terminalModeState struct {
	AltScreen  bool  `json:"altScreen,omitempty"`
	MouseMask  uint8 `json:"mouseMask,omitempty"`
	MouseSGR   bool  `json:"mouseSGR,omitempty"`
	MouseUTF8  bool  `json:"mouseUTF8,omitempty"`
	MouseURXVT bool  `json:"mouseURXVT,omitempty"`
}

type terminalSession struct {
	id          string
	workspaceID string
	terminalID  string
	path        string
	mu          sync.Mutex

	client       *sessiond.Client
	stream       *sessiond.Stream
	streamCancel context.CancelFunc
	streamID     string
	detaching    bool

	altScreen  bool
	mouseMask  uint8
	mouseSGR   bool
	mouseUTF8  bool
	mouseURXVT bool

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
	if terminalID == "" {
		return workspaceID
	}
	return workspaceID + terminalSessionSeparator + terminalID
}

func parseTerminalSessionID(sessionID string) (string, string, bool) {
	parts := strings.SplitN(sessionID, terminalSessionSeparator, 2)
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return sessionID, "", false
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

func (s *terminalSession) snapshot() (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.starting || s.closed || s.client == nil {
		return time.Time{}, false
	}
	return s.lastActivity, true
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

func (s *terminalSession) mouseEnabled() bool {
	return s.mouseMask != 0
}

func (s *terminalSession) mouseEncoding() string {
	if s.mouseSGR {
		return "sgr"
	}
	if s.mouseURXVT {
		return "urxvt"
	}
	if s.mouseUTF8 {
		return "utf8"
	}
	return "x10"
}

func (s *terminalSession) Write(data string) error {
	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := client.Send(ctx, s.id, data)
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
	s.streamID = ""
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
