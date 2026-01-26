package termemu

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type ColorKind uint8

const (
	ColorDefault ColorKind = iota
	ColorIndexed
	ColorRGB
)

type Color struct {
	Kind  ColorKind
	Index uint8
	R     uint8
	G     uint8
	B     uint8
}

type Attr struct {
	Fg        Color
	Bg        Color
	Bold      bool
	Dim       bool
	Italic    bool
	Underline bool
	Inverse   bool
	Hidden    bool
	Strike    bool
}

type Cell struct {
	Ch   rune
	Attr Attr
}

type Row struct {
	Cells []Cell
}

type Cursor struct {
	Row int
	Col int
}

type Modes struct {
	Wrap          bool
	Origin        bool
	CursorVisible bool
	AltScreen     bool
}

type Charset uint8

const (
	charsetASCII Charset = iota
	charsetDEC
)

type Snapshot struct {
	Cols         int
	Rows         int
	Modes        Modes
	Cursor       Cursor
	SavedCursor  Cursor
	Attr         Attr
	SavedAttr    Attr
	SavedG0      Charset
	SavedG1      Charset
	SavedShifted bool
	ScrollTop    int
	ScrollBottom int
	Primary      []Row
	Alt          []Row
	AltActive    bool
	G0           Charset
	G1           Charset
	Shifted      bool
	TabStops     []bool
}

type Terminal struct {
	mu sync.Mutex

	cols int
	rows int

	primary []Row
	alt     []Row
	history []Row
	// historyMax is the maximum number of rows preserved in the primary screen history.
	// A value <= 0 disables history capture.
	historyMax int

	cursor       Cursor
	saved        Cursor
	savedAttr    Attr
	savedG0      Charset
	savedG1      Charset
	savedShifted bool
	attr         Attr
	modes        Modes
	g0           Charset
	g1           Charset
	shifted      bool
	tabStops     []bool
	lastRune     rune
	wrapNext     bool

	scrollTop    int
	scrollBottom int

	state               parseState
	escIntermediate     byte
	csiBuf              []byte
	oscBuf              []byte
	utf8Buf             []byte
	escStringPendingEsc bool

	respond func([]byte)
}

type parseState uint8

const (
	stateGround parseState = iota
	stateEscape
	stateEscapeCharset
	stateEscapeSharp
	stateCSI
	stateOSC
	stateEscapeString
)

var (
	traceOnce sync.Once
	traceOn   bool
	traceLog  *os.File
	traceMu   sync.Mutex
)

func traceEnabled() bool {
	traceOnce.Do(func() {
		traceOn = envTruthy(os.Getenv("WORKSET_TERMEMU_TRACE"))
		if !traceOn {
			return
		}
		home, err := os.UserHomeDir()
		if err != nil {
			traceOn = false
			return
		}
		logPath := filepath.Join(home, ".workset", "termemu_trace.log")
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			traceOn = false
			return
		}
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			traceOn = false
			return
		}
		traceLog = file
	})
	return traceOn && traceLog != nil
}

func tracef(format string, args ...any) {
	if !traceEnabled() {
		return
	}
	traceMu.Lock()
	defer traceMu.Unlock()
	_, _ = fmt.Fprintf(traceLog, time.Now().Format(time.RFC3339Nano)+" "+format+"\n", args...)
}

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func New(cols, rows int) *Terminal {
	t := &Terminal{
		cols: cols,
		rows: rows,
		modes: Modes{
			Wrap:          true,
			CursorVisible: true,
		},
		g0:           charsetASCII,
		g1:           charsetDEC,
		shifted:      false,
		savedG0:      charsetASCII,
		savedG1:      charsetDEC,
		savedShifted: false,
		tabStops:     defaultTabStops(cols),
		scrollTop:    0,
		scrollBottom: rows - 1,
		historyMax:   0,
	}
	t.primary = blankRows(rows, cols)
	t.alt = blankRows(rows, cols)
	return t
}

func (t *Terminal) SetResponder(fn func([]byte)) {
	t.mu.Lock()
	t.respond = fn
	t.mu.Unlock()
}

func (t *Terminal) Cursor() Cursor {
	t.mu.Lock()
	cursor := t.cursor
	t.mu.Unlock()
	return cursor
}

// SetHistoryLimit configures how many lines of primary-screen history to retain.
// A value <= 0 disables history capture.
func (t *Terminal) SetHistoryLimit(lines int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.historyMax = lines
	if t.historyMax <= 0 {
		t.history = nil
		return
	}
	if len(t.history) > t.historyMax {
		t.history = t.history[len(t.history)-t.historyMax:]
	}
}

// HistoryLen returns the number of retained history rows.
func (t *Terminal) HistoryLen() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.history)
}

// HistoryRows returns a copy of the history rows for testing/debugging.
func (t *Terminal) HistoryRows() []Row {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Row, len(t.history))
	for i, row := range t.history {
		out[i] = cloneRow(row)
	}
	return out
}

