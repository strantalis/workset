package sessiond

import "testing"

func TestSanitizeProtocolOutputDropsRawOSC11ColorResponse(t *testing.T) {
	session := newSession(DefaultOptions(), "output-filter-raw-11", "")
	input := []byte("A\x1b]11;rgb:1414/1f1f/2e2e\x1b\\B")

	got := session.sanitizeProtocolOutput(input)
	if string(got) != "AB" {
		t.Fatalf("expected raw OSC11 response to be removed, got %q", string(got))
	}
}

func TestSanitizeProtocolOutputDropsCaretEncodedOSC11ColorResponse(t *testing.T) {
	session := newSession(DefaultOptions(), "output-filter-caret-11", "")
	input := []byte("A^[]11;rgb:1414/1f1f/2e2e^[\\B")

	got := session.sanitizeProtocolOutput(input)
	if string(got) != "AB" {
		t.Fatalf("expected caret OSC11 response to be removed, got %q", string(got))
	}
}

func TestSanitizeProtocolOutputDropsCaretEncodedOSC11BelResponse(t *testing.T) {
	session := newSession(DefaultOptions(), "output-filter-caret-bel", "")
	input := []byte("A^[]11;rgb:14/1f/2e^GB")

	got := session.sanitizeProtocolOutput(input)
	if string(got) != "AB" {
		t.Fatalf("expected caret OSC11 BEL response to be removed, got %q", string(got))
	}
}

func TestSanitizeProtocolOutputDropsSplitRawOSC11AcrossChunks(t *testing.T) {
	session := newSession(DefaultOptions(), "output-filter-split-raw", "")

	first := session.sanitizeProtocolOutput([]byte("A\x1b]11;rgb:1414/1f"))
	if string(first) != "A" {
		t.Fatalf("expected first output chunk to keep plain prefix only, got %q", string(first))
	}

	second := session.sanitizeProtocolOutput([]byte("1f/2e2e\x1b\\B"))
	if string(second) != "B" {
		t.Fatalf("expected second output chunk to drop split raw OSC11 sequence, got %q", string(second))
	}
}

func TestSanitizeProtocolOutputDropsSplitCaretOSC11AcrossChunks(t *testing.T) {
	session := newSession(DefaultOptions(), "output-filter-split-caret", "")

	first := session.sanitizeProtocolOutput([]byte("A^[]11;rgb:1414/1f"))
	if string(first) != "A" {
		t.Fatalf("expected first output chunk to keep plain prefix only, got %q", string(first))
	}

	second := session.sanitizeProtocolOutput([]byte("1f/2e2e^[\\B"))
	if string(second) != "B" {
		t.Fatalf("expected second output chunk to drop split caret OSC11 sequence, got %q", string(second))
	}
}

func TestSanitizeProtocolOutputKeepsNonColorOSC(t *testing.T) {
	session := newSession(DefaultOptions(), "output-filter-keep-title", "")
	input := []byte("A\x1b]2;terminal title\x1b\\B")

	got := session.sanitizeProtocolOutput(input)
	if string(got) != string(input) {
		t.Fatalf("expected non-color OSC to pass through unchanged, got %q", string(got))
	}
}
