package terminalservice

import "context"

func (s *Session) handleProtocolOutput(ctx context.Context, raw []byte) {
	var sanitized []byte
	s.outputMu.Lock()

	s.recordRaw(raw)
	if len(raw) == 0 {
		s.outputMu.Unlock()
		return
	}
	outSeq := s.debugOutputSeq.Add(1)
	sanitized = s.sanitizeProtocolOutput(ctx, raw)
	debugLogf(
		"session_output id=%s seq=%d raw={%s} sanitized={%s}",
		s.id,
		outSeq,
		summarizeBytes(raw, 48),
		summarizeBytes(sanitized, 48),
	)
	if len(sanitized) == 0 {
		s.outputMu.Unlock()
		return
	}
	s.trackTerminalModes(sanitized)
	s.logProtocol(ctx, "out", sanitized)
	s.mu.Lock()
	s.bumpActivityLocked()
	s.mu.Unlock()
	s.recordOutput(sanitized)
	s.outputMu.Unlock()
	s.notifySubscribers()
}

func (s *Session) sanitizeProtocolOutput(_ context.Context, raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	return sanitizeTerminalOutputStreaming(raw)
}
