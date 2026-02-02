package sessiond

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/strantalis/workset/pkg/kitty"
	"github.com/strantalis/workset/pkg/termemu"
	"github.com/strantalis/workset/pkg/unifiedlog"
)

type streamEvent struct {
	kind  string
	data  []byte
	kitty *kitty.Event
	modes *modeSnapshot
}

type subscriber struct {
	ch        chan streamEvent
	streamID  string
	creditMu  sync.Mutex
	credit    int64
	creditCh  chan struct{}
	done      chan struct{}
	closed    bool
	closeOnce sync.Once
	lastAckAt time.Time
}

func (s *subscriber) close() {
	s.closeOnce.Do(func() {
		s.creditMu.Lock()
		s.closed = true
		s.creditMu.Unlock()
		close(s.done)
		close(s.ch)
	})
}

func newSubscriber(streamID string, initialCredit int64) *subscriber {
	sub := &subscriber{
		ch:       make(chan streamEvent, 64),
		streamID: streamID,
		credit:   initialCredit,
		creditCh: make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
	if initialCredit > 0 {
		sub.lastAckAt = time.Now()
	}
	return sub
}

func (s *subscriber) addCredit(bytes int64) {
	if bytes <= 0 {
		return
	}
	s.creditMu.Lock()
	if s.closed {
		s.creditMu.Unlock()
		return
	}
	s.credit += bytes
	s.lastAckAt = time.Now()
	s.creditMu.Unlock()
	select {
	case s.creditCh <- struct{}{}:
	default:
	}
}

func (s *subscriber) waitForCredit(need int64, timeout time.Duration) bool {
	if need <= 0 {
		return true
	}
	deadline := time.Now().Add(timeout)
	for {
		s.creditMu.Lock()
		if s.closed {
			s.creditMu.Unlock()
			return false
		}
		if s.credit >= need {
			s.credit -= need
			s.creditMu.Unlock()
			return true
		}
		s.creditMu.Unlock()
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		timer := time.NewTimer(remaining)
		select {
		case <-s.creditCh:
			timer.Stop()
		case <-s.done:
			timer.Stop()
			return false
		case <-timer.C:
			return false
		}
	}
}

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

type modeSnapshot struct {
	AltScreen  bool  `json:"altScreen"`
	MouseMask  uint8 `json:"mouseMask"`
	MouseSGR   bool  `json:"mouseSGR"`
	MouseUTF8  bool  `json:"mouseUTF8"`
	MouseURXVT bool  `json:"mouseURXVT"`
	TuiMode    bool  `json:"tuiMode"`
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

func (s *Session) hasSubscribers() bool {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()
	return len(s.subscribers) > 0
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
	}
	if onClose != nil {
		onClose(s)
	}
	s.closeSubscribers()
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

func (s *Session) subscribe(streamID string) *subscriber {
	if streamID == "" {
		streamID = newStreamID()
	}
	sub := newSubscriber(streamID, 0)
	s.subscribersMu.Lock()
	s.subscribers[sub] = struct{}{}
	s.streams[streamID] = sub
	s.subscribersMu.Unlock()
	return sub
}

func (s *Session) unsubscribe(sub *subscriber) {
	s.subscribersMu.Lock()
	_, ok := s.subscribers[sub]
	if ok {
		delete(s.subscribers, sub)
		if sub.streamID != "" {
			delete(s.streams, sub.streamID)
		}
	}
	s.subscribersMu.Unlock()
	if !ok {
		return
	}
	sub.close()
}

func (s *Session) closeSubscribers() {
	s.subscribersMu.Lock()
	subs := make([]*subscriber, 0, len(s.subscribers))
	for sub := range s.subscribers {
		subs = append(subs, sub)
	}
	s.subscribers = make(map[*subscriber]struct{})
	s.streams = make(map[string]*subscriber)
	s.subscribersMu.Unlock()
	for _, sub := range subs {
		sub.close()
	}
}

func (s *Session) ack(streamID string, bytes int64) error {
	if bytes <= 0 {
		return nil
	}
	s.subscribersMu.Lock()
	sub := s.streams[streamID]
	s.subscribersMu.Unlock()
	if sub == nil {
		return errors.New("stream not found")
	}
	sub.addCredit(bytes)
	return nil
}

func (s *Session) broadcast(data []byte) {
	if len(data) == 0 {
		return
	}
	var overflow []*subscriber
	s.subscribersMu.Lock()
	for sub := range s.subscribers {
		select {
		case sub.ch <- streamEvent{kind: "data", data: data}:
		default:
			overflow = append(overflow, sub)
		}
	}
	s.subscribersMu.Unlock()
	for _, sub := range overflow {
		s.unsubscribe(sub)
		debugLogf("session_stream_drop id=%s stream=%s reason=buffer_overflow", s.id, sub.streamID)
	}
}

func (s *Session) broadcastKitty(events []kitty.Event) {
	if len(events) == 0 {
		return
	}
	var overflow []*subscriber
	s.subscribersMu.Lock()
	for sub := range s.subscribers {
		overflowed := false
		for i := range events {
			ev := events[i]
			select {
			case sub.ch <- streamEvent{kind: "kitty", kitty: &ev}:
			default:
				overflowed = true
			}
			if overflowed {
				break
			}
		}
		if overflowed {
			overflow = append(overflow, sub)
		}
	}
	s.subscribersMu.Unlock()
	for _, sub := range overflow {
		s.unsubscribe(sub)
		debugLogf("session_stream_drop id=%s stream=%s reason=kitty_overflow", s.id, sub.streamID)
	}
}

func (s *Session) broadcastModes(modes modeSnapshot) {
	var overflow []*subscriber
	s.subscribersMu.Lock()
	for sub := range s.subscribers {
		select {
		case sub.ch <- streamEvent{kind: "modes", modes: &modes}:
		default:
			overflow = append(overflow, sub)
		}
	}
	s.subscribersMu.Unlock()
	for _, sub := range overflow {
		s.unsubscribe(sub)
		debugLogf("session_stream_drop id=%s stream=%s reason=modes_overflow", s.id, sub.streamID)
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
			raw := buf[:n]
			s.recordRaw(raw)
			normalized := s.c1Normalizer.Normalize(raw)
			if len(normalized) == 0 {
				continue
			}
			cleaned := normalized
			var kittyEvents []kitty.Event
			if s.kittyState != nil {
				cursor := kitty.Cursor{}
				if s.emu != nil {
					pos := s.emu.Cursor()
					cursor = kitty.Cursor{Row: pos.Row, Col: pos.Col}
				}
				cleaned, kittyEvents = s.kittyDecoder.Process(normalized, cursor, s.kittyState)
			}
			if len(kittyEvents) > 0 {
				s.broadcastKitty(kittyEvents)
			}
			if len(cleaned) == 0 {
				if len(kittyEvents) > 0 {
					continue
				}
				continue
			}
			if s.emu != nil {
				s.emu.Write(ctx, cleaned)
				s.maybePersistSnapshot()
			}
			s.logProtocol(ctx, "out", cleaned)
			filtered := filterTerminalOutputStreaming(cleaned, &s.escapeFilter)
			if len(filtered) == 0 {
				continue
			}
			s.mu.Lock()
			s.bumpActivityLocked()
			altChanged, mouseChanged := s.noteModesLocked(cleaned)
			altActive := s.altScreen
			mouseActive := s.mouseMask != 0
			mouseSGR := s.mouseSGR
			mouseEncoding := s.mouseEncoding()
			var modesSnapshot modeSnapshot
			if altChanged || mouseChanged {
				modesSnapshot = s.currentModesLocked()
			}
			s.mu.Unlock()
			if altChanged {
				debugLogf("session_alt_screen id=%s active=%t", s.id, altActive)
			}
			if mouseChanged {
				debugLogf("session_mouse_mode id=%s active=%t sgr=%t encoding=%s", s.id, mouseActive, mouseSGR, mouseEncoding)
			}
			if altChanged || containsClearScreen(cleaned) {
				if s.kittyState != nil {
					s.broadcastKitty(s.kittyState.ClearAll())
				}
			}
			if altChanged || mouseChanged {
				s.broadcastModes(modesSnapshot)
			}
			s.recordOutput(filtered)
			s.broadcast(filtered)
		}
		if err != nil {
			s.closeWithReason("closed")
			return
		}
	}
}

func (s *Session) restoreSnapshot() {
	if s.statePath == "" || s.emu == nil {
		return
	}
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		return
	}
	if err := s.emu.UnmarshalBinary(data); err != nil {
		return
	}
	if s.emu.IsAltScreen() {
		s.tuiMode = true
		s.altScreen = true
	} else {
		s.tuiMode = false
		s.altScreen = false
	}
	if s.kittyStatePath != "" && s.kittyState != nil {
		kittyData, err := os.ReadFile(s.kittyStatePath)
		if err == nil {
			var snapshot kitty.Snapshot
			if err := json.Unmarshal(kittyData, &snapshot); err == nil {
				s.kittyState.Restore(snapshot)
			}
		}
	}
	if s.modesPath != "" {
		modesData, err := os.ReadFile(s.modesPath)
		if err == nil {
			var modes modeSnapshot
			if err := json.Unmarshal(modesData, &modes); err == nil {
				s.mouseMask = modes.MouseMask
				s.mouseSGR = modes.MouseSGR
				s.mouseUTF8 = modes.MouseUTF8
				s.mouseURXVT = modes.MouseURXVT
				if !s.emu.IsAltScreen() {
					s.tuiMode = modes.TuiMode
					s.altScreen = modes.AltScreen
				}
			}
		}
	}
}

