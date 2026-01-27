package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/strantalis/workset/pkg/kitty"
	"github.com/strantalis/workset/pkg/sessiond"
	"github.com/strantalis/workset/pkg/worksetapi"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type TerminalPayload struct {
	WorkspaceID string `json:"workspaceId"`
	TerminalID  string `json:"terminalId"`
	Data        string `json:"data"`
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

	tuiMode      bool
	altScreen    bool
	seqTail      []byte
	mouseMask    uint8
	mouseSGR     bool
	mouseUTF8    bool
	mouseURXVT   bool
	c1Normalizer c1Normalizer
	escapeFilter escapeStringFilter

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

func (s *terminalSession) noteModes(data []byte) (bool, bool) {
	if len(data) == 0 {
		return false, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
		if s.mouseMask == 0 {
			s.tuiMode = false
		}
	}
	s.applyMouseModes(merged)
	if s.mouseMask != 0 {
		s.tuiMode = true
	} else if !s.altScreen {
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

func (s *terminalSession) applyMouseModes(data []byte) {
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

func (s *terminalSession) parseMouseCSI(data []byte, start int) int {
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

func (s *terminalSession) setMouseMask(bit uint8, on bool) {
	mask := uint8(1 << bit)
	if on {
		s.mouseMask |= mask
		return
	}
	s.mouseMask &^= mask
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

func (a *App) StartWorkspaceTerminal(workspaceID, terminalID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	sessionID := terminalSessionID(workspaceID, terminalID)

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	root, err := a.resolveWorkspaceRoot(ctx, workspaceID)
	if err != nil {
		return err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}

	for {
		a.terminalMu.Lock()
		existing := a.terminals[sessionID]
		if existing != nil {
			a.terminalMu.Unlock()
			waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := existing.waitReady(waitCtx)
			cancel()
			if err != nil {
				return err
			}
			existing.mu.Lock()
			hasSession := existing.client != nil
			existing.mu.Unlock()
			if hasSession {
				return nil
			}
			a.terminalMu.Lock()
			if current := a.terminals[sessionID]; current == existing {
				delete(a.terminals, sessionID)
			}
			a.terminalMu.Unlock()
			continue
		}

		session := newTerminalSession(workspaceID, terminalID, root)
		var restore terminalModeState
		var hasRestore bool
		if a.restoredModes != nil {
			restore, hasRestore = a.restoredModes[sessionID]
		}
		if hasRestore {
			session.mu.Lock()
			session.altScreen = restore.AltScreen
			session.tuiMode = restore.AltScreen
			session.mouseMask = restore.MouseMask
			session.mouseSGR = restore.MouseSGR
			session.mouseUTF8 = restore.MouseUTF8
			session.mouseURXVT = restore.MouseURXVT
			session.mu.Unlock()
		}
		client, err := a.getSessiondClient()
		if err != nil {
			a.terminalMu.Unlock()
			return err
		}
		session.client = client
		a.terminals[sessionID] = session
		a.terminalMu.Unlock()

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		resp, createErr := session.client.Create(ctx, sessionID, root)
		if createErr == nil && resp.Existing {
			session.resumed = true
		}
		err = createErr
		cancel()
		if err == nil {
			session.mu.Lock()
			session.lastActivity = time.Now()
			session.mu.Unlock()
		}
		session.markReady(err)
		if err != nil {
			a.terminalMu.Lock()
			if current := a.terminals[sessionID]; current == session {
				delete(a.terminals, sessionID)
			}
			a.terminalMu.Unlock()
			a.emitTerminalLifecycle("error", session, err.Error())
			return err
		}
		a.ensureIdleWatcher(session)
		if session.resumed {
			a.emitTerminalLifecycle("started", session, "Session resumed.")
		} else {
			a.emitTerminalLifecycle("started", session, "")
		}
		emitModes := hasRestore
		if emitModes {
			session.mu.Lock()
			altScreen := session.altScreen
			mouseEnabled := session.mouseEnabled()
			mouseSGR := session.mouseSGR
			mouseEncoding := session.mouseEncoding()
			session.mu.Unlock()
			a.emitTerminalModes(session, altScreen, mouseEnabled, mouseSGR, mouseEncoding)
			_ = a.persistTerminalState()
		}
		go a.streamTerminal(session)
		return nil
	}
}

type TerminalCreatePayload struct {
	WorkspaceID string `json:"workspaceId"`
	TerminalID  string `json:"terminalId"`
}

func (a *App) CreateWorkspaceTerminal(workspaceID string) (TerminalCreatePayload, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalCreatePayload{}, fmt.Errorf("workspace id required")
	}
	terminalID := uuid.NewString()
	if err := a.StartWorkspaceTerminal(workspaceID, terminalID); err != nil {
		return TerminalCreatePayload{}, err
	}
	return TerminalCreatePayload{WorkspaceID: workspaceID, TerminalID: terminalID}, nil
}

func (a *App) WriteWorkspaceTerminal(workspaceID, terminalID, data string) error {
	session, err := a.getTerminal(workspaceID, terminalID)
	if err != nil {
		return err
	}
	return session.Write(data)
}

func (a *App) GetTerminalBacklog(workspaceID, terminalID string, since int64) (TerminalBacklogPayload, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalBacklogPayload{}, fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return TerminalBacklogPayload{}, fmt.Errorf("terminal id required")
	}
	sessionID := terminalSessionID(workspaceID, terminalID)
	client, err := a.getSessiondClient()
	if err != nil {
		return TerminalBacklogPayload{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	backlog, err := client.Backlog(ctx, sessionID, since)
	if err != nil {
		return TerminalBacklogPayload{}, err
	}
	return TerminalBacklogPayload{
		WorkspaceID: workspaceID,
		TerminalID:  terminalID,
		Data:        backlog.Data,
		NextOffset:  backlog.NextOffset,
		Truncated:   backlog.Truncated,
		Source:      backlog.Source,
	}, nil
}

func (a *App) GetTerminalSnapshot(workspaceID, terminalID string) (TerminalSnapshotPayload, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalSnapshotPayload{}, fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return TerminalSnapshotPayload{}, fmt.Errorf("terminal id required")
	}
	sessionID := terminalSessionID(workspaceID, terminalID)
	client, err := a.getSessiondClient()
	if err != nil {
		return TerminalSnapshotPayload{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	snap, err := client.Snapshot(ctx, sessionID)
	if err != nil {
		return TerminalSnapshotPayload{}, err
	}
	return TerminalSnapshotPayload{
		WorkspaceID: workspaceID,
		TerminalID:  terminalID,
		Data:        snap.Data,
		Source:      snap.Source,
		Kitty:       snap.Kitty,
	}, nil
}

func (a *App) GetTerminalBootstrap(workspaceID, terminalID string) (TerminalBootstrapPayload, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalBootstrapPayload{}, fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return TerminalBootstrapPayload{}, fmt.Errorf("terminal id required")
	}
	sessionID := terminalSessionID(workspaceID, terminalID)
	client, err := a.getSessiondClient()
	if err != nil {
		return TerminalBootstrapPayload{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	bootstrap, err := client.Bootstrap(ctx, sessionID)
	if err != nil {
		return TerminalBootstrapPayload{}, err
	}
	backlog := sessiond.BacklogResponse{}
	useBacklog := bootstrap.SafeToReplay && bootstrap.Snapshot == ""
	if useBacklog {
		if b, err := client.Backlog(ctx, sessionID, 0); err == nil && b.Data != "" {
			backlog = b
		} else {
			useBacklog = false
		}
	}
	a.terminalMu.Lock()
	session := a.terminals[sessionID]
	a.terminalMu.Unlock()
	if session != nil {
		session.mu.Lock()
		session.altScreen = bootstrap.AltScreen
		if bootstrap.AltScreen {
			session.tuiMode = true
		}
		if bootstrap.MouseMask != 0 {
			session.mouseMask = bootstrap.MouseMask
		} else if bootstrap.Mouse {
			session.mouseMask = 1
		} else {
			session.mouseMask = 0
		}
		session.mouseSGR = bootstrap.MouseSGR
		session.mouseUTF8 = bootstrap.MouseEncoding == "utf8"
		session.mouseURXVT = bootstrap.MouseEncoding == "urxvt"
		session.mu.Unlock()
		_ = a.persistTerminalState()
	}
	if useBacklog {
		return TerminalBootstrapPayload{
			WorkspaceID:      workspaceID,
			TerminalID:       terminalID,
			Backlog:          backlog.Data,
			BacklogSource:    backlog.Source,
			BacklogTruncated: backlog.Truncated,
			NextOffset:       backlog.NextOffset,
			Source:           "sessiond",
			AltScreen:        bootstrap.AltScreen,
			Mouse:            bootstrap.Mouse,
			MouseSGR:         bootstrap.MouseSGR,
			MouseEncoding:    bootstrap.MouseEncoding,
			SafeToReplay:     bootstrap.SafeToReplay,
		}, nil
	}
	return TerminalBootstrapPayload{
		WorkspaceID:      workspaceID,
		TerminalID:       terminalID,
		Snapshot:         bootstrap.Snapshot,
		SnapshotSource:   bootstrap.SnapshotSource,
		Kitty:            bootstrap.Kitty,
		Backlog:          bootstrap.Backlog,
		BacklogSource:    bootstrap.BacklogSource,
		BacklogTruncated: bootstrap.BacklogTruncated,
		NextOffset:       bootstrap.NextOffset,
		Source:           "sessiond",
		AltScreen:        bootstrap.AltScreen,
		Mouse:            bootstrap.Mouse,
		MouseSGR:         bootstrap.MouseSGR,
		MouseEncoding:    bootstrap.MouseEncoding,
		SafeToReplay:     bootstrap.SafeToReplay,
	}, nil
}

func (a *App) LogTerminalDebug(payload TerminalDebugPayload) {
	logTerminalDebug(payload)
}

func (a *App) GetWorkspaceTerminalStatus(workspaceID, terminalID string) TerminalStatusPayload {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalStatusPayload{Active: false, Error: "workspace id required"}
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return TerminalStatusPayload{WorkspaceID: workspaceID, Active: false, Error: "terminal id required"}
	}
	sessionID := terminalSessionID(workspaceID, terminalID)
	a.terminalMu.Lock()
	session := a.terminals[sessionID]
	a.terminalMu.Unlock()
	if session != nil {
		session.mu.Lock()
		hasSession := !session.closed && session.client != nil
		session.mu.Unlock()
		if hasSession {
			return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: true}
		}
	}
	client, err := a.getSessiondClient()
	if err != nil {
		return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: false, Error: err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	list, err := client.List(ctx)
	if err != nil {
		return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: false, Error: err.Error()}
	}
	for _, info := range list.Sessions {
		if info.SessionID == sessionID && info.Running {
			return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: true}
		}
	}
	return TerminalStatusPayload{WorkspaceID: workspaceID, TerminalID: terminalID, Active: false}
}

func (a *App) ResizeWorkspaceTerminal(workspaceID, terminalID string, cols, rows int) error {
	session, err := a.getTerminal(workspaceID, terminalID)
	if err != nil {
		return err
	}
	if cols < 2 {
		cols = 2
	}
	if rows < 1 {
		rows = 1
	}
	return session.Resize(cols, rows)
}

func (a *App) StopWorkspaceTerminal(workspaceID, terminalID string) error {
	a.terminalMu.Lock()
	sessionID := terminalSessionID(workspaceID, terminalID)
	session, ok := a.terminals[sessionID]
	if ok {
		delete(a.terminals, sessionID)
	}
	a.terminalMu.Unlock()
	if !ok {
		return nil
	}
	session.mu.Lock()
	client := session.client
	session.mu.Unlock()
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = client.Stop(ctx, sessionID)
		cancel()
	}
	err := session.CloseWithReason("closed")
	a.emitTerminalLifecycle("closed", session, "")
	_ = a.persistTerminalState()
	return err
}

func (a *App) getTerminal(workspaceID, terminalID string) (*terminalSession, error) {
	a.terminalMu.Lock()
	sessionID := terminalSessionID(workspaceID, terminalID)
	session := a.terminals[sessionID]
	a.terminalMu.Unlock()
	if session == nil {
		return nil, fmt.Errorf("terminal not found")
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err := session.waitReady(waitCtx)
	cancel()
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (a *App) latestTerminalForWorkspace(workspaceID string) *terminalSession {
	a.terminalMu.Lock()
	defer a.terminalMu.Unlock()
	var selected *terminalSession
	var latest time.Time
	for _, session := range a.terminals {
		if session.workspaceID != workspaceID {
			continue
		}
		session.mu.Lock()
		last := session.lastActivity
		closed := session.closed
		session.mu.Unlock()
		if closed {
			continue
		}
		if selected == nil || last.After(latest) {
			selected = session
			latest = last
		}
	}
	return selected
}

func (a *App) streamTerminal(session *terminalSession) {
	session.mu.Lock()
	client := session.client
	session.mu.Unlock()
	if client == nil {
		a.emitTerminalLifecycle("error", session, "sessiond unavailable")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	session.mu.Lock()
	session.streamCancel = cancel
	session.mu.Unlock()
	stream, first, err := client.Attach(ctx, session.id, 0, false)
	if err != nil {
		session.mu.Lock()
		session.client = nil
		session.mu.Unlock()
		a.emitTerminalLifecycle("error", session, err.Error())
		return
	}
	session.mu.Lock()
	session.stream = stream
	session.mu.Unlock()
	if first.Type == "error" && first.Error != "" {
		session.mu.Lock()
		session.client = nil
		session.mu.Unlock()
		a.emitTerminalLifecycle("error", session, first.Error)
		return
	}
	applyModes := func(data string) {
		if data == "" {
			return
		}
		session.bumpActivity()
		altChanged, mouseChanged := session.noteModes([]byte(data))
		session.mu.Lock()
		altScreen := session.altScreen
		mouseEnabled := session.mouseEnabled()
		mouseSGR := session.mouseSGR
		mouseEncoding := session.mouseEncoding()
		session.mu.Unlock()
		if mouseChanged || altChanged {
			a.emitTerminalModes(session, altScreen, mouseEnabled, mouseSGR, mouseEncoding)
			_ = a.persistTerminalState()
		}
	}
	for {
		var msg sessiond.StreamMessage
		if err := stream.Next(&msg); err != nil {
			session.mu.Lock()
			session.client = nil
			session.mu.Unlock()
			break
		}
		if msg.Type == "backlog" {
			if msg.Truncated {
				a.emitTerminalLifecycle("started", session, "Backlog truncated; skipping replay.")
				continue
			}
			if msg.Data != "" {
				applyModes(msg.Data)
				wruntime.EventsEmit(a.ctx, "terminal:data", TerminalPayload{
					WorkspaceID: session.workspaceID,
					TerminalID:  session.terminalID,
					Data:        msg.Data,
				})
			}
		}
		if msg.Type == "kitty_snapshot" && msg.Kitty != nil {
			wruntime.EventsEmit(a.ctx, "terminal:kitty", TerminalKittyPayload{
				WorkspaceID: session.workspaceID,
				TerminalID:  session.terminalID,
				Event:       *msg.Kitty,
			})
		}
		if msg.Type == "kitty" && msg.Kitty != nil {
			wruntime.EventsEmit(a.ctx, "terminal:kitty", TerminalKittyPayload{
				WorkspaceID: session.workspaceID,
				TerminalID:  session.terminalID,
				Event:       *msg.Kitty,
			})
		}
		if msg.Type == "data" && msg.Data != "" {
			applyModes(msg.Data)
			wruntime.EventsEmit(a.ctx, "terminal:data", TerminalPayload{
				WorkspaceID: session.workspaceID,
				TerminalID:  session.terminalID,
				Data:        msg.Data,
			})
		}
		if msg.Type == "closed" {
			break
		}
	}
	_ = session.CloseWithReason("closed")
	a.emitTerminalLifecycle("closed", session, "")
	return
}

func (a *App) invalidateTerminalSessions(reason string) {
	a.terminalMu.Lock()
	sessions := make([]*terminalSession, 0, len(a.terminals))
	for _, session := range a.terminals {
		sessions = append(sessions, session)
	}
	a.terminalMu.Unlock()
	for _, session := range sessions {
		_ = session.CloseWithReason("sessiond reset")
		session.mu.Lock()
		session.client = nil
		session.mu.Unlock()
		if reason != "" {
			a.emitTerminalLifecycle("error", session, reason)
		}
	}
}

var (
	terminalFilterOnce    sync.Once
	terminalFilterEnabled bool
	terminalFilterDebug   bool
	terminalFilterLog     *os.File
	terminalFilterMu      sync.Mutex

	terminalDebugOnce    sync.Once
	terminalDebugEnabled bool
	terminalDebugLog     *os.File
	terminalDebugMu      sync.Mutex
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

func terminalDebugLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "terminal_debug.log"), nil
}

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func terminalDebugConfig() bool {
	terminalDebugOnce.Do(func() {
		terminalDebugEnabled = envTruthy(os.Getenv("WORKSET_TERMINAL_DEBUG_LOG"))
		if !terminalDebugEnabled {
			return
		}
		logPath := strings.TrimSpace(os.Getenv("WORKSET_TERMINAL_DEBUG_LOG_PATH"))
		if logPath == "" {
			path, err := terminalDebugLogPath()
			if err != nil {
				terminalDebugEnabled = false
				return
			}
			logPath = path
		}
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			terminalDebugEnabled = false
			return
		}
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			terminalDebugEnabled = false
			return
		}
		terminalDebugLog = file
	})
	return terminalDebugEnabled && terminalDebugLog != nil
}

func logTerminalDebug(payload TerminalDebugPayload) {
	if !terminalDebugConfig() {
		return
	}
	if payload.Event == "" {
		payload.Event = "event"
	}
	details := strings.ReplaceAll(payload.Details, "\n", "\\n")
	terminalDebugMu.Lock()
	defer terminalDebugMu.Unlock()
	_, _ = fmt.Fprintf(
		terminalDebugLog,
		"%s event=%s workspace=%s terminal=%s details=%s\n",
		time.Now().Format(time.RFC3339Nano),
		payload.Event,
		payload.WorkspaceID,
		payload.TerminalID,
		details,
	)
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
	const bel = 0x07
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
	if last < len(data) {
		out = append(out, data[last:]...)
	}
	return out
}

type escapeStringFilter struct {
	enabled    bool
	debug      bool
	configured bool
	active     bool
	pendingEsc bool
	kind       string
	logBuf     []byte
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
	if f.active {
		end := scanEscapeStringTerminator(data, 0)
		if end == 0 {
			f.appendLog(data)
			if f.enabled {
				return nil
			}
			return data
		}
		f.appendLog(data[:end])
		if f.debug && len(f.logBuf) > 0 {
			logTerminalFilter(f.kind, f.logBuf)
		}
		f.reset()
		if f.enabled {
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

func (a *App) resolveWorkspaceRoot(ctx context.Context, workspaceID string) (string, error) {
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, err := a.service.ListWorkspaces(ctx)
	if err != nil {
		return "", err
	}
	for _, workspace := range result.Workspaces {
		if workspace.Name == workspaceID {
			return workspace.Path, nil
		}
	}
	return "", fmt.Errorf("workspace not found: %s", workspaceID)
}

func (a *App) emitTerminalLifecycle(status string, session *terminalSession, message string) {
	if session == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, "terminal:lifecycle", TerminalLifecyclePayload{
		WorkspaceID: session.workspaceID,
		TerminalID:  session.terminalID,
		Status:      status,
		Message:     message,
	})
}

func (a *App) emitTerminalModes(session *terminalSession, altScreen, mouse, mouseSGR bool, mouseEncoding string) {
	if session == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, "terminal:modes", TerminalModesPayload{
		WorkspaceID:   session.workspaceID,
		TerminalID:    session.terminalID,
		AltScreen:     altScreen,
		Mouse:         mouse,
		MouseSGR:      mouseSGR,
		MouseEncoding: mouseEncoding,
	})
}

func (a *App) ensureIdleWatcher(session *terminalSession) {
	session.mu.Lock()
	if session.client != nil {
		session.mu.Unlock()
		return
	}
	if session.idleTimer != nil {
		session.mu.Unlock()
		return
	}
	idleTimeout := a.terminalIdleTimeout()
	if idleTimeout <= 0 {
		session.mu.Unlock()
		return
	}
	session.idleTimeout = idleTimeout
	session.lastActivity = time.Now()
	session.idleTimer = time.AfterFunc(session.idleTimeout, func() {
		a.handleIdleTimeout(session)
	})
	session.mu.Unlock()
}

func (a *App) terminalIdleTimeout() time.Duration {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	cfg, _, err := a.service.GetConfig(ctx)
	if err != nil {
		return 30 * time.Minute
	}
	raw := strings.TrimSpace(cfg.Defaults.TerminalIdleTimeout)
	if raw == "" {
		return 30 * time.Minute
	}
	switch strings.ToLower(raw) {
	case "0", "off", "disabled", "false":
		return 0
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout < 0 {
		return 30 * time.Minute
	}
	return timeout
}

func (a *App) handleIdleTimeout(session *terminalSession) {
	a.terminalMu.Lock()
	current := a.terminals[session.id]
	if current != session {
		a.terminalMu.Unlock()
		return
	}
	delete(a.terminals, session.id)
	a.terminalMu.Unlock()

	_ = session.CloseWithReason("idle")
	a.emitTerminalLifecycle("idle", session, "")
	_ = a.persistTerminalState()
}

func (a *App) persistTerminalState() error {
	statePath, err := a.terminalStatePath()
	if err != nil {
		return err
	}
	state := a.snapshotTerminalState()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0o644)
}

func (a *App) snapshotTerminalState() terminalState {
	a.terminalMu.Lock()
	defer a.terminalMu.Unlock()
	entries := make([]terminalStateEntry, 0, len(a.terminals))
	for _, session := range a.terminals {
		if lastActive, ok := session.snapshot(); ok {
			session.mu.Lock()
			modes := terminalModeState{
				AltScreen:  session.altScreen,
				MouseMask:  session.mouseMask,
				MouseSGR:   session.mouseSGR,
				MouseUTF8:  session.mouseUTF8,
				MouseURXVT: session.mouseURXVT,
			}
			session.mu.Unlock()
			entries = append(entries, terminalStateEntry{
				WorkspaceID: session.workspaceID,
				TerminalID:  session.terminalID,
				LastActive:  lastActive,
				Modes:       &modes,
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastActive.After(entries[j].LastActive)
	})
	return terminalState{Sessions: entries}
}

func (a *App) terminalStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "ui_sessions.json"), nil
}

func (a *App) restoreTerminalSessions(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	statePath, err := a.terminalStatePath()
	var state terminalState
	if err == nil {
		if data, readErr := os.ReadFile(statePath); readErr == nil {
			if jsonErr := json.Unmarshal(data, &state); jsonErr == nil {
				a.terminalMu.Lock()
				if a.restoredModes == nil {
					a.restoredModes = map[string]terminalModeState{}
				}
				for _, entry := range state.Sessions {
					if entry.Modes != nil {
						sessionID := terminalSessionID(entry.WorkspaceID, entry.TerminalID)
						a.restoredModes[sessionID] = *entry.Modes
					}
				}
				a.terminalMu.Unlock()
			}
		}
	}
	if client, err := a.getSessiondClient(); err == nil {
		listCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		list, err := client.List(listCtx)
		cancel()
		if err == nil && len(list.Sessions) > 0 {
			for _, info := range list.Sessions {
				if info.Running {
					workspaceID, terminalID, _ := parseTerminalSessionID(info.SessionID)
					_ = a.StartWorkspaceTerminal(workspaceID, terminalID)
				}
			}
			return
		}
	}
	for _, entry := range state.Sessions {
		_ = a.StartWorkspaceTerminal(entry.WorkspaceID, entry.TerminalID)
	}
}
