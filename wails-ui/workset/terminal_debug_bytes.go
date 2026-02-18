package main

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func summarizeTerminalBytes(data []byte, limit int) string {
	if len(data) == 0 {
		return "len=0"
	}
	if limit <= 0 {
		limit = 64
	}
	n := len(data)
	if n > limit {
		n = limit
	}
	escCount := 0
	c1Count := 0
	ctrlCount := 0
	for _, b := range data {
		if b == 0x1b {
			escCount++
		}
		if b >= 0x80 && b <= 0x9f {
			c1Count++
		}
		if b < 0x20 && b != '\n' && b != '\r' && b != '\t' {
			ctrlCount++
		}
	}
	var hex strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			hex.WriteByte(' ')
		}
		hex.WriteString(fmt.Sprintf("%02x", data[i]))
	}
	preview := previewTerminalBytes(data[:n])
	truncated := ""
	if len(data) > n {
		truncated = " truncated=true"
	}
	return fmt.Sprintf(
		"len=%d esc=%d c1=%d ctrl=%d head_hex=%q head_txt=%q%s",
		len(data),
		escCount,
		c1Count,
		ctrlCount,
		hex.String(),
		preview,
		truncated,
	)
}

func previewTerminalBytes(data []byte) string {
	var b strings.Builder
	for _, c := range data {
		switch c {
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\\':
			b.WriteString("\\\\")
		default:
			if c >= 0x20 && c <= 0x7e {
				b.WriteByte(c)
			} else {
				b.WriteString(fmt.Sprintf("\\x%02x", c))
			}
		}
	}
	return b.String()
}

func summarizeTerminalBase64(dataB64 string, limit int) string {
	payload, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return fmt.Sprintf("decode_error=%q b64_len=%d", err.Error(), len(dataB64))
	}
	return summarizeTerminalBytes(payload, limit)
}
