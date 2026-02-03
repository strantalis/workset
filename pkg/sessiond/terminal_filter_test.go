package sessiond

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/strantalis/workset/pkg/termemu"
)

func enableTerminalFilter(t *testing.T) {
	t.Helper()
	t.Setenv("WORKSET_TERMINAL_FILTER", "1")
	t.Setenv("WORKSET_TERMINAL_FILTER_DEBUG", "0")
	t.Setenv("WORKSET_TERMINAL_FILTER_DROP_COLORS", "0")
	terminalFilterOnce = sync.Once{}
	terminalFilterEnabled = false
	terminalFilterDebug = false
	terminalFilterDropOSC = false
	terminalFilterLog = nil
}

func enableTerminalFilterDropColors(t *testing.T) {
	t.Helper()
	t.Setenv("WORKSET_TERMINAL_FILTER", "1")
	t.Setenv("WORKSET_TERMINAL_FILTER_DEBUG", "0")
	t.Setenv("WORKSET_TERMINAL_FILTER_DROP_COLORS", "1")
	terminalFilterOnce = sync.Once{}
	terminalFilterEnabled = false
	terminalFilterDebug = false
	terminalFilterDropOSC = false
	terminalFilterLog = nil
}

func TestFilterTerminalOutputStreamingDropsResponderResponses(t *testing.T) {
	enableTerminalFilter(t)

	emu := termemu.New(80, 24)
	var responses [][]byte
	emu.SetResponder(func(resp []byte) {
		responses = append(responses, resp)
	})
	emu.Write(context.Background(), []byte("\x1b[6n\x1b[c\x1b[>c"))
	if len(responses) == 0 {
		t.Fatalf("expected responder output, got none")
	}

	raw := bytes.Join(responses, nil)
	var filter escapeStringFilter
	filtered := filterTerminalOutputStreaming(raw, &filter)
	if len(filtered) != 0 {
		t.Fatalf("expected responder output to be filtered, got %q", filtered)
	}
}

func TestFilterTerminalOutputStreamingLeavesRequestsAndSGR(t *testing.T) {
	enableTerminalFilter(t)

	input := []byte("hi\x1b[6n\x1b[c\x1b[31mred\x1b[0m")
	var filter escapeStringFilter
	got := filterTerminalOutputStreaming(input, &filter)
	if !bytes.Equal(got, input) {
		t.Fatalf("expected output to be unchanged, got %q", got)
	}
}

func TestFilterTerminalOutputStreamingDropsOSCColorResponsesAcrossChunks(t *testing.T) {
	enableTerminalFilterDropColors(t)

	seq := []byte("\x1b]10;rgb:aa/bb/cc\x07")
	part1 := seq[:len(seq)-1]
	part2 := seq[len(seq)-1:]

	var filter escapeStringFilter
	if got := filterTerminalOutputStreaming(part1, &filter); len(got) != 0 {
		t.Fatalf("expected no output while OSC is in-flight, got %q", got)
	}
	if got := filterTerminalOutputStreaming(part2, &filter); len(got) != 0 {
		t.Fatalf("expected OSC response to be dropped, got %q", got)
	}
	if got := filterTerminalOutputStreaming([]byte("ok"), &filter); string(got) != "ok" {
		t.Fatalf("expected filter to resume after OSC, got %q", got)
	}
}

func TestFilterTerminalOutputStreamingLeavesOSCQueries(t *testing.T) {
	enableTerminalFilter(t)

	input := []byte("\x1b]10;?\x07ready")
	var filter escapeStringFilter
	got := filterTerminalOutputStreaming(input, &filter)
	if !bytes.Equal(got, input) {
		t.Fatalf("expected OSC query to be preserved, got %q", got)
	}
}