func (s *Session) maybePersistSnapshot() {
	if s.snapshotEvery <= 0 || s.statePath == "" || s.emu == nil {
		return
	}
	now := time.Now()
	if now.Sub(s.lastSnapshot) < s.snapshotEvery {
		return
	}
	s.snapshotMu.Lock()
	if s.snapshotInFlight {
		s.snapshotMu.Unlock()
		return
	}
	s.snapshotInFlight = true
	s.lastSnapshot = now
	s.snapshotMu.Unlock()
	go func() {
		s.persistSnapshot()
		s.snapshotMu.Lock()
		s.snapshotInFlight = false
		s.snapshotMu.Unlock()
	}()
}

func (s *Session) persistSnapshot() {
	if s.statePath == "" || s.emu == nil {
		return
	}
	data, err := s.emu.MarshalBinary()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.statePath), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(s.statePath, data, 0o644)
	if s.kittyStatePath != "" && s.kittyState != nil {
		snapshot := s.kittyState.Snapshot()
		kittyData, err := json.Marshal(snapshot)
		if err == nil {
			_ = os.WriteFile(s.kittyStatePath, kittyData, 0o644)
		}
	}
	if s.modesPath != "" {
		s.mu.Lock()
		modes := modeSnapshot{
			AltScreen:  s.altScreen,
			MouseMask:  s.mouseMask,
			MouseSGR:   s.mouseSGR,
			MouseUTF8:  s.mouseUTF8,
			MouseURXVT: s.mouseURXVT,
			TuiMode:    s.tuiMode,
		}
		s.mu.Unlock()
		if modesData, err := json.Marshal(modes); err == nil {
			_ = os.WriteFile(s.modesPath, modesData, 0o644)
		}
	}
}

