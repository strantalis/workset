package sessiond

import (
	"bytes"
	"context"

	"github.com/strantalis/workset/pkg/kitty"
)

type modeSnapshot struct {
	AltScreen  bool  `json:"altScreen"`
	MouseMask  uint8 `json:"mouseMask"`
	MouseSGR   bool  `json:"mouseSGR"`
	MouseUTF8  bool  `json:"mouseUTF8"`
	MouseURXVT bool  `json:"mouseURXVT"`
	TuiMode    bool  `json:"tuiMode"`
}

func (s *Session) handleProtocolOutput(ctx context.Context, raw []byte) {
	s.recordRaw(raw)
	normalized := s.c1Normalizer.Normalize(raw)
	if len(normalized) == 0 {
		return
	}
	cleaned := normalized
	var kittyEvents []kitty.Event
	if s.kittyState != nil {
		cursor := kitty.Cursor{}
		if s.emu != nil {
			pos := s.emu.Cursor()
			cursor = kitty.Cursor{Row: pos.Row, Col: pos.Col}
		}
		cleaned, kittyEvents = s.kittyDecoder.Process(normalized, cursor, s.kittyState)
	}
	if len(kittyEvents) > 0 {
		s.broadcastKitty(kittyEvents)
	}
	if len(cleaned) == 0 {
		if len(kittyEvents) > 0 {
			return
		}
		return
	}
	if s.emu != nil {
		s.emu.Write(ctx, cleaned)
		s.maybePersistSnapshot()
	}
	s.logProtocol(ctx, "out", cleaned)
	filtered := filterTerminalOutputStreaming(cleaned, &s.escapeFilter)
	if len(filtered) == 0 {
		return
	}
	s.mu.Lock()
	s.bumpActivityLocked()
	altChanged, mouseChanged := s.noteModesLocked(cleaned)
	altActive := s.altScreen
	mouseActive := s.mouseMask != 0
	mouseSGR := s.mouseSGR
	mouseEncoding := s.mouseEncoding()
	var modesSnapshot modeSnapshot
	if altChanged || mouseChanged {
		modesSnapshot = s.currentModesLocked()
	}
	s.mu.Unlock()
	if altChanged {
		debugLogf("session_alt_screen id=%s active=%t", s.id, altActive)
	}
	if mouseChanged {
		debugLogf("session_mouse_mode id=%s active=%t sgr=%t encoding=%s", s.id, mouseActive, mouseSGR, mouseEncoding)
	}
	if altChanged || containsClearScreen(cleaned) {
		if s.kittyState != nil {
			s.broadcastKitty(s.kittyState.ClearAll())
		}
	}
	if altChanged || mouseChanged {
		s.broadcastModes(modesSnapshot)
	}
	s.recordOutput(filtered)
	s.broadcast(filtered)
}

func (s *Session) noteModesLocked(data []byte) (bool, bool) {
	if len(data) == 0 {
		return false, false
	}
	const tailMax = 64
	prevAlt := s.altScreen
	prevMask := s.mouseMask
	prevSGR := s.mouseSGR
	prevUTF8 := s.mouseUTF8
	prevURXVT := s.mouseURXVT
	merged := append(append([]byte{}, s.seqTail...), data...)
	if containsAltScreenEnter(merged) {
		s.tuiMode = true
		s.altScreen = true
	}
	if containsAltScreenExit(merged) {
		s.altScreen = false
		s.tuiMode = false
	}
	s.applyMouseModes(merged)
	if !s.altScreen {
		s.tuiMode = false
	}
	if len(merged) > tailMax {
		merged = merged[len(merged)-tailMax:]
	}
	s.seqTail = merged
	altChanged := prevAlt != s.altScreen
	mouseChanged := prevMask != s.mouseMask || prevSGR != s.mouseSGR || prevUTF8 != s.mouseUTF8 || prevURXVT != s.mouseURXVT
	return altChanged, mouseChanged
}

func (s *Session) currentModesLocked() modeSnapshot {
	return modeSnapshot{
		AltScreen:  s.altScreen,
		MouseMask:  s.mouseMask,
		MouseSGR:   s.mouseSGR,
		MouseUTF8:  s.mouseUTF8,
		MouseURXVT: s.mouseURXVT,
		TuiMode:    s.tuiMode,
	}
}

func containsAltScreenEnter(data []byte) bool {
	return bytes.Contains(data, []byte("\x1b[?1049h")) ||
		bytes.Contains(data, []byte("\x1b[?1047h")) ||
		bytes.Contains(data, []byte("\x1b[?47h"))
}

func containsAltScreenExit(data []byte) bool {
	return bytes.Contains(data, []byte("\x1b[?1049l")) ||
		bytes.Contains(data, []byte("\x1b[?1047l")) ||
		bytes.Contains(data, []byte("\x1b[?47l"))
}

func containsClearScreen(data []byte) bool {
	return bytes.Contains(data, []byte("\x1b[2J")) ||
		bytes.Contains(data, []byte("\x1b[3J"))
}

func (s *Session) applyMouseModes(data []byte) {
	for i := 0; i < len(data); i++ {
		if data[i] == 0x1b {
			if i+2 < len(data) && data[i+1] == '[' && data[i+2] == '?' {
				i = s.parseMouseCSI(data, i+3)
			}
			continue
		}
		if data[i] == 0x9b {
			if i+1 < len(data) && data[i+1] == '?' {
				i = s.parseMouseCSI(data, i+2)
			}
		}
	}
}

func (s *Session) parseMouseCSI(data []byte, start int) int {
	params := make([]int, 0, 4)
	val := 0
	hasVal := false
	for i := start; i < len(data); i++ {
		b := data[i]
		if b >= '0' && b <= '9' {
			val = val*10 + int(b-'0')
			hasVal = true
			continue
		}
		if b == ';' {
			if hasVal {
				params = append(params, val)
			} else {
				params = append(params, 0)
			}
			val = 0
			hasVal = false
			continue
		}
		if b >= 0x40 && b <= 0x7e {
			if hasVal || len(params) > 0 {
				params = append(params, val)
			}
			if b == 'h' || b == 'l' {
				on := b == 'h'
				for _, p := range params {
					switch p {
					case 1000:
						s.setMouseMask(0, on)
					case 1002:
						s.setMouseMask(1, on)
					case 1003:
						s.setMouseMask(2, on)
					case 1005:
						s.mouseUTF8 = on
					case 1015:
						s.mouseURXVT = on
					case 1006:
						s.mouseSGR = on
					}
				}
			}
			return i
		}
	}
	return len(data) - 1
}

func (s *Session) setMouseMask(bit uint8, on bool) {
	mask := uint8(1 << bit)
	if on {
		s.mouseMask |= mask
		return
	}
	s.mouseMask &^= mask
}

func (s *Session) mouseEncoding() string {
	if s.mouseSGR {
		return "sgr"
	}
	if s.mouseURXVT {
		return "urxvt"
	}
	if s.mouseUTF8 {
		return "utf8"
	}
	return "x10"
}
