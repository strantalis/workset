package sessiond

import "bytes"

type terminalModeState struct {
	altScreen bool
	mouse1000 bool
	mouse1002 bool
	mouse1003 bool
	mouse1005 bool
	mouse1006 bool
	mouse1015 bool
}

type terminalModeParser struct {
	state      int
	privateCSI bool
	params     []int
	current    int
	hasCurrent bool
}

const (
	modeParserGround = iota
	modeParserEsc
	modeParserCSI
)

func (s *Session) trackTerminalModes(data []byte) {
	for _, b := range data {
		s.consumeModeByte(b)
	}
}

func (s *Session) consumeModeByte(b byte) {
	switch s.modeParser.state {
	case modeParserGround:
		if b == 0x1b {
			s.modeParser.state = modeParserEsc
		}
	case modeParserEsc:
		if b == '[' {
			s.modeParser.state = modeParserCSI
			s.modeParser.privateCSI = false
			s.modeParser.params = s.modeParser.params[:0]
			s.modeParser.current = 0
			s.modeParser.hasCurrent = false
			return
		}
		if b == 0x1b {
			s.modeParser.state = modeParserEsc
			return
		}
		s.resetModeParser()
	case modeParserCSI:
		if b == '?' && !s.modeParser.privateCSI && len(s.modeParser.params) == 0 && !s.modeParser.hasCurrent {
			s.modeParser.privateCSI = true
			return
		}
		if b >= '0' && b <= '9' {
			s.modeParser.current = (s.modeParser.current * 10) + int(b-'0')
			s.modeParser.hasCurrent = true
			return
		}
		if b == ';' {
			if s.modeParser.hasCurrent {
				s.modeParser.params = append(s.modeParser.params, s.modeParser.current)
				s.modeParser.current = 0
				s.modeParser.hasCurrent = false
			}
			return
		}
		if (b == 'h' || b == 'l') && s.modeParser.privateCSI {
			if s.modeParser.hasCurrent {
				s.modeParser.params = append(s.modeParser.params, s.modeParser.current)
			}
			enabled := b == 'h'
			for _, param := range s.modeParser.params {
				s.applyPrivateMode(param, enabled)
			}
			s.resetModeParser()
			return
		}
		if b == 0x1b {
			s.modeParser.state = modeParserEsc
			return
		}
		s.resetModeParser()
	}
}

func (s *Session) applyPrivateMode(param int, enabled bool) {
	switch param {
	case 47, 1047, 1049:
		s.modeState.altScreen = enabled
	case 1000:
		s.modeState.mouse1000 = enabled
	case 1002:
		s.modeState.mouse1002 = enabled
	case 1003:
		s.modeState.mouse1003 = enabled
	case 1005:
		s.modeState.mouse1005 = enabled
	case 1006:
		s.modeState.mouse1006 = enabled
	case 1015:
		s.modeState.mouse1015 = enabled
	}
}

func (s *Session) resetModeParser() {
	s.modeParser.state = modeParserGround
	s.modeParser.privateCSI = false
	s.modeParser.params = s.modeParser.params[:0]
	s.modeParser.current = 0
	s.modeParser.hasCurrent = false
}

func (s *Session) modeReplayPrefixLocked() []byte {
	var out bytes.Buffer
	if s.modeState.altScreen {
		out.WriteString("\x1b[?1049h")
	}
	if s.modeState.mouse1000 {
		out.WriteString("\x1b[?1000h")
	}
	if s.modeState.mouse1002 {
		out.WriteString("\x1b[?1002h")
	}
	if s.modeState.mouse1003 {
		out.WriteString("\x1b[?1003h")
	}
	if s.modeState.mouse1005 {
		out.WriteString("\x1b[?1005h")
	}
	if s.modeState.mouse1006 {
		out.WriteString("\x1b[?1006h")
	}
	if s.modeState.mouse1015 {
		out.WriteString("\x1b[?1015h")
	}
	if out.Len() == 0 {
		return nil
	}
	return out.Bytes()
}
