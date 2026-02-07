package termemu

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
	for i := range n {
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
	for range n {
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
	for range n {
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
	for range n {
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
	for range n {
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
	for range n {
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

func cloneRow(row Row) Row {
	if len(row.Cells) == 0 {
		return Row{}
	}
	out := Row{Cells: make([]Cell, len(row.Cells))}
	copy(out.Cells, row.Cells)
	return out
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
	for i := range newRows {
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
	for i := range cols {
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
