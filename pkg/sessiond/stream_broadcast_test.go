package sessiond

import (
	"testing"
	"time"
)

func TestNotifyAndPullDeliversData(t *testing.T) {
	session := &Session{
		subscribers: make(map[*subscriber]struct{}),
		streams:     make(map[string]*subscriber),
		buffer:      newTerminalBuffer(64 * 1024),
	}
	sub := session.subscribe("pull-test-stream", 0)
	t.Cleanup(func() {
		session.unsubscribe(sub)
	})

	// Write data to the ring buffer and notify.
	session.buffer.Append([]byte("hello"))
	session.notifySubscribers()

	select {
	case <-sub.notify:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for notification")
	}

	data, nextOffset, _ := session.pullBuffer(sub)
	if string(data) != "hello" {
		t.Fatalf("expected %q, got %q", "hello", data)
	}
	if nextOffset != 5 {
		t.Fatalf("expected offset 5, got %d", nextOffset)
	}
}

func TestNotifyCoalescesMultipleWrites(t *testing.T) {
	session := &Session{
		subscribers: make(map[*subscriber]struct{}),
		streams:     make(map[string]*subscriber),
		buffer:      newTerminalBuffer(64 * 1024),
	}
	sub := session.subscribe("coalesce-test", 0)
	t.Cleanup(func() {
		session.unsubscribe(sub)
	})

	// Multiple writes before the subscriber drains.
	session.buffer.Append([]byte("first"))
	session.notifySubscribers()
	session.buffer.Append([]byte("second"))
	session.notifySubscribers()

	select {
	case <-sub.notify:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for notification")
	}

	// A single pull should return both writes coalesced.
	data, nextOffset, _ := session.pullBuffer(sub)
	if string(data) != "firstsecond" {
		t.Fatalf("expected coalesced data %q, got %q", "firstsecond", data)
	}
	if nextOffset != 11 {
		t.Fatalf("expected offset 11, got %d", nextOffset)
	}
}

func TestSlowSubscriberDoesNotBlockBroadcast(t *testing.T) {
	session := &Session{
		subscribers: make(map[*subscriber]struct{}),
		streams:     make(map[string]*subscriber),
		buffer:      newTerminalBuffer(64 * 1024),
	}
	slow := session.subscribe("slow-stream", 0)
	fast := session.subscribe("fast-stream", 0)
	t.Cleanup(func() {
		session.unsubscribe(slow)
		session.unsubscribe(fast)
	})

	// Fill the notify channel on slow (capacity 1) so it can't accept more.
	slow.notify <- struct{}{}

	// This should NOT block — slow already has a pending notification.
	session.buffer.Append([]byte("data"))
	done := make(chan struct{})
	go func() {
		session.notifySubscribers()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("notifySubscribers blocked on slow subscriber")
	}

	// Fast subscriber should still get notified.
	select {
	case <-fast.notify:
	case <-time.After(time.Second):
		t.Fatalf("fast subscriber not notified")
	}

	// Slow subscriber should still be subscribed (not evicted).
	session.subscribersMu.Lock()
	_, stillSubscribed := session.subscribers[slow]
	session.subscribersMu.Unlock()
	if !stillSubscribed {
		t.Fatalf("slow subscriber should not be evicted")
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
	sub := newSubscriber("mode-clear-stream", 0)
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

func TestGetStreamStateReturnsSortedStreamIDs(t *testing.T) {
	session := &Session{
		subscribers: make(map[*subscriber]struct{}),
		streams:     make(map[string]*subscriber),
	}
	subB := newSubscriber("stream-b", 0)
	subA := newSubscriber("stream-a", 0)
	session.subscribers[subB] = struct{}{}
	session.subscribers[subA] = struct{}{}
	session.streams[subB.streamID] = subB
	session.streams[subA.streamID] = subA

	state := session.getStreamState()
	if state.Count != 2 {
		t.Fatalf("expected 2 subscribers, got %d", state.Count)
	}
	if len(state.StreamIDs) != 2 {
		t.Fatalf("expected 2 stream ids, got %d", len(state.StreamIDs))
	}
	if state.StreamIDs[0] != "stream-a" || state.StreamIDs[1] != "stream-b" {
		t.Fatalf("expected sorted stream ids, got %v", state.StreamIDs)
	}
}

func TestPullBufferStartsFromSubscriberOffset(t *testing.T) {
	session := &Session{
		subscribers: make(map[*subscriber]struct{}),
		streams:     make(map[string]*subscriber),
		buffer:      newTerminalBuffer(64 * 1024),
	}
	// Pre-populate buffer.
	session.buffer.Append([]byte("old-data"))
	// Subscribe starting after the existing data.
	sub := session.subscribe("offset-test", 8)
	t.Cleanup(func() {
		session.unsubscribe(sub)
	})

	// Pull should return nothing — subscriber is caught up.
	data, _, _ := session.pullBuffer(sub)
	if len(data) != 0 {
		t.Fatalf("expected no data on initial pull, got %q", data)
	}

	// Now write new data.
	session.buffer.Append([]byte("new-data"))
	data, nextOffset, _ := session.pullBuffer(sub)
	if string(data) != "new-data" {
		t.Fatalf("expected %q, got %q", "new-data", data)
	}
	if nextOffset != 16 {
		t.Fatalf("expected offset 16, got %d", nextOffset)
	}
}
