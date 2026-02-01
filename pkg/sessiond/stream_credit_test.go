package sessiond

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestStreamCreditGatesData(t *testing.T) {
	client, cleanup := startTestServerWithOptions(t, func(opts *Options) {
		opts.StreamInitialCredit = 1
		opts.StreamCreditTimeout = 2 * time.Second
	})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Create(ctx, "credit-test", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	stream, first, err := client.Attach(ctx, "credit-test", 0, false, "credit-stream")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if first.Type == "error" {
		t.Fatalf("attach error: %s", first.Error)
	}

	dataCh := make(chan StreamMessage, 1)
	errCh := make(chan error, 1)
	go func() {
		for {
			var msg StreamMessage
			if err := stream.Next(&msg); err != nil {
				errCh <- err
				return
			}
			if msg.Type == "data" {
				dataCh <- msg
				return
			}
		}
	}()

	sendCtx, sendCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer sendCancel()
	if err := client.Send(sendCtx, "credit-test", "printf 'READY\\n'\n"); err != nil {
		t.Fatalf("send output: %v", err)
	}

	select {
	case msg := <-dataCh:
		t.Fatalf("unexpected data before credit: %q", msg.Data)
	case err := <-errCh:
		t.Fatalf("unexpected stream error before credit: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	ackCtx, ackCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer ackCancel()
	if err := client.Ack(ackCtx, "credit-test", "credit-stream", 1024*1024); err != nil {
		t.Fatalf("ack: %v", err)
	}

	select {
	case msg := <-dataCh:
		if msg.Type != "data" || msg.Data == "" {
			t.Fatalf("expected data after credit, got %+v", msg)
		}
	case err := <-errCh:
		t.Fatalf("unexpected stream error after credit: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for data after credit")
	}
}

func TestStreamCreditTimeoutClosesStream(t *testing.T) {
	opts := DefaultOptions()
	opts.StreamInitialCredit = 0
	opts.StreamCreditTimeout = 200 * time.Millisecond
	session := newSession(opts, "timeout-test", "/tmp")
	server := &Server{
		opts:     opts,
		sessions: map[string]*Session{"timeout-test": session},
	}

	clientConn, serverConn := net.Pipe()
	defer func() {
		_ = clientConn.Close()
	}()
	attachLine, err := json.Marshal(AttachRequest{
		Type:      "attach",
		SessionID: "timeout-test",
		StreamID:  "timeout-stream",
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
	var first StreamMessage
	if err := dec.Decode(&first); err != nil {
		t.Fatalf("attach response: %v", err)
	}
	if first.Type == "error" {
		t.Fatalf("attach error: %s", first.Error)
	}

	deadline := time.Now().Add(1 * time.Second)
	for !session.hasSubscribers() {
		if time.Now().After(deadline) {
			t.Fatalf("subscriber not ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	session.broadcast([]byte("DATA"))

	errCh := make(chan error, 1)
	go func() {
		var msg StreamMessage
		errCh <- dec.Decode(&msg)
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatalf("expected stream to close on credit timeout")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected stream to close on credit timeout")
	}
	<-done
}

func TestSubscriberWaitForCreditResumes(t *testing.T) {
	sub := newSubscriber("test-stream", 0)
	done := make(chan bool, 1)
	go func() {
		done <- sub.waitForCredit(10, 500*time.Millisecond)
	}()

	time.Sleep(50 * time.Millisecond)
	sub.addCredit(10)

	select {
	case ok := <-done:
		if !ok {
			t.Fatalf("expected waitForCredit to succeed after credit")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for waitForCredit to resume")
	}
}

func TestSubscriberWaitForCreditTimesOut(t *testing.T) {
	sub := newSubscriber("test-stream", 0)
	if ok := sub.waitForCredit(10, 50*time.Millisecond); ok {
		t.Fatalf("expected waitForCredit to time out without credit")
	}
}

func TestSubscriberCreditConsumption(t *testing.T) {
	sub := newSubscriber("test-stream", 20)
	if ok := sub.waitForCredit(10, 50*time.Millisecond); !ok {
		t.Fatalf("expected waitForCredit to succeed with credit")
	}
	sub.creditMu.Lock()
	remaining := sub.credit
	sub.creditMu.Unlock()
	if remaining != 10 {
		t.Fatalf("expected remaining credit 10, got %d", remaining)
	}
}
