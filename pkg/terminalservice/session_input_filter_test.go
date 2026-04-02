package terminalservice

import (
	"context"
	"testing"
)

func TestSanitizeProtocolInputKeepsOSC11ColorResponse(t *testing.T) {
	session := newSession(DefaultOptions(), "input-filter-keep-11", "")
	input := []byte("A\x1b]11;rgb:1414/1f1f/2e2e\x1b\\B")

	got := session.sanitizeProtocolInput(context.Background(), input)
	if string(got) != string(input) {
		t.Fatalf("expected OSC11 response to pass through unchanged, got %q", string(got))
	}
}

func TestSanitizeProtocolInputKeepsOSC11ColorResponseC1(t *testing.T) {
	session := newSession(DefaultOptions(), "input-filter-keep-11-c1", "")
	input := []byte("A\x9d11;rgb:1414/1f1f/2e2e\x9cB")

	got := session.sanitizeProtocolInput(context.Background(), input)
	if string(got) != string(input) {
		t.Fatalf("expected OSC11 C1 response to pass through unchanged, got %q", string(got))
	}
}

func TestSanitizeProtocolInputKeepsOSC4ColorResponseBel(t *testing.T) {
	session := newSession(DefaultOptions(), "input-filter-keep-4-bel", "")
	input := []byte("X\x1b]4;0;rgb:2e2e/3434/3636\aY")

	got := session.sanitizeProtocolInput(context.Background(), input)
	if string(got) != string(input) {
		t.Fatalf("expected OSC4 BEL response to pass through unchanged, got %q", string(got))
	}
}

func TestSanitizeProtocolInputKeepsNonColorOSC(t *testing.T) {
	session := newSession(DefaultOptions(), "input-filter-keep-title", "")
	input := []byte("A\x1b]2;terminal title\x1b\\B")

	got := session.sanitizeProtocolInput(context.Background(), input)
	if string(got) != string(input) {
		t.Fatalf("expected non-color OSC to pass through unchanged, got %q", string(got))
	}
}

func TestSanitizeProtocolInputKeepsSplitOSC11AcrossChunks(t *testing.T) {
	session := newSession(DefaultOptions(), "input-filter-split-keep", "")

	firstInput := []byte("A\x1b]11;rgb:1414/1f")
	first := session.sanitizeProtocolInput(context.Background(), firstInput)
	if string(first) != string(firstInput) {
		t.Fatalf("expected first split chunk to pass through unchanged, got %q", string(first))
	}

	secondInput := []byte("1f/2e2e\x1b\\B")
	second := session.sanitizeProtocolInput(context.Background(), secondInput)
	if string(second) != string(secondInput) {
		t.Fatalf("expected second split chunk to pass through unchanged, got %q", string(second))
	}
}

func TestSanitizeProtocolInputKeepsOSC10ColorResponse(t *testing.T) {
	session := newSession(DefaultOptions(), "input-filter-keep-10", "")
	input := []byte("A\x1b]10;rgb:1414/1f1f/2e2e\x1b\\B")

	got := session.sanitizeProtocolInput(context.Background(), input)
	if string(got) != string(input) {
		t.Fatalf("expected OSC10 color response to pass through unchanged, got %q", string(got))
	}
}

func TestSanitizeProtocolInputKeepsOSC4ColorResponse(t *testing.T) {
	session := newSession(DefaultOptions(), "input-filter-keep-4", "")
	input := []byte("A\x1b]4;0;rgb:1414/1f1f/2e2e\x1b\\B")

	got := session.sanitizeProtocolInput(context.Background(), input)
	if string(got) != string(input) {
		t.Fatalf("expected OSC4 color response to pass through unchanged, got %q", string(got))
	}
}
