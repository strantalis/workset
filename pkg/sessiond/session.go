package sessiond

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
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

func (s *Session) snapshot() SnapshotResponse {
	s.mu.Lock()
	emu := s.emu
	kittyState := s.kittyState
	id := s.id
	altScreen := s.altScreen
	mouseMask := s.mouseMask
	mouseEnabled := mouseMask != 0
	mouseSGR := s.mouseSGR
	mouseEncoding := s.mouseEncoding()
	tuiMode := s.tuiMode
	s.mu.Unlock()
	data := ""
	if emu != nil {
		if !altScreen && !tuiMode && emu.HistoryLen() > 0 {
			data = emu.SnapshotANSIWithHistory()
		} else {
			data = emu.SnapshotANSI()
		}
	}
	if emu != nil && emu.IsAltScreen() {
		altScreen = true
	}
	safeToReplay := !altScreen && !tuiMode
	var kittySnapshot *kitty.Snapshot
	if kittyState != nil {
		snap := kittyState.Snapshot()
		if len(snap.Images) > 0 || len(snap.Placements) > 0 {
			kittySnapshot = &snap
		}
	}
	return SnapshotResponse{
		SessionID:     id,
		Data:          data,
		Source:        "snapshot",
		Kitty:         kittySnapshot,
		AltScreen:     altScreen,
		MouseMask:     mouseMask,
		Mouse:         mouseEnabled,
		MouseSGR:      mouseSGR,
		MouseEncoding: mouseEncoding,
		SafeToReplay:  safeToReplay,
	}
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

func (s *Session) start(ctx context.Context) error {
	execName, execArgs := resolveShellCommand()
	cmd := exec.CommandContext(ctx, execName, execArgs...)
	cmd.Dir = s.cwd
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"WORKSET_WORKSPACE="+s.id,
		"WORKSET_ROOT="+s.cwd,
	)
	cmd.Env = setEnv(cmd.Env, "COLORTERM", "truecolor")
	cmd.Env = setEnv(cmd.Env, "SHELL", execName)

	ptmx, err := startPTY(cmd)
	if err != nil {
		return err
	}
	if err := s.openTranscript(); err != nil {
		_ = ptmx.Close()
		return err
	}
	s.openRecord()
	s.restoreSnapshot()

	s.mu.Lock()
	s.cmd = cmd
	s.pty = ptmx
	s.startedAt = time.Now()
	s.lastActivity = s.startedAt
	s.mu.Unlock()
	debugLogf("session_start id=%s cwd=%s", s.id, s.cwd)
	if s.emu != nil {
		s.emu.SetResponder(func(resp []byte) {
			if s.hasSubscribers() {
				return
			}
			_ = s.write(ctx, string(resp))
		})
	}

	if s.opts.IdleTimeout > 0 {
		s.idleTimer = time.AfterFunc(s.opts.IdleTimeout, func() {
			s.closeWithReason("idle")
		})
	}
	go s.readLoop(ctx)
	return nil
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

func (s *Session) closeWithReason(reason string) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if reason != "" {
		s.closeReason = reason
	}
	onClose := s.onClose
	debugLogf("session_close id=%s reason=%s", s.id, s.closeReason)
	idleTimer := s.idleTimer
	s.idleTimer = nil
	pty := s.pty
	s.pty = nil
	transcriptFile := s.transcriptFile
	s.transcriptFile = nil
	recordFile := s.recordFile
	s.recordFile = nil
	cmd := s.cmd
	s.mu.Unlock()
	if idleTimer != nil {
		_ = idleTimer.Stop()
	}
	if pty != nil {
		_ = pty.Close()
	}
	if transcriptFile != nil {
		_ = transcriptFile.Close()
	}
	if recordFile != nil {
		_ = recordFile.Close()
	}
	s.persistSnapshot()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		waitForCommandExit(cmd, 2*time.Second)
	}
	if onClose != nil {
		onClose(s)
	}
	s.closeSubscribers()
}

func waitForCommandExit(cmd *exec.Cmd, timeout time.Duration) {
	if cmd == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	if timeout <= 0 {
		<-done
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
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

func (s *Session) backlog(since int64) (BacklogResponse, error) {
	s.mu.Lock()
	tui := s.tuiMode
	emu := s.emu
	s.mu.Unlock()
	if tui {
		if emu != nil {
			snapshot := emu.SnapshotANSI()
			if snapshot != "" {
				return BacklogResponse{
					SessionID: s.id,
					Data:      snapshot,
					Truncated: false,
					Source:    "snapshot",
				}, nil
			}
		}
		return BacklogResponse{
			SessionID: s.id,
			Data:      "",
			Truncated: true,
			Source:    "tui",
		}, nil
	}
	if emu != nil && emu.IsAltScreen() {
		return BacklogResponse{
			SessionID: s.id,
			Data:      emu.SnapshotANSI(),
			Truncated: false,
			Source:    "snapshot",
		}, nil
	}
	if since == 0 && emu != nil && emu.HistoryLen() > 0 {
		return BacklogResponse{
			SessionID: s.id,
			Data:      emu.SnapshotANSIWithHistory(),
			Truncated: false,
			Source:    "history",
		}, nil
	}
	if since < 0 {
		since = 0
	}
	if s.buffer != nil {
		data, next, truncated := s.buffer.ReadSince(since)
		if len(data) > 0 || next > 0 {
			return BacklogResponse{
				SessionID:  s.id,
				Data:       string(data),
				NextOffset: next,
				Truncated:  truncated,
				Source:     "buffer",
			}, nil
		}
	}
	data, truncated, err := s.readTranscriptTail(s.opts.TranscriptTailBytes)
	if err != nil {
		return BacklogResponse{}, err
	}
	return BacklogResponse{
		SessionID:  s.id,
		Data:       string(data),
		NextOffset: 0,
		Truncated:  truncated,
		Source:     "transcript",
	}, nil
}

func resolveShellCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		if shell := os.Getenv("COMSPEC"); shell != "" {
			return shell, nil
		}
		return "cmd.exe", nil
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = lookupUserShell()
	}
	if shell == "" {
		shell = "/bin/sh"
	}
	switch strings.ToLower(filepath.Base(shell)) {
	case "zsh", "bash":
		return shell, []string{"-l", "-i"}
	case "fish":
		return shell, []string{"-l"}
	default:
		return shell, nil
	}
}

func lookupUserShell() string {
	current, err := user.Current()
	if err != nil || current.Username == "" {
		return ""
	}
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		if parts[0] == current.Username {
			return strings.TrimSpace(parts[6])
		}
	}
	return ""
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