func (t *Terminal) Resize(cols, rows int) {
	if cols < 2 {
		cols = 2
	}
	if rows < 1 {
		rows = 1
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if cols == t.cols && rows == t.rows {
		return
	}
	t.cols = cols
	t.rows = rows
	t.primary = resizeRows(t.primary, rows, cols)
	t.alt = resizeRows(t.alt, rows, cols)
	if t.cursor.Row >= rows {
		t.cursor.Row = rows - 1
	}
	if t.cursor.Col >= cols {
		t.cursor.Col = cols - 1
	}
	t.scrollTop = 0
	t.scrollBottom = rows - 1
	t.tabStops = resizeTabStops(t.tabStops, cols)
	t.wrapNext = false
	if len(t.history) > 0 {
		t.history = nil
	}
}

func (t *Terminal) Write(data []byte) {
	if len(data) == 0 {
		return
	}
	var responses [][]byte
	t.mu.Lock()
	for len(data) > 0 {
		b := data[0]
		data = data[1:]
		switch t.state {
		case stateGround:
			if b == 0x1b {
				t.state = stateEscape
				continue
			}
			if b < 0x20 {
				t.handleControl(b)
				continue
			}
			if len(t.utf8Buf) == 0 {
				switch b {
				case 0x9b:
					t.state = stateCSI
					t.csiBuf = t.csiBuf[:0]
					continue
				case 0x9d:
					t.state = stateOSC
					t.oscBuf = t.oscBuf[:0]
					continue
				case 0x90, 0x98, 0x9e, 0x9f:
					t.state = stateEscapeString
					t.escStringPendingEsc = false
					continue
				}
			}
			t.utf8Buf = append(t.utf8Buf, b)
			for len(t.utf8Buf) > 0 {
				if !utf8.FullRune(t.utf8Buf) {
					break
				}
				r, size := utf8.DecodeRune(t.utf8Buf)
				if r == utf8.RuneError && size == 1 {
					t.utf8Buf = t.utf8Buf[1:]
					continue
				}
				t.utf8Buf = t.utf8Buf[size:]
				t.putRune(r)
			}
		case stateEscape:
			switch b {
			case '[':
				t.state = stateCSI
				t.csiBuf = t.csiBuf[:0]
			case ']':
				t.state = stateOSC
				t.oscBuf = t.oscBuf[:0]
			case 'P', 'X', '^', '_':
				t.state = stateEscapeString
				t.escStringPendingEsc = false
			case '7':
				t.saveCursor()
				t.state = stateGround
			case '8':
				t.restoreCursor()
				t.state = stateGround
			case 'D':
				t.index()
				t.state = stateGround
			case 'M':
				t.reverseIndex()
				t.state = stateGround
			case 'E':
				t.cursor.Col = 0
				t.index()
				t.state = stateGround
			case 'H':
				t.setTabStop(t.cursor.Col)
				t.state = stateGround
			case '(':
				t.escIntermediate = b
				t.state = stateEscapeCharset
			case ')':
				t.escIntermediate = b
				t.state = stateEscapeCharset
			case '#':
				t.state = stateEscapeSharp
			case 'c':
				t.reset()
				t.state = stateGround
			case '=':
				t.state = stateGround
			case '>':
				t.state = stateGround
			default:
				tracef("ESC b=0x%02x char=%q", b, b)
				t.state = stateGround
			}
		case stateEscapeCharset:
			t.designateCharset(t.escIntermediate, b)
			t.state = stateGround
		case stateEscapeSharp:
			if b == '8' {
				t.alignScreen()
			}
			t.state = stateGround
		case stateCSI:
			t.csiBuf = append(t.csiBuf, b)
			if b >= 0x40 && b <= 0x7e {
				t.handleCSI(t.csiBuf, &responses)
				t.csiBuf = t.csiBuf[:0]
				t.state = stateGround
			}
		case stateOSC:
			if b == 0x07 || b == 0x9c {
				t.state = stateGround
				t.oscBuf = t.oscBuf[:0]
				continue
			}
			if b == 0x1b {
				if len(data) > 0 && data[0] == '\\' {
					data = data[1:]
					t.state = stateGround
					t.oscBuf = t.oscBuf[:0]
					continue
				}
			}
			t.oscBuf = append(t.oscBuf, b)
		case stateEscapeString:
			if t.escStringPendingEsc {
				if b == '\\' {
					t.state = stateGround
					t.escStringPendingEsc = false
					continue
				}
				t.escStringPendingEsc = false
			}
			if b == 0x07 || b == 0x9c {
				t.state = stateGround
				continue
			}
			if b == 0x1b {
				t.escStringPendingEsc = true
				continue
			}
		}
	}
	responder := t.respond
	t.mu.Unlock()
	if responder != nil {
		for _, resp := range responses {
			if len(resp) > 0 {
				responder(resp)
			}
		}
	}
}

func (t *Terminal) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return Snapshot{
		Cols:         t.cols,
		Rows:         t.rows,
		Modes:        t.modes,
		Cursor:       t.cursor,
		SavedCursor:  t.saved,
		Attr:         t.attr,
		SavedAttr:    t.savedAttr,
		SavedG0:      t.savedG0,
		SavedG1:      t.savedG1,
		SavedShifted: t.savedShifted,
		ScrollTop:    t.scrollTop,
		ScrollBottom: t.scrollBottom,
		Primary:      cloneRows(t.primary),
		Alt:          cloneRows(t.alt),
		AltActive:    t.modes.AltScreen,
		G0:           t.g0,
		G1:           t.g1,
		Shifted:      t.shifted,
		TabStops:     cloneTabStops(t.tabStops),
	}
}

