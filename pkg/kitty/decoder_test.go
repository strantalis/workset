package kitty

import (
	"strings"
	"testing"
)

func TestDecoderTransmitPlace(t *testing.T) {
	state := NewState()
	var dec Decoder
	seq := "hello\x1b_Ga=T,f=24,s=1,v=1,c=2,r=3;AAAAAA==\x1b\\world"
	out, events := dec.Process([]byte(seq), Cursor{Row: 1, Col: 2}, state)
	if len(events) < 2 {
		t.Fatalf("expected image + placement events, got %d", len(events))
	}
	output := string(out)
	if !strings.Contains(output, "\x1b[2C") || !strings.Contains(output, "\x1b[3B") {
		t.Fatalf("expected cursor move escape sequences in output, got %q", output)
	}
}

func TestDecoderMultiChunk(t *testing.T) {
	state := NewState()
	var dec Decoder
	first := "\x1b_Ga=T,f=24,s=1,v=1,c=1,r=1,m=1;AAAA\x1b\\"
	out, events := dec.Process([]byte(first), Cursor{}, state)
	if len(out) != 0 || len(events) != 0 {
		t.Fatalf("expected no output/events for first chunk, got %d/%d", len(out), len(events))
	}
	second := "\x1b_Gm=0;AA==\x1b\\"
	_, events = dec.Process([]byte(second), Cursor{}, state)
	if len(events) == 0 {
		t.Fatalf("expected events after final chunk")
	}
}

func TestDecoderDeleteAll(t *testing.T) {
	state := NewState()
	var dec Decoder
	seq := "\x1b_Ga=T,f=24,s=1,v=1,c=1,r=1;AAAAAA==\x1b\\"
	_, _ = dec.Process([]byte(seq), Cursor{}, state)
	deleteSeq := "\x1b_Ga=d,d=a;\x1b\\"
	_, events := dec.Process([]byte(deleteSeq), Cursor{}, state)
	found := false
	for _, ev := range events {
		if ev.Kind == "delete" && ev.Delete != nil && ev.Delete.All {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected delete all event")
	}
}
