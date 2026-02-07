package termemu

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestTerminalWriteSnapshot(t *testing.T) {
	term := New(4, 2)
	term.Write(context.Background(), []byte("hi"))
	snap := term.Snapshot()
	if got := snap.Primary[0].Cells[0].Ch; got != 'h' {
		t.Fatalf("expected h, got %q", got)
	}
	if got := snap.Primary[0].Cells[1].Ch; got != 'i' {
		t.Fatalf("expected i, got %q", got)
	}
}

func TestTerminalHistoryCapture(t *testing.T) {
	emu := New(10, 2)
	emu.SetHistoryLimit(4)
	emu.Write(context.Background(), []byte("one\r\ntwo\r\nthree\r\n"))

	history := rowsText(emu.HistoryRows())
	if history != "one\ntwo" {
		t.Fatalf("unexpected history:\n%s", history)
	}

	snap := emu.SnapshotANSIWithHistory()
	if !strings.Contains(snap, "one") || !strings.Contains(snap, "two") {
		t.Fatalf("expected snapshot to include history lines, got %q", snap)
	}
}

func TestTerminalSnapshotANSIPrimary(t *testing.T) {
	term := New(2, 1)
	term.Write(context.Background(), []byte("A"))

	got := term.SnapshotANSI()
	want := "\x1b[?1049l\x1b[2J\x1b[H\x1b[1;1HA \x1b[0m\x1b[?25h\x1b[1;2H"
	if got != want {
		t.Fatalf("unexpected ANSI snapshot:\nwant: %q\n got: %q", want, got)
	}
}

func TestWriteRowANSITrimTrailingWhitespace(t *testing.T) {
	row := Row{
		Cells: []Cell{
			{Ch: 'A'},
			{Ch: 'B', Attr: Attr{Bold: true}},
			{Ch: ' '},
			{Ch: 0},
		},
	}
	var b strings.Builder
	writeRowANSI(&b, row, 4)
	if got := b.String(); got != "A\x1b[0;1;39;49mB" {
		t.Fatalf("unexpected row ANSI: %q", got)
	}
}

func TestTerminalAltScreen(t *testing.T) {
	term := New(4, 2)
	term.Write(context.Background(), []byte("\x1b[?1049h"))
	if !term.IsAltScreen() {
		t.Fatalf("expected alt screen on")
	}
	term.Write(context.Background(), []byte("x"))
	snap := term.Snapshot()
	if !snap.AltActive {
		t.Fatalf("expected alt active in snapshot")
	}
	if got := snap.Alt[0].Cells[0].Ch; got != 'x' {
		t.Fatalf("expected x in alt screen, got %q", got)
	}
}

func TestTerminalDecLineDrawing(t *testing.T) {
	term := New(4, 1)
	term.Write(context.Background(), []byte("\x1b)0\x0eqx\x0f"))
	snap := term.Snapshot()
	if got := snap.Primary[0].Cells[0].Ch; got != '─' {
		t.Fatalf("expected line drawing, got %q", got)
	}
	if got := snap.Primary[0].Cells[1].Ch; got != '│' {
		t.Fatalf("expected line drawing, got %q", got)
	}
}

func TestTerminalTabStops(t *testing.T) {
	term := New(16, 1)
	term.Write(context.Background(), []byte("a\tb"))
	snap := term.Snapshot()
	if got := snap.Primary[0].Cells[0].Ch; got != 'a' {
		t.Fatalf("expected a, got %q", got)
	}
	if got := snap.Primary[0].Cells[8].Ch; got != 'b' {
		t.Fatalf("expected b at tab stop, got %q", got)
	}
}

func TestTerminalOriginMode(t *testing.T) {
	term := New(5, 5)
	term.Write(context.Background(), []byte("\x1b[2;4r"))
	term.Write(context.Background(), []byte("\x1b[?6h"))
	term.Write(context.Background(), []byte("\x1b[H"))
	term.Write(context.Background(), []byte("X"))
	snap := term.Snapshot()
	if got := snap.Primary[1].Cells[0].Ch; got != 'X' {
		t.Fatalf("expected X at top of scroll region, got %q", got)
	}
}