func (t *Terminal) Restore(snapshot Snapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cols = snapshot.Cols
	t.rows = snapshot.Rows
	t.modes = snapshot.Modes
	t.cursor = snapshot.Cursor
	t.saved = snapshot.SavedCursor
	t.attr = snapshot.Attr
	t.savedAttr = snapshot.SavedAttr
	t.savedG0 = snapshot.SavedG0
	t.savedG1 = snapshot.SavedG1
	t.savedShifted = snapshot.SavedShifted
	t.scrollTop = snapshot.ScrollTop
	t.scrollBottom = snapshot.ScrollBottom
	t.primary = cloneRows(snapshot.Primary)
	t.alt = cloneRows(snapshot.Alt)
	t.modes.AltScreen = snapshot.AltActive
	t.g0 = snapshot.G0
	t.g1 = snapshot.G1
	t.shifted = snapshot.Shifted
	if snapshot.TabStops != nil {
		t.tabStops = cloneTabStops(snapshot.TabStops)
	} else {
		t.tabStops = defaultTabStops(t.cols)
		if snapshot.G1 == 0 {
			t.g1 = charsetDEC
		}
		if snapshot.SavedG1 == 0 {
			t.savedG1 = charsetDEC
		}
	}
}

func (t *Terminal) SnapshotANSI() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snapshotANSILocked()
}

// SnapshotANSIWithHistory returns an ANSI snapshot that preserves primary-screen history.
// It is only emitted when the terminal is in the primary screen and history exists.
func (t *Terminal) SnapshotANSIWithHistory() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.modes.AltScreen || len(t.history) == 0 {
		return t.snapshotANSILocked()
	}
	var b strings.Builder
	for _, row := range t.history {
		writeRowANSI(&b, row, t.cols)
		b.WriteString("\x1b[0m\r\n")
	}
	b.WriteString(t.snapshotANSILocked())
	return b.String()
}

func (t *Terminal) snapshotANSILocked() string {
	screen := t.active()
	var b strings.Builder
	if t.modes.AltScreen {
		b.WriteString("\x1b[?1049h")
	} else {
		b.WriteString("\x1b[?1049l")
	}
	b.WriteString("\x1b[2J\x1b[H")
	current := Attr{}
	for r := 0; r < t.rows; r++ {
		b.WriteString(fmt.Sprintf("\x1b[%d;1H", r+1))
		row := screen[r]
		for c := 0; c < t.cols; c++ {
			cell := row.Cells[c]
			if cell.Attr != current {
				b.WriteString(sgrForAttr(cell.Attr))
				current = cell.Attr
			}
			if cell.Ch == 0 {
				b.WriteByte(' ')
			} else {
				b.WriteRune(cell.Ch)
			}
		}
	}
	b.WriteString("\x1b[0m")
	if t.modes.CursorVisible {
		b.WriteString("\x1b[?25h")
	} else {
		b.WriteString("\x1b[?25l")
	}
	b.WriteString(fmt.Sprintf("\x1b[%d;%dH", t.cursor.Row+1, t.cursor.Col+1))
	return b.String()
}

func (t *Terminal) IsAltScreen() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.modes.AltScreen
}

func (t *Terminal) MarshalBinary() ([]byte, error) {
	snap := t.Snapshot()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(snap); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (t *Terminal) UnmarshalBinary(data []byte) error {
	var snap Snapshot
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&snap); err != nil {
		return err
	}
	t.Restore(snap)
	return nil
}

func (t *Terminal) handleControl(b byte) {
	if b != 0x07 {
		t.wrapNext = false
	}
	switch b {
	case 0x07: // BEL
	case 0x08: // BS
		if t.cursor.Col > 0 {
			t.cursor.Col--
		}
	case 0x09: // TAB
		t.advanceTab(1)
	case 0x0a, 0x0b, 0x0c: // LF, VT, FF
		t.index()
	case 0x0d: // CR
		t.cursor.Col = 0
	case 0x0e: // SO
		t.shifted = true
	case 0x0f: // SI
		t.shifted = false
	default:
		tracef("CTRL b=0x%02x", b)
	}
}

