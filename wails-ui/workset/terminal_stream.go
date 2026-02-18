package main

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/strantalis/workset/pkg/sessiond"
)

var attachSessionStream = func(
	client *sessiond.Client,
	ctx context.Context,
	sessionID string,
	since int64,
	withBuffer bool,
	streamID string,
) (terminalStream, sessiond.StreamMessage, error) {
	return client.Attach(ctx, sessionID, since, withBuffer, streamID)
}

var terminalOutputSeq atomic.Uint64

func (a *App) streamTerminal(session *terminalSession) {
	ctx, cancel := context.WithCancel(context.Background())
	streamOwner := a.workspaceTerminalOwner(session.workspaceID)
	session.mu.Lock()
	if session.streamCancel != nil || session.stream != nil {
		session.mu.Unlock()
		cancel()
		return
	}
	// Claim the stream slot before attaching so concurrent calls can't
	// establish duplicate sessiond streams for the same terminal.
	session.streamCancel = cancel
	session.streamOwner = streamOwner
	client := session.client
	session.mu.Unlock()
	if client == nil {
		session.mu.Lock()
		session.streamCancel = nil
		session.streamOwner = ""
		session.mu.Unlock()
		cancel()
		return
	}
	defer cancel()
	stream, first, err := attachSessionStream(client, ctx, session.id, 0, false, "")
	if err != nil {
		session.mu.Lock()
		session.streamCancel = nil
		session.streamOwner = ""
		session.client = nil
		session.mu.Unlock()
		return
	}
	session.mu.Lock()
	session.stream = stream
	session.streamOwner = streamOwner
	session.mu.Unlock()
	defer func() {
		if !session.releaseStream(stream) {
			return
		}
		_ = session.CloseWithReason("closed")
	}()
	if first.Type == "error" && first.Error != "" {
		session.mu.Lock()
		session.client = nil
		session.mu.Unlock()
		return
	}
	handleMessage := func(msg sessiond.StreamMessage) bool {
		switch msg.Type {
		case "ready":
			return true
		case "data":
			if msg.DataB64 == "" || msg.Len <= 0 {
				return true
			}
			outputSeq := terminalOutputSeq.Add(1)
			logTerminalDebug(TerminalDebugPayload{
				WorkspaceID: session.workspaceID,
				TerminalID:  session.terminalID,
				Event:       "app_output_chunk",
				Details: fmt.Sprintf(
					`{"seq":%d,"streamId":%q,"declaredBytes":%d,"summary":%q}`,
					outputSeq,
					msg.StreamID,
					msg.Len,
					summarizeTerminalBase64(msg.DataB64, 48),
				),
			})
			session.bumpActivity()
			emitRuntimeEvent(a.ctx, EventTerminalData, TerminalPayload{
				WorkspaceID: session.workspaceID,
				TerminalID:  session.terminalID,
				DataB64:     msg.DataB64,
				Bytes:       msg.Len,
				Seq:         int64(outputSeq),
			})
		case "error":
			return false
		case "closed":
			return false
		}
		return true
	}
	if !handleMessage(first) {
		_ = session.CloseWithReason("closed")
		return
	}
	for {
		var msg sessiond.StreamMessage
		if err := stream.Next(&msg); err != nil {
			session.mu.Lock()
			// Ignore terminal stream shutdown from stale streams during ownership
			// handoff; only the active stream can invalidate the session client.
			if session.stream == stream {
				session.client = nil
			}
			session.mu.Unlock()
			break
		}
		if !handleMessage(msg) {
			break
		}
	}
}