func (s *Session) noteModesLocked(data []byte) (bool, bool) {
	if len(data) == 0 {
		return false, false
	}
	const tailMax = 64
	prevAlt := s.altScreen
	prevMask := s.mouseMask
	prevSGR := s.mouseSGR
	prevUTF8 := s.mouseUTF8
	prevURXVT := s.mouseURXVT
	merged := append(append([]byte{}, s.seqTail...), data...)
	if containsAltScreenEnter(merged) {
		s.tuiMode = true
		s.altScreen = true
	}
	if containsAltScreenExit(merged) {
		s.altScreen = false
		s.tuiMode = false
	}
	s.applyMouseModes(merged)
	if !s.altScreen {
		s.tuiMode = false
	}
	if len(merged) > tailMax {
		merged = merged[len(merged)-tailMax:]
	}
	s.seqTail = merged
	altChanged := prevAlt != s.altScreen
	mouseChanged := prevMask != s.mouseMask || prevSGR != s.mouseSGR || prevUTF8 != s.mouseUTF8 || prevURXVT != s.mouseURXVT
	return altChanged, mouseChanged
}

func (s *Session) currentModesLocked() modeSnapshot {
	return modeSnapshot{
		AltScreen:  s.altScreen,
		MouseMask:  s.mouseMask,
		MouseSGR:   s.mouseSGR,
		MouseUTF8:  s.mouseUTF8,
		MouseURXVT: s.mouseURXVT,
		TuiMode:    s.tuiMode,
	}
}

func containsAltScreenEnter(data []byte) bool {
	return bytes.Contains(data, []byte("\x1b[?1049h")) ||
		bytes.Contains(data, []byte("\x1b[?1047h")) ||
		bytes.Contains(data, []byte("\x1b[?47h"))
}