func TestTerminalEraseChars(t *testing.T) {
	term := New(5, 1)
	term.Write(context.Background(), []byte("abcde"))
	term.Write(context.Background(), []byte("\x1b[1D\x1b[2X"))
	snap := term.Snapshot()
	if got := snap.Primary[0].Cells[2].Ch; got != 'c' {
		t.Fatalf("expected c, got %q", got)
	}
	if got := snap.Primary[0].Cells[3].Ch; got != ' ' {
		t.Fatalf("expected space after erase, got %q", got)
	}
	if got := snap.Primary[0].Cells[4].Ch; got != ' ' {
		t.Fatalf("expected space after erase, got %q", got)
	}
}

func TestTerminalRepeat(t *testing.T) {
	term := New(4, 1)
	term.Write(context.Background(), []byte("A\x1b[3b"))
	snap := term.Snapshot()
	for i := range 4 {
		if got := snap.Primary[0].Cells[i].Ch; got != 'A' {
			t.Fatalf("expected A at %d, got %q", i, got)
		}
	}
}

func TestTerminalIgnoreEscapeString(t *testing.T) {
	term := New(6, 1)
	term.Write(context.Background(), []byte("hi"))
	term.Write(context.Background(), []byte("\x1bPqignored\x1b\\"))
	term.Write(context.Background(), []byte("ok"))
	snap := term.Snapshot()
	if got := snap.Primary[0].Cells[0].Ch; got != 'h' {
		t.Fatalf("expected h, got %q", got)
	}
	if got := snap.Primary[0].Cells[1].Ch; got != 'i' {
		t.Fatalf("expected i, got %q", got)
	}
	if got := snap.Primary[0].Cells[2].Ch; got != 'o' {
		t.Fatalf("expected o after escape string, got %q", got)
	}
	if got := snap.Primary[0].Cells[3].Ch; got != 'k' {
		t.Fatalf("expected k after escape string, got %q", got)
	}
}

func TestTerminalUTF8Continuation(t *testing.T) {
	term := New(4, 1)
	term.Write(context.Background(), []byte("☁X"))
	snap := term.Snapshot()
	if got := snap.Primary[0].Cells[0].Ch; got != '☁' {
		t.Fatalf("expected cloud rune, got %q", got)
	}
	if got := snap.Primary[0].Cells[1].Ch; got != 'X' {
		t.Fatalf("expected X after UTF-8 rune, got %q", got)
	}
}

func TestTerminalDSRReportsCursorPosition(t *testing.T) {
	term := New(10, 2)
	var responses [][]byte
	term.SetResponder(func(data []byte) {
		responses = append(responses, append([]byte(nil), data...))
	})

	term.Write(context.Background(), []byte("ab\x1b[6n"))

	if len(responses) != 1 {
		t.Fatalf("expected one response, got %d", len(responses))
	}
	if got := string(responses[0]); got != "\x1b[1;3R" {
		t.Fatalf("unexpected DSR response: %q", got)
	}
}

func TestTerminalDECRQMModeQuery(t *testing.T) {
	term := New(10, 2)
	var responses [][]byte
	term.SetResponder(func(data []byte) {
		responses = append(responses, append([]byte(nil), data...))
	})

	term.Write(context.Background(), []byte("\x1b[?25$p"))
	if len(responses) != 1 {
		t.Fatalf("expected one response, got %d", len(responses))
	}
	if got := string(responses[0]); got != "\x1b[?25;1$y" {
		t.Fatalf("unexpected DECRQM response: %q", got)
	}

	responses = nil
	term.Write(context.Background(), []byte("\x1b[?25l\x1b[?25$p"))
	if len(responses) != 1 {
		t.Fatalf("expected one response after disabling cursor, got %d", len(responses))
	}
	if got := string(responses[0]); got != "\x1b[?25;2$y" {
		t.Fatalf("unexpected DECRQM response after mode change: %q", got)
	}
}

