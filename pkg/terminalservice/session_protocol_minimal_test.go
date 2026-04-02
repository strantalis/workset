package terminalservice

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// waitPull waits for a notification and pulls data from the session buffer.
func waitPull(t *testing.T, session *Session, sub *subscriber) []byte {
	t.Helper()
	select {
	case _, ok := <-sub.notify:
		if !ok {
			t.Fatal("subscriber closed unexpectedly")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for notification")
	}
	data, _, _ := session.pullBuffer(sub)
	return data
}

func TestHandleProtocolOutputForwardsText(t *testing.T) {
	session := newSession(DefaultOptions(), "single-pipeline", "")
	sub := session.subscribe("stream-test", 0)
	defer session.unsubscribe(sub)

	raw := []byte("hello world\r\n")
	session.handleProtocolOutput(context.Background(), raw)

	data := waitPull(t, session, sub)
	if !bytes.Equal(data, raw) {
		t.Fatalf("expected raw bytes to pass through, got %x", data)
	}
}

func TestHandleProtocolOutputForwardsKittyAPC(t *testing.T) {
	session := newSession(DefaultOptions(), "single-pipeline-kitty", "")
	sub := session.subscribe("stream-test", 0)
	defer session.unsubscribe(sub)

	kittyAPC := []byte("\x1b_Gi=31337,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\")
	session.handleProtocolOutput(context.Background(), kittyAPC)

	data := waitPull(t, session, sub)
	if !bytes.Equal(data, kittyAPC) {
		t.Fatalf("expected kitty APC bytes to pass through, got %x", data)
	}
}

func TestHandleProtocolOutputForwardsRawC1BytesUnchanged(t *testing.T) {
	session := newSession(DefaultOptions(), "single-pipeline-c1-raw", "")
	sub := session.subscribe("stream-test", 0)
	defer session.unsubscribe(sub)

	raw := []byte{0x9b, '3', '1', 'm', 'X'}
	session.handleProtocolOutput(context.Background(), raw)

	data := waitPull(t, session, sub)
	if !bytes.Equal(data, raw) {
		t.Fatalf("expected raw C1 bytes to pass through unchanged, got %x", data)
	}
}

func TestHandleProtocolOutputForwardsUTF8ContinuationBytes(t *testing.T) {
	session := newSession(DefaultOptions(), "single-pipeline-utf8", "")
	sub := session.subscribe("stream-test", 0)
	defer session.unsubscribe(sub)

	// U+1F600 GRINNING FACE includes 0x9F as a UTF-8 continuation byte.
	raw := []byte("A\xf0\x9f\x98\x80B")
	session.handleProtocolOutput(context.Background(), raw)

	data := waitPull(t, session, sub)
	if !bytes.Equal(data, raw) {
		t.Fatalf("expected UTF-8 bytes to pass through unchanged, got %x", data)
	}
}

func TestHandleProtocolOutputForwardsKittyAPC7Bit(t *testing.T) {
	session := newSession(DefaultOptions(), "single-pipeline-kitty-strip-7bit", "")
	sub := session.subscribe("stream-test", 0)
	defer session.unsubscribe(sub)

	raw := []byte("A\x1b_Gi=31,s=1;AAAA\x1b\\B")
	session.handleProtocolOutput(context.Background(), raw)

	data := waitPull(t, session, sub)
	if !bytes.Equal(data, raw) {
		t.Fatalf("expected kitty APC bytes to pass through, got %q (%x)", string(data), data)
	}
}

func TestHandleProtocolOutputForwardsKittyAPCAcrossChunks(t *testing.T) {
	session := newSession(DefaultOptions(), "single-pipeline-kitty-strip-chunks", "")
	sub := session.subscribe("stream-test", 0)
	defer session.unsubscribe(sub)

	session.handleProtocolOutput(context.Background(), []byte("A\x1b_"))
	session.handleProtocolOutput(context.Background(), []byte("Gi=31;AAAA"))
	session.handleProtocolOutput(context.Background(), []byte("\x1b\\B"))

	// With the pull model, notifications may coalesce.  Drain all available
	// data and verify the concatenated result.
	time.Sleep(50 * time.Millisecond) // let all notifications settle
	var all []byte
	for {
		select {
		case _, ok := <-sub.notify:
			if !ok {
				t.Fatal("subscriber closed")
			}
			data, _, _ := session.pullBuffer(sub)
			all = append(all, data...)
		default:
			goto done
		}
	}
done:
	expected := []byte("A\x1b_Gi=31;AAAA\x1b\\B")
	if !bytes.Equal(all, expected) {
		t.Fatalf("expected coalesced output %q, got %q", string(expected), string(all))
	}
}