func (t *Terminal) handleCSI(seq []byte, responses *[][]byte) {
	if len(seq) == 0 {
		return
	}
	final := seq[len(seq)-1]
	switch final {
	case 'm', 'b', 'n', 'c', 'p', 't':
	default:
		t.wrapNext = false
	}
	params, priv := parseCSIParams(seq[:len(seq)-1])
	switch final {
	case 'A':
		t.cursor.Row -= max(1, param(params, 0, 1))
		t.clampCursor()
	case 'B':
		t.cursor.Row += max(1, param(params, 0, 1))
		t.clampCursor()
	case 'C':
		t.cursor.Col += max(1, param(params, 0, 1))
		t.clampCursor()
	case 'D':
		t.cursor.Col -= max(1, param(params, 0, 1))
		t.clampCursor()
	case 'E':
		t.cursor.Row += max(1, param(params, 0, 1))
		t.cursor.Col = 0
		t.clampCursor()
	case 'F':
		t.cursor.Row -= max(1, param(params, 0, 1))
		t.cursor.Col = 0
		t.clampCursor()
	case 'G':
		t.cursor.Col = clamp(param(params, 0, 1)-1, 0, t.cols-1)
		t.clampCursor()
	case 'd':
		row := param(params, 0, 1) - 1
		if t.modes.Origin {
			row = t.scrollTop + row
		}
		t.cursor.Row = row
		t.clampCursor()
	case 'H', 'f':
		row := param(params, 0, 1)
		col := param(params, 1, 1)
		targetRow := row - 1
		if t.modes.Origin {
			targetRow = t.scrollTop + row - 1
		}
		t.cursor.Row = targetRow
		t.cursor.Col = col - 1
		t.clampCursor()
	case 'J':
		mode := param(params, 0, 0)
		t.eraseInDisplay(mode)
	case 'K':
		mode := param(params, 0, 0)
		t.eraseInLine(mode)
	case 'L':
		n := max(1, param(params, 0, 1))
		t.insertLines(n)
	case 'M':
		n := max(1, param(params, 0, 1))
		t.deleteLines(n)
	case 'P':
		n := max(1, param(params, 0, 1))
		t.deleteChars(n)
	case '@':
		n := max(1, param(params, 0, 1))
		t.insertChars(n)
	case 'X':
		n := max(1, param(params, 0, 1))
		t.eraseChars(n)
	case 'b':
		n := max(1, param(params, 0, 1))
		t.repeatLast(n)
	case 'I':
		n := max(1, param(params, 0, 1))
		t.advanceTab(n)
	case 'Z':
		n := max(1, param(params, 0, 1))
		t.backTab(n)
	case 'g':
		t.clearTabStop(param(params, 0, 0))
	case 'S':
		n := max(1, param(params, 0, 1))
		t.scrollUp(n)
	case 'T':
		n := max(1, param(params, 0, 1))
		t.scrollDown(n)
	case 'r':
		top := param(params, 0, 1) - 1
		bottom := param(params, 1, t.rows) - 1
		if top < 0 {
			top = 0
		}
		if bottom >= t.rows {
			bottom = t.rows - 1
		}
		if bottom <= top {
			top = 0
			bottom = t.rows - 1
		}
		t.scrollTop = top
		t.scrollBottom = bottom
		if t.modes.Origin {
			t.cursor.Row = t.scrollTop
		} else {
			t.cursor.Row = 0
		}
		t.cursor.Col = 0
		t.clampCursor()
	case 'm':
		t.applySGR(params)
	case 's':
		t.saveCursor()
	case 'u':
		t.restoreCursor()
	case 'h':
		t.setMode(priv, params, true)
	case 'l':
		t.setMode(priv, params, false)
	case 'n':
		t.handleDSR(priv, params, responses)
	case 'c':
		t.handleDA(priv, responses)
	case 't':
		t.handleWindowReport(params, responses)
	case 'p':
		t.handleDECRQM(priv, params, seq, responses)
	default:
		tracef("CSI priv=%q params=%v final=%q raw=%x", priv, params, final, seq)
	}
}

func (t *Terminal) setMode(priv byte, params []int, on bool) {
	if priv != '?' {
		return
	}
	for _, p := range params {
		switch p {
		case 25:
			t.modes.CursorVisible = on
		case 7:
			t.modes.Wrap = on
		case 6:
			t.modes.Origin = on
			t.wrapNext = false
			if on {
				t.cursor.Row = t.scrollTop
			} else {
				t.cursor.Row = 0
			}
			t.cursor.Col = 0
			t.clampCursor()
		case 47, 1047:
			if on {
				t.enterAlt(false)
			} else {
				t.exitAlt(false)
			}
		case 1049:
			if on {
				t.enterAlt(true)
			} else {
				t.exitAlt(true)
			}
		}
	}
}

func (t *Terminal) handleDSR(priv byte, params []int, responses *[][]byte) {
	if priv != 0 {
		return
	}
	switch param(params, 0, 0) {
	case 5:
		queueResponse(responses, "\x1b[0n")
	case 6:
		row := t.cursor.Row + 1
		col := t.cursor.Col + 1
		if t.modes.Origin {
			row = (t.cursor.Row - t.scrollTop) + 1
			if row < 1 {
				row = 1
			}
		}
		queueResponse(responses, fmt.Sprintf("\x1b[%d;%dR", row, col))
	}
}

func (t *Terminal) handleDA(priv byte, responses *[][]byte) {
	if priv == '>' {
		queueResponse(responses, "\x1b[>0;0;0c")
		return
	}
	queueResponse(responses, "\x1b[?1;2c")
}

func (t *Terminal) handleWindowReport(params []int, responses *[][]byte) {
	switch param(params, 0, 0) {
	case 14:
		queueResponse(responses, fmt.Sprintf("\x1b[4;%d;%dt", t.rows, t.cols))
	case 18:
		queueResponse(responses, fmt.Sprintf("\x1b[8;%d;%dt", t.rows, t.cols))
	}
}

func (t *Terminal) handleDECRQM(priv byte, params []int, seq []byte, responses *[][]byte) {
	if priv != '?' {
		return
	}
	if !bytes.Contains(seq, []byte("$")) {
		return
	}
	for _, p := range params {
		state := t.modeState(p)
		queueResponse(responses, fmt.Sprintf("\x1b[?%d;%d$y", p, state))
	}
}

func (t *Terminal) modeState(p int) int {
	switch p {
	case 6:
		if t.modes.Origin {
			return 1
		}
		return 2
	case 7:
		if t.modes.Wrap {
			return 1
		}
		return 2
	case 25:
		if t.modes.CursorVisible {
			return 1
		}
		return 2
	case 47, 1047, 1049:
		if t.modes.AltScreen {
			return 1
		}
		return 2
	default:
		return 0
	}
}

