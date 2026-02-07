package main

import (
	"errors"
	"testing"

	"github.com/strantalis/workset/pkg/sessiond"
)

type fakeTerminalStream struct {
	id         string
	closeCalls int
}

func (f *fakeTerminalStream) Next(_ *sessiond.StreamMessage) error {
	return errors.New("not implemented")
}

func (f *fakeTerminalStream) ID() string {
	return f.id
}

func (f *fakeTerminalStream) Close() error {
	f.closeCalls++
	return nil
}

func TestTerminalSessionReleaseStreamClosesAndClearsCurrent(t *testing.T) {
	session := &terminalSession{}
	stream := &fakeTerminalStream{id: "stream-1"}
	session.stream = stream
	session.streamID = "stream-1"
	session.streamCancel = func() {}

	detaching, releasedCurrent := session.releaseStream(stream)

	if detaching {
		t.Fatalf("expected detaching false")
	}
	if !releasedCurrent {
		t.Fatalf("expected releasedCurrent true")
	}
	if stream.closeCalls != 1 {
		t.Fatalf("expected stream close called once, got %d", stream.closeCalls)
	}
	if session.stream != nil {
		t.Fatalf("expected stream cleared")
	}
	if session.streamID != "" {
		t.Fatalf("expected stream id cleared, got %q", session.streamID)
	}
	if session.streamCancel != nil {
		t.Fatalf("expected stream cancel cleared")
	}
}

func TestTerminalSessionReleaseStreamDetachingResetsFlag(t *testing.T) {
	session := &terminalSession{detaching: true}
	stream := &fakeTerminalStream{id: "stream-1"}
	session.stream = stream
	session.streamID = "stream-1"
	session.streamCancel = func() {}

	detaching, releasedCurrent := session.releaseStream(stream)

	if !detaching {
		t.Fatalf("expected detaching true")
	}
	if !releasedCurrent {
		t.Fatalf("expected releasedCurrent true")
	}
	if stream.closeCalls != 1 {
		t.Fatalf("expected stream close called once, got %d", stream.closeCalls)
	}
	if session.detaching {
		t.Fatalf("expected detaching flag reset")
	}
}

func TestTerminalSessionReleaseStreamDoesNotClearDifferentCurrentStream(t *testing.T) {
	session := &terminalSession{}
	oldStream := &fakeTerminalStream{id: "old"}
	currentStream := &fakeTerminalStream{id: "current"}
	session.stream = currentStream
	session.streamID = "current"
	originalCancel := func() {}
	session.streamCancel = originalCancel

	detaching, releasedCurrent := session.releaseStream(oldStream)

	if detaching {
		t.Fatalf("expected detaching false")
	}
	if releasedCurrent {
		t.Fatalf("expected releasedCurrent false")
	}
	if oldStream.closeCalls != 1 {
		t.Fatalf("expected old stream close called once, got %d", oldStream.closeCalls)
	}
	if session.stream != currentStream {
		t.Fatalf("expected current stream unchanged")
	}
	if session.streamID != "current" {
		t.Fatalf("expected current stream id unchanged, got %q", session.streamID)
	}
	if session.streamCancel == nil {
		t.Fatalf("expected current stream cancel unchanged")
	}
}
