package sessiond

import "sync"

type bufferChunk struct {
	start int64
	data  []byte
}

type terminalBuffer struct {
	mu       sync.Mutex
	maxBytes int
	chunks   []bufferChunk
	size     int
	total    int64
}

func newTerminalBuffer(maxBytes int) *terminalBuffer {
	if maxBytes < 64*1024 {
		maxBytes = 64 * 1024
	}
	return &terminalBuffer{maxBytes: maxBytes}
}

func (b *terminalBuffer) Append(data []byte) {
	if len(data) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	copied := make([]byte, len(data))
	copy(copied, data)
	b.chunks = append(b.chunks, bufferChunk{
		start: b.total,
		data:  copied,
	})
	b.total += int64(len(copied))
	b.size += len(copied)
	for b.size > b.maxBytes && len(b.chunks) > 0 {
		oldest := b.chunks[0]
		b.chunks = b.chunks[1:]
		b.size -= len(oldest.data)
	}
}

func (b *terminalBuffer) ReadSince(offset int64) ([]byte, int64, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.chunks) == 0 {
		return nil, b.total, false
	}
	oldest := b.chunks[0].start
	truncated := false
	if offset < oldest {
		offset = oldest
		truncated = true
	}
	out := make([]byte, 0, b.size)
	for _, chunk := range b.chunks {
		end := chunk.start + int64(len(chunk.data))
		if end <= offset {
			continue
		}
		if offset > chunk.start {
			start := int(offset - chunk.start)
			if start < len(chunk.data) {
				out = append(out, chunk.data[start:]...)
			}
			continue
		}
		out = append(out, chunk.data...)
	}
	return out, b.total, truncated
}
