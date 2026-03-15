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
	session.broadcast(shared[:5], 5)

	// Simulate the PTY read loop reusing the same backing array for the next read.
	copy(shared, []byte("second"))
	session.broadcast(shared[:6], 11)

	select {
	case event := <-sub.ch:
		if string(event.data) != "first" {
			t.Fatalf("expected first payload, got %q", event.data)
		}
		if event.nextOffset != 5 {
			t.Fatalf("expected first next offset 5, got %d", event.nextOffset)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for first event")
	}

	select {
	case event := <-sub.ch:
		if string(event.data) != "second" {
			t.Fatalf("expected second payload, got %q", event.data)
		}
		if event.nextOffset != 11 {
			t.Fatalf("expected second next offset 11, got %d", event.nextOffset)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for second event")
	}
}

func TestUnsubscribePreservesMouseModesWhenLastSubscriberLeaves(t *testing.T) {
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
	if !modeState.mouse1002 {
		t.Fatalf("expected mouse1002 to remain enabled")
	}
	if !modeState.mouse1006 {
		t.Fatalf("expected mouse1006 to remain enabled")
	}
}

func TestSessionBroadcastDropsStalledSubscribers(t *testing.T) {
	session := &Session{
		subscribers: make(map[*subscriber]struct{}),
		streams:     make(map[string]*subscriber),
	}
	healthy := newSubscriber("healthy-stream")
	stalled := newSubscriber("stalled-stream")
	t.Cleanup(func() {
		session.unsubscribe(healthy)
		session.unsubscribe(stalled)
	})
	session.subscribers[healthy] = struct{}{}
	session.streams[healthy.streamID] = healthy
	session.subscribers[stalled] = struct{}{}
	session.streams[stalled.streamID] = stalled

	for i := 0; i < cap(stalled.ch); i += 1 {
		stalled.ch <- streamEvent{
			data:       []byte("busy"),
			nextOffset: int64(i + 1),
		}
	}

	session.broadcast([]byte("fresh"), 99)

	select {
	case event := <-healthy.ch:
		if string(event.data) != "fresh" {
			t.Fatalf("expected healthy subscriber payload %q, got %q", "fresh", event.data)
		}
		if event.nextOffset != 99 {
			t.Fatalf("expected healthy subscriber offset 99, got %d", event.nextOffset)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for healthy subscriber event")
	}

	session.subscribersMu.Lock()
	_, stillSubscribed := session.subscribers[stalled]
	session.subscribersMu.Unlock()
	if stillSubscribed {
		t.Fatalf("expected stalled subscriber to be removed")
	}
	select {
	case <-stalled.done:
	default:
		t.Fatalf("expected stalled subscriber to be closed")
	}
}
