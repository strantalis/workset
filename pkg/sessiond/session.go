package sessiond

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/strantalis/workset/pkg/unifiedlog"
)

type Session struct {
	id   string
	cwd  string
	cmd  *exec.Cmd
	pty  *os.File
	opts Options

	mu                 sync.Mutex
	outputMu           sync.Mutex
	buffer             *terminalBuffer
	transcriptPath     string
	transcriptFile     *os.File
	transcriptSize     int64
	recordPath         string
	recordFile         *os.File
	recordEnabled      bool
	recordMu           sync.Mutex
	protocolLog        *unifiedlog.Logger
	startedAt          time.Time
	lastActivity       time.Time
	idleTimer          *time.Timer
	closed             bool
	closeReason        string
	inputOwner         string
	onClose            func(*Session)
	subscribers        map[*subscriber]struct{}
	streams            map[string]*subscriber
	subscribersMu      sync.Mutex
	protocolInAPC      bool
	protocolAPCEsc     bool
	protocolPendingEsc bool
	debugInputSeq      atomic.Uint64
	debugOutputSeq     atomic.Uint64
	modeState          terminalModeState
	modeParser         terminalModeParser
}

func newSession(opts Options, id, cwd string) *Session {
	return &Session{
		id:            id,
		cwd:           cwd,
		opts:          opts,
		buffer:        newTerminalBuffer(opts.BufferBytes),
		recordEnabled: opts.RecordPty,
		protocolLog:   opts.ProtocolLogger,
		subscribers:   make(map[*subscriber]struct{}),
		streams:       make(map[string]*subscriber),
	}
}

func (s *Session) info() SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SessionInfo{
		SessionID:  s.id,
		Cwd:        s.cwd,
		StartedAt:  s.startedAt.Format(time.RFC3339),
		LastActive: s.lastActivity.Format(time.RFC3339),
		Running:    s.cmd != nil && !s.closed,
	}
}

func (s *Session) writeForOwner(ctx context.Context, data, owner string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	owner = strings.TrimSpace(owner)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pty == nil {
		return errors.New("terminal not started")
	}
	currentOwner := strings.TrimSpace(s.inputOwner)
	switch {
	case owner == "":
		if currentOwner != "" {
			return fmt.Errorf("terminal input lease held by %q", currentOwner)
		}
	case currentOwner == "":
		s.inputOwner = owner
	case currentOwner != owner:
		return fmt.Errorf("terminal input lease held by %q", currentOwner)
	}
	s.logProtocol(ctx, "in", []byte(data))
	inSeq := s.debugInputSeq.Add(1)
	debugLogf(
		"session_input id=%s seq=%d owner=%q summary=%s",
		s.id,
		inSeq,
		owner,
		summarizeBytes([]byte(data), 48),
	)
	if _, err := s.pty.Write([]byte(data)); err != nil {
		return err
	}
	s.bumpActivityLocked()
	return nil
}

func (s *Session) setInputOwner(owner string) string {
	owner = strings.TrimSpace(owner)
	s.mu.Lock()
	s.inputOwner = owner
	s.mu.Unlock()
	return owner
}

func (s *Session) getInputOwner() string {
	s.mu.Lock()
	owner := strings.TrimSpace(s.inputOwner)
	s.mu.Unlock()
	return owner
}

func (s *Session) resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pty == nil {
		return errors.New("terminal not started")
	}
	if cols < 2 {
		cols = 2
	}
	if rows < 1 {
		rows = 1
	}
	err := resizePTY(s.pty, cols, rows)
	if err == nil {
		debugLogf("session_resize id=%s cols=%d rows=%d", s.id, cols, rows)
	}
	return err
}

func (s *Session) isClosed() bool {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	return closed
}

func (s *Session) isRunning() bool {
	s.mu.Lock()
	running := s.cmd != nil && !s.closed
	s.mu.Unlock()
	return running
}

func (s *Session) bumpActivityLocked() {
	s.lastActivity = time.Now()
	if s.idleTimer != nil {
		_ = s.idleTimer.Stop()
		s.idleTimer.Reset(s.opts.IdleTimeout)
	}
}

func (s *Session) readLoop(ctx context.Context) {
	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			s.closeWithReason("context_done")
			return
		}
		n, err := s.pty.Read(buf)
		if n > 0 {
			s.handleProtocolOutput(ctx, buf[:n])
		}
		if err != nil {
			s.closeWithReason("closed")
			return
		}
	}
}
