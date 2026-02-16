package sessiond

import (
	"context"
	"strings"
	"testing"
)

func TestModeReplayPrefixTracksEnableDisable(t *testing.T) {
	session := newSession(DefaultOptions(), "mode-state", "/tmp")

	session.handleProtocolOutput(context.Background(), []byte("\x1b[?1049h\x1b[?1002h\x1b[?1006h"))
	session.outputMu.Lock()
	initial := string(session.modeReplayPrefixLocked())
	session.outputMu.Unlock()
	if !strings.Contains(initial, "\x1b[?1049h") {
		t.Fatalf("expected alt-screen enable in replay prefix, got %q", initial)
	}
	if !strings.Contains(initial, "\x1b[?1002h") {
		t.Fatalf("expected mouse1002 enable in replay prefix, got %q", initial)
	}
	if !strings.Contains(initial, "\x1b[?1006h") {
		t.Fatalf("expected mouse1006 enable in replay prefix, got %q", initial)
	}

	session.handleProtocolOutput(context.Background(), []byte("\x1b[?1049l\x1b[?1002l"))
	session.outputMu.Lock()
	afterDisable := string(session.modeReplayPrefixLocked())
	session.outputMu.Unlock()
	if strings.Contains(afterDisable, "\x1b[?1049h") {
		t.Fatalf("did not expect alt-screen enable after disable, got %q", afterDisable)
	}
	if strings.Contains(afterDisable, "\x1b[?1002h") {
		t.Fatalf("did not expect mouse1002 enable after disable, got %q", afterDisable)
	}
	if !strings.Contains(afterDisable, "\x1b[?1006h") {
		t.Fatalf("expected mouse1006 to remain enabled, got %q", afterDisable)
	}
}

func TestModeReplayPrefixParsesSplitCSI(t *testing.T) {
	session := newSession(DefaultOptions(), "mode-state-split", "/tmp")

	session.handleProtocolOutput(context.Background(), []byte("\x1b[?10"))
	session.handleProtocolOutput(context.Background(), []byte("06h"))

	session.outputMu.Lock()
	got := string(session.modeReplayPrefixLocked())
	session.outputMu.Unlock()
	if !strings.Contains(got, "\x1b[?1006h") {
		t.Fatalf("expected split CSI mode to be tracked, got %q", got)
	}
}