func TestTerminalDSROriginAltInteraction(t *testing.T) {
	term := New(12, 5)
	var responses [][]byte
	term.SetResponder(func(data []byte) {
		responses = append(responses, append([]byte(nil), data...))
	})

	term.Write(context.Background(), []byte("\x1b[2;4r\x1b[?6h\x1b[6n"))
	if len(responses) != 1 {
		t.Fatalf("expected one response, got %d", len(responses))
	}
	if got := string(responses[0]); got != "\x1b[1;1R" {
		t.Fatalf("unexpected DSR response with origin on: %q", got)
	}

	responses = nil
	term.Write(context.Background(), []byte("\x1b[2;3H\x1b[6n"))
	if len(responses) != 1 {
		t.Fatalf("expected one response after moving cursor, got %d", len(responses))
	}
	if got := string(responses[0]); got != "\x1b[2;3R" {
		t.Fatalf("unexpected DSR response in origin mode: %q", got)
	}

	responses = nil
	term.Write(context.Background(), []byte("\x1b[?1049h"))
	term.Write(context.Background(), []byte("ab\x1b[6n"))
	if len(responses) != 1 {
		t.Fatalf("expected one response in alt screen, got %d", len(responses))
	}
	if got := string(responses[0]); got != "\x1b[1;3R" {
		t.Fatalf("unexpected DSR response in alt screen: %q", got)
	}

	responses = nil
	term.Write(context.Background(), []byte("\x1b[?1049l\x1b[6n"))
	if len(responses) != 1 {
		t.Fatalf("expected one response after restoring from alt, got %d", len(responses))
	}
	if got := string(responses[0]); got != "\x1b[2;3R" {
		t.Fatalf("unexpected DSR response after alt restore: %q", got)
	}
}

func TestTerminalDSRIncompleteAndPrivateIgnored(t *testing.T) {
	term := New(10, 2)
	var responses [][]byte
	term.SetResponder(func(data []byte) {
		responses = append(responses, append([]byte(nil), data...))
	})

	term.Write(context.Background(), []byte("\x1b[6"))
	if len(responses) != 0 {
		t.Fatalf("expected no response for incomplete DSR, got %d", len(responses))
	}

	term.Write(context.Background(), []byte("n"))
	if len(responses) != 1 {
		t.Fatalf("expected one response when completing DSR, got %d", len(responses))
	}
	if got := string(responses[0]); got != "\x1b[1;1R" {
		t.Fatalf("unexpected split DSR response: %q", got)
	}

	responses = nil
	term.Write(context.Background(), []byte("\x1b[?6n\x1b[0n"))
	if len(responses) != 0 {
		t.Fatalf("expected no response for unsupported DSR forms, got %d", len(responses))
	}
}

func TestTerminalDECRQMMalformedAndIncomplete(t *testing.T) {
	term := New(10, 2)
	var responses [][]byte
	term.SetResponder(func(data []byte) {
		responses = append(responses, append([]byte(nil), data...))
	})

	term.Write(context.Background(), []byte("\x1b[?25p"))
	if len(responses) != 0 {
		t.Fatalf("expected no response without DECRQM '$', got %d", len(responses))
	}

	term.Write(context.Background(), []byte("\x1b[?999$p"))
	if len(responses) != 1 {
		t.Fatalf("expected one response for unknown DECRQM mode, got %d", len(responses))
	}
	if got := string(responses[0]); got != "\x1b[?999;0$y" {
		t.Fatalf("unexpected DECRQM unknown-mode response: %q", got)
	}

	responses = nil
	term.Write(context.Background(), []byte("\x1b[25$p"))
	if len(responses) != 0 {
		t.Fatalf("expected no response without private marker, got %d", len(responses))
	}

	term.Write(context.Background(), []byte("\x1b[?25$"))
	if len(responses) != 0 {
		t.Fatalf("expected no response for incomplete DECRQM, got %d", len(responses))
	}
	term.Write(context.Background(), []byte("p"))
	if len(responses) != 1 {
		t.Fatalf("expected one response when completing DECRQM, got %d", len(responses))
	}
	if got := string(responses[0]); got != "\x1b[?25;1$y" {
		t.Fatalf("unexpected split DECRQM response: %q", got)
	}
}

