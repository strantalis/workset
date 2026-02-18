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
