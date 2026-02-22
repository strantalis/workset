package sessiond

import "bytes"

const maxPendingOSCOutputBytes = 4096

type terminalOutputFilter struct {
	pendingRawOSC   []byte
	pendingCaretOSC []byte
}

func sanitizeTerminalOutputStreaming(raw []byte, filter *terminalOutputFilter) []byte {
	if len(raw) == 0 {
		return nil
	}
	stageOne := filterRawOSCColorResponses(raw, &filter.pendingRawOSC)
	if len(stageOne) == 0 {
		return nil
	}
	return filterCaretOSCColorResponses(stageOne, &filter.pendingCaretOSC)
}

func filterRawOSCColorResponses(raw []byte, pending *[]byte) []byte {
	data := raw
	if len(*pending) > 0 {
		combined := make([]byte, 0, len(*pending)+len(raw))
		combined = append(combined, *pending...)
		combined = append(combined, raw...)
		*pending = (*pending)[:0]
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
			if len(tail) <= maxPendingOSCOutputBytes {
				*pending = append((*pending)[:0], tail...)
				break
			}
			out = append(out, tail...)
			break
		}

		sequenceEnd := terminatorStart + terminatorLen
		payload := data[idx+2 : terminatorStart]
		if shouldDropOSCColorResponse(payload) {
			idx = sequenceEnd
			continue
		}
		out = append(out, data[idx:sequenceEnd]...)
		idx = sequenceEnd
	}
	return out
}

func filterCaretOSCColorResponses(raw []byte, pending *[]byte) []byte {
	data := raw
	if len(*pending) > 0 {
		combined := make([]byte, 0, len(*pending)+len(raw))
		combined = append(combined, *pending...)
		combined = append(combined, raw...)
		*pending = (*pending)[:0]
		data = combined
	}

	out := make([]byte, 0, len(data))
	for idx := 0; idx < len(data); {
		if !isCaretOSCStart(data, idx) {
			out = append(out, data[idx])
			idx += 1
			continue
		}

		terminatorStart, terminatorLen, complete := findCaretOSCTerminator(data, idx+3)
		if !complete {
			tail := data[idx:]
			if len(tail) <= maxPendingOSCOutputBytes {
				*pending = append((*pending)[:0], tail...)
				break
			}
			out = append(out, tail...)
			break
		}

		sequenceEnd := terminatorStart + terminatorLen
		payload := data[idx+3 : terminatorStart]
		if shouldDropOSCColorResponse(payload) {
			idx = sequenceEnd
			continue
		}
		out = append(out, data[idx:sequenceEnd]...)
		idx = sequenceEnd
	}
	return out
}

func isCaretOSCStart(data []byte, idx int) bool {
	return idx+2 < len(data) && data[idx] == '^' && data[idx+1] == '[' && data[idx+2] == ']'
}

func findCaretOSCTerminator(data []byte, start int) (terminatorStart int, terminatorLen int, complete bool) {
	for idx := start; idx < len(data); idx += 1 {
		// Caret BEL form: ^G
		if data[idx] == '^' && idx+1 < len(data) && data[idx+1] == 'G' {
			return idx, 2, true
		}
		// Caret ST form: ^[\
		if data[idx] == '^' && idx+2 < len(data) && data[idx+1] == '[' && data[idx+2] == '\\' {
			return idx, 3, true
		}
	}
	return 0, 0, false
}

func shouldDropOSCColorResponse(payload []byte) bool {
	command, rest, ok := bytes.Cut(payload, []byte(";"))
	if !ok || len(rest) == 0 {
		return false
	}

	restLower := bytes.ToLower(rest)
	if !bytes.Contains(restLower, []byte("rgb:")) {
		return false
	}

	switch string(command) {
	case "10", "11":
		return bytes.HasPrefix(rest, []byte("?;rgb:"))
	case "4":
		_, tail, ok := bytes.Cut(rest, []byte(";"))
		if !ok || len(tail) == 0 {
			return false
		}
		return bytes.HasPrefix(tail, []byte("?;rgb:"))
	default:
		return false
	}
}
