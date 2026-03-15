package sessiond

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	websocketStreamPath       = "/stream"
	websocketBinaryHeaderSize = 8
	websocketReadLimitBytes   = 1 << 20
)

func (s *Server) startWebsocketServer() (net.Listener, *http.Server, error) {
	host := strings.TrimSpace(s.opts.WebSocketHost)
	if host == "" {
		host = DefaultOptions().WebSocketHost
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return nil, nil, err
	}
	token, err := newWebsocketToken()
	if err != nil {
		_ = ln.Close()
		return nil, nil, err
	}
	s.wsURL = fmt.Sprintf("ws://%s%s", ln.Addr().String(), websocketStreamPath)
	s.wsToken = token
	mux := http.NewServeMux()
	mux.HandleFunc(websocketStreamPath, s.handleWebsocketAttach)
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return ln, server, nil
}

func (s *Server) handleWebsocketAttach(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		CompressionMode:    websocket.CompressionDisabled,
	})
	if err != nil {
		return
	}
	conn.SetReadLimit(websocketReadLimitBytes)
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "closed")
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	messageType, payload, err := conn.Read(ctx)
	if err != nil {
		logServerf("ws_attach_handshake_failed remote=%s err=%v", r.RemoteAddr, err)
		return
	}
	if messageType != websocket.MessageText {
		_ = s.writeWebsocketControl(ctx, conn, StreamMessage{
			Type:  "error",
			Error: "attach request must be a text frame",
		})
		_ = conn.Close(websocket.StatusPolicyViolation, "attach request must be a text frame")
		return
	}

	var req AttachRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		_ = s.writeWebsocketControl(ctx, conn, StreamMessage{Type: "error", Error: err.Error()})
		_ = conn.Close(websocket.StatusInvalidFramePayloadData, "invalid attach request")
		return
	}
	if req.ProtocolVersion != ProtocolVersion {
		_ = s.writeWebsocketControl(ctx, conn, StreamMessage{
			Type:  "error",
			Error: fmt.Sprintf("protocol mismatch: server=%d client=%d", ProtocolVersion, req.ProtocolVersion),
		})
		_ = conn.Close(websocket.StatusPolicyViolation, "protocol mismatch")
		return
	}
	if subtleTokenMismatch(strings.TrimSpace(req.Token), s.wsToken) {
		_ = s.writeWebsocketControl(ctx, conn, StreamMessage{Type: "error", Error: "invalid websocket token"})
		_ = conn.Close(websocket.StatusPolicyViolation, "invalid websocket token")
		return
	}
	session := s.get(req.SessionID)
	if session == nil {
		_ = s.writeWebsocketControl(ctx, conn, StreamMessage{Type: "error", Error: "session not found"})
		_ = conn.Close(websocket.StatusNormalClosure, "session not found")
		return
	}
	if !session.isRunning() {
		_ = s.writeWebsocketControl(ctx, conn, StreamMessage{Type: "error", Error: "session not running"})
		_ = conn.Close(websocket.StatusNormalClosure, "session not running")
		return
	}

	streamID := strings.TrimSpace(req.StreamID)
	if streamID == "" {
		streamID = newStreamID()
	}

	session.outputMu.Lock()
	snapshot := session.snapshotAttachLocked(req)
	sub := session.subscribe(streamID, snapshot.ready.ReplayNext)
	session.outputMu.Unlock()
	state := session.getStreamState()
	logServerf(
		"ws_attach_open session=%s stream=%s client=%q remote=%s since=%d cols=%d rows=%d owner=%q subscribers=%d streams=%q replay_next=%d replay_bytes=%d snapshot=%t snapshot_bytes=%d replay_truncated=%t replay_skipped=%t",
		req.SessionID,
		streamID,
		req.ClientID,
		r.RemoteAddr,
		req.Since,
		req.Cols,
		req.Rows,
		snapshot.ready.Owner,
		state.Count,
		state.StreamIDs,
		snapshot.ready.ReplayNext,
		len(snapshot.replay),
		len(snapshot.snapshot) > 0,
		len(snapshot.snapshot),
		snapshot.ready.ReplayTruncated,
		snapshot.ready.ReplaySkipped,
	)

	closeReason := "handler_exit"
	closeCode := websocket.StatusNormalClosure
	closeText := "closed"
	var closeMu sync.Mutex
	setClose := func(reason string, code websocket.StatusCode, text string) {
		closeMu.Lock()
		closeReason = reason
		closeCode = code
		closeText = text
		closeMu.Unlock()
	}
	getClose := func() (string, websocket.StatusCode, string) {
		closeMu.Lock()
		reason := closeReason
		code := closeCode
		text := closeText
		closeMu.Unlock()
		return reason, code, text
	}
	defer func() {
		reason, code, text := getClose()
		session.unsubscribeWithReason(sub, reason)
		state := session.getStreamState()
		logServerf(
			"ws_attach_close session=%s stream=%s client=%q reason=%s code=%d text=%q owner=%q subscribers=%d streams=%q sub_offset=%d",
			req.SessionID,
			streamID,
			req.ClientID,
			reason,
			code,
			text,
			session.getInputOwner(),
			state.Count,
			state.StreamIDs,
			sub.getOffset(),
		)
	}()

	streamCtx, streamCancel := context.WithCancel(r.Context())
	defer streamCancel()
	var writeMu sync.Mutex

	writeControl := func(ctx context.Context, message StreamMessage) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return s.writeWebsocketControl(ctx, conn, message)
	}

	writeBinary := func(ctx context.Context, nextOffset int64, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return s.writeWebsocketBinary(ctx, conn, nextOffset, data)
	}

	if err := writeControl(streamCtx, StreamMessage{
		Type:      "ready",
		SessionID: req.SessionID,
		StreamID:  streamID,
		Ready:     &snapshot.ready,
	}); err != nil {
		setClose("write_ready_failed", websocket.StatusNormalClosure, err.Error())
		return
	}
	if len(snapshot.snapshot) > 0 {
		if err := writeControl(streamCtx, StreamMessage{
			Type:      "snapshot",
			SessionID: req.SessionID,
			StreamID:  streamID,
			Snapshot:  snapshot.snapshot,
		}); err != nil {
			setClose("write_snapshot_failed", websocket.StatusNormalClosure, err.Error())
			return
		}
	}

	go func() {
		defer streamCancel()
		for {
			messageType, payload, err := conn.Read(streamCtx)
			if err != nil {
				if streamCtx.Err() != nil {
					return
				}
				var closeErr websocket.CloseError
				if errors.As(err, &closeErr) &&
					(closeErr.Code == websocket.StatusNormalClosure ||
						closeErr.Code == websocket.StatusGoingAway) {
					setClose("client_closed", closeErr.Code, closeErr.Reason)
					return
				}
				setClose("client_read_failed", websocket.StatusNormalClosure, err.Error())
				return
			}
			if messageType != websocket.MessageText {
				_ = writeControl(streamCtx, StreamMessage{
					Type:  "error",
					Error: "control request must be a text frame",
				})
				setClose(
					"invalid_control_frame",
					websocket.StatusPolicyViolation,
					"control request must be a text frame",
				)
				return
			}

			var controlReq WebsocketControlRequest
			if err := json.Unmarshal(payload, &controlReq); err != nil {
				_ = writeControl(streamCtx, StreamMessage{Type: "error", Error: err.Error()})
				logServerf("ws_control_decode_failed session=%s stream=%s err=%v", req.SessionID, streamID, err)
				continue
			}
			if controlReq.ProtocolVersion != 0 && controlReq.ProtocolVersion != ProtocolVersion {
				_ = writeControl(streamCtx, StreamMessage{
					Type: "error",
					Error: fmt.Sprintf(
						"protocol mismatch: server=%d client=%d",
						ProtocolVersion,
						controlReq.ProtocolVersion,
					),
				})
				continue
			}
			response, err := s.handleWebsocketControlRequest(streamCtx, session, controlReq)
			if err != nil {
				_ = writeControl(streamCtx, StreamMessage{Type: "error", Error: err.Error()})
				logServerf(
					"ws_control_failed session=%s stream=%s type=%s owner=%q err=%v",
					req.SessionID,
					streamID,
					controlReq.Type,
					controlReq.Owner,
					err,
				)
				continue
			}
			if response != nil {
				_ = writeControl(streamCtx, *response)
			}
			if controlReq.Type == "stop" {
				setClose("stop_request", websocket.StatusNormalClosure, "stop requested")
				return
			}
		}
	}()

	if len(snapshot.replay) > 0 {
		if err := writeBinary(streamCtx, snapshot.ready.ReplayNext, snapshot.replay); err != nil {
			setClose("write_replay_failed", websocket.StatusNormalClosure, err.Error())
			return
		}
	}

	for {
		select {
		case <-streamCtx.Done():
			reason, _, _ := getClose()
			if reason == "handler_exit" {
				text := "context done"
				if err := streamCtx.Err(); err != nil {
					text = err.Error()
				}
				setClose("context_done", websocket.StatusNormalClosure, text)
			}
			return
		case _, ok := <-sub.notify:
			if !ok {
				_ = writeControl(streamCtx, StreamMessage{
					Type:      "closed",
					SessionID: req.SessionID,
					StreamID:  streamID,
				})
				setClose("session_closed", websocket.StatusNormalClosure, "subscriber closed")
				return
			}
			data, nextOffset, _ := session.pullBuffer(sub)
			if len(data) == 0 {
				continue
			}
			if err := writeBinary(streamCtx, nextOffset, data); err != nil {
				setClose("write_chunk_failed", websocket.StatusNormalClosure, err.Error())
				return
			}
		}
	}
}

