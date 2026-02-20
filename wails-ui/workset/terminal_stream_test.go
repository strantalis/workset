package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/strantalis/workset/pkg/sessiond"
)

type stubTerminalStream struct {
	id         string
	nextErr    error
	closeCalls int32
}

func (s *stubTerminalStream) Next(_ *sessiond.StreamMessage) error {
	if s.nextErr != nil {
		return s.nextErr
	}
	return errors.New("stream closed")
}

func (s *stubTerminalStream) ID() string {
	return s.id
}

func (s *stubTerminalStream) Close() error {
	atomic.AddInt32(&s.closeCalls, 1)
	return nil
}

type blockingTerminalStream struct {
	id         string
	closed     chan struct{}
	closeOnce  sync.Once
	closeCalls int32
}

func newBlockingTerminalStream(id string) *blockingTerminalStream {
	return &blockingTerminalStream{
		id:     id,
		closed: make(chan struct{}),
	}
}

func (s *blockingTerminalStream) Next(_ *sessiond.StreamMessage) error {
	<-s.closed
	return errors.New("stream closed")
}

func (s *blockingTerminalStream) ID() string {
	return s.id
}

func (s *blockingTerminalStream) Close() error {
	atomic.AddInt32(&s.closeCalls, 1)
	s.closeOnce.Do(func() {
		close(s.closed)
	})
	return nil
}

func waitForCalls(t *testing.T, counter *atomic.Int32, want int32, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if counter.Load() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected %d attach calls, got %d", want, counter.Load())
}

func TestStreamTerminalPreventsConcurrentDuplicateAttach(t *testing.T) {
	originalAttach := attachSessionStream
	t.Cleanup(func() {
		attachSessionStream = originalAttach
	})

	app := NewApp()
	session := newTerminalSession("ws", "term", "/tmp")
	session.client = &sessiond.Client{}
	session.markReady(nil)

	var attachCalls atomic.Int32
	attachRelease := make(chan struct{})
	attachSessionStream = func(
		_ *sessiond.Client,
		_ context.Context,
		_ string,
		_ int64,
		_ bool,
		_ string,
	) (terminalStream, sessiond.StreamMessage, error) {
		attachCalls.Add(1)
		<-attachRelease
		return &stubTerminalStream{id: "stream-1", nextErr: errors.New("done")}, sessiond.StreamMessage{Type: "ready"}, nil
	}

	done1 := make(chan struct{})
	go func() {
		defer close(done1)
		app.streamTerminal(session)
	}()

	waitForCalls(t, &attachCalls, 1, time.Second)

	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		app.streamTerminal(session)
	}()

	select {
	case <-done2:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("second streamTerminal call should return immediately")
	}

	if got := attachCalls.Load(); got != 1 {
		t.Fatalf("expected single attach call while first stream is attaching, got %d", got)
	}

	close(attachRelease)

	select {
	case <-done1:
	case <-time.After(2 * time.Second):
		t.Fatal("first streamTerminal call did not exit")
	}
}

func TestStreamTerminalStaleCloseDoesNotClearClient(t *testing.T) {
	originalAttach := attachSessionStream
	t.Cleanup(func() {
		attachSessionStream = originalAttach
	})

	app := NewApp()
	session := newTerminalSession("ws", "term", "/tmp")
	session.client = &sessiond.Client{}
	session.markReady(nil)

	staleStream := newBlockingTerminalStream("stream-stale")
	attachSessionStream = func(
		_ *sessiond.Client,
		_ context.Context,
		_ string,
		_ int64,
		_ bool,
		_ string,
	) (terminalStream, sessiond.StreamMessage, error) {
		return staleStream, sessiond.StreamMessage{Type: "ready"}, nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.streamTerminal(session)
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		session.mu.Lock()
		attached := session.stream == staleStream
		session.mu.Unlock()
		if attached {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	session.mu.Lock()
	session.stream = &stubTerminalStream{id: "stream-replacement"}
	session.mu.Unlock()

	_ = staleStream.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected stale stream goroutine to exit")
	}

	session.mu.Lock()
	client := session.client
	session.mu.Unlock()
	if client == nil {
		t.Fatal("expected stale stream close to preserve active session client")
	}
}

func TestStreamTerminalAttachErrorClearsClient(t *testing.T) {
	originalAttach := attachSessionStream
	t.Cleanup(func() {
		attachSessionStream = originalAttach
	})

	app := NewApp()
	session := newTerminalSession("ws", "term", "/tmp")
	session.client = &sessiond.Client{}
	session.markReady(nil)

	attachSessionStream = func(
		_ *sessiond.Client,
		_ context.Context,
		_ string,
		_ int64,
		_ bool,
		_ string,
	) (terminalStream, sessiond.StreamMessage, error) {
		return nil, sessiond.StreamMessage{}, errors.New("attach failed")
	}

	app.streamTerminal(session)

	session.mu.Lock()
	client := session.client
	stream := session.stream
	streamCancel := session.streamCancel
	session.mu.Unlock()
	if client != nil {
		t.Fatal("expected attach error to clear cached session client")
	}
	if stream != nil {
		t.Fatal("expected stream to remain nil after attach error")
	}
	if streamCancel != nil {
		t.Fatal("expected stream cancel to be released after attach error")
	}
}

func TestStreamTerminalReadErrorKeepsSessionAlive(t *testing.T) {
	originalAttach := attachSessionStream
	t.Cleanup(func() {
		attachSessionStream = originalAttach
	})

	app := NewApp()
	session := newTerminalSession("ws", "term", "/tmp")
	session.client = &sessiond.Client{}
	session.markReady(nil)

	attachSessionStream = func(
		_ *sessiond.Client,
		_ context.Context,
		_ string,
		_ int64,
		_ bool,
		_ string,
	) (terminalStream, sessiond.StreamMessage, error) {
		return &stubTerminalStream{
			id:      "stream-live",
			nextErr: errors.New("temporary decode failure"),
		}, sessiond.StreamMessage{Type: "ready"}, nil
	}

	app.streamTerminal(session)

	session.mu.Lock()
	client := session.client
	closed := session.closed
	stream := session.stream
	session.mu.Unlock()
	if client == nil {
		t.Fatal("expected transient stream read error to preserve session client")
	}
	if closed {
		t.Fatal("expected transient stream read error to keep session open")
	}
	if stream != nil {
		t.Fatal("expected stream reference to be released after read error")
	}
}

func TestStreamTerminalSessionGoneErrorClearsClient(t *testing.T) {
	originalAttach := attachSessionStream
	t.Cleanup(func() {
		attachSessionStream = originalAttach
	})

	app := NewApp()
	session := newTerminalSession("ws", "term", "/tmp")
	session.client = &sessiond.Client{}
	session.markReady(nil)

	attachSessionStream = func(
		_ *sessiond.Client,
		_ context.Context,
		_ string,
		_ int64,
		_ bool,
		_ string,
	) (terminalStream, sessiond.StreamMessage, error) {
		return &stubTerminalStream{id: "stream-gone"}, sessiond.StreamMessage{
			Type:  "error",
			Error: "session not found",
		}, nil
	}

	app.streamTerminal(session)

	session.mu.Lock()
	client := session.client
	closed := session.closed
	session.mu.Unlock()
	if client != nil {
		t.Fatal("expected session-gone error to clear session client")
	}
	if closed {
		t.Fatal("expected session-gone error to avoid closing local session record")
	}
}
