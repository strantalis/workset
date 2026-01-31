package unifiedlog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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
	handler   *handler
	log       *slog.Logger
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
	h := &handler{
		w:     file,
		mu:    &sync.Mutex{},
		attrs: []slog.Attr{slog.String("component", component)},
	}
	return &Logger{
		component: component,
		handler:   h,
		log:       slog.New(h),
	}, nil
}

func (l *Logger) Write(entry Entry) {
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
	_ = l.handler.Handle(context.Background(), record)
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

type handler struct {
	w     io.Writer
	mu    *sync.Mutex
	attrs []slog.Attr
	group string
}

func (h *handler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *handler) Handle(_ context.Context, record slog.Record) error {
	if h == nil || h.w == nil {
		return nil
	}
	fields := map[string]string{
		"component": "",
		"category":  "",
		"dir":       "none",
		"action":    "",
		"detail":    "",
		"len":       "0",
		"hex":       "",
		"ascii":     "",
	}
	for _, attr := range h.attrs {
		h.addAttr(fields, "", attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		h.addAttr(fields, "", attr)
		return true
	})
	detail := fields["detail"]
	if detail == "" {
		detail = record.Message
	}
	detail = sanitizeField(detail)
	ascii := sanitizeField(fields["ascii"])
	ts := record.Time
	if ts.IsZero() {
		ts = time.Now()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_, _ = fmt.Fprintf(
		h.w,
		"ts=%s component=%s category=%s dir=%s action=%s detail=%q len=%s hex=%s ascii=%q\n",
		ts.Format(time.RFC3339Nano),
		fields["component"],
		fields["category"],
		fields["dir"],
		fields["action"],
		detail,
		fields["len"],
		fields["hex"],
		ascii,
	)
	return nil
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	combined := append([]slog.Attr{}, h.attrs...)
	combined = append(combined, attrs...)
	return &handler{
		w:     h.w,
		mu:    h.mu,
		attrs: combined,
		group: h.group,
	}
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	group := name
	if h.group != "" {
		group = h.group + "." + name
	}
	return &handler{
		w:     h.w,
		mu:    h.mu,
		attrs: h.attrs,
		group: group,
	}
}

func (h *handler) addAttr(fields map[string]string, prefix string, attr slog.Attr) {
	key := attr.Key
	if prefix != "" {
		key = prefix + "." + key
	}
	if h.group != "" {
		key = h.group + "." + key
	}
	if attr.Value.Kind() == slog.KindGroup {
		for _, child := range attr.Value.Group() {
			h.addAttr(fields, key, child)
		}
		return
	}
	value := attrToString(attr.Value)
	if key == "" {
		return
	}
	fields[key] = value
}

func attrToString(value slog.Value) string {
	switch value.Kind() {
	case slog.KindString:
		return value.String()
	case slog.KindInt64:
		return fmt.Sprintf("%d", value.Int64())
	case slog.KindUint64:
		return fmt.Sprintf("%d", value.Uint64())
	case slog.KindFloat64:
		return fmt.Sprintf("%v", value.Float64())
	case slog.KindBool:
		if value.Bool() {
			return "true"
		}
		return "false"
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(value.Any())
	}
}
