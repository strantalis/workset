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
	session.streamCancel = func() {}

	releasedCurrent := session.releaseStream(stream)

	if !releasedCurrent {
		t.Fatalf("expected releasedCurrent true")
	}
	if stream.closeCalls != 1 {
		t.Fatalf("expected stream close called once, got %d", stream.closeCalls)
	}
	if session.stream != nil {
		t.Fatalf("expected stream cleared")
	}
	if session.streamCancel != nil {
		t.Fatalf("expected stream cancel cleared")
	}
}

func TestTerminalSessionReleaseStreamDoesNotClearDifferentCurrentStream(t *testing.T) {
	session := &terminalSession{}
	oldStream := &fakeTerminalStream{id: "old"}
	currentStream := &fakeTerminalStream{id: "current"}
	session.stream = currentStream
	originalCancel := func() {}
	session.streamCancel = originalCancel

	releasedCurrent := session.releaseStream(oldStream)

	if releasedCurrent {
		t.Fatalf("expected releasedCurrent false")
	}
	if oldStream.closeCalls != 1 {
		t.Fatalf("expected old stream close called once, got %d", oldStream.closeCalls)
	}
	if session.stream != currentStream {
		t.Fatalf("expected current stream unchanged")
	}
	if session.streamCancel == nil {
		t.Fatalf("expected current stream cancel unchanged")
	}
}
