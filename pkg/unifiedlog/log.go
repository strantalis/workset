package unifiedlog

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
	handler   slog.Handler
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
	return &Logger{
		component: component,
		file:      file,
		handler:   slog.NewTextHandler(file, &slog.HandlerOptions{Level: slog.LevelInfo}),
	}, nil
}

func (l *Logger) Write(ctx context.Context, entry Entry) {
	if l == nil || l.handler == nil {
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

	record := slog.NewRecord(entry.Timestamp, slog.LevelInfo, "protocol", 0)
	record.AddAttrs(
		slog.String("component", entry.Component),
		slog.String("category", entry.Category),
		slog.String("dir", entry.Direction),
		slog.String("action", entry.Action),
		slog.String("detail", entry.Detail),
		slog.Int("len", entry.Len),
		slog.String("hex", entry.Hex),
		slog.String("ascii", entry.ASCII),
	)
	if ctx == nil {
		return
	}
	_ = l.handler.Handle(ctx, record)
}

func (l *Logger) Log(ctx context.Context, category, direction, action, detail string, seq []byte) {
	if l == nil {
		return
	}
	l.Write(ctx, Entry{
		Component: l.component,
		Category:  category,
		Direction: direction,
		Action:    action,
		Detail:    detail,
		Len:       len(seq),
		Hex:       hex.EncodeToString(seq),
		ASCII:     string(seq),
	})
}

func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
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
