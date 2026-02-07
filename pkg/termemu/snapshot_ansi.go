package termemu

import (
	"fmt"
	"strconv"
	"strings"
)

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
				return []string{strconv.Itoa(base + int(c.Index-8))}
			}
			return []string{strconv.Itoa(base + int(c.Index))}
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