func queueResponse(responses *[][]byte, value string) {
	if responses == nil || value == "" {
		return
	}
	*responses = append(*responses, []byte(value))
}

func (t *Terminal) applySGR(params []int) {
	if len(params) == 0 {
		t.attr = Attr{}
		return
	}
	for i := 0; i < len(params); i++ {
		p := params[i]
		switch {
		case p == 0:
			t.attr = Attr{}
		case p == 1:
			t.attr.Bold = true
		case p == 2:
			t.attr.Dim = true
		case p == 3:
			t.attr.Italic = true
		case p == 4:
			t.attr.Underline = true
		case p == 7:
			t.attr.Inverse = true
		case p == 22:
			t.attr.Bold = false
			t.attr.Dim = false
		case p == 23:
			t.attr.Italic = false
		case p == 24:
			t.attr.Underline = false
		case p == 27:
			t.attr.Inverse = false
		case p == 39:
			t.attr.Fg = Color{}
		case p == 49:
			t.attr.Bg = Color{}
		case p >= 30 && p <= 37:
			t.attr.Fg = Color{Kind: ColorIndexed, Index: uint8(p - 30)}
		case p >= 40 && p <= 47:
			t.attr.Bg = Color{Kind: ColorIndexed, Index: uint8(p - 40)}
		case p >= 90 && p <= 97:
			t.attr.Fg = Color{Kind: ColorIndexed, Index: uint8(p - 90 + 8)}
		case p >= 100 && p <= 107:
			t.attr.Bg = Color{Kind: ColorIndexed, Index: uint8(p - 100 + 8)}
		case p == 38 || p == 48:
			if i+1 >= len(params) {
				continue
			}
			mode := params[i+1]
			if mode == 5 && i+2 < len(params) {
				color := Color{Kind: ColorIndexed, Index: uint8(params[i+2])}
				if p == 38 {
					t.attr.Fg = color
				} else {
					t.attr.Bg = color
				}
				i += 2
			} else if mode == 2 && i+4 < len(params) {
				color := Color{
					Kind: ColorRGB,
					R:    uint8(params[i+2]),
					G:    uint8(params[i+3]),
					B:    uint8(params[i+4]),
				}
				if p == 38 {
					t.attr.Fg = color
				} else {
					t.attr.Bg = color
				}
				i += 4
			}
		}
	}
}

func (t *Terminal) putRune(r rune) {
	if r == 0 {
		return
	}
	if t.wrapNext && t.modes.Wrap {
		t.cursor.Col = 0
		t.index()
		t.wrapNext = false
	}
	r = t.mapRune(r)
	screen := t.active()
	if t.cursor.Row < 0 || t.cursor.Row >= t.rows {
		return
	}
	if t.cursor.Col < 0 || t.cursor.Col >= t.cols {
		return
	}
	screen[t.cursor.Row].Cells[t.cursor.Col] = Cell{Ch: r, Attr: t.attr}
	t.lastRune = r
	if t.cursor.Col == t.cols-1 {
		if t.modes.Wrap {
			t.wrapNext = true
		}
		return
	}
	t.cursor.Col++
}

func (t *Terminal) active() []Row {
	if t.modes.AltScreen {
		return t.alt
	}
	return t.primary
}

func (t *Terminal) saveCursor() {
	t.saved = t.cursor
	t.savedAttr = t.attr
	t.savedG0 = t.g0
	t.savedG1 = t.g1
	t.savedShifted = t.shifted
}

func (t *Terminal) restoreCursor() {
	t.cursor = t.saved
	t.attr = t.savedAttr
	t.g0 = t.savedG0
	t.g1 = t.savedG1
	t.shifted = t.savedShifted
	t.wrapNext = false
	t.clampCursor()
}

func (t *Terminal) enterAlt(save bool) {
	if save {
		t.saveCursor()
	}
	t.modes.AltScreen = true
	t.clearScreen(t.alt)
	t.cursor = Cursor{}
	t.wrapNext = false
	t.clampCursor()
}

func (t *Terminal) exitAlt(restore bool) {
	t.modes.AltScreen = false
	if restore {
		t.restoreCursor()
		return
	}
	t.cursor = Cursor{}
	t.wrapNext = false
	t.clampCursor()
}

func (t *Terminal) reset() {
	t.modes = Modes{
		Wrap:          true,
		CursorVisible: true,
	}
	t.attr = Attr{}
	t.cursor = Cursor{}
	t.saved = Cursor{}
	t.savedAttr = Attr{}
	t.g0 = charsetASCII
	t.g1 = charsetDEC
	t.shifted = false
	t.savedG0 = t.g0
	t.savedG1 = t.g1
	t.savedShifted = t.shifted
	t.scrollTop = 0
	t.scrollBottom = t.rows - 1
	t.tabStops = defaultTabStops(t.cols)
	t.clearScreen(t.primary)
	t.clearScreen(t.alt)
	t.modes.AltScreen = false
	t.lastRune = 0
	t.wrapNext = false
	t.state = stateGround
	t.csiBuf = t.csiBuf[:0]
	t.oscBuf = t.oscBuf[:0]
	t.utf8Buf = t.utf8Buf[:0]
	t.escStringPendingEsc = false
}

