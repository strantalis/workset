package sessiond

import "context"

func (s *Session) handleProtocolOutput(ctx context.Context, raw []byte) {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()

	s.recordRaw(raw)
	if len(raw) == 0 {
		return
	}
	outSeq := s.debugOutputSeq.Add(1)
	sanitized := s.sanitizeProtocolOutput(raw)
	debugLogf(
		"session_output id=%s seq=%d raw={%s} sanitized={%s}",
		s.id,
		outSeq,
		summarizeBytes(raw, 48),
		summarizeBytes(sanitized, 48),
	)
	if len(sanitized) == 0 {
		return
	}
	s.trackTerminalModes(sanitized)
	s.logProtocol(ctx, "out", sanitized)
	s.mu.Lock()
	s.bumpActivityLocked()
	s.mu.Unlock()
	s.recordOutput(sanitized)
	s.broadcast(sanitized)
}

func (s *Session) sanitizeProtocolOutput(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}

	out := make([]byte, 0, len(raw))

	for _, b := range raw {
		if s.protocolInAPC {
			if s.protocolAPCEsc {
				s.protocolAPCEsc = false
				if b == '\\' || b == 0x9c {
					s.protocolInAPC = false
					continue
				}
				if b == 0x1b {
					s.protocolAPCEsc = true
				}
				continue
			}
			if b == 0x1b {
				s.protocolAPCEsc = true
				continue
			}
			if b == 0x9c {
				s.protocolInAPC = false
			}
			continue
		}

		if s.protocolPendingEsc {
			s.protocolPendingEsc = false
			if b == '_' {
				// 7-bit APC start (kitty graphics): drop until ST.
				s.protocolInAPC = true
				s.protocolAPCEsc = false
				continue
			}
			out = append(out, 0x1b)
		}

		if b == 0x1b {
			// Keep ESC pending so we can detect APC start across chunk boundaries.
			s.protocolPendingEsc = true
			continue
		}
		out = append(out, b)
	}

	return out
}
