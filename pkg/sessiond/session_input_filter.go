package sessiond

import (
	"context"
)

const maxPendingOSCInputBytes = 4096

type terminalInputFilter struct {
	pendingOSC []byte
}

func (s *Session) sanitizeProtocolInput(ctx context.Context, raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	return sanitizeTerminalInputStreaming(ctx, s, raw)
}

func sanitizeTerminalInputStreaming(ctx context.Context, session *Session, raw []byte) []byte {
	filter := &session.inputFilter
	data := raw
	if len(filter.pendingOSC) > 0 {
		combined := make([]byte, 0, len(filter.pendingOSC)+len(raw))
		combined = append(combined, filter.pendingOSC...)
		combined = append(combined, raw...)
		filter.pendingOSC = filter.pendingOSC[:0]
		data = combined
	}

	out := make([]byte, 0, len(data))
	for idx := 0; idx < len(data); {
		if data[idx] != 0x1b || idx+1 >= len(data) || data[idx+1] != ']' {
			out = append(out, data[idx])
			idx += 1
			continue
		}

		terminatorStart, terminatorLen, complete := findOSCTerminator(data, idx+2)
		if !complete {
			tail := data[idx:]
			if len(tail) <= maxPendingOSCInputBytes {
				filter.pendingOSC = append(filter.pendingOSC[:0], tail...)
				break
			}
			// Fallback: avoid unbounded buffering if a malformed OSC never terminates.
			out = append(out, tail...)
			break
		}

		sequenceEnd := terminatorStart + terminatorLen
		sequence := data[idx:sequenceEnd]
		payload := data[idx+2 : terminatorStart]
		if shouldDropOSCColorResponse(payload) {
			if session.protocolLog != nil && ctx != nil {
				session.protocolLog.Log(
					ctx,
					"terminal.protocol",
					"in",
					"drop",
					"osc_color_query_response",
					sequence,
				)
			}
			idx = sequenceEnd
			continue
		}

		out = append(out, sequence...)
		idx = sequenceEnd
	}
	return out
}

func findOSCTerminator(data []byte, start int) (terminatorStart int, terminatorLen int, complete bool) {
	for idx := start; idx < len(data); idx += 1 {
		switch data[idx] {
		case 0x07:
			return idx, 1, true
		case 0x1b:
			if idx+1 < len(data) && data[idx+1] == '\\' {
				return idx, 2, true
			}
		}
	}
	return 0, 0, false
}
