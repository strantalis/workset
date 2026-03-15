package sessiond

import (
	"context"
	"testing"
)

func TestSanitizeProtocolOutputKeepsCaretEncodedOSC11ColorResponse(t *testing.T) {
	session := newSession(DefaultOptions(), "output-filter-keep-caret-11", "")
	input := []byte("A^[]11;rgb:1414/1f1f/2e2e^[\\B")

	got := session.sanitizeProtocolOutput(context.Background(), input)
	if string(got) != string(input) {
		t.Fatalf("expected caret OSC11 response to pass through unchanged, got %q", string(got))
	}
}

func TestSanitizeProtocolOutputKeepsCaretEncodedOSC11BelResponse(t *testing.T) {
	session := newSession(DefaultOptions(), "output-filter-keep-caret-bel", "")
	input := []byte("A^[]11;rgb:14/1f/2e^GB")

	got := session.sanitizeProtocolOutput(context.Background(), input)
	if string(got) != string(input) {
		t.Fatalf("expected caret OSC11 BEL response to pass through unchanged, got %q", string(got))
	}
}

func TestSanitizeProtocolOutputKeepsSplitCaretOSC11AcrossChunks(t *testing.T) {
	session := newSession(DefaultOptions(), "output-filter-keep-split-caret", "")

	firstInput := []byte("A^[]11;rgb:1414/1f")
	first := session.sanitizeProtocolOutput(context.Background(), firstInput)
	if string(first) != string(firstInput) {
		t.Fatalf("expected first split output chunk to pass through unchanged, got %q", string(first))
	}

	secondInput := []byte("1f/2e2e^[\\B")
	second := session.sanitizeProtocolOutput(context.Background(), secondInput)
	if string(second) != string(secondInput) {
		t.Fatalf("expected second split output chunk to pass through unchanged, got %q", string(second))
	}
}

func TestSanitizeProtocolOutputKeepsNonColorOSC(t *testing.T) {
	session := newSession(DefaultOptions(), "output-filter-keep-title", "")
	input := []byte("A\x1b]2;terminal title\x1b\\B")

	got := session.sanitizeProtocolOutput(context.Background(), input)
	if string(got) != string(input) {
		t.Fatalf("expected non-color OSC to pass through unchanged, got %q", string(got))
	}
}

func TestSanitizeProtocolOutputKeepsRawOSCColorSetSequence(t *testing.T) {
	session := newSession(DefaultOptions(), "output-filter-keep-raw-set", "")
	input := []byte("A\x1b]10;rgb:1414/1f1f/2e2e\x1b\\B")

	got := session.sanitizeProtocolOutput(context.Background(), input)
	if string(got) != string(input) {
		t.Fatalf("expected OSC10 color set sequence to pass through unchanged, got %q", string(got))
	}
}

func TestSanitizeProtocolOutputKeepsCaretOSC4ColorSequence(t *testing.T) {
	session := newSession(DefaultOptions(), "output-filter-keep-caret-4", "")
	input := []byte("A^[]4;0;rgb:1414/1f1f/2e2e^[\\B")

	got := session.sanitizeProtocolOutput(context.Background(), input)
	if string(got) != string(input) {
		t.Fatalf("expected caret OSC4 color sequence to pass through unchanged, got %q", string(got))
	}
}
