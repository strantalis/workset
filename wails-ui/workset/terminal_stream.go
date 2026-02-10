package main

import (
	"context"
	"fmt"
	"time"

	"github.com/strantalis/workset/pkg/kitty"
	"github.com/strantalis/workset/pkg/sessiond"
)

const bootstrapReplayChunkSize = 64 * 1024

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

func (a *App) restartTerminalStream(session *terminalSession) {
	if session == nil {
		return
	}
	var cancel context.CancelFunc
	var stream terminalStream
	session.mu.Lock()
	if session.streamCancel != nil {
		session.detaching = true
		cancel = session.streamCancel
		stream = session.stream
	}
	session.mu.Unlock()
	if cancel != nil {
		cancel()
		if stream != nil {
			_ = stream.Close()
		}
		deadline := time.Now().Add(2 * time.Second)
		for {
			session.mu.Lock()
			done := session.streamCancel == nil
			session.mu.Unlock()
			if done || time.Now().After(deadline) {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	go a.streamTerminal(session)
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
	defer cancel()
	session.mu.Lock()
	session.streamCancel = cancel
	session.mu.Unlock()
	stream, first, err := attachSessionStream(client, ctx, session.id, 0, false, "")
	if err != nil {
		session.mu.Lock()
		session.streamCancel = nil
		session.mu.Unlock()
		session.mu.Lock()
		session.client = nil
		session.mu.Unlock()
		a.emitTerminalLifecycle("error", session, err.Error())
		return
	}
	session.mu.Lock()
	session.stream = stream
	session.streamID = stream.ID()
	session.mu.Unlock()
	defer func() {
		detaching, releasedCurrent := session.releaseStream(stream)
		if detaching || !releasedCurrent {
			return
		}
		_ = session.CloseWithReason("closed")
		a.emitTerminalLifecycle("closed", session, "")
	}()
	if first.Type == "error" && first.Error != "" {
		session.mu.Lock()
		session.client = nil
		session.mu.Unlock()
		a.emitTerminalLifecycle("error", session, first.Error)
		return
	}
	applyStreamModes := func(msg sessiond.StreamMessage, force bool) {
		mouseMask := msg.MouseMask
		if mouseMask == 0 && msg.Mouse {
			mouseMask = 1
		}
		mouseEncoding := msg.MouseEncoding
		if mouseEncoding == "" && msg.MouseSGR {
			mouseEncoding = "sgr"
		}
		session.mu.Lock()
		prevAlt := session.altScreen
		prevMask := session.mouseMask
		prevSGR := session.mouseSGR
		prevEncoding := session.mouseEncoding()
		session.altScreen = msg.AltScreen
		session.mouseMask = mouseMask
		session.mouseSGR = msg.MouseSGR
		session.mouseUTF8 = mouseEncoding == "utf8"
		session.mouseURXVT = mouseEncoding == "urxvt"
		altScreen := session.altScreen
		mouseEnabled := session.mouseEnabled()
		mouseSGR := session.mouseSGR
		currentEncoding := session.mouseEncoding()
		session.mu.Unlock()
		changed := prevAlt != altScreen || prevMask != mouseMask || prevSGR != mouseSGR || prevEncoding != currentEncoding
		if changed || force {
			a.emitTerminalModes(session, altScreen, mouseEnabled, mouseSGR, currentEncoding)
			_ = a.persistTerminalState()
		}
	}
	handleMessage := func(msg sessiond.StreamMessage) bool {
		switch msg.Type {
		case "bootstrap":
			applyStreamModes(msg, true)
			emitRuntimeEvent(a.ctx, EventTerminalBootstrap, TerminalBootstrapPayload{
				WorkspaceID:      session.workspaceID,
				TerminalID:       session.terminalID,
				SnapshotSource:   msg.SnapshotSource,
				BacklogSource:    msg.BacklogSource,
				BacklogTruncated: msg.BacklogTruncated,
				NextOffset:       msg.NextOffset,
				Source:           "sessiond",
				AltScreen:        msg.AltScreen,
				Mouse:            msg.Mouse,
				MouseSGR:         msg.MouseSGR,
				MouseEncoding:    msg.MouseEncoding,
				SafeToReplay:     msg.SafeToReplay,
				InitialCredit:    msg.InitialCredit,
			})
			if msg.Kitty != nil {
				emitRuntimeEvent(a.ctx, EventTerminalKitty, TerminalKittyPayload{
					WorkspaceID: session.workspaceID,
					TerminalID:  session.terminalID,
					Event:       *msg.Kitty,
				})
			}
			if msg.BacklogTruncated {
				a.emitTerminalLifecycle("started", session, "Backlog truncated; skipping replay.")
			}
		case "bootstrap_done":
			emitRuntimeEvent(a.ctx, EventTerminalBootstrapDone, TerminalBootstrapDonePayload{
				WorkspaceID: session.workspaceID,
				TerminalID:  session.terminalID,
			})
		case "modes":
			applyStreamModes(msg, false)
		case "kitty":
			if msg.Kitty != nil {
				emitRuntimeEvent(a.ctx, EventTerminalKitty, TerminalKittyPayload{
					WorkspaceID: session.workspaceID,
					TerminalID:  session.terminalID,
					Event:       *msg.Kitty,
				})
			}
		case "data":
			if msg.Data == "" {
				return true
			}
			session.bumpActivity()
			bytes := msg.Len
			if bytes <= 0 {
				bytes = len(msg.Data)
			}
			emitRuntimeEvent(a.ctx, EventTerminalData, TerminalPayload{
				WorkspaceID: session.workspaceID,
				TerminalID:  session.terminalID,
				Data:        msg.Data,
				Bytes:       bytes,
			})
		case "error":
			if msg.Error != "" {
				a.emitTerminalLifecycle("error", session, msg.Error)
			}
			return false
		case "closed":
			return false
		}
		return true
	}
	if !handleMessage(first) {
		_ = session.CloseWithReason("closed")
		a.emitTerminalLifecycle("closed", session, "")
		return
	}
	for {
		var msg sessiond.StreamMessage
		if err := stream.Next(&msg); err != nil {
			session.mu.Lock()
			detaching := session.detaching
			if !detaching {
				session.client = nil
			}
			session.mu.Unlock()
			break
		}
		if !handleMessage(msg) {
			break
		}
	}
	return
}

func (a *App) emitBootstrapReplay(session *terminalSession) error {
	if session == nil {
		return nil
	}
	session.mu.Lock()
	client := session.client
	session.mu.Unlock()
	if client == nil {
		return fmt.Errorf("sessiond unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	bootstrap, err := client.Bootstrap(ctx, session.id)
	cancel()
	if err != nil {
		return err
	}
	emitRuntimeEvent(a.ctx, EventTerminalBootstrap, TerminalBootstrapPayload{
		WorkspaceID:      session.workspaceID,
		TerminalID:       session.terminalID,
		SnapshotSource:   bootstrap.SnapshotSource,
		BacklogSource:    bootstrap.BacklogSource,
		BacklogTruncated: bootstrap.BacklogTruncated,
		NextOffset:       bootstrap.NextOffset,
		Source:           "sessiond",
		AltScreen:        bootstrap.AltScreen,
		Mouse:            bootstrap.Mouse,
		MouseSGR:         bootstrap.MouseSGR,
		MouseEncoding:    bootstrap.MouseEncoding,
		SafeToReplay:     bootstrap.SafeToReplay,
		InitialCredit:    bootstrap.InitialCredit,
	})
	if bootstrap.Kitty != nil {
		emitRuntimeEvent(a.ctx, EventTerminalKitty, TerminalKittyPayload{
			WorkspaceID: session.workspaceID,
			TerminalID:  session.terminalID,
			Event: kitty.Event{
				Kind:     "snapshot",
				Snapshot: bootstrap.Kitty,
			},
		})
	}
	if !bootstrap.SafeToReplay {
		emitRuntimeEvent(a.ctx, EventTerminalBootstrapDone, TerminalBootstrapDonePayload{
			WorkspaceID: session.workspaceID,
			TerminalID:  session.terminalID,
		})
		return nil
	}
	data := bootstrap.Snapshot
	if data == "" {
		data = bootstrap.Backlog
	}
	if data != "" {
		buf := []byte(data)
		for len(buf) > 0 {
			n := bootstrapReplayChunkSize
			if len(buf) < n {
				n = len(buf)
			}
			chunk := buf[:n]
			emitRuntimeEvent(a.ctx, EventTerminalData, TerminalPayload{
				WorkspaceID: session.workspaceID,
				TerminalID:  session.terminalID,
				Data:        string(chunk),
				Bytes:       len(chunk),
			})
			buf = buf[n:]
		}
	}
	emitRuntimeEvent(a.ctx, EventTerminalBootstrapDone, TerminalBootstrapDonePayload{
		WorkspaceID: session.workspaceID,
		TerminalID:  session.terminalID,
	})
	return nil
}

func (a *App) emitTerminalLifecycle(status string, session *terminalSession, message string) {
	if session == nil {
		return
	}
	emitRuntimeEvent(a.ctx, EventTerminalLifecycle, TerminalLifecyclePayload{
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
	emitRuntimeEvent(a.ctx, EventTerminalModes, TerminalModesPayload{
		WorkspaceID:   session.workspaceID,
		TerminalID:    session.terminalID,
		AltScreen:     altScreen,
		Mouse:         mouse,
		MouseSGR:      mouseSGR,
		MouseEncoding: mouseEncoding,
	})
}
