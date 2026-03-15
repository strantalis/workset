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
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "closed")
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	messageType, payload, err := conn.Read(ctx)
	if err != nil {
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
	sub := session.subscribe(streamID)
	session.outputMu.Unlock()
	defer session.unsubscribe(sub)

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
		return
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
					return
				}
				return
			}
			if messageType != websocket.MessageText {
				_ = writeControl(streamCtx, StreamMessage{
					Type:  "error",
					Error: "control request must be a text frame",
				})
				return
			}

			var controlReq WebsocketControlRequest
			if err := json.Unmarshal(payload, &controlReq); err != nil {
				_ = writeControl(streamCtx, StreamMessage{Type: "error", Error: err.Error()})
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
			if err := s.handleWebsocketControlRequest(streamCtx, session, controlReq); err != nil {
				_ = writeControl(streamCtx, StreamMessage{Type: "error", Error: err.Error()})
				continue
			}
			if controlReq.Type == "stop" {
				return
			}
		}
	}()

	if len(snapshot.replay) > 0 {
		if err := writeBinary(streamCtx, snapshot.ready.ReplayNext, snapshot.replay); err != nil {
			return
		}
	}

	for {
		select {
		case <-streamCtx.Done():
			return
		case event, ok := <-sub.ch:
			if !ok {
				_ = writeControl(streamCtx, StreamMessage{
					Type:      "closed",
					SessionID: req.SessionID,
					StreamID:  streamID,
				})
				return
			}
			if err := writeBinary(streamCtx, event.nextOffset, event.data); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleWebsocketControlRequest(
	ctx context.Context,
	session *Session,
	req WebsocketControlRequest,
) error {
	switch req.Type {
	case "input":
		return session.writeForOwner(ctx, req.Data, req.Owner)
	case "resize":
		return session.resizeForOwner(req.Cols, req.Rows, req.Owner)
	case "set_owner":
		session.setInputOwner(req.Owner)
		return nil
	case "stop":
		if err := session.stopForOwner(req.Owner); err != nil {
			return err
		}
		s.remove(session.id)
		return nil
	case "ping":
		return nil
	default:
		return fmt.Errorf("unsupported websocket request type %q", req.Type)
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
