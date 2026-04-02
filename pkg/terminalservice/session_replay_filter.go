package terminalservice

import "bytes"

// stripReplayQueries removes terminal query escape sequences from replay data
// that would cause the terminal emulator to generate protocol responses.
//
// During live operation, programs send these queries and consume the responses.
// During replay, no program is waiting for responses — the terminal emulator
// re-processes the queries and sends responses to the shell's stdin as garbled
// text (e.g., ";1R62;22c" visible at the prompt).
//
// This function handles three sequence families:
//   - CSI queries: DSR, DA1–DA3, DECXCPR, XTVERSION, Kitty keyboard
//   - OSC color queries: foreground/background/cursor color probes
//   - DCS queries: DECRQSS, XTGETTCAP
//
// Only well-defined query sequences are stripped. Mode-setting sequences,
// cursor movement, colors, and all other escape sequences pass through
// unchanged.
func stripReplayQueries(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	if !bytes.ContainsRune(data, 0x1b) {
		return data
	}
	out := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		if data[i] != 0x1b {
			out = append(out, data[i])
			i++
			continue
		}
		if n := matchCSIQuery(data[i:]); n > 0 {
			i += n
			continue
		}
		if n := matchOSCQuery(data[i:]); n > 0 {
			i += n
			continue
		}
		if n := matchDCSQuery(data[i:]); n > 0 {
			i += n
			continue
		}
		out = append(out, data[i])
		i++
	}
	if len(out) == len(data) {
		return data
	}
	return out
}

// matchCSIQuery checks if data starts with a CSI query sequence and returns
// the number of bytes consumed, or 0 if no match.
//
// CSI format: ESC [ <prefix?> <params?> <final>
//
// Matched queries:
//
//	\e[5n        DSR operating status
//	\e[6n        DSR cursor position (any leading zeros)
//	\e[?6n       DECXCPR extended cursor position
//	\e[c         DA1 primary device attributes
//	\e[0c        DA1 with explicit zero (any leading zeros)
//	\e[>c        DA2 secondary device attributes
//	\e[>0c       DA2 with explicit zero
//	\e[=c        DA3 tertiary device attributes
//	\e[=0c       DA3 with explicit zero
//	\e[>q        XTVERSION terminal version query
//	\e[?u        Kitty keyboard protocol query
func matchCSIQuery(data []byte) int {
	if len(data) < 3 || data[0] != 0x1b || data[1] != '[' {
		return 0
	}
	j := 2

	// Optional private prefix: ? > =
	prefix := byte(0)
	if j < len(data) && (data[j] == '?' || data[j] == '>' || data[j] == '=') {
		prefix = data[j]
		j++
	}

	// Read parameter digits (no semicolons expected for query sequences).
	paramStart := j
	for j < len(data) && data[j] >= '0' && data[j] <= '9' {
		j++
	}
	if j >= len(data) {
		return 0
	}

	finalByte := data[j]
	paramLen := j - paramStart

	// Parse single numeric parameter (0 if absent).
	param := 0
	for k := paramStart; k < j; k++ {
		param = param*10 + int(data[k]-'0')
	}

	switch finalByte {
	case 'n':
		if prefix == 0 && (param == 5 || param == 6) && paramLen > 0 {
			return j + 1
		}
		if prefix == '?' && param == 6 && paramLen > 0 {
			return j + 1
		}
	case 'c':
		if prefix == 0 && (paramLen == 0 || param == 0) {
			return j + 1
		}
		if (prefix == '>' || prefix == '=') && (paramLen == 0 || param == 0) {
			return j + 1
		}
	case 'q':
		if prefix == '>' && (paramLen == 0 || param == 0) {
			return j + 1
		}
	case 'u':
		if prefix == '?' && paramLen == 0 {
			return j + 1
		}
	}

	return 0
}

// matchOSCQuery checks if data starts with an OSC color query and returns
// the number of bytes consumed, or 0 if no match.
//
// OSC color queries: \e]<code>;?\e\\ or \e]<code>;?\x07
// Codes: 10 (foreground), 11 (background), 12 (cursor color),
// 13–19 (additional color queries).
func matchOSCQuery(data []byte) int {
	if len(data) < 5 || data[0] != 0x1b || data[1] != ']' {
		return 0
	}
	j := 2

	// Read OSC code digits.
	codeStart := j
	for j < len(data) && data[j] >= '0' && data[j] <= '9' {
		j++
	}
	if j == codeStart || j >= len(data) || data[j] != ';' {
		return 0
	}

	code := 0
	for k := codeStart; k < j; k++ {
		code = code*10 + int(data[k]-'0')
	}
	// OSC 10–19 are color queries when followed by '?'.
	if code < 10 || code > 19 {
		return 0
	}

	j++ // skip ';'
	if j >= len(data) || data[j] != '?' {
		return 0
	}
	j++ // skip '?'

	// Terminator: BEL (\x07) or ST (\e\\).
	if j < len(data) && data[j] == 0x07 {
		return j + 1
	}
	if j+1 < len(data) && data[j] == 0x1b && data[j+1] == '\\' {
		return j + 2
	}

	return 0
}

// matchDCSQuery checks if data starts with a DCS query and returns
// the number of bytes consumed, or 0 if no match.
//
// DCS format: ESC P <content> ST
// ST (String Terminator): ESC \ or \x9c
//
// Matched queries:
//
//	\eP$q...\e\\   DECRQSS (Request Status String)
//	\eP+q...\e\\   XTGETTCAP (Request Termcap)
func matchDCSQuery(data []byte) int {
	if len(data) < 4 || data[0] != 0x1b || data[1] != 'P' {
		return 0
	}
	// Check for known DCS query prefixes.
	if data[2] != '$' && data[2] != '+' {
		return 0
	}
	if data[3] != 'q' {
		return 0
	}

	// Scan for ST terminator: ESC \ or \x9c.
	for j := 4; j < len(data); j++ {
		if data[j] == 0x9c {
			return j + 1
		}
		if data[j] == 0x1b && j+1 < len(data) && data[j+1] == '\\' {
			return j + 2
		}
	}

	return 0
}
