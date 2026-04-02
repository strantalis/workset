package terminalservice

import "context"

func (s *Session) sanitizeProtocolInput(_ context.Context, raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
