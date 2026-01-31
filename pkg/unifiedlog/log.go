package unifiedlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	Timestamp time.Time
	Component string
	Category  string
	Direction string
	Action    string
	Detail    string
	Len       int
	Hex       string
	ASCII     string
}

type Logger struct {
	component string
	file      *os.File
	mu        sync.Mutex
}

func DefaultLogDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workset", "terminal_logs"), nil
}

func Open(component, dir string) (*Logger, error) {
	if strings.TrimSpace(component) == "" {
		component = "unknown"
	}
	if dir == "" {
		var err error
		dir, err = DefaultLogDir()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("unified_%s.log", sanitizeComponent(component))
	path := filepath.Join(dir, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{component: component, file: file}, nil
}

func (l *Logger) Write(entry Entry) {
	if l == nil || l.file == nil {
		return
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.Component == "" {
		entry.Component = l.component
	}
	entry.Detail = sanitizeField(entry.Detail)
	entry.ASCII = sanitizeField(entry.ASCII)

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(
		l.file,
		"ts=%s component=%s category=%s dir=%s action=%s detail=%q len=%d hex=%s ascii=%q\n",
		entry.Timestamp.Format(time.RFC3339Nano),
		entry.Component,
		entry.Category,
		entry.Direction,
		entry.Action,
		entry.Detail,
		entry.Len,
		entry.Hex,
		entry.ASCII,
	)
}

func (l *Logger) Log(category, direction, action, detail string, seq []byte) {
	if l == nil {
		return
	}
	l.Write(Entry{
		Component: l.component,
		Category:  category,
		Direction: direction,
		Action:    action,
		Detail:    detail,
		Len:       len(seq),
		Hex:       fmt.Sprintf("%x", seq),
		ASCII:     string(seq),
	})
}

func sanitizeComponent(component string) string {
	if component == "" {
		return "unknown"
	}
	component = strings.TrimSpace(component)
	component = strings.ReplaceAll(component, " ", "_")
	component = strings.ReplaceAll(component, string(os.PathSeparator), "_")
	return component
}

func sanitizeField(value string) string {
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\r")
	value = strings.ReplaceAll(value, "\t", "\\t")
	return value
}
