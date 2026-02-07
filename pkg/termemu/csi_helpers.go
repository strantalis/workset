package termemu

import "bytes"

func parseCSIParams(buf []byte) ([]int, byte) {
	if len(buf) == 0 {
		return nil, 0
	}
	priv := byte(0)
	if buf[0] == '?' || buf[0] == '>' {
		priv = buf[0]
		buf = buf[1:]
	}
	if len(buf) == 0 {
		return nil, priv
	}
	parts := bytes.Split(buf, []byte(";"))
	params := make([]int, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			params = append(params, 0)
			continue
		}
		val := 0
		for _, b := range part {
			if b < '0' || b > '9' {
				continue
			}
			val = val*10 + int(b-'0')
		}
		params = append(params, val)
	}
	return params, priv
}

func param(params []int, idx int, fallback int) int {
	if idx < len(params) {
		if params[idx] == 0 {
			return fallback
		}
		return params[idx]
	}
	return fallback
}

func clamp(value, minVal, maxVal int) int {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}
