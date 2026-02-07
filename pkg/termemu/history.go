package termemu

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
