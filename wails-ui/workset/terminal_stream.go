package main

import (
	"context"
	"fmt"
	"strings"
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

var terminalSessionGoneMarkers = []string{
	"session not found",
	"session not running",
}

func isTerminalSessionGoneError(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	for _, marker := range terminalSessionGoneMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

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
		_ = session.releaseStream(stream)
	}()
	streamEndedAsSessionGone := false
	handleMessage := func(msg sessiond.StreamMessage) (bool, bool) {
		switch msg.Type {
		case "ready":
			return true, false
		case "data":
			if msg.DataB64 == "" || msg.Len <= 0 {
				return true, false
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
				WindowName:  streamOwner,
				DataB64:     msg.DataB64,
				Bytes:       msg.Len,
				Seq:         int64(outputSeq),
			})
		case "error":
			return false, isTerminalSessionGoneError(msg.Error)
		case "closed":
			return false, true
		}
		return true, false
	}
	if continueStream, sessionGone := handleMessage(first); !continueStream {
		streamEndedAsSessionGone = sessionGone
		if streamEndedAsSessionGone {
			session.mu.Lock()
			if session.stream == stream {
				session.client = nil
			}
			session.mu.Unlock()
		}
		return
	}
	for {
		var msg sessiond.StreamMessage
		if err := stream.Next(&msg); err != nil {
			break
		}
		continueStream, sessionGone := handleMessage(msg)
		if !continueStream {
			streamEndedAsSessionGone = sessionGone
			break
		}
	}
	if !streamEndedAsSessionGone {
		return
	}
	session.mu.Lock()
	if session.stream == stream {
		session.client = nil
	}
	session.mu.Unlock()
}
