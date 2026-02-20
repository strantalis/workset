package sessiond

import (
	"testing"
	"time"
)

func TestSessionBroadcastClonesReusedSourceBuffer(t *testing.T) {
	session := &Session{
		subscribers: make(map[*subscriber]struct{}),
		streams:     make(map[string]*subscriber),
	}
	sub := newSubscriber("clone-test-stream")
	t.Cleanup(func() {
		session.unsubscribe(sub)
	})
	session.subscribers[sub] = struct{}{}
	session.streams[sub.streamID] = sub

	shared := make([]byte, 32)
	copy(shared, []byte("first"))
	session.broadcast(shared[:5])

	// Simulate the PTY read loop reusing the same backing array for the next read.
	copy(shared, []byte("second"))
	session.broadcast(shared[:6])

	select {
	case event := <-sub.ch:
		if string(event) != "first" {
			t.Fatalf("expected first payload, got %q", event)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for first event")
	}

	select {
	case event := <-sub.ch:
		if string(event) != "second" {
			t.Fatalf("expected second payload, got %q", event)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for second event")
	}
}

func TestUnsubscribeClearsMouseModesWhenLastSubscriberLeaves(t *testing.T) {
	session := &Session{
		subscribers: make(map[*subscriber]struct{}),
		streams:     make(map[string]*subscriber),
		modeState: terminalModeState{
			altScreen: true,
			mouse1002: true,
			mouse1006: true,
		},
	}
	sub := newSubscriber("mode-clear-stream")
	session.subscribers[sub] = struct{}{}
	session.streams[sub.streamID] = sub

	session.unsubscribe(sub)

	session.outputMu.Lock()
	modeState := session.modeState
	session.outputMu.Unlock()
	if !modeState.altScreen {
		t.Fatalf("expected alt-screen mode to remain enabled")
	}
	if modeState.mouse1002 {
		t.Fatalf("expected mouse1002 to be cleared")
	}
	if modeState.mouse1006 {
		t.Fatalf("expected mouse1006 to be cleared")
	}
}
