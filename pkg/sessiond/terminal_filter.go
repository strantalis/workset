package sessiond

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	terminalFilterOnce    sync.Once
	terminalFilterEnabled bool
	terminalFilterDebug   bool
	terminalFilterDropOSC bool
	terminalFilterLog     *os.File
	terminalFilterMu      sync.Mutex
)

func terminalFilterConfig() (bool, bool) {
	terminalFilterOnce.Do(func() {
		terminalFilterEnabled = envTruthy(os.Getenv("WORKSET_TERMINAL_FILTER"))
		terminalFilterDebug = envTruthy(os.Getenv("WORKSET_TERMINAL_FILTER_DEBUG"))
		terminalFilterDropOSC = envTruthy(os.Getenv("WORKSET_TERMINAL_FILTER_DROP_COLORS"))
		if terminalFilterDebug {
			logPath, err := terminalFilterLogPath()
			if err != nil {
				terminalFilterDebug = false
				return
			}
			if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
				terminalFilterDebug = false
				return
			}
			file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				terminalFilterDebug = false
				return
			}
			terminalFilterLog = file
		}
	})
	return terminalFilterEnabled, terminalFilterDebug
}

func terminalFilterLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "terminal_filter.log"), nil
}

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func logTerminalFilter(kind string, seq []byte) {
	if terminalFilterLog == nil {
		return
	}
	terminalFilterMu.Lock()
	defer terminalFilterMu.Unlock()
	_, _ = fmt.Fprintf(
		terminalFilterLog,
		"%s %s len=%d hex=%x ascii=%q\n",
		time.Now().Format(time.RFC3339Nano),
		kind,
		len(seq),
		seq,
		seq,
	)
}

