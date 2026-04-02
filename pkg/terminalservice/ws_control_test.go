package terminalservice

import (
	"context"
	"testing"
)

func TestHandleWebsocketControlRequestStopRemovesSession(t *testing.T) {
	server := NewServer(DefaultOptions())
	session := newSession(DefaultOptions(), "ws-control", "/tmp")
	server.sessions[session.id] = session

	if _, err := server.handleWebsocketControlRequest(context.Background(), session, WebsocketControlRequest{
		Type: "stop",
	}); err != nil {
		t.Fatalf("stop session: %v", err)
	}
	if server.get(session.id) != nil {
		t.Fatalf("expected stopped session to be removed from server registry")
	}
}

func TestHandleWebsocketControlRequestRejectsUnsupportedMessages(t *testing.T) {
	server := NewServer(DefaultOptions())
	session := newSession(DefaultOptions(), "ws-control", "/tmp")

	_, err := server.handleWebsocketControlRequest(context.Background(), session, WebsocketControlRequest{
		Type: "unknown",
	})
	if err == nil {
		t.Fatal("expected unsupported control message to fail")
	}
}
