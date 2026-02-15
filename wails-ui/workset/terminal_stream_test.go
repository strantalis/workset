package main

import (
	"context"
	"errors"
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