func (t *Terminal) designateCharset(target byte, final byte) {
	var cs Charset
	switch final {
	case '0':
		cs = charsetDEC
	case 'B':
		cs = charsetASCII
	default:
		return
	}
	switch target {
	case '(':
		t.g0 = cs
	case ')':
		t.g1 = cs
	}
}

func (t *Terminal) alignScreen() {
	screen := t.active()
	for r := 0; r < t.rows; r++ {
		for c := 0; c < t.cols; c++ {
			screen[r].Cells[c] = Cell{Ch: 'E', Attr: t.attr}
		}
	}
	t.cursor = Cursor{}
	t.clampCursor()
}

func (t *Terminal) activeCharset() Charset {
	if t.shifted {
		return t.g1
	}
	return t.g0
}

func (t *Terminal) mapRune(r rune) rune {
	if r < 0x20 || r > 0x7e {
		return r
	}
	if t.activeCharset() != charsetDEC {
		return r
	}
	if mapped, ok := decSpecials[r]; ok {
		return mapped
	}
	return r
}

func (t *Terminal) setTabStop(col int) {
	if col < 0 || col >= t.cols {
		return
	}
	if len(t.tabStops) != t.cols {
		t.tabStops = resizeTabStops(t.tabStops, t.cols)
	}
	t.tabStops[col] = true
}

func (t *Terminal) clearTabStop(mode int) {
	switch mode {
	case 0:
		if t.cursor.Col >= 0 && t.cursor.Col < len(t.tabStops) {
			t.tabStops[t.cursor.Col] = false
		}
	case 3:
		for i := range t.tabStops {
			t.tabStops[i] = false
		}
	}
}

func (t *Terminal) advanceTab(n int) {
	if n <= 0 {
		return
	}
	for i := 0; i < n; i++ {
		t.cursor.Col = t.nextTabStop(t.cursor.Col)
	}
}

func (t *Terminal) backTab(n int) {
	if n <= 0 {
		return
	}
	for i := 0; i < n; i++ {
		t.cursor.Col = t.prevTabStop(t.cursor.Col)
	}
}

func (t *Terminal) nextTabStop(col int) int {
	for i := col + 1; i < t.cols; i++ {
		if i < len(t.tabStops) && t.tabStops[i] {
			return i
		}
	}
	if t.cols > 0 {
		return t.cols - 1
	}
	return 0
}

func (t *Terminal) prevTabStop(col int) int {
	for i := col - 1; i >= 0; i-- {
		if i < len(t.tabStops) && t.tabStops[i] {
			return i
		}
	}
	return 0
}

func (t *Terminal) eraseChars(n int) {
	if n <= 0 {
		return
	}
	if t.cursor.Row < 0 || t.cursor.Row >= t.rows {
		return
	}
	if t.cursor.Col < 0 || t.cursor.Col >= t.cols {
		return
	}
	row := t.active()[t.cursor.Row]
	for i := 0; i < n; i++ {
		col := t.cursor.Col + i
		if col >= t.cols {
			break
		}
		row.Cells[col] = Cell{Ch: ' ', Attr: t.attr}
	}
}

func (t *Terminal) repeatLast(n int) {
	if n <= 0 || t.lastRune == 0 {
		return
	}
	for i := 0; i < n; i++ {
		t.putRune(t.lastRune)
	}
}

func (t *Terminal) clampCursor() {
	if t.cursor.Col < 0 {
		t.cursor.Col = 0
	}
	if t.cursor.Col >= t.cols {
		t.cursor.Col = t.cols - 1
	}
	minRow := 0
	maxRow := t.rows - 1
	if t.modes.Origin {
		minRow = t.scrollTop
		maxRow = t.scrollBottom
	}
	if t.cursor.Row < minRow {
		t.cursor.Row = minRow
	}
	if t.cursor.Row > maxRow {
		t.cursor.Row = maxRow
	}
}

func (t *Terminal) index() {
	if t.cursor.Row == t.scrollBottom {
		t.scrollUp(1)
		return
	}
	if t.cursor.Row < t.rows-1 {
		t.cursor.Row++
	}
}

func (t *Terminal) reverseIndex() {
	if t.cursor.Row == t.scrollTop {
		t.scrollDown(1)
		return
	}
	if t.cursor.Row > 0 {
		t.cursor.Row--
	}
}

func (t *Terminal) eraseInDisplay(mode int) {
	screen := t.active()
	switch mode {
	case 0:
		t.eraseInLine(0)
		for r := t.cursor.Row + 1; r < t.rows; r++ {
			clearRow(screen[r], t.attr)
		}
	case 1:
		for r := 0; r < t.cursor.Row; r++ {
			clearRow(screen[r], t.attr)
		}
		t.eraseInLine(1)
	case 2, 3:
		if mode == 2 {
			t.appendHistoryScreen()
		}
		for r := 0; r < t.rows; r++ {
			clearRow(screen[r], t.attr)
		}
		if mode == 3 && !t.modes.AltScreen && len(t.history) > 0 {
			t.history = nil
		}
	}
}

func (t *Terminal) eraseInLine(mode int) {
	screen := t.active()
	row := screen[t.cursor.Row]
	switch mode {
	case 0:
		for c := t.cursor.Col; c < t.cols; c++ {
			row.Cells[c] = Cell{Ch: ' ', Attr: t.attr}
		}
	case 1:
		for c := 0; c <= t.cursor.Col && c < t.cols; c++ {
			row.Cells[c] = Cell{Ch: ' ', Attr: t.attr}
		}
	case 2:
		clearRow(row, t.attr)
	}
}

