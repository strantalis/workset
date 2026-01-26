package termemu

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestTerminalWriteSnapshot(t *testing.T) {
	term := New(4, 2)
	term.Write([]byte("hi"))
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
	emu.Write([]byte("one\r\ntwo\r\nthree\r\n"))

	history := rowsText(emu.HistoryRows())
	if history != "one\ntwo" {
		t.Fatalf("unexpected history:\n%s", history)
	}

	snap := emu.SnapshotANSIWithHistory()
	if !strings.Contains(snap, "one") || !strings.Contains(snap, "two") {
		t.Fatalf("expected snapshot to include history lines, got %q", snap)
	}
}

func TestTerminalAltScreen(t *testing.T) {
	term := New(4, 2)
	term.Write([]byte("\x1b[?1049h"))
	if !term.IsAltScreen() {
		t.Fatalf("expected alt screen on")
	}
	term.Write([]byte("x"))
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
	term.Write([]byte("\x1b)0\x0eqx\x0f"))
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
	term.Write([]byte("a\tb"))
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
	term.Write([]byte("\x1b[2;4r"))
	term.Write([]byte("\x1b[?6h"))
	term.Write([]byte("\x1b[H"))
	term.Write([]byte("X"))
	snap := term.Snapshot()
	if got := snap.Primary[1].Cells[0].Ch; got != 'X' {
		t.Fatalf("expected X at top of scroll region, got %q", got)
	}
}

func TestTerminalEraseChars(t *testing.T) {
	term := New(5, 1)
	term.Write([]byte("abcde"))
	term.Write([]byte("\x1b[1D\x1b[2X"))
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
	term.Write([]byte("A\x1b[3b"))
	snap := term.Snapshot()
	for i := 0; i < 4; i++ {
		if got := snap.Primary[0].Cells[i].Ch; got != 'A' {
			t.Fatalf("expected A at %d, got %q", i, got)
		}
	}
}

func TestTerminalIgnoreEscapeString(t *testing.T) {
	term := New(6, 1)
	term.Write([]byte("hi"))
	term.Write([]byte("\x1bPqignored\x1b\\"))
	term.Write([]byte("ok"))
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
	term.Write([]byte("☁X"))
	snap := term.Snapshot()
	if got := snap.Primary[0].Cells[0].Ch; got != '☁' {
		t.Fatalf("expected cloud rune, got %q", got)
	}
	if got := snap.Primary[0].Cells[1].Ch; got != 'X' {
		t.Fatalf("expected X after UTF-8 rune, got %q", got)
	}
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
			emu.Write(data)
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
	max := len(gotLines)
	if len(wantLines) > max {
		max = len(wantLines)
	}
	for i := 0; i < max; i++ {
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
