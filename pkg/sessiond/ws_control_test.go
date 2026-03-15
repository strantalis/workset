package sessiond

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHandleWebsocketControlRequestSetOwnerAndStop(t *testing.T) {
	server := NewServer(DefaultOptions())
	session := newSession(DefaultOptions(), "ws-control", "/tmp")
	server.sessions[session.id] = session

	if _, err := server.handleWebsocketControlRequest(context.Background(), session, WebsocketControlRequest{
		Type:  "set_owner",
		Owner: "popout",
	}); err != nil {
		t.Fatalf("set owner: %v", err)
	}
	if owner := session.getInputOwner(); owner != "popout" {
		t.Fatalf("expected websocket owner to be updated, got %q", owner)
	}

	if _, err := server.handleWebsocketControlRequest(context.Background(), session, WebsocketControlRequest{
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

	_, err := server.handleWebsocketControlRequest(context.Background(), session, WebsocketControlRequest{
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

	_, err := server.handleWebsocketControlRequest(context.Background(), session, WebsocketControlRequest{
		Type: "unknown",
	})
	if err == nil {
		t.Fatal("expected unsupported control message to fail")
	}
}

func TestHandleWebsocketControlRequestStoresSnapshot(t *testing.T) {
	server := NewServer(DefaultOptions())
	session := newSession(DefaultOptions(), "ws-control", "/tmp")
	session.setInputOwner("main")
	server.sessions[session.id] = session

	response, err := server.handleWebsocketControlRequest(context.Background(), session, WebsocketControlRequest{
		Type:      "snapshot",
		Owner:     "main",
		RequestID: "req-1",
		Snapshot:  json.RawMessage(testSnapshotPayload),
	})
	if err != nil {
		t.Fatalf("snapshot publish: %v", err)
	}
	if response == nil || response.Type != "snapshot_ack" || response.RequestID != "req-1" {
		t.Fatalf("expected snapshot ack response, got %+v", response)
	}
	if session.snapshot.nextOffset != 5 {
		t.Fatalf("expected snapshot offset 5, got %d", session.snapshot.nextOffset)
	}
}