func (t *Terminal) insertLines(n int) {
	if t.cursor.Row < t.scrollTop || t.cursor.Row > t.scrollBottom {
		return
	}
	screen := t.active()
	for i := 0; i < n; i++ {
		for r := t.scrollBottom; r > t.cursor.Row; r-- {
			screen[r] = screen[r-1]
		}
		screen[t.cursor.Row] = blankRow(t.cols)
	}
}

func (t *Terminal) deleteLines(n int) {
	if t.cursor.Row < t.scrollTop || t.cursor.Row > t.scrollBottom {
		return
	}
	screen := t.active()
	for i := 0; i < n; i++ {
		for r := t.cursor.Row; r < t.scrollBottom; r++ {
			screen[r] = screen[r+1]
		}
		screen[t.scrollBottom] = blankRow(t.cols)
	}
}

func (t *Terminal) deleteChars(n int) {
	screen := t.active()
	row := screen[t.cursor.Row]
	if n <= 0 {
		return
	}
	if t.cursor.Col >= t.cols {
		return
	}
	for c := t.cursor.Col; c < t.cols; c++ {
		src := c + n
		if src < t.cols {
			row.Cells[c] = row.Cells[src]
		} else {
			row.Cells[c] = Cell{Ch: ' ', Attr: t.attr}
		}
	}
}

func (t *Terminal) insertChars(n int) {
	screen := t.active()
	row := screen[t.cursor.Row]
	if n <= 0 {
		return
	}
	for c := t.cols - 1; c >= t.cursor.Col; c-- {
		src := c - n
		if src >= t.cursor.Col {
			row.Cells[c] = row.Cells[src]
		} else {
			row.Cells[c] = Cell{Ch: ' ', Attr: t.attr}
		}
	}
}

func (t *Terminal) scrollUp(n int) {
	if n <= 0 {
		return
	}
	screen := t.active()
	captureHistory := !t.modes.AltScreen && t.historyMax > 0 && t.scrollTop == 0
	for i := 0; i < n; i++ {
		if captureHistory {
			t.appendHistoryRow(screen[t.scrollTop])
		}
		for r := t.scrollTop; r < t.scrollBottom; r++ {
			screen[r] = screen[r+1]
		}
		screen[t.scrollBottom] = blankRow(t.cols)
	}
}

func (t *Terminal) scrollDown(n int) {
	if n <= 0 {
		return
	}
	screen := t.active()
	for i := 0; i < n; i++ {
		for r := t.scrollBottom; r > t.scrollTop; r-- {
			screen[r] = screen[r-1]
		}
		screen[t.scrollTop] = blankRow(t.cols)
	}
}

func (t *Terminal) clearScreen(rows []Row) {
	for i := range rows {
		clearRow(rows[i], Attr{})
	}
}

func (t *Terminal) appendHistoryRow(row Row) {
	if t.historyMax <= 0 {
		return
	}
	t.history = append(t.history, cloneRow(row))
	if len(t.history) > t.historyMax {
		t.history = t.history[len(t.history)-t.historyMax:]
	}
}

func (t *Terminal) appendHistoryScreen() {
	if t.historyMax <= 0 || t.modes.AltScreen {
		return
	}
	screen := t.active()
	last := -1
	for i := len(screen) - 1; i >= 0; i-- {
		if rowHasContent(screen[i]) {
			last = i
			break
		}
	}
	if last < 0 {
		return
	}
	for i := 0; i <= last; i++ {
		t.appendHistoryRow(screen[i])
	}
}

func cloneRow(row Row) Row {
	if len(row.Cells) == 0 {
		return Row{}
	}
	out := Row{Cells: make([]Cell, len(row.Cells))}
	copy(out.Cells, row.Cells)
	return out
}

func writeRowANSI(b *strings.Builder, row Row, cols int) {
	if cols <= 0 {
		return
	}
	if len(row.Cells) < cols {
		cols = len(row.Cells)
	}
	lastNonSpace := -1
	for i := cols - 1; i >= 0; i-- {
		cell := row.Cells[i]
		ch := cell.Ch
		if ch == 0 {
			ch = ' '
		}
		if ch != ' ' {
			lastNonSpace = i
			break
		}
	}
	if lastNonSpace < 0 {
		return
	}
	current := Attr{}
	for i := 0; i <= lastNonSpace; i++ {
		cell := row.Cells[i]
		if cell.Attr != current {
			b.WriteString(sgrForAttr(cell.Attr))
			current = cell.Attr
		}
		ch := cell.Ch
		if ch == 0 {
			ch = ' '
		}
		b.WriteRune(ch)
	}
}

func rowHasContent(row Row) bool {
	for _, cell := range row.Cells {
		ch := cell.Ch
		if ch == 0 {
			ch = ' '
		}
		if ch != ' ' {
			return true
		}
	}
	return false
}

func blankRow(cols int) Row {
	row := Row{Cells: make([]Cell, cols)}
	for i := range row.Cells {
		row.Cells[i] = Cell{Ch: ' ', Attr: Attr{}}
	}
	return row
}

