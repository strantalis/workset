package sessiond

import "context"

func (s *Session) logProtocol(ctx context.Context, direction string, data []byte) {
	if s.protocolLog == nil || len(data) == 0 {
		return
	}
	if ctx == nil {
		return
	}
	s.protocolLog.Log(ctx, "pty", direction, "write", s.id, data)
}
