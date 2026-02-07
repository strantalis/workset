package termemu

import (
	"bytes"
	"context"
	"fmt"
)

func (t *Terminal) handleControl(ctx context.Context, b byte) {
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
		tracef(ctx, "CTRL b=0x%02x", b)
	}
}

func (t *Terminal) handleCSI(ctx context.Context, seq []byte, responses *[][]byte) {
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
		tracef(ctx, "CSI priv=%q params=%v final=%q raw=%x", priv, params, final, seq)
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
