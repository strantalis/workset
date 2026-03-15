package sessiond

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"os/exec"
	"testing"
	"time"
)

const testSnapshotPayload = `{"version":1,"nextOffset":5,"cols":80,"rows":24,"activeBuffer":"normal","normalViewportY":0,"cursor":{"x":0,"y":0,"visible":true},"modes":{"dec":[],"ansi":[]},"normalTail":["hello"]}`

func TestStoreSnapshotForOwnerRejectsNonOwnerAndIgnoresStaleOffset(t *testing.T) {
	session := newSession(DefaultOptions(), "snapshot-owner", "/tmp")
	session.setInputOwner("main")

	if err := session.storeSnapshotForOwner(json.RawMessage(testSnapshotPayload), "main"); err != nil {
		t.Fatalf("store snapshot: %v", err)
	}
	if session.snapshot.nextOffset != 5 {
		t.Fatalf("expected next offset 5, got %d", session.snapshot.nextOffset)
	}

	err := session.storeSnapshotForOwner(
		json.RawMessage(`{"version":1,"nextOffset":6,"cols":80,"rows":24,"activeBuffer":"normal","normalViewportY":0,"cursor":{"x":0,"y":0,"visible":true},"modes":{"dec":[],"ansi":[]},"normalTail":["new"]}`),
		"popout",
	)
	if err == nil {
		t.Fatal("expected snapshot publish to fail for non-owner")
	}
	if session.snapshot.nextOffset != 5 {
		t.Fatalf("expected snapshot offset to remain 5 after rejection, got %d", session.snapshot.nextOffset)
	}

	if err := session.storeSnapshotForOwner(
		json.RawMessage(`{"version":1,"nextOffset":4,"cols":80,"rows":24,"activeBuffer":"normal","normalViewportY":0,"cursor":{"x":0,"y":0,"visible":true},"modes":{"dec":[],"ansi":[]},"normalTail":["stale"]}`),
		"main",
	); err != nil {
		t.Fatalf("store stale snapshot: %v", err)
	}
	if session.snapshot.nextOffset != 5 {
		t.Fatalf("expected stale snapshot to be ignored, got %d", session.snapshot.nextOffset)
	}
}

func TestAttachSendsSnapshotBeforeReplayWhenDimensionsMatch(t *testing.T) {
	opts := DefaultOptions()
	session := newSession(opts, "snapshot-attach", "/tmp")
	session.cmd = &exec.Cmd{}
	session.handleProtocolOutput(context.Background(), []byte("hello world"))
	if err := session.storeSnapshotForOwner(json.RawMessage(testSnapshotPayload), "main"); err != nil {
		t.Fatalf("store snapshot: %v", err)
	}
	server := &Server{
		opts:     opts,
		sessions: map[string]*Session{"snapshot-attach": session},
	}

	clientConn, serverConn := net.Pipe()
	defer func() {
		_ = clientConn.Close()
	}()

	attachLine, err := json.Marshal(AttachRequest{
		ProtocolVersion: ProtocolVersion,
		Type:            "attach",
		SessionID:       "snapshot-attach",
		StreamID:        "snapshot-stream",
		WithBuffer:      true,
		Cols:            80,
		Rows:            24,
	})
	if err != nil {
		t.Fatalf("marshal attach: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = serverConn.Close() }()
		server.handleAttach(serverConn, attachLine)
	}()

	dec := json.NewDecoder(clientConn)
	var readyMsg StreamMessage
	if err := dec.Decode(&readyMsg); err != nil {
		t.Fatalf("attach ready response: %v", err)
	}
	ready := requireAttachReady(t, readyMsg)
	if ready.ReplayStart != 5 {
		t.Fatalf("expected replay to resume from snapshot offset 5, got %+v", ready)
	}

	var snapshotMsg StreamMessage
	if err := dec.Decode(&snapshotMsg); err != nil {
		t.Fatalf("attach snapshot response: %v", err)
	}
	if snapshotMsg.Type != "snapshot" {
		t.Fatalf("expected snapshot message, got %+v", snapshotMsg)
	}
	if string(snapshotMsg.Snapshot) != testSnapshotPayload {
		t.Fatalf("unexpected snapshot payload %q", string(snapshotMsg.Snapshot))
	}

	var replayMsg StreamMessage
	if err := dec.Decode(&replayMsg); err != nil {
		t.Fatalf("attach replay response: %v", err)
	}
	if replayMsg.Type != "data" {
		t.Fatalf("expected replay data after snapshot, got %+v", replayMsg)
	}
	payload, err := base64.StdEncoding.DecodeString(replayMsg.DataB64)
	if err != nil {
		t.Fatalf("decode replay payload: %v", err)
	}
	if string(payload) != " world" {
		t.Fatalf("expected replay payload after snapshot offset, got %q", string(payload))
	}

	_ = clientConn.Close()
	session.closeSubscribers()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("attach handler did not exit")
	}
}

func TestAttachUsesSnapshotWhenDimensionsDiffer(t *testing.T) {
	opts := DefaultOptions()
	session := newSession(opts, "snapshot-attach", "/tmp")
	session.cmd = &exec.Cmd{}
	session.handleProtocolOutput(context.Background(), []byte("hello world"))
	if err := session.storeSnapshotForOwner(json.RawMessage(testSnapshotPayload), "main"); err != nil {
		t.Fatalf("store snapshot: %v", err)
	}
	server := &Server{
		opts:     opts,
		sessions: map[string]*Session{"snapshot-attach": session},
	}

	clientConn, serverConn := net.Pipe()
	defer func() {
		_ = clientConn.Close()
	}()

	attachLine, err := json.Marshal(AttachRequest{
		ProtocolVersion: ProtocolVersion,
		Type:            "attach",
		SessionID:       "snapshot-attach",
		StreamID:        "snapshot-stream",
		WithBuffer:      true,
		Cols:            120,
		Rows:            24,
	})
	if err != nil {
		t.Fatalf("marshal attach: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = serverConn.Close() }()
		server.handleAttach(serverConn, attachLine)
	}()

	dec := json.NewDecoder(clientConn)
	var readyMsg StreamMessage
	if err := dec.Decode(&readyMsg); err != nil {
		t.Fatalf("attach ready response: %v", err)
	}
	ready := requireAttachReady(t, readyMsg)
	if ready.ReplayStart != 5 {
		t.Fatalf("expected replay to resume from snapshot offset even when dimensions differ, got %+v", ready)
	}

	var snapshotMsg StreamMessage
	if err := dec.Decode(&snapshotMsg); err != nil {
		t.Fatalf("attach snapshot response: %v", err)
	}
	if snapshotMsg.Type != "snapshot" {
		t.Fatalf("expected snapshot message when dimensions differ, got %+v", snapshotMsg)
	}

	var replayMsg StreamMessage
	if err := dec.Decode(&replayMsg); err != nil {
		t.Fatalf("attach replay response: %v", err)
	}
	if replayMsg.Type != "data" {
		t.Fatalf("expected replay data after snapshot, got %+v", replayMsg)
	}

	_ = clientConn.Close()
	session.closeSubscribers()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("attach handler did not exit")
	}
}
