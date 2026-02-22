package sessiond

import (
	"context"
	"testing"
)

func TestSanitizeProtocolInputDropsOSC11ColorResponse(t *testing.T) {
	session := newSession(DefaultOptions(), "input-filter-drop-11", "")
	input := []byte("A\x1b]11;rgb:1414/1f1f/2e2e\x1b\\B")

	got := session.sanitizeProtocolInput(context.Background(), input)
	if string(got) != "AB" {
		t.Fatalf("expected OSC11 response to be removed, got %q", string(got))
	}
	if len(session.inputFilter.pendingOSC) != 0 {
		t.Fatalf("expected no pending OSC bytes, got %d", len(session.inputFilter.pendingOSC))
	}
}

func TestSanitizeProtocolInputDropsOSC4ColorResponseBel(t *testing.T) {
	session := newSession(DefaultOptions(), "input-filter-drop-4", "")
	input := []byte("X\x1b]4;0;rgb:2e2e/3434/3636\aY")

	got := session.sanitizeProtocolInput(context.Background(), input)
	if string(got) != "XY" {
		t.Fatalf("expected OSC4 color response to be removed, got %q", string(got))
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

func TestSanitizeProtocolInputDropsSplitOSC11AcrossChunks(t *testing.T) {
	session := newSession(DefaultOptions(), "input-filter-split", "")

	first := session.sanitizeProtocolInput(context.Background(), []byte("A\x1b]11;rgb:1414/1f"))
	if string(first) != "A" {
		t.Fatalf("expected first chunk output to keep prefix text only, got %q", string(first))
	}
	if len(session.inputFilter.pendingOSC) == 0 {
		t.Fatal("expected pending OSC bytes after split first chunk")
	}

	second := session.sanitizeProtocolInput(context.Background(), []byte("1f/2e2e\x1b\\B"))
	if string(second) != "B" {
		t.Fatalf("expected second chunk output to drop OSC and keep suffix text, got %q", string(second))
	}
	if len(session.inputFilter.pendingOSC) != 0 {
		t.Fatalf("expected pending OSC buffer to clear after terminator, got %d", len(session.inputFilter.pendingOSC))
	}
}
