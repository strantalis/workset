package sessiond

import "testing"

func TestTerminalBufferReadSince(t *testing.T) {
	buf := newTerminalBuffer(64 * 1024)
	buf.Append([]byte("hello"))
	data, next, truncated := buf.ReadSince(0)
	if string(data) != "hello" {
		t.Fatalf("expected data %q, got %q", "hello", string(data))
	}
	if next != 5 {
		t.Fatalf("expected next offset 5, got %d", next)
	}
	if truncated {
		t.Fatalf("did not expect truncated backlog")
	}
}

func TestTerminalBufferTruncation(t *testing.T) {
	buf := newTerminalBuffer(64 * 1024)
	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = 'a'
	}
	for range 200 {
		buf.Append(payload)
	}
	data, next, truncated := buf.ReadSince(0)
	if len(data) == 0 {
		t.Fatalf("expected data after truncation")
	}
	if next == 0 {
		t.Fatalf("expected non-zero next offset")
	}
	if !truncated {
		t.Fatalf("expected truncation when reading from offset 0")
	}
}