func filterTerminalOutput(data []byte) []byte {
	const esc = 0x1b
	if len(data) == 0 {
		return data
	}
	enabled, debug := terminalFilterConfig()
	if !enabled && !debug {
		return data
	}
	var out []byte
	last := 0
	dropped := false
	for i := 0; i < len(data); i++ {
		if data[i] != esc || i+1 >= len(data) {
			continue
		}
		switch data[i+1] {
		case ']':
			end, drop := scanOSC(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("OSC", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case 'P':
			end, drop := scanEscapeString(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("DCS", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case '_':
			end, drop := scanEscapeString(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("APC", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case '^':
			end, drop := scanEscapeString(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("PM", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case 'X':
			end, drop := scanEscapeString(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("SOS", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case '[':
			end, drop := scanCSI(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("CSI", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		default:
			continue
		}
	}
	if !enabled {
		return data
	}
	if !dropped || out == nil {
		return data
	}
	if out == nil {
		return data
	}
	if last < len(data) {
		out = append(out, data[last:]...)
	}
	return out
}

type c1Normalizer struct {
	utf8Tail []byte
}

func (n *c1Normalizer) Normalize(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	if len(n.utf8Tail) > 0 {
		data = append(n.utf8Tail, data...)
		n.utf8Tail = nil
	}
	var out []byte
	i := 0
	for i < len(data) {
		b := data[i]
		if b < 0x80 {
			if out != nil {
				out = append(out, b)
			}
			i++
			continue
		}
		if !utf8.FullRune(data[i:]) {
			if out == nil {
				out = make([]byte, 0, len(data))
				out = append(out, data[:i]...)
			}
			n.utf8Tail = append(n.utf8Tail, data[i:]...)
			break
		}
		r, size := utf8.DecodeRune(data[i:])
		if r != utf8.RuneError || size > 1 {
			if out != nil {
				out = append(out, data[i:i+size]...)
			}
			i += size
			continue
		}
		mapped := mapC1Control(b)
		if mapped == nil {
			if out != nil {
				out = append(out, b)
			}
			i++
			continue
		}
		if out == nil {
			out = make([]byte, 0, len(data)+len(mapped))
			out = append(out, data[:i]...)
		}
		out = append(out, mapped...)
		i++
	}
	if out == nil {
		return data
	}
	return out
}

func mapC1Control(b byte) []byte {
	switch b {
	case 0x84: // IND
		return []byte{0x1b, 'D'}
	case 0x85: // NEL
		return []byte{0x1b, 'E'}
	case 0x88: // HTS
		return []byte{0x1b, 'H'}
	case 0x8d: // RI
		return []byte{0x1b, 'M'}
	case 0x8e: // SS2
		return []byte{0x1b, 'N'}
	case 0x8f: // SS3
		return []byte{0x1b, 'O'}
	case 0x90: // DCS
		return []byte{0x1b, 'P'}
	case 0x98: // SOS
		return []byte{0x1b, 'X'}
	case 0x9b: // CSI
		return []byte{0x1b, '['}
	case 0x9c: // ST
		return []byte{0x1b, '\\'}
	case 0x9d: // OSC
		return []byte{0x1b, ']'}
	case 0x9e: // PM
		return []byte{0x1b, '^'}
	case 0x9f: // APC
		return []byte{0x1b, '_'}
	default:
		return nil
	}
}

type escapeStringFilter struct {
	enabled    bool
	debug      bool
	configured bool
	active     bool
	pendingEsc bool
	kind       string
	logBuf     []byte
	pending    []byte
	truncated  bool
}

func (f *escapeStringFilter) ensureConfig() {
	if f.configured {
		return
	}
	f.enabled, f.debug = terminalFilterConfig()
	f.configured = true
}

func (f *escapeStringFilter) reset() {
	f.active = false
	f.pendingEsc = false
	f.kind = ""
	f.logBuf = nil
	f.pending = nil
	f.truncated = false
}

func (f *escapeStringFilter) appendLog(data []byte) {
	const maxLog = 4096
	if !f.debug || len(data) == 0 {
		return
	}
	if len(f.logBuf) >= maxLog {
		return
	}
	remain := maxLog - len(f.logBuf)
	if len(data) > remain {
		data = data[:remain]
	}
	f.logBuf = append(f.logBuf, data...)
}

func (f *escapeStringFilter) appendPending(data []byte) {
	const maxPending = 64 * 1024
	if len(data) == 0 || f.truncated {
		return
	}
	if len(f.pending)+len(data) > maxPending {
		remain := maxPending - len(f.pending)
		if remain > 0 {
			f.pending = append(f.pending, data[:remain]...)
		}
		f.truncated = true
		return
	}
	f.pending = append(f.pending, data...)
}

func filterTerminalOutputStreaming(data []byte, f *escapeStringFilter) []byte {
	const esc = 0x1b
	if len(data) == 0 {
		return data
	}
	f.ensureConfig()
	if !f.enabled && !f.debug {
		return data
	}
	if f.pendingEsc {
		data = append([]byte{esc}, data...)
		f.pendingEsc = false
	}
	var prefix []byte
	if f.active {
		end := scanEscapeStringTerminator(data, 0)
		if end == 0 {
			f.appendLog(data)
			if f.kind == "OSC" {
				f.appendPending(data)
			}
			if f.enabled {
				return nil
			}
			return data
		}
		f.appendLog(data[:end])
		if f.kind == "OSC" {
			f.appendPending(data[:end])
		}
		if f.debug && len(f.logBuf) > 0 {
			logTerminalFilter(f.kind, f.logBuf)
		}
		pending := f.pending
		kind := f.kind
		truncated := f.truncated
		f.reset()
		if f.enabled {
			if kind == "OSC" && !truncated && !shouldDropOSC(extractOSCPayload(pending)) {
				prefix = pending
			}
			data = data[end:]
		} else {
			return data
		}
	}
	if !f.enabled {
		return filterTerminalOutput(data)
	}
	enabled, debug := f.enabled, f.debug
	var out []byte
	last := 0
	dropped := false
	if len(prefix) > 0 {
		out = make([]byte, 0, len(prefix)+len(data))
		out = append(out, prefix...)
		dropped = true
	}
	for i := 0; i < len(data); i++ {
		if data[i] != esc || i+1 >= len(data) {
			if data[i] == esc && i+1 >= len(data) {
				if f.enabled || f.debug {
					if out == nil && i > 0 {
						out = make([]byte, 0, len(data))
						out = append(out, data[:i]...)
					} else if out != nil && last < i {
						out = append(out, data[last:i]...)
					}
					f.pendingEsc = true
					if out == nil {
						return nil
					}
					return out
				}
			}
			continue
		}
		switch data[i+1] {
		case ']':
			end, drop := scanOSC(data, i)
			if end == i {
				f.active = true
				f.kind = "OSC"
				f.appendLog(data[i:])
				f.appendPending(data[i:])
				if out == nil {
					out = make([]byte, 0, len(data))
					out = append(out, data[:i]...)
				}
				return out
			}
			if drop {
				if debug {
					logTerminalFilter("OSC", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case 'P':
			kind := "DCS"
			end, drop := scanEscapeString(data, i)
			if end == i {
				f.active = true
				f.kind = kind
				f.appendLog(data[i:])
				if out == nil {
					out = make([]byte, 0, len(data))
					out = append(out, data[:i]...)
				}
				return out
			}
			if drop {
				if debug {
					logTerminalFilter(kind, data[i:end])
				}
				dropped = true
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case '_':
			kind := "APC"
			end, drop := scanEscapeString(data, i)
			if end == i {
				f.active = true
				f.kind = kind
				f.appendLog(data[i:])
				if out == nil {
					out = make([]byte, 0, len(data))
					out = append(out, data[:i]...)
				}
				return out
			}
			if drop {
				if debug {
					logTerminalFilter(kind, data[i:end])
				}
				dropped = true
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case '^':
			kind := "PM"
			end, drop := scanEscapeString(data, i)
			if end == i {
				f.active = true
				f.kind = kind
				f.appendLog(data[i:])
				if out == nil {
					out = make([]byte, 0, len(data))
					out = append(out, data[:i]...)
				}
				return out
			}
			if drop {
				if debug {
					logTerminalFilter(kind, data[i:end])
				}
				dropped = true
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case 'X':
			kind := "SOS"
			end, drop := scanEscapeString(data, i)
			if end == i {
				f.active = true
				f.kind = kind
				f.appendLog(data[i:])
				if out == nil {
					out = make([]byte, 0, len(data))
					out = append(out, data[:i]...)
				}
				return out
			}
			if drop {
				if debug {
					logTerminalFilter(kind, data[i:end])
				}
				dropped = true
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		case '[':
			end, drop := scanCSI(data, i)
			if end == i {
				continue
			}
			if drop {
				if debug {
					logTerminalFilter("CSI", data[i:end])
				}
				dropped = true
				if !enabled {
					i = end - 1
					continue
				}
				if out == nil {
					out = make([]byte, 0, len(data))
				}
				if last < i {
					out = append(out, data[last:i]...)
				}
				last = end
			}
			i = end - 1
		default:
			continue
		}
	}
	if !dropped || out == nil {
		return data
	}
	if last < len(data) {
		out = append(out, data[last:]...)
	}
	return out
}

func extractOSCPayload(seq []byte) []byte {
	const esc = 0x1b
	const bel = 0x07
	if len(seq) < 3 || seq[0] != esc || seq[1] != ']' {
		return nil
	}
	end := len(seq)
	switch {
	case seq[end-1] == bel:
		end--
	case end >= 2 && seq[end-2] == esc && seq[end-1] == '\\':
		end -= 2
	default:
		return nil
	}
	if end <= 2 {
		return nil
	}
	return seq[2:end]
}

func scanOSC(data []byte, start int) (int, bool) {
	const esc = 0x1b
	const bel = 0x07
	i := start + 2
	for i < len(data) {
		switch data[i] {
		case bel:
			return i + 1, shouldDropOSC(data[start+2 : i])
		case esc:
			if i+1 < len(data) && data[i+1] == '\\' {
				return i + 2, shouldDropOSC(data[start+2 : i])
			}
		}
		i++
	}
	return start, false
}

func scanEscapeString(data []byte, start int) (int, bool) {
	const esc = 0x1b
	const bel = 0x07
	i := start + 2
	for i < len(data) {
		switch data[i] {
		case bel:
			return i + 1, true
		case esc:
			if i+1 < len(data) && data[i+1] == '\\' {
				return i + 2, true
			}
		}
		i++
	}
	return start, false
}

func scanEscapeStringTerminator(data []byte, start int) int {
	const esc = 0x1b
	const bel = 0x07
	i := start
	for i < len(data) {
		switch data[i] {
		case bel:
			return i + 1
		case esc:
			if i+1 < len(data) && data[i+1] == '\\' {
				return i + 2
			}
		}
		i++
	}
	return 0
}

func shouldDropOSC(payload []byte) bool {
	if !terminalFilterDropOSC {
		return false
	}
	if len(payload) == 0 {
		return false
	}
	hasRGB := false
	for i := 0; i+3 < len(payload); i++ {
		if payload[i] == 'r' && payload[i+1] == 'g' && payload[i+2] == 'b' && payload[i+3] == ':' {
			hasRGB = true
			break
		}
	}
	if !hasRGB {
		return false
	}
	if len(payload) >= 3 && payload[0] == '1' && payload[1] == '0' && payload[2] == ';' {
		return true
	}
	if len(payload) >= 3 && payload[0] == '1' && payload[1] == '1' && payload[2] == ';' {
		return true
	}
	if len(payload) >= 2 && payload[0] == '4' && payload[1] == ';' {
		return true
	}
	return false
}

func scanCSI(data []byte, start int) (int, bool) {
	i := start + 2
	for i < len(data) {
		b := data[i]
		if b >= 0x40 && b <= 0x7e {
			return i + 1, shouldDropCSI(data[start+2:i], b)
		}
		i++
	}
	return start, false
}

func shouldDropCSI(params []byte, final byte) bool {
	if final == 'R' {
		return true
	}
	if final == 'c' {
		for _, b := range params {
			if b == '?' || b == '>' {
				return true
			}
		}
	}
	return false
}

func (s *Session) logProtocol(ctx context.Context, direction string, data []byte) {
	if s.protocolLog == nil || len(data) == 0 {
		return
	}
	const esc = 0x1b
	for i := 0; i < len(data); i++ {
		if data[i] != esc || i+1 >= len(data) {
			continue
		}
		switch data[i+1] {
		case ']':
			end, _ := scanOSC(data, i)
			if end == i || end > len(data) {
				continue
			}
			payloadEnd := end
			if payloadEnd >= 2 && data[payloadEnd-2] == esc && data[payloadEnd-1] == '\\' {
				payloadEnd -= 2
			} else if payloadEnd >= 1 && data[payloadEnd-1] == 0x07 {
				payloadEnd--
			}
			if payloadEnd < i+2 {
				payloadEnd = i + 2
			}
			payload := data[i+2 : payloadEnd]
			s.logOSCProtocol(ctx, direction, data[i:end], payload)
			i = end - 1
		case '[':
			end, _ := scanCSI(data, i)
			if end == i || end > len(data) {
				continue
			}
			if end-1 <= i+1 {
				continue
			}
			final := data[end-1]
			params := data[i+2 : end-1]
			s.logCSIProtocol(ctx, direction, data[i:end], params, final)
			i = end - 1
		default:
			continue
		}
	}
}

func (s *Session) logOSCProtocol(ctx context.Context, direction string, seq []byte, payload []byte) {
	if s.protocolLog == nil {
		return
	}
	if isOSCColorQueryRequest(payload) {
		s.protocolLog.Log(ctx, "terminal.protocol", direction, "event", "osc_color_query_request", seq)
	}
	if shouldDropOSC(payload) {
		s.protocolLog.Log(ctx, "terminal.protocol", direction, "drop", "osc_color_query_response", seq)
	}
}

func (s *Session) logCSIProtocol(ctx context.Context, direction string, seq []byte, params []byte, final byte) {
	if s.protocolLog == nil {
		return
	}
	switch final {
	case 'n':
		s.protocolLog.Log(ctx, "terminal.protocol", direction, "event", "dsr_request", seq)
	case 'R':
		s.protocolLog.Log(ctx, "terminal.protocol", direction, "drop", "dsr_response", seq)
	case 'c':
		if hasCSIQueryPrefix(params) {
			s.protocolLog.Log(ctx, "terminal.protocol", direction, "drop", "device_attributes_response", seq)
		} else {
			s.protocolLog.Log(ctx, "terminal.protocol", direction, "event", "device_attributes_request", seq)
		}
	}
}

func hasCSIQueryPrefix(params []byte) bool {
	for _, b := range params {
		if b == '?' || b == '>' {
			return true
		}
	}
	return false
}

func isOSCColorQueryRequest(payload []byte) bool {
	if len(payload) < 4 {
		return false
	}
	if bytes.HasPrefix(payload, []byte("10;?")) || bytes.HasPrefix(payload, []byte("11;?")) {
		return true
	}
	if len(payload) >= 2 && payload[0] == '4' && payload[1] == ';' {
		return bytes.Contains(payload, []byte(";?"))
	}
	return false
}
