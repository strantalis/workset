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
		return nil
	}
	return sanitizeTerminalOutputStreaming(raw, &s.outputFilter)
}
