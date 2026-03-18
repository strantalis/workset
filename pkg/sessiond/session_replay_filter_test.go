package sessiond

import (
	"testing"
)

func TestStripReplayQueries(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		// --- basics ---
		{
			name:  "empty input",
			input: nil,
			want:  nil,
		},
		{
			name:  "no escape sequences",
			input: []byte("hello world"),
			want:  []byte("hello world"),
		},

		// --- CSI: DSR queries ---
		{
			name:  "DSR cursor position query",
			input: []byte("\x1b[6n"),
			want:  []byte{},
		},
		{
			name:  "DSR cursor position with leading zero",
			input: []byte("\x1b[06n"),
			want:  []byte{},
		},
		{
			name:  "DSR cursor position with multiple leading zeros",
			input: []byte("\x1b[006n"),
			want:  []byte{},
		},
		{
			name:  "DSR operating status query",
			input: []byte("\x1b[5n"),
			want:  []byte{},
		},
		{
			name:  "DSR operating status with leading zero",
			input: []byte("\x1b[05n"),
			want:  []byte{},
		},
		{
			name:  "DECXCPR extended cursor position",
			input: []byte("\x1b[?6n"),
			want:  []byte{},
		},
		{
			name:  "DECXCPR with leading zero",
			input: []byte("\x1b[?06n"),
			want:  []byte{},
		},

		// --- CSI: DA queries ---
		{
			name:  "primary DA query bare",
			input: []byte("\x1b[c"),
			want:  []byte{},
		},
		{
			name:  "primary DA query with zero",
			input: []byte("\x1b[0c"),
			want:  []byte{},
		},
		{
			name:  "primary DA query with leading zeros",
			input: []byte("\x1b[00c"),
			want:  []byte{},
		},
		{
			name:  "secondary DA query bare",
			input: []byte("\x1b[>c"),
			want:  []byte{},
		},
		{
			name:  "secondary DA query with zero",
			input: []byte("\x1b[>0c"),
			want:  []byte{},
		},
		{
			name:  "tertiary DA query bare",
			input: []byte("\x1b[=c"),
			want:  []byte{},
		},
		{
			name:  "tertiary DA query with zero",
			input: []byte("\x1b[=0c"),
			want:  []byte{},
		},

		// --- CSI: XTVERSION ---
		{
			name:  "XTVERSION query bare",
			input: []byte("\x1b[>q"),
			want:  []byte{},
		},
		{
			name:  "XTVERSION query with zero",
			input: []byte("\x1b[>0q"),
			want:  []byte{},
		},

		// --- CSI: Kitty keyboard ---
		{
			name:  "Kitty keyboard protocol query",
			input: []byte("\x1b[?u"),
			want:  []byte{},
		},
		{
			name:  "Kitty keyboard with param is NOT a query",
			input: []byte("\x1b[?1u"),
			want:  []byte("\x1b[?1u"),
		},

		// --- OSC color queries ---
		{
			name:  "OSC 10 foreground query ST",
			input: []byte("\x1b]10;?\x1b\\"),
			want:  []byte{},
		},
		{
			name:  "OSC 10 foreground query BEL",
			input: []byte("\x1b]10;?\x07"),
			want:  []byte{},
		},
		{
			name:  "OSC 11 background query ST",
			input: []byte("\x1b]11;?\x1b\\"),
			want:  []byte{},
		},
		{
			name:  "OSC 12 cursor color query BEL",
			input: []byte("\x1b]12;?\x07"),
			want:  []byte{},
		},
		{
			name:  "OSC 10 set value is NOT a query",
			input: []byte("\x1b]10;rgb:ff/00/00\x1b\\"),
			want:  []byte("\x1b]10;rgb:ff/00/00\x1b\\"),
		},

		// --- DCS queries ---
		{
			name:  "DECRQSS query",
			input: []byte("\x1b" + "P$q m\x1b\\"),
			want:  []byte{},
		},
		{
			name:  "XTGETTCAP query",
			input: []byte("\x1b" + "P+q544e\x1b\\"),
			want:  []byte{},
		},
		{
			name:  "DCS with C1 ST terminator",
			input: []byte("\x1b" + "P$q m\x9c"),
			want:  []byte{},
		},

		// --- mixed / embedded ---
		{
			name:  "queries embedded in normal output",
			input: []byte("prompt$ \x1b[6n\x1b[cmore text"),
			want:  []byte("prompt$ more text"),
		},
		{
			name:  "preserves non-query escape sequences",
			input: []byte("\x1b[1;1H\x1b[2J\x1b[?1049h\x1b[31m"),
			want:  []byte("\x1b[1;1H\x1b[2J\x1b[?1049h\x1b[31m"),
		},
		{
			name:  "preserves cursor movement and colors alongside stripped queries",
			input: []byte("\x1b[1;1H\x1b[6n\x1b[31mhello\x1b[c"),
			want:  []byte("\x1b[1;1H\x1b[31mhello"),
		},
		{
			name:  "multiple queries in sequence",
			input: []byte("\x1b[6n\x1b[c\x1b[5n\x1b[>c"),
			want:  []byte{},
		},
		{
			name:  "preserves CSI sequences with similar prefixes",
			input: []byte("\x1b[6A\x1b[5B\x1b[0m"),
			want:  []byte("\x1b[6A\x1b[5B\x1b[0m"),
		},
		{
			name:  "does not strip DSR with non-query param",
			input: []byte("\x1b[1n\x1b[3n"),
			want:  []byte("\x1b[1n\x1b[3n"),
		},
		{
			name:  "does not strip DA with non-zero param",
			input: []byte("\x1b[1c"),
			want:  []byte("\x1b[1c"),
		},
		{
			name:  "mixed CSI OSC and DCS queries stripped together",
			input: []byte("A\x1b[6n\x1b]10;?\x07\x1b[>cB"),
			want:  []byte("AB"),
		},
		{
			name:  "all query types with leading zeros",
			input: []byte("\x1b[06n\x1b[05n\x1b[00c\x1b[>00c\x1b[=00c\x1b[?06n"),
			want:  []byte{},
		},
		{
			name:  "preserves mode-setting sequences that look like queries",
			input: []byte("\x1b[?1000h\x1b[?1006h\x1b[?25h"),
			want:  []byte("\x1b[?1000h\x1b[?1006h\x1b[?25h"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripReplayQueries(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("stripReplayQueries() = %q, want nil", got)
				}
				return
			}
			if string(got) != string(tt.want) {
				t.Errorf("stripReplayQueries() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchCSIQuery(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  int
	}{
		{"DSR 6n", []byte("\x1b[6n"), 4},
		{"DSR 06n", []byte("\x1b[06n"), 5},
		{"DSR 5n", []byte("\x1b[5n"), 4},
		{"DECXCPR ?6n", []byte("\x1b[?6n"), 5},
		{"DA1 bare", []byte("\x1b[c"), 3},
		{"DA1 0c", []byte("\x1b[0c"), 4},
		{"DA2 >c", []byte("\x1b[>c"), 4},
		{"DA3 =c", []byte("\x1b[=c"), 4},
		{"XTVERSION >q", []byte("\x1b[>q"), 4},
		{"Kitty ?u", []byte("\x1b[?u"), 4},
		{"cursor up not query", []byte("\x1b[6A"), 0},
		{"mode set not query", []byte("\x1b[?25h"), 0},
		{"DA with param 1 not query", []byte("\x1b[1c"), 0},
		{"DSR with param 3 not query", []byte("\x1b[3n"), 0},
		{"too short", []byte("\x1b["), 0},
		{"not CSI", []byte("\x1b]6n"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchCSIQuery(tt.input)
			if got != tt.want {
				t.Errorf("matchCSIQuery(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchOSCQuery(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  int
	}{
		{"OSC 10 ST", []byte("\x1b]10;?\x1b\\"), 8},
		{"OSC 10 BEL", []byte("\x1b]10;?\x07"), 7},
		{"OSC 11 ST", []byte("\x1b]11;?\x1b\\"), 8},
		{"OSC 12 BEL", []byte("\x1b]12;?\x07"), 7},
		{"OSC 10 set (not query)", []byte("\x1b]10;red\x07"), 0},
		{"OSC 4 (not color query)", []byte("\x1b]4;?\x07"), 0},
		{"OSC 20 (out of range)", []byte("\x1b]20;?\x07"), 0},
		{"no terminator", []byte("\x1b]10;?"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchOSCQuery(tt.input)
			if got != tt.want {
				t.Errorf("matchOSCQuery(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchDCSQuery(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  int
	}{
		{"DECRQSS", []byte("\x1bP$q m\x1b\\"), 8},
		{"XTGETTCAP", []byte("\x1bP+q544e\x1b\\"), 10},
		{"C1 ST", []byte("\x1bP$q m\x9c"), 7},
		{"not DCS", []byte("\x1b[c"), 0},
		{"unknown DCS prefix", []byte("\x1bP!q\x1b\\"), 0},
		{"no terminator", []byte("\x1bP$qfoo"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchDCSQuery(tt.input)
			if got != tt.want {
				t.Errorf("matchDCSQuery(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
