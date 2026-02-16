package sessiond

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/strantalis/workset/pkg/unifiedlog"
)

type Server struct {
	opts     Options
	sessions map[string]*Session
	creating map[string]*createCall
	mu       sync.Mutex
	shutdown func()
}

type createCall struct {
	done    chan struct{}
	session *Session
	err     error
}

func NewServer(opts Options) *Server {
	if opts.SocketPath == "" {
		path, err := DefaultSocketPath()
		if err == nil {
			opts.SocketPath = path
		}
	}
	if opts.TranscriptDir == "" {
		dir, err := DefaultTranscriptDir()
		if err == nil {
			opts.TranscriptDir = dir
		}
	}
	if opts.RecordDir == "" {
		dir, err := DefaultRecordDir()
		if err == nil {
			opts.RecordDir = dir
		}
	}
	if opts.BufferBytes == 0 {
		opts.BufferBytes = DefaultOptions().BufferBytes
	}
	if opts.TranscriptMaxBytes == 0 {
		opts.TranscriptMaxBytes = DefaultOptions().TranscriptMaxBytes
	}
	if opts.TranscriptTrimThreshold == 0 {
		opts.TranscriptTrimThreshold = DefaultOptions().TranscriptTrimThreshold
	}
	if opts.TranscriptTailBytes == 0 {
		opts.TranscriptTailBytes = DefaultOptions().TranscriptTailBytes
	}
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = DefaultOptions().IdleTimeout
	}
	if opts.ProtocolLogEnabled && opts.ProtocolLogger == nil {
		logger, err := unifiedlog.Open("sessiond", opts.ProtocolLogDir)
		if err != nil {
			logServerf("protocol_log_open_failed err=%v", err)
		} else {
			opts.ProtocolLogger = logger
		}
	}
	return &Server{
		opts:     opts,
		sessions: make(map[string]*Session),
		creating: make(map[string]*createCall),
	}
}

func (s *Server) SetShutdown(fn func()) {
	s.shutdown = fn
}

