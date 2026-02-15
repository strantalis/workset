package sessiond

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func (s *Session) recordOutput(data []byte) {
	if s.buffer != nil {
		s.buffer.Append(data)
	}
	s.mu.Lock()
	file := s.transcriptFile
	s.mu.Unlock()
	if file == nil {
		return
	}
	if _, err := file.Write(data); err == nil {
		s.mu.Lock()
		s.transcriptSize += int64(len(data))
		s.mu.Unlock()
	}
	s.trimTranscript()
}

func (s *Session) openTranscript() error {
	if s.opts.TranscriptDir == "" {
		return nil
	}
	safe := sanitizeID(s.id)
	if safe == "" {
		safe = "session"
	}
	if err := os.MkdirAll(s.opts.TranscriptDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(s.opts.TranscriptDir, safe+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	s.transcriptPath = path
	s.transcriptFile = file
	s.transcriptSize = info.Size()
	return nil
}

func (s *Session) openRecord() {
	if !s.recordEnabled || s.opts.RecordDir == "" {
		return
	}
	safe := sanitizeID(s.id)
	if safe == "" {
		safe = "session"
	}
	if err := os.MkdirAll(s.opts.RecordDir, 0o755); err != nil {
		return
	}
	name := fmt.Sprintf("%s-%s.ptylog", safe, time.Now().Format("20060102-150405"))
	path := filepath.Join(s.opts.RecordDir, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	s.recordPath = path
	s.recordFile = file
}

func (s *Session) recordRaw(data []byte) {
	if s.recordFile == nil || len(data) == 0 {
		return
	}
	s.recordMu.Lock()
	defer s.recordMu.Unlock()
	_, _ = s.recordFile.Write(data)
}

func (s *Session) readTranscriptTail(maxBytes int64) ([]byte, bool, error) {
	if s.transcriptPath == "" {
		return nil, false, nil
	}
	file, err := os.Open(s.transcriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer func() {
		_ = file.Close()
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	size := info.Size()
	if size == 0 {
		return nil, false, nil
	}
	start := int64(0)
	truncated := false
	if maxBytes > 0 && size > maxBytes {
		start = size - maxBytes
		truncated = true
	}
	if _, err := file.Seek(start, 0); err != nil {
		return nil, false, err
	}
	buf, err := io.ReadAll(file)
	if err != nil {
		return nil, false, err
	}
	return buf, truncated, nil
}

func (s *Session) trimTranscript() {
	if s.transcriptPath == "" || s.transcriptFile == nil {
		return
	}
	s.mu.Lock()
	size := s.transcriptSize
	s.mu.Unlock()
	if size <= s.opts.TranscriptTrimThreshold {
		return
	}
	_ = s.transcriptFile.Close()
	data, truncated, err := s.readTranscriptTail(s.opts.TranscriptMaxBytes)
	if err != nil {
		return
	}
	if !truncated {
		file, err := os.OpenFile(s.transcriptPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return
		}
		s.mu.Lock()
		s.transcriptFile = file
		s.transcriptSize = info.Size()
		s.mu.Unlock()
		return
	}
	tmp := s.transcriptPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	if err := os.Rename(tmp, s.transcriptPath); err != nil {
		return
	}
	file, err := os.OpenFile(s.transcriptPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return
	}
	s.mu.Lock()
	s.transcriptFile = file
	s.transcriptSize = info.Size()
	s.mu.Unlock()
}