func (s *Server) handleWebsocketControlRequest(
	ctx context.Context,
	session *Session,
	req WebsocketControlRequest,
) (*StreamMessage, error) {
	switch req.Type {
	case "input":
		return nil, session.writeForOwner(ctx, req.Data, req.Owner)
	case "resize":
		err := session.resizeForOwner(req.Cols, req.Rows, req.Owner)
		if err == nil {
			logServerf("ws_control session=%s type=resize owner=%q cols=%d rows=%d", session.id, req.Owner, req.Cols, req.Rows)
		}
		return nil, err
	case "set_owner":
		session.setInputOwner(req.Owner)
		logServerf("ws_control session=%s type=set_owner owner=%q", session.id, req.Owner)
		return nil, nil
	case "stop":
		if err := session.stopForOwner(req.Owner); err != nil {
			return nil, err
		}
		logServerf("ws_control session=%s type=stop owner=%q", session.id, req.Owner)
		s.remove(session.id)
		return nil, nil
	case "snapshot":
		var envelope terminalSnapshotEnvelope
		if err := json.Unmarshal(req.Snapshot, &envelope); err != nil {
			return nil, errors.New("invalid terminal snapshot: " + err.Error())
		}
		if err := session.storeSnapshotForOwner(req.Snapshot, req.Owner); err != nil {
			return nil, err
		}
		logServerf(
			"ws_control session=%s type=snapshot owner=%q request_id=%q next_offset=%d cols=%d rows=%d",
			session.id,
			req.Owner,
			req.RequestID,
			envelope.NextOffset,
			envelope.Cols,
			envelope.Rows,
		)
		return &StreamMessage{
			Type:      "snapshot_ack",
			SessionID: session.id,
			RequestID: req.RequestID,
		}, nil
	case "ping":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported websocket request type %q", req.Type)
	}
}

func (s *Server) writeWebsocketControl(
	ctx context.Context,
	conn *websocket.Conn,
	message StreamMessage,
) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, payload)
}

func (s *Server) writeWebsocketBinary(
	ctx context.Context,
	conn *websocket.Conn,
	nextOffset int64,
	data []byte,
) error {
	if len(data) == 0 {
		return nil
	}
	if nextOffset < 0 {
		nextOffset = 0
	}
	payload := make([]byte, websocketBinaryHeaderSize+len(data))
	binary.BigEndian.PutUint64(payload[:websocketBinaryHeaderSize], uint64(nextOffset))
	copy(payload[websocketBinaryHeaderSize:], data)
	return conn.Write(ctx, websocket.MessageBinary, payload)
}

func newWebsocketToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func subtleTokenMismatch(actual, expected string) bool {
	if actual == "" || expected == "" {
		return true
	}
	if len(actual) != len(expected) {
		return true
	}
	mismatch := byte(0)
	for i := 0; i < len(actual); i += 1 {
		mismatch |= actual[i] ^ expected[i]
	}
	return mismatch != 0
}
