package sessiond

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestHandleProtocolOutputForwardsText(t *testing.T) {
	session := newSession(DefaultOptions(), "single-pipeline", "")
	sub := session.subscribe("stream-test")
	defer session.unsubscribe(sub)

	raw := []byte("hello world\r\n")
	session.handleProtocolOutput(context.Background(), raw)

	select {
	case event := <-sub.ch:
		if !bytes.Equal(event, raw) {
			t.Fatalf("expected raw bytes to pass through, got %x", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected stream event")
	}
}

func TestHandleProtocolOutputForwardsKittyAPC(t *testing.T) {
	session := newSession(DefaultOptions(), "single-pipeline-kitty", "")
	sub := session.subscribe("stream-test")
	defer session.unsubscribe(sub)

	kittyAPC := []byte("\x1b_Gi=31337,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\")
	session.handleProtocolOutput(context.Background(), kittyAPC)

	select {
	case event := <-sub.ch:
		if !bytes.Equal(event, kittyAPC) {
			t.Fatalf("expected kitty APC bytes to pass through, got %x", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected stream event")
	}
}

func TestHandleProtocolOutputForwardsRawC1BytesUnchanged(t *testing.T) {
	session := newSession(DefaultOptions(), "single-pipeline-c1-raw", "")
	sub := session.subscribe("stream-test")
	defer session.unsubscribe(sub)

	raw := []byte{0x9b, '3', '1', 'm', 'X'}
	session.handleProtocolOutput(context.Background(), raw)

	select {
	case event := <-sub.ch:
		if !bytes.Equal(event, raw) {
			t.Fatalf("expected raw C1 bytes to pass through unchanged, got %x", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected stream event")
	}
}

func TestHandleProtocolOutputForwardsUTF8ContinuationBytes(t *testing.T) {
	session := newSession(DefaultOptions(), "single-pipeline-utf8", "")
	sub := session.subscribe("stream-test")
	defer session.unsubscribe(sub)

	// U+1F600 GRINNING FACE includes 0x9F as a UTF-8 continuation byte.
	raw := []byte("A\xf0\x9f\x98\x80B")
	session.handleProtocolOutput(context.Background(), raw)

	select {
	case event := <-sub.ch:
		if !bytes.Equal(event, raw) {
			t.Fatalf("expected UTF-8 bytes to pass through unchanged, got %x", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected stream event")
	}
}

func TestHandleProtocolOutputForwardsKittyAPC7Bit(t *testing.T) {
	session := newSession(DefaultOptions(), "single-pipeline-kitty-strip-7bit", "")
	sub := session.subscribe("stream-test")
	defer session.unsubscribe(sub)

	raw := []byte("A\x1b_Gi=31,s=1;AAAA\x1b\\B")
	session.handleProtocolOutput(context.Background(), raw)

	select {
	case event := <-sub.ch:
		if !bytes.Equal(event, raw) {
			t.Fatalf("expected kitty APC bytes to pass through, got %q (%x)", string(event), event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected stream event")
	}
}

func TestHandleProtocolOutputForwardsKittyAPCAcrossChunks(t *testing.T) {
	session := newSession(DefaultOptions(), "single-pipeline-kitty-strip-chunks", "")
	sub := session.subscribe("stream-test")
	defer session.unsubscribe(sub)

	session.handleProtocolOutput(context.Background(), []byte("A\x1b_"))
	session.handleProtocolOutput(context.Background(), []byte("Gi=31;AAAA"))
	session.handleProtocolOutput(context.Background(), []byte("\x1b\\B"))

	first := <-sub.ch
	second := <-sub.ch
	third := <-sub.ch
	if string(first) != "A\x1b_" {
		t.Fatalf("expected first chunk to pass through APC prefix, got %q", string(first))
	}
	if string(second) != "Gi=31;AAAA" {
		t.Fatalf("expected second chunk to pass through APC payload, got %q", string(second))
	}
	if string(third) != "\x1b\\B" {
		t.Fatalf("expected third chunk to retain APC terminator and suffix, got %q", string(third))
	}
}
