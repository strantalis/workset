package termemu

import (
	"bytes"
	"encoding/gob"
)

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
