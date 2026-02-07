package termemu

import (
	"context"
	"sync"
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

func (t *Terminal) Write(ctx context.Context, data []byte) {
	if len(data) == 0 {
		return
	}
	var responses [][]byte
	t.mu.Lock()
	for len(data) > 0 {
		b := data[0]
		data = data[1:]
		t.parseByte(ctx, b, &data, &responses)
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

func (t *Terminal) IsAltScreen() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.modes.AltScreen
}