func blankRows(rows, cols int) []Row {
	out := make([]Row, rows)
	for i := range out {
		out[i] = blankRow(cols)
	}
	return out
}

func resizeRows(rows []Row, newRows, cols int) []Row {
	out := make([]Row, newRows)
	for i := 0; i < newRows; i++ {
		if i < len(rows) {
			row := rows[i]
			if len(row.Cells) != cols {
				row.Cells = resizeCells(row.Cells, cols)
			}
			out[i] = row
		} else {
			out[i] = blankRow(cols)
		}
	}
	return out
}

func resizeCells(cells []Cell, cols int) []Cell {
	if len(cells) == cols {
		return cells
	}
	out := make([]Cell, cols)
	for i := 0; i < cols; i++ {
		if i < len(cells) {
			out[i] = cells[i]
		} else {
			out[i] = Cell{Ch: ' ', Attr: Attr{}}
		}
	}
	return out
}

func clearRow(row Row, attr Attr) {
	for i := range row.Cells {
		row.Cells[i] = Cell{Ch: ' ', Attr: attr}
	}
}

func cloneRows(rows []Row) []Row {
	out := make([]Row, len(rows))
	for i := range rows {
		row := rows[i]
		cells := make([]Cell, len(row.Cells))
		copy(cells, row.Cells)
		out[i] = Row{Cells: cells}
	}
	return out
}

var decSpecials = map[rune]rune{
	'a': '▒',
	'b': '␉',
	'c': '␌',
	'd': '␍',
	'e': '␊',
	'f': '°',
	'g': '±',
	'h': '␤',
	'i': '␋',
	'j': '┘',
	'k': '┐',
	'l': '┌',
	'm': '└',
	'n': '┼',
	'o': '⎺',
	'p': '⎻',
	'q': '─',
	'r': '⎼',
	's': '⎽',
	't': '├',
	'u': '┤',
	'v': '┴',
	'w': '┬',
	'x': '│',
	'y': '≤',
	'z': '≥',
	'{': 'π',
	'|': '≠',
	'}': '£',
	'~': '·',
}

func defaultTabStops(cols int) []bool {
	stops := make([]bool, cols)
	for i := 0; i < cols; i++ {
		if i > 0 && i%8 == 0 {
			stops[i] = true
		}
	}
	return stops
}

func resizeTabStops(existing []bool, cols int) []bool {
	stops := make([]bool, cols)
	copy(stops, existing)
	for i := len(existing); i < cols; i++ {
		if i > 0 && i%8 == 0 {
			stops[i] = true
		}
	}
	return stops
}

func cloneTabStops(stops []bool) []bool {
	if stops == nil {
		return nil
	}
	out := make([]bool, len(stops))
	copy(out, stops)
	return out
}

func parseCSIParams(buf []byte) ([]int, byte) {
	if len(buf) == 0 {
		return nil, 0
	}
	priv := byte(0)
	if buf[0] == '?' || buf[0] == '>' {
		priv = buf[0]
		buf = buf[1:]
	}
	if len(buf) == 0 {
		return nil, priv
	}
	parts := bytes.Split(buf, []byte(";"))
	params := make([]int, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			params = append(params, 0)
			continue
		}
		val := 0
		for _, b := range part {
			if b < '0' || b > '9' {
				continue
			}
			val = val*10 + int(b-'0')
		}
		params = append(params, val)
	}
	return params, priv
}

func sgrForAttr(attr Attr) string {
	codes := []string{"0"}
	if attr.Bold {
		codes = append(codes, "1")
	}
	if attr.Dim {
		codes = append(codes, "2")
	}
	if attr.Italic {
		codes = append(codes, "3")
	}
	if attr.Underline {
		codes = append(codes, "4")
	}
	if attr.Inverse {
		codes = append(codes, "7")
	}
	if attr.Hidden {
		codes = append(codes, "8")
	}
	if attr.Strike {
		codes = append(codes, "9")
	}
	codes = append(codes, colorToSGR(attr.Fg, true)...)
	codes = append(codes, colorToSGR(attr.Bg, false)...)
	return "\x1b[" + strings.Join(codes, ";") + "m"
}

func colorToSGR(c Color, fg bool) []string {
	switch c.Kind {
	case ColorDefault:
		if fg {
			return []string{"39"}
		}
		return []string{"49"}
	case ColorIndexed:
		if c.Index < 16 {
			base := 30
			if !fg {
				base = 40
			}
			if c.Index >= 8 {
				base += 60
				return []string{fmt.Sprintf("%d", base+int(c.Index-8))}
			}
			return []string{fmt.Sprintf("%d", base+int(c.Index))}
		}
		if fg {
			return []string{fmt.Sprintf("38;5;%d", c.Index)}
		}
		return []string{fmt.Sprintf("48;5;%d", c.Index)}
	case ColorRGB:
		if fg {
			return []string{fmt.Sprintf("38;2;%d;%d;%d", c.R, c.G, c.B)}
		}
		return []string{fmt.Sprintf("48;2;%d;%d;%d", c.R, c.G, c.B)}
	default:
		if fg {
			return []string{"39"}
		}
		return []string{"49"}
	}
}

func param(params []int, idx int, fallback int) int {
	if idx < len(params) {
		if params[idx] == 0 {
			return fallback
		}
		return params[idx]
	}
	return fallback
}

func clamp(value, minVal, maxVal int) int {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
