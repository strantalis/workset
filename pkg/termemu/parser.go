package termemu

import (
	"context"
	"unicode/utf8"
)

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

func (t *Terminal) parseByte(ctx context.Context, b byte, data *[]byte, responses *[][]byte) {
	switch t.state {
	case stateGround:
		if b == 0x1b {
			t.state = stateEscape
			return
		}
		if b < 0x20 {
			t.handleControl(ctx, b)
			return
		}
		if len(t.utf8Buf) == 0 {
			switch b {
			case 0x9b:
				t.state = stateCSI
				t.csiBuf = t.csiBuf[:0]
				return
			case 0x9d:
				t.state = stateOSC
				t.oscBuf = t.oscBuf[:0]
				return
			case 0x90, 0x98, 0x9e, 0x9f:
				t.state = stateEscapeString
				t.escStringPendingEsc = false
				return
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
			tracef(ctx, "ESC b=0x%02x char=%q", b, b)
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
			t.handleCSI(ctx, t.csiBuf, responses)
			t.csiBuf = t.csiBuf[:0]
			t.state = stateGround
		}
	case stateOSC:
		if b == 0x07 || b == 0x9c {
			t.state = stateGround
			t.oscBuf = t.oscBuf[:0]
			return
		}
		if b == 0x1b {
			if len(*data) > 0 && (*data)[0] == '\\' {
				*data = (*data)[1:]
				t.state = stateGround
				t.oscBuf = t.oscBuf[:0]
				return
			}
		}
		t.oscBuf = append(t.oscBuf, b)
	case stateEscapeString:
		if t.escStringPendingEsc {
			if b == '\\' {
				t.state = stateGround
				t.escStringPendingEsc = false
				return
			}
			t.escStringPendingEsc = false
		}
		if b == 0x07 || b == 0x9c {
			t.state = stateGround
			return
		}
		if b == 0x1b {
			t.escStringPendingEsc = true
			return
		}
	}
}