func (s *Server) Listen(ctx context.Context) error {
	if s.opts.SocketPath == "" {
		return errors.New("socket path required")
	}
	if err := os.MkdirAll(filepath.Dir(s.opts.SocketPath), 0o755); err != nil {
		logServerf("mkdir_error path=%s err=%v", filepath.Dir(s.opts.SocketPath), err)
		return err
	}
	if err := os.Remove(s.opts.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		logServerf("socket_remove_error path=%s err=%v", s.opts.SocketPath, err)
	}
	logServerf("listen_start socket=%s", s.opts.SocketPath)
	ln, err := net.Listen("unix", s.opts.SocketPath)
	if err != nil && shouldRetryListen(err) {
		logServerf("listen_retry socket=%s err=%v", s.opts.SocketPath, err)
		if rmErr := os.Remove(s.opts.SocketPath); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			logServerf("listen_retry_remove_error socket=%s err=%v", s.opts.SocketPath, rmErr)
		}
		time.Sleep(100 * time.Millisecond)
		ln, err = net.Listen("unix", s.opts.SocketPath)
	}
	if err != nil {
		logServerf("listen_failed socket=%s err=%v", s.opts.SocketPath, err)
		return err
	}
	logServerf("listen_ready socket=%s", s.opts.SocketPath)
	defer func() {
		_ = ln.Close()
		_ = os.Remove(s.opts.SocketPath)
		s.closeAll()
	}()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, os.ErrClosed) {
				logServerf("listen_closed socket=%s", s.opts.SocketPath)
				return nil
			}
			continue
		}
		go s.handleConn(ctx, conn)
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return
	}
	line = bytesTrimSpace(line)
	if len(line) == 0 {
		return
	}
	var envelope struct {
		Type   string `json:"type"`
		Method string `json:"method"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil {
		return
	}
	if envelope.Type == "attach" {
		s.handleAttach(conn, line)
		return
	}
	s.handleControl(ctx, conn, line)
}

func (s *Server) handleControl(ctx context.Context, conn net.Conn, line []byte) {
	var req ControlRequest
	if err := json.Unmarshal(line, &req); err != nil {
		_ = json.NewEncoder(conn).Encode(ControlResponse{OK: false, Error: err.Error()})
		return
	}
	if req.ProtocolVersion != ProtocolVersion {
		s.writeError(conn, fmt.Errorf("protocol mismatch: server=%d client=%d", ProtocolVersion, req.ProtocolVersion))
		return
	}
	switch req.Method {
	case "ping":
		_ = json.NewEncoder(conn).Encode(ControlResponse{OK: true})
	case "create":
		var params CreateRequest
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.writeError(conn, err)
			return
		}
		session, existing, err := s.getOrCreate(ctx, params.SessionID, params.Cwd)
		if err != nil {
			s.writeError(conn, err)
			return
		}
		_ = json.NewEncoder(conn).Encode(ControlResponse{
			OK: true,
			Result: CreateResponse{
				SessionID: session.id,
				Existing:  existing,
			},
		})
	case "send":
		var params SendRequest
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.writeError(conn, err)
			return
		}
		session := s.get(params.SessionID)
		if session == nil {
			s.writeError(conn, errors.New("session not found"))
			return
		}
		if err := session.writeForOwner(ctx, params.Data, params.Owner); err != nil {
			s.writeError(conn, err)
			return
		}
		_ = json.NewEncoder(conn).Encode(ControlResponse{OK: true})
	case "resize":
		var params ResizeRequest
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.writeError(conn, err)
			return
		}
		session := s.get(params.SessionID)
		if session == nil {
			s.writeError(conn, errors.New("session not found"))
			return
		}
		if err := session.resize(params.Cols, params.Rows); err != nil {
			s.writeError(conn, err)
			return
		}
		_ = json.NewEncoder(conn).Encode(ControlResponse{OK: true})
	case "stop":
		var params StopRequest
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.writeError(conn, err)
			return
		}
		session := s.get(params.SessionID)
		if session != nil {
			session.closeWithReason("closed")
			s.remove(params.SessionID)
		}
		_ = json.NewEncoder(conn).Encode(ControlResponse{OK: true})
	case "list":
		s.mu.Lock()
		snap := make([]*Session, 0, len(s.sessions))
		for _, session := range s.sessions {
			snap = append(snap, session)
		}
		s.mu.Unlock()
		start := time.Now()
		sessions := make([]SessionInfo, 0, len(snap))
		for _, session := range snap {
			sessions = append(sessions, session.info())
		}
		if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
			logServerf("list_slow count=%d duration=%s", len(sessions), elapsed)
		}
		_ = json.NewEncoder(conn).Encode(ControlResponse{OK: true, Result: ListResponse{Sessions: sessions}})
	case "set_owner":
		var params OwnerRequest
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.writeError(conn, err)
			return
		}
		session := s.get(params.SessionID)
		if session == nil {
			s.writeError(conn, errors.New("session not found"))
			return
		}
		owner := session.setInputOwner(params.Owner)
		_ = json.NewEncoder(conn).Encode(ControlResponse{
			OK: true,
			Result: OwnerResponse{
				SessionID: params.SessionID,
				Owner:     owner,
			},
		})
	case "get_owner":
		var params OwnerRequest
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.writeError(conn, err)
			return
		}
		session := s.get(params.SessionID)
		if session == nil {
			s.writeError(conn, errors.New("session not found"))
			return
		}
		_ = json.NewEncoder(conn).Encode(ControlResponse{
			OK: true,
			Result: OwnerResponse{
				SessionID: params.SessionID,
				Owner:     session.getInputOwner(),
			},
		})
	case "info":
		exe, err := os.Executable()
		if err != nil {
			s.writeError(conn, err)
			return
		}
		hash, err := BinaryHash(exe)
		if err != nil {
			s.writeError(conn, err)
			return
		}
		_ = json.NewEncoder(conn).Encode(ControlResponse{
			OK: true,
			Result: InfoResponse{
				Executable: exe,
				BinaryHash: hash,
			},
		})
	case "shutdown":
		var params ShutdownRequest
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				logServerf("shutdown_request_decode_failed err=%v", err)
			}
		}
		source := strings.TrimSpace(params.Source)
		if source == "" {
			source = "unknown"
		}
		reason := strings.TrimSpace(params.Reason)
		exe := strings.TrimSpace(params.Executable)
		if exe == "" {
			exe = "unknown"
		}
		logServerf("shutdown_requested source=%q reason=%q pid=%d exe=%q", source, reason, params.PID, exe)
		_ = json.NewEncoder(conn).Encode(ControlResponse{OK: true})
		logServerf("shutdown_ack")
		go func() {
			if s.shutdown != nil {
				s.shutdown()
			}
			s.closeAll()
		}()
	default:
		s.writeError(conn, fmt.Errorf("unknown method %q", req.Method))
	}
}

func (s *Server) handleAttach(conn net.Conn, line []byte) {
	enc := json.NewEncoder(conn)
	var req AttachRequest
	if err := json.Unmarshal(line, &req); err != nil {
		_ = enc.Encode(StreamMessage{Type: "error", Error: err.Error()})
		return
	}
	if req.ProtocolVersion != ProtocolVersion {
		_ = enc.Encode(StreamMessage{
			Type:  "error",
			Error: fmt.Sprintf("protocol mismatch: server=%d client=%d", ProtocolVersion, req.ProtocolVersion),
		})
		return
	}
	session := s.get(req.SessionID)
	if session == nil {
		_ = enc.Encode(StreamMessage{Type: "error", Error: "session not found"})
		return
	}
	if !session.isRunning() {
		_ = enc.Encode(StreamMessage{Type: "error", Error: "session not running"})
		return
	}
	streamID := strings.TrimSpace(req.StreamID)
	if streamID == "" {
		streamID = newStreamID()
	}
	var replay []byte
	// Snapshot buffered output before subscribing while the output lock is held
	// so newly attached viewers can reconstruct terminal mode/state.
	session.outputMu.Lock()
	prefix := session.modeReplayPrefixLocked()
	if session.buffer != nil {
		replay, _, _ = session.buffer.ReadSince(0)
	}
	if len(prefix) > 0 {
		withPrefix := make([]byte, 0, len(prefix)+len(replay))
		withPrefix = append(withPrefix, prefix...)
		withPrefix = append(withPrefix, replay...)
		replay = withPrefix
	}
	sub := session.subscribe(streamID)
	session.outputMu.Unlock()
	defer session.unsubscribe(sub)
	if err := enc.Encode(StreamMessage{Type: "ready", SessionID: req.SessionID, StreamID: streamID}); err != nil {
		return
	}
	if len(replay) > 0 {
		if err := enc.Encode(StreamMessage{
			Type:      "data",
			SessionID: req.SessionID,
			StreamID:  streamID,
			DataB64:   base64.StdEncoding.EncodeToString(replay),
			Len:       len(replay),
		}); err != nil {
			return
		}
	}
	for event := range sub.ch {
		if err := enc.Encode(StreamMessage{
			Type:      "data",
			SessionID: req.SessionID,
			StreamID:  streamID,
			DataB64:   base64.StdEncoding.EncodeToString(event),
			Len:       len(event),
		}); err != nil {
			return
		}
	}
	_ = enc.Encode(StreamMessage{Type: "closed", SessionID: req.SessionID, StreamID: streamID})
}

func (s *Server) writeError(conn net.Conn, err error) {
	_ = json.NewEncoder(conn).Encode(ControlResponse{OK: false, Error: err.Error()})
}

func (s *Server) get(id string) *Session {
	s.mu.Lock()
	session := s.sessions[id]
	s.mu.Unlock()
	if session == nil {
		return nil
	}
	if session.isClosed() {
		s.remove(id)
		return nil
	}
	return session
}

func (s *Server) remove(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

func (s *Server) closeAll() {
	s.mu.Lock()
	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	s.sessions = make(map[string]*Session)
	s.mu.Unlock()
	if len(sessions) > 0 {
		logServerf("close_all count=%d", len(sessions))
	}
	for _, session := range sessions {
		session.closeWithReason("shutdown")
	}
}

func (s *Server) getOrCreate(ctx context.Context, id, cwd string) (*Session, bool, error) {
	if id == "" {
		return nil, false, errors.New("session id required")
	}
	for {
		s.mu.Lock()
		if s.creating == nil {
			s.creating = make(map[string]*createCall)
		}
		existing := s.sessions[id]
		if existing != nil {
			s.mu.Unlock()
			if existing.isClosed() {
				s.remove(id)
				continue
			}
			if !existing.isRunning() {
				s.remove(id)
				continue
			}
			return existing, true, nil
		}
		if call := s.creating[id]; call != nil {
			done := call.done
			s.mu.Unlock()
			select {
			case <-done:
				if call.err != nil {
					return nil, false, call.err
				}
				if call.session == nil || call.session.isClosed() || !call.session.isRunning() {
					return nil, false, errors.New("session not running")
				}
				return call.session, true, nil
			case <-ctx.Done():
				return nil, false, ctx.Err()
			}
		}
		call := &createCall{done: make(chan struct{})}
		s.creating[id] = call
		s.mu.Unlock()

		session := newSession(s.opts, id, cwd)
		session.onClose = s.onSessionClosed
		err := session.start(ctx)

		s.mu.Lock()
		delete(s.creating, id)
		if err == nil && !session.isClosed() && session.isRunning() {
			s.sessions[id] = session
		}
		s.mu.Unlock()

		call.session = session
		call.err = err
		close(call.done)

		if err != nil {
			return nil, false, err
		}
		if session.isClosed() || !session.isRunning() {
			return nil, false, errors.New("session not running")
		}
		return session, false, nil
	}
}

func (s *Server) onSessionClosed(session *Session) {
	if session == nil {
		return
	}
	s.remove(session.id)
}

func bytesTrimSpace(input []byte) []byte {
	start := 0
	for start < len(input) && (input[start] == ' ' || input[start] == '\n' || input[start] == '\r' || input[start] == '\t') {
		start++
	}
	end := len(input)
	for end > start && (input[end-1] == ' ' || input[end-1] == '\n' || input[end-1] == '\r' || input[end-1] == '\t') {
		end--
	}
	return input[start:end]
}

func shouldRetryListen(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "address already in use") || strings.Contains(msg, "file exists")
}

func logServerf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "sessiond: "+format+"\n", args...)
}