func containsAltScreenExit(data []byte) bool {
	return bytes.Contains(data, []byte("\x1b[?1049l")) ||
		bytes.Contains(data, []byte("\x1b[?1047l")) ||
		bytes.Contains(data, []byte("\x1b[?47l"))
}

func containsClearScreen(data []byte) bool {
	return bytes.Contains(data, []byte("\x1b[2J")) ||
		bytes.Contains(data, []byte("\x1b[3J"))
}

func (s *Session) applyMouseModes(data []byte) {
	for i := 0; i < len(data); i++ {
		if data[i] == 0x1b {
			if i+2 < len(data) && data[i+1] == '[' && data[i+2] == '?' {
				i = s.parseMouseCSI(data, i+3)
			}
			continue
		}
		if data[i] == 0x9b {
			if i+1 < len(data) && data[i+1] == '?' {
				i = s.parseMouseCSI(data, i+2)
			}
		}
	}
}

func (s *Session) parseMouseCSI(data []byte, start int) int {
	params := make([]int, 0, 4)
	val := 0
	hasVal := false
	for i := start; i < len(data); i++ {
		b := data[i]
		if b >= '0' && b <= '9' {
			val = val*10 + int(b-'0')
			hasVal = true
			continue
		}
		if b == ';' {
			if hasVal {
				params = append(params, val)
			} else {
				params = append(params, 0)
			}
			val = 0
			hasVal = false
			continue
		}
		if b >= 0x40 && b <= 0x7e {
			if hasVal || len(params) > 0 {
				params = append(params, val)
			}
			if b == 'h' || b == 'l' {
				on := b == 'h'
				for _, p := range params {
					switch p {
					case 1000:
						s.setMouseMask(0, on)
					case 1002:
						s.setMouseMask(1, on)
					case 1003:
						s.setMouseMask(2, on)
					case 1005:
						s.mouseUTF8 = on
					case 1015:
						s.mouseURXVT = on
					case 1006:
						s.mouseSGR = on
					}
				}
			}
			return i
		}
	}
	return len(data) - 1
}

func (s *Session) setMouseMask(bit uint8, on bool) {
	mask := uint8(1 << bit)
	if on {
		s.mouseMask |= mask
		return
	}
	s.mouseMask &^= mask
}

func (s *Session) mouseEncoding() string {
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

var (
	terminalFilterOnce    sync.Once
	terminalFilterEnabled bool
	terminalFilterDebug   bool
	terminalFilterLog     *os.File
	terminalFilterMu      sync.Mutex
)

func terminalFilterConfig() (bool, bool) {
	terminalFilterOnce.Do(func() {
		terminalFilterEnabled = envTruthy(os.Getenv("WORKSET_TERMINAL_FILTER"))
		terminalFilterDebug = envTruthy(os.Getenv("WORKSET_TERMINAL_FILTER_DEBUG"))
		if terminalFilterDebug {
			logPath, err := terminalFilterLogPath()
			if err != nil {
				terminalFilterDebug = false
				return
			}
			if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
				terminalFilterDebug = false
				return
			}
			file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				terminalFilterDebug = false
				return
			}
			terminalFilterLog = file
		}
	})
	return terminalFilterEnabled, terminalFilterDebug
}

func terminalFilterLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "terminal_filter.log"), nil
}

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func logTerminalFilter(kind string, seq []byte) {
	if terminalFilterLog == nil {
		return
	}
	terminalFilterMu.Lock()
	defer terminalFilterMu.Unlock()
	_, _ = fmt.Fprintf(
		terminalFilterLog,
		"%s %s len=%d hex=%x ascii=%q\n",
		time.Now().Format(time.RFC3339Nano),
		kind,
		len(seq),
		seq,
		seq,
	)
}

