package sessiond

func sanitizeTerminalOutputStreaming(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
