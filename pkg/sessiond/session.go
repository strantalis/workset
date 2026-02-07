package sessiond

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/strantalis/workset/pkg/kitty"
	"github.com/strantalis/workset/pkg/termemu"
	"github.com/strantalis/workset/pkg/unifiedlog"
)

type Session struct {
	id   string
	cwd  string
	cmd  *exec.Cmd
	pty  *os.File
	opts Options

	mu               sync.Mutex
	buffer           *terminalBuffer
	transcriptPath   string
	transcriptFile   *os.File
	transcriptSize   int64
	recordPath       string
	recordFile       *os.File
	recordEnabled    bool
	recordMu         sync.Mutex
	tuiMode          bool
	altScreen        bool
	mouseMask        uint8
	mouseSGR         bool
	mouseUTF8        bool
	mouseURXVT       bool
	seqTail          []byte
	c1Normalizer     c1Normalizer
	escapeFilter     escapeStringFilter
	protocolLog      *unifiedlog.Logger
	kittyDecoder     kitty.Decoder
	kittyState       *kitty.State
	kittyStatePath   string
	emu              *termemu.Terminal
	statePath        string
	modesPath        string
	lastSnapshot     time.Time
	snapshotEvery    time.Duration
	snapshotMu       sync.Mutex
	snapshotInFlight bool
	streamInitial    int64
	streamTimeout    time.Duration
	startedAt        time.Time
	lastActivity     time.Time
	idleTimer        *time.Timer
	closed           bool
	closeReason      string
	onClose          func(*Session)
	subscribers      map[*subscriber]struct{}
	streams          map[string]*subscriber
	subscribersMu    sync.Mutex
}

func (s *Session) bootstrap() (BootstrapResponse, error) {
	snap := s.snapshot()
	resp := BootstrapResponse{
		SessionID:      snap.SessionID,
		Snapshot:       snap.Data,
		SnapshotSource: snap.Source,
		Kitty:          snap.Kitty,
		AltScreen:      snap.AltScreen,
		MouseMask:      snap.MouseMask,
		Mouse:          snap.Mouse,
		MouseSGR:       snap.MouseSGR,
		MouseEncoding:  snap.MouseEncoding,
		SafeToReplay:   snap.SafeToReplay,
		InitialCredit:  s.streamInitial,
	}
	altScreen := false
	mouseMask := uint8(0)
	mouseEnabled := false
	mouseSGR := false
	mouseEncoding := ""
	safeToReplay := false
	s.mu.Lock()
	altScreen = s.altScreen
	mouseMask = s.mouseMask
	mouseEnabled = mouseMask != 0
	mouseSGR = s.mouseSGR
	mouseEncoding = s.mouseEncoding()
	tuiMode := s.tuiMode
	s.mu.Unlock()
	if s.emu != nil && s.emu.IsAltScreen() {
		altScreen = true
	}
	safeToReplay = !altScreen && !tuiMode
	resp.AltScreen = altScreen
	resp.MouseMask = mouseMask
	resp.Mouse = mouseEnabled
	resp.MouseSGR = mouseSGR
	resp.MouseEncoding = mouseEncoding
	resp.SafeToReplay = safeToReplay
	if snap.Data != "" {
		debugLogf(
			"session_bootstrap id=%s snapshot_bytes=%d snapshot_source=%s backlog_bytes=0 backlog_source= alt_screen=%t",
			s.id,
			len(snap.Data),
			snap.Source,
			altScreen,
		)
		return resp, nil
	}
	backlog, err := s.backlog(0)
	if err != nil {
		return resp, err
	}
	resp.Backlog = backlog.Data
	resp.NextOffset = backlog.NextOffset
	resp.BacklogTruncated = backlog.Truncated
	resp.BacklogSource = backlog.Source
	if backlog.Truncated {
		resp.SafeToReplay = false
	}
	debugLogf(
		"session_bootstrap id=%s snapshot_bytes=0 snapshot_source=%s backlog_bytes=%d backlog_source=%s alt_screen=%t truncated=%t",
		s.id,
		snap.Source,
		len(backlog.Data),
		backlog.Source,
		altScreen,
		backlog.Truncated,
	)
	return resp, nil
}

func newSession(opts Options, id, cwd string) *Session {
	statePath := ""
	kittyStatePath := ""
	modesPath := ""
	if opts.StateDir != "" {
		safe := sanitizeID(id)
		if safe == "" {
			safe = "session"
		}
		statePath = filepath.Join(opts.StateDir, safe+".state")
		kittyStatePath = statePath + ".kitty.json"
		modesPath = statePath + ".modes.json"
	}
	emu := termemu.New(80, 24)
	if opts.HistoryLines > 0 {
		emu.SetHistoryLimit(opts.HistoryLines)
	}
	return &Session{
		id:             id,
		cwd:            cwd,
		opts:           opts,
		buffer:         newTerminalBuffer(opts.BufferBytes),
		emu:            emu,
		kittyState:     kitty.NewState(),
		kittyStatePath: kittyStatePath,
		recordEnabled:  opts.RecordPty,
		protocolLog:    opts.ProtocolLogger,
		statePath:      statePath,
		modesPath:      modesPath,
		snapshotEvery:  opts.SnapshotInterval,
		streamInitial:  opts.StreamInitialCredit,
		streamTimeout:  opts.StreamCreditTimeout,
		subscribers:    make(map[*subscriber]struct{}),
		streams:        make(map[string]*subscriber),
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

func (s *Session) write(ctx context.Context, data string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pty == nil {
		return errors.New("terminal not started")
	}
	s.logProtocol(ctx, "in", []byte(data))
	if _, err := s.pty.Write([]byte(data)); err != nil {
		return err
	}
	s.bumpActivityLocked()
	return nil
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
	if s.emu != nil {
		s.emu.Resize(cols, rows)
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