func filterTerminalOutput(data []byte) []byte {
	const esc = 0x1b
	if len(data) == 0 {
		return data
	}
	enabled, debug := terminalFilterConfig()
	if !enabled && !debug {
		return data
	}
	var out []byte
	last := 0
	dropped := false
	for i := 0; i < len(data); i++ {
		if data[i] != esc || i+1 >= len(data) {
			continue
		}
		switch data[i+1] {
		case ']':
			end, drop := scanOSC(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("OSC", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case 'P':
			end, drop := scanEscapeString(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("DCS", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case '_':
			end, drop := scanEscapeString(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("APC", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case '^':
			end, drop := scanEscapeString(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("PM", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case 'X':
			end, drop := scanEscapeString(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("SOS", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case '[':
			end, drop := scanCSI(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("CSI", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		default:
			continue
		}
	}
	if !enabled {
		return data
	}
	if !dropped || out == nil {
		return data
	}
	if out == nil {
		return data
	}
	if last < len(data) {
		out = append(out, data[last:]...)
	}
	return out
}

type c1Normalizer struct {
	utf8Tail []byte
}

func (n *c1Normalizer) Normalize(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	if len(n.utf8Tail) > 0 {
		data = append(n.utf8Tail, data...)
		n.utf8Tail = nil
	}
	var out []byte
	i := 0
	for i < len(data) {
		b := data[i]
		if b < 0x80 {
			if out != nil {
				out = append(out, b)
			}
			i++
			continue
		}
		if !utf8.FullRune(data[i:]) {
			if out == nil {
				out = make([]byte, 0, len(data))
				out = append(out, data[:i]...)
			}
			n.utf8Tail = append(n.utf8Tail, data[i:]...)
			break
		}
		r, size := utf8.DecodeRune(data[i:])
		if r != utf8.RuneError || size > 1 {
			if out != nil {
				out = append(out, data[i:i+size]...)
			}
			i += size
			continue
		}
		mapped := mapC1Control(b)
		if mapped == nil {
			if out != nil {
				out = append(out, b)
			}
			i++
			continue
		}
		if out == nil {
			out = make([]byte, 0, len(data)+len(mapped))
			out = append(out, data[:i]...)
		}
		out = append(out, mapped...)
		i++
	}
	if out == nil {
		return data
	}
	return out
}

func mapC1Control(b byte) []byte {
	switch b {
	case 0x84: // IND
		return []byte{0x1b, 'D'}
	case 0x85: // NEL
		return []byte{0x1b, 'E'}
	case 0x88: // HTS
		return []byte{0x1b, 'H'}
	case 0x8d: // RI
		return []byte{0x1b, 'M'}
	case 0x8e: // SS2
		return []byte{0x1b, 'N'}
	case 0x8f: // SS3
		return []byte{0x1b, 'O'}
	case 0x90: // DCS
		return []byte{0x1b, 'P'}
	case 0x98: // SOS
		return []byte{0x1b, 'X'}
	case 0x9b: // CSI
		return []byte{0x1b, '['}
	case 0x9c: // ST
		return []byte{0x1b, '\\'}
	case 0x9d: // OSC
		return []byte{0x1b, ']'}
	case 0x9e: // PM
		return []byte{0x1b, '^'}
	case 0x9f: // APC
		return []byte{0x1b, '_'}
	default:
		return nil
	}
}

type escapeStringFilter struct {
	enabled    bool
	debug      bool
	configured bool
	active     bool
	pendingEsc bool
	kind       string
	logBuf     []byte
	pending    []byte
	truncated  bool
}

func (f *escapeStringFilter) ensureConfig() {
	if f.configured {
		return
	}
	f.enabled, f.debug = terminalFilterConfig()
	f.configured = true
}

func (f *escapeStringFilter) reset() {
	f.active = false
	f.pendingEsc = false
	f.kind = ""
	f.logBuf = nil
	f.pending = nil
	f.truncated = false
}

func (f *escapeStringFilter) appendLog(data []byte) {
	const maxLog = 4096
	if !f.debug || len(data) == 0 {
		return
	}
	if len(f.logBuf) >= maxLog {
		return
	}
	remain := maxLog - len(f.logBuf)
	if len(data) > remain {
		data = data[:remain]
	}
	f.logBuf = append(f.logBuf, data...)
}

func (f *escapeStringFilter) appendPending(data []byte) {
	const maxPending = 64 * 1024
	if len(data) == 0 || f.truncated {
		return
	}
	if len(f.pending)+len(data) > maxPending {
		remain := maxPending - len(f.pending)
		if remain > 0 {
			f.pending = append(f.pending, data[:remain]...)
		}
		f.truncated = true
		return
	}
	f.pending = append(f.pending, data...)
}

func filterTerminalOutputStreaming(data []byte, f *escapeStringFilter) []byte {
	const esc = 0x1b
	if len(data) == 0 {
		return data
	}
	f.ensureConfig()
	if !f.enabled && !f.debug {
		return data
	}
	if f.pendingEsc {
		data = append([]byte{esc}, data...)
		f.pendingEsc = false
	}
	var prefix []byte
	if f.active {
		end := scanEscapeStringTerminator(data, 0)
		if end == 0 {
			f.appendLog(data)
			if f.kind == "OSC" {
				f.appendPending(data)
			}
			if f.enabled {
				return nil
			}
			return data
		}
		f.appendLog(data[:end])
		if f.kind == "OSC" {
			f.appendPending(data[:end])
		}
		if f.debug && len(f.logBuf) > 0 {
			logTerminalFilter(f.kind, f.logBuf)
		}
		pending := f.pending
		kind := f.kind
		truncated := f.truncated
		f.reset()
		if f.enabled {
			if kind == "OSC" && !truncated && !shouldDropOSC(extractOSCPayload(pending)) {
				prefix = pending
			}
			data = data[end:]
		} else {
			return data
		}
	}
	if !f.enabled {
		return filterTerminalOutput(data)
	}
	enabled, debug := f.enabled, f.debug
	var out []byte
	last := 0
	dropped := false
	if len(prefix) > 0 {
		out = make([]byte, 0, len(prefix)+len(data))
		out = append(out, prefix...)
		dropped = true
	}
	for i := 0; i < len(data); i++ {
		if data[i] != esc || i+1 >= len(data) {
			if data[i] == esc && i+1 >= len(data) {
				if f.enabled || f.debug {
					if out == nil && i > 0 {
						out = make([]byte, 0, len(data))
						out = append(out, data[:i]...)
					} else if out != nil && last < i {
						out = append(out, data[last:i]...)
					}
					f.pendingEsc = true
					if out == nil {
						return nil
					}
					return out
				}
			}
			continue
		}
		switch data[i+1] {
		case ']':
			end, drop := scanOSC(data, i)
			if end == i {
				f.active = true
				f.kind = "OSC"
				f.appendLog(data[i:])
				f.appendPending(data[i:])
				if out == nil {
					out = make([]byte, 0, len(data))
					out = append(out, data[:i]...)
				}
				return out
			}
			if drop {
				if debug {
					logTerminalFilter("OSC", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case 'P':
			kind := "DCS"
			end, drop := scanEscapeString(data, i)
			if end == i {
				f.active = true
				f.kind = kind
				f.appendLog(data[i:])
				if out == nil {
					out = make([]byte, 0, len(data))
					out = append(out, data[:i]...)
				}
				return out
			}
			if drop {
				if debug {
					logTerminalFilter(kind, data[i:end])
				}
				dropped = true
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case '_':
			kind := "APC"
			end, drop := scanEscapeString(data, i)
			if end == i {
				f.active = true
				f.kind = kind
				f.appendLog(data[i:])
				if out == nil {
					out = make([]byte, 0, len(data))
					out = append(out, data[:i]...)
				}
				return out
			}
			if drop {
				if debug {
					logTerminalFilter(kind, data[i:end])
				}
				dropped = true
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case '^':
			kind := "PM"
			end, drop := scanEscapeString(data, i)
			if end == i {
				f.active = true
				f.kind = kind
				f.appendLog(data[i:])
				if out == nil {
					out = make([]byte, 0, len(data))
					out = append(out, data[:i]...)
				}
				return out
			}
			if drop {
				if debug {
					logTerminalFilter(kind, data[i:end])
				}
				dropped = true
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case 'X':
			kind := "SOS"
			end, drop := scanEscapeString(data, i)
			if end == i {
				f.active = true
				f.kind = kind
				f.appendLog(data[i:])
				if out == nil {
					out = make([]byte, 0, len(data))
					out = append(out, data[:i]...)
				}
				return out
			}
			if drop {
				if debug {
					logTerminalFilter(kind, data[i:end])
				}
				dropped = true
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case '[':
			end, drop := scanCSI(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("CSI", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		default:
			continue
		}
	}
	if !dropped || out == nil {
		return data
	}
	if last < len(data) {
		out = append(out, data[last:]...)
	}
	return out
}

func extractOSCPayload(seq []byte) []byte {
	const esc = 0x1b
	const bel = 0x07
	if len(seq) < 3 || seq[0] != esc || seq[1] != ']' {
		return nil
	}
	end := len(seq)
	switch {
	case seq[end-1] == bel:
		end--
	case end >= 2 && seq[end-2] == esc && seq[end-1] == '\\':
		end -= 2
	default:
		return nil
	}
	if end <= 2 {
		return nil
	}
	return seq[2:end]
}

func scanOSC(data []byte, start int) (int, bool) {
	const esc = 0x1b
	const bel = 0x07
	i := start + 2
	for i < len(data) {
		switch data[i] {
		case bel:
			return i + 1, shouldDropOSC(data[start+2 : i])
		case esc:
			if i+1 < len(data) && data[i+1] == '\\' {
				return i + 2, shouldDropOSC(data[start+2 : i])
			}
		}
		i++
	}
	return start, false
}

func scanEscapeString(data []byte, start int) (int, bool) {
	const esc = 0x1b
	const bel = 0x07
	i := start + 2
	for i < len(data) {
		switch data[i] {
		case bel:
			return i + 1, true
		case esc:
			if i+1 < len(data) && data[i+1] == '\\' {
				return i + 2, true
			}
		}
		i++
	}
	return start, false
}

func scanEscapeStringTerminator(data []byte, start int) int {
	const esc = 0x1b
	const bel = 0x07
	i := start
	for i < len(data) {
		switch data[i] {
		case bel:
			return i + 1
		case esc:
			if i+1 < len(data) && data[i+1] == '\\' {
				return i + 2
			}
		}
		i++
	}
	return 0
}

func shouldDropOSC(payload []byte) bool {
	if len(payload) == 0 {
		return false
	}
	hasRGB := false
	for i := 0; i+3 < len(payload); i++ {
		if payload[i] == 'r' && payload[i+1] == 'g' && payload[i+2] == 'b' && payload[i+3] == ':' {
			hasRGB = true
			break
		}
	}
	if !hasRGB {
		return false
	}
	if len(payload) >= 3 && payload[0] == '1' && payload[1] == '0' && payload[2] == ';' {
		return true
	}
	if len(payload) >= 3 && payload[0] == '1' && payload[1] == '1' && payload[2] == ';' {
		return true
	}
	if len(payload) >= 2 && payload[0] == '4' && payload[1] == ';' {
		return true
	}
	return false
}

func scanCSI(data []byte, start int) (int, bool) {
	i := start + 2
	for i < len(data) {
		b := data[i]
		if b >= 0x40 && b <= 0x7e {
			return i + 1, shouldDropCSI(data[start+2:i], b)
		}
		i++
	}
	return start, false
}

func shouldDropCSI(params []byte, final byte) bool {
	if final == 'R' {
		return true
	}
	if final == 'c' {
		for _, b := range params {
			if b == '?' || b == '>' {
				return true
			}
		}
	}
	return false
}

func (s *Session) logProtocol(ctx context.Context, direction string, data []byte) {
	if s.protocolLog == nil || len(data) == 0 {
		return
	}
	const esc = 0x1b
	for i := 0; i < len(data); i++ {
		if data[i] != esc || i+1 >= len(data) {
			continue
		}
		switch data[i+1] {
		case ']':
			end, _ := scanOSC(data, i)
			if end == i || end > len(data) {
				continue
			}
			payloadEnd := end
			if payloadEnd >= 2 && data[payloadEnd-2] == esc && data[payloadEnd-1] == '\\' {
				payloadEnd -= 2
			} else if payloadEnd >= 1 && data[payloadEnd-1] == 0x07 {
				payloadEnd--
			}
			if payloadEnd < i+2 {
				payloadEnd = i + 2
			}
			payload := data[i+2 : payloadEnd]
			s.logOSCProtocol(ctx, direction, data[i:end], payload)
			i = end - 1
		case '[':
			end, _ := scanCSI(data, i)
			if end == i || end > len(data) {
				continue
			}
			if end-1 <= i+1 {
				continue
			}
			final := data[end-1]
			params := data[i+2 : end-1]
			s.logCSIProtocol(ctx, direction, data[i:end], params, final)
			i = end - 1
		default:
			continue
		}
	}
}

func (s *Session) logOSCProtocol(ctx context.Context, direction string, seq []byte, payload []byte) {
	if s.protocolLog == nil {
		return
	}
	if isOSCColorQueryRequest(payload) {
		s.protocolLog.Log(ctx, "terminal.protocol", direction, "event", "osc_color_query_request", seq)
	}
	if shouldDropOSC(payload) {
		s.protocolLog.Log(ctx, "terminal.protocol", direction, "drop", "osc_color_query_response", seq)
	}
}

func (s *Session) logCSIProtocol(ctx context.Context, direction string, seq []byte, params []byte, final byte) {
	if s.protocolLog == nil {
		return
	}
	switch final {
	case 'n':
		s.protocolLog.Log(ctx, "terminal.protocol", direction, "event", "dsr_request", seq)
	case 'R':
		s.protocolLog.Log(ctx, "terminal.protocol", direction, "drop", "dsr_response", seq)
	case 'c':
		if hasCSIQueryPrefix(params) {
			s.protocolLog.Log(ctx, "terminal.protocol", direction, "drop", "device_attributes_response", seq)
		} else {
			s.protocolLog.Log(ctx, "terminal.protocol", direction, "event", "device_attributes_request", seq)
		}
	}
}

func hasCSIQueryPrefix(params []byte) bool {
	for _, b := range params {
		if b == '?' || b == '>' {
			return true
		}
	}
	return false
}

func isOSCColorQueryRequest(payload []byte) bool {
	if len(payload) < 4 {
		return false
	}
	if bytes.HasPrefix(payload, []byte("10;?")) || bytes.HasPrefix(payload, []byte("11;?")) {
		return true
	}
	if len(payload) >= 2 && payload[0] == '4' && payload[1] == ';' {
		return bytes.Contains(payload, []byte(";?"))
	}
	return false
}

func (s *Session) recordOutput(data []byte) {
	if s.buffer != nil {
		s.buffer.Append(data)
	}
	s.mu.Lock()
	file := s.transcriptFile
	s.mu.Unlock()
	if file == nil {
		return
	}
	if _, err := file.Write(data); err == nil {
		s.mu.Lock()
		s.transcriptSize += int64(len(data))
		s.mu.Unlock()
	}
	s.trimTranscript()
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

func (s *Session) openTranscript() error {
	if s.opts.TranscriptDir == "" {
		return nil
	}
	safe := sanitizeID(s.id)
	if safe == "" {
		safe = "session"
	}
	if err := os.MkdirAll(s.opts.TranscriptDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(s.opts.TranscriptDir, safe+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	s.transcriptPath = path
	s.transcriptFile = file
	s.transcriptSize = info.Size()
	return nil
}

func (s *Session) openRecord() {
	if !s.recordEnabled || s.opts.RecordDir == "" {
		return
	}
	safe := sanitizeID(s.id)
	if safe == "" {
		safe = "session"
	}
	if err := os.MkdirAll(s.opts.RecordDir, 0o755); err != nil {
		return
	}
	name := fmt.Sprintf("%s-%s.ptylog", safe, time.Now().Format("20060102-150405"))
	path := filepath.Join(s.opts.RecordDir, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	s.recordPath = path
	s.recordFile = file
}

func (s *Session) recordRaw(data []byte) {
	if s.recordFile == nil || len(data) == 0 {
		return
	}
	s.recordMu.Lock()
	defer s.recordMu.Unlock()
	_, _ = s.recordFile.Write(data)
}

func (s *Session) readTranscriptTail(maxBytes int64) ([]byte, bool, error) {
	if s.transcriptPath == "" {
		return nil, false, nil
	}
	file, err := os.Open(s.transcriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer func() {
		_ = file.Close()
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	size := info.Size()
	if size == 0 {
		return nil, false, nil
	}
	start := int64(0)
	truncated := false
	if maxBytes > 0 && size > maxBytes {
		start = size - maxBytes
		truncated = true
	}
	if _, err := file.Seek(start, 0); err != nil {
		return nil, false, err
	}
	buf, err := io.ReadAll(file)
	if err != nil {
		return nil, false, err
	}
	return buf, truncated, nil
}

func (s *Session) trimTranscript() {
	if s.transcriptPath == "" || s.transcriptFile == nil {
		return
	}
	s.mu.Lock()
	size := s.transcriptSize
	s.mu.Unlock()
	if size <= s.opts.TranscriptTrimThreshold {
		return
	}
	_ = s.transcriptFile.Close()
	data, truncated, err := s.readTranscriptTail(s.opts.TranscriptMaxBytes)
	if err != nil {
		return
	}
	if !truncated {
		file, err := os.OpenFile(s.transcriptPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return
		}
		s.mu.Lock()
		s.transcriptFile = file
		s.transcriptSize = info.Size()
		s.mu.Unlock()
		return
	}
	tmp := s.transcriptPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	if err := os.Rename(tmp, s.transcriptPath); err != nil {
		return
	}
	file, err := os.OpenFile(s.transcriptPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return
	}
	s.mu.Lock()
	s.transcriptFile = file
	s.transcriptSize = info.Size()
	s.mu.Unlock()
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
