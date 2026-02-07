package termemu

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
	for range n {
		t.cursor.Col = t.nextTabStop(t.cursor.Col)
	}
}

func (t *Terminal) backTab(n int) {
	if n <= 0 {
		return
	}
	for range n {
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

func defaultTabStops(cols int) []bool {
	stops := make([]bool, cols)
	for i := range cols {
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