func TestTerminalTabStopsOriginAltInteraction(t *testing.T) {
	term := New(20, 5)
	term.Write(context.Background(), []byte("\x1b[2;5r\x1b[?6h\x1b[H\x1b[5G\x1bH"))

	term.Write(context.Background(), []byte("\x1b[?1049h\x1b[H\tX"))
	altSnap := term.Snapshot()
	if !altSnap.AltActive {
		t.Fatalf("expected alt screen to be active")
	}
	if got := altSnap.Alt[1].Cells[4].Ch; got != 'X' {
		t.Fatalf("expected X at custom tab stop in alt screen, got %q", got)
	}

	term.Write(context.Background(), []byte("\x1b[?1049l\x1b[H\tY"))
	primarySnap := term.Snapshot()
	if primarySnap.AltActive {
		t.Fatalf("expected primary screen to be active")
	}
	if got := primarySnap.Primary[1].Cells[4].Ch; got != 'Y' {
		t.Fatalf("expected Y at custom tab stop after leaving alt, got %q", got)
	}
}

func TestTerminalMalformedAndIncompleteEscapeSequences(t *testing.T) {
	t.Run("incomplete csi is swallowed until final", func(t *testing.T) {
		term := New(8, 1)
		term.Write(context.Background(), []byte("ab"))
		term.Write(context.Background(), []byte("\x1b["))
		term.Write(context.Background(), []byte("x"))
		term.Write(context.Background(), []byte("cd"))

		snap := term.Snapshot()
		if got := snap.Primary[0].Cells[0].Ch; got != 'a' {
			t.Fatalf("expected a, got %q", got)
		}
		if got := snap.Primary[0].Cells[1].Ch; got != 'b' {
			t.Fatalf("expected b, got %q", got)
		}
		if got := snap.Primary[0].Cells[2].Ch; got != 'c' {
			t.Fatalf("expected c after malformed CSI, got %q", got)
		}
		if got := snap.Primary[0].Cells[3].Ch; got != 'd' {
			t.Fatalf("expected d after malformed CSI, got %q", got)
		}
	})

	t.Run("unterminated escape string spans writes", func(t *testing.T) {
		term := New(8, 1)
		term.Write(context.Background(), []byte("a\x1bPignored"))
		term.Write(context.Background(), []byte("still"))
		term.Write(context.Background(), []byte("\x1b\\b"))

		snap := term.Snapshot()
		if got := snap.Primary[0].Cells[0].Ch; got != 'a' {
			t.Fatalf("expected a, got %q", got)
		}
		if got := snap.Primary[0].Cells[1].Ch; got != 'b' {
			t.Fatalf("expected b after terminating escape string, got %q", got)
		}
	})
}

func TestTerminalReplayGoldens(t *testing.T) {
	cases := []struct {
		name   string
		log    string
		screen string
	}{
		{
			name:   "codex",
			log:    "testdata/codex_bootstrap.log",
			screen: "testdata/codex_bootstrap.screen",
		},
		{
			name:   "claude",
			log:    "testdata/claude_bootstrap.log",
			screen: "testdata/claude_bootstrap.screen",
		},
		{
			name:   "opencode",
			log:    "testdata/opencode_bootstrap.log",
			screen: "testdata/opencode_bootstrap.screen",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.log)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			want, err := os.ReadFile(tc.screen)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			emu := New(200, 24)
			emu.Write(context.Background(), data)
			got := snapshotText(emu.Snapshot())
			wantText := strings.TrimRight(string(want), "\n")
			if got != wantText {
				t.Fatalf("golden mismatch:\n%s", firstDiff(got, wantText))
			}
		})
	}
}

func snapshotText(snap Snapshot) string {
	rows := snap.Primary
	if snap.AltActive {
		rows = snap.Alt
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		buf := make([]rune, len(row.Cells))
		for i, cell := range row.Cells {
			r := cell.Ch
			if r == 0 {
				r = ' '
			}
			buf[i] = r
		}
		lines = append(lines, strings.TrimRight(string(buf), " "))
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func rowsText(rows []Row) string {
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		buf := make([]rune, len(row.Cells))
		for i, cell := range row.Cells {
			r := cell.Ch
			if r == 0 {
				r = ' '
			}
			buf[i] = r
		}
		lines = append(lines, strings.TrimRight(string(buf), " "))
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func firstDiff(got, want string) string {
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(want, "\n")
	maxLines := len(gotLines)
	if len(wantLines) > maxLines {
		maxLines = len(wantLines)
	}
	for i := 0; i < maxLines; i++ {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			return "line " + strconv.Itoa(i+1) + "\nwant: " + w + "\n got: " + g
		}
	}
	return "content differs"
}
