package sessiond

import (
	"context"
	"testing"
)

func TestHandleWebsocketControlRequestSetOwnerAndStop(t *testing.T) {
	server := NewServer(DefaultOptions())
	session := newSession(DefaultOptions(), "ws-control", "/tmp")
	server.sessions[session.id] = session

	if err := server.handleWebsocketControlRequest(context.Background(), session, WebsocketControlRequest{
		Type:  "set_owner",
		Owner: "popout",
	}); err != nil {
		t.Fatalf("set owner: %v", err)
	}
	if owner := session.getInputOwner(); owner != "popout" {
		t.Fatalf("expected websocket owner to be updated, got %q", owner)
	}

	if err := server.handleWebsocketControlRequest(context.Background(), session, WebsocketControlRequest{
		Type:  "stop",
		Owner: "popout",
	}); err != nil {
		t.Fatalf("stop session: %v", err)
	}
	if server.get(session.id) != nil {
		t.Fatalf("expected stopped session to be removed from server registry")
	}
}

func TestHandleWebsocketControlRequestRejectsStopForNonOwner(t *testing.T) {
	server := NewServer(DefaultOptions())
	session := newSession(DefaultOptions(), "ws-control", "/tmp")
	session.setInputOwner("popout")
	server.sessions[session.id] = session

	err := server.handleWebsocketControlRequest(context.Background(), session, WebsocketControlRequest{
		Type:  "stop",
		Owner: "main",
	})
	if err == nil {
		t.Fatal("expected stop to fail for non-owner")
	}
	if server.get(session.id) == nil {
		t.Fatal("expected session to remain registered after rejected stop")
	}
}

func TestHandleWebsocketControlRequestRejectsUnsupportedMessages(t *testing.T) {
	server := NewServer(DefaultOptions())
	session := newSession(DefaultOptions(), "ws-control", "/tmp")

	err := server.handleWebsocketControlRequest(context.Background(), session, WebsocketControlRequest{
		Type: "unknown",
	})
	if err == nil {
		t.Fatal("expected unsupported control message to fail")
	}
}
