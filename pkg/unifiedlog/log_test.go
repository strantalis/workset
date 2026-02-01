package unifiedlog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeComponent(t *testing.T) {
	component := " one two" + string(os.PathSeparator) + "three "
	got := sanitizeComponent(component)
	if got != "one_two_three" {
		t.Fatalf("unexpected component: got %q want %q", got, "one_two_three")
	}
	if sanitizeComponent("") != "unknown" {
		t.Fatal("expected empty component to map to unknown")
	}
}

func TestSanitizeField(t *testing.T) {
	got := sanitizeField("a\nb\rc\t")
	if got != "a\\nb\\rc\\t" {
		t.Fatalf("unexpected sanitized field: got %q", got)
	}
}

func TestOpenCreatesFileAndLogs(t *testing.T) {
	dir := t.TempDir()
	logger, err := Open("term/emu log", dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = logger.Close() }()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one log file, got %d", len(entries))
	}
	if entries[0].Name() != "unified_term_emu_log.log" {
		t.Fatalf("unexpected log filename: %q", entries[0].Name())
	}

	logger.Log(context.Background(), "proto", "out", "write", "detail\n", []byte("hi\t"))
	if err := logger.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, "detail=") || !strings.Contains(output, "hex=") || !strings.Contains(output, "ascii=") {
		t.Fatalf("expected log output to include detail/hex/ascii fields, got %q", output)
	}
}

func TestLoggerNilSafe(t *testing.T) {
	var logger *Logger
	logger.Write(context.Background(), Entry{})
	logger.Log(context.Background(), "proto", "out", "write", "detail", []byte("hi"))
	if err := logger.Close(); err != nil {
		t.Fatalf("expected nil close to succeed, got %v", err)
	}

	logger = &Logger{}
	logger.Write(context.Background(), Entry{})
	if err := logger.Close(); err != nil {
		t.Fatalf("expected close with nil file to succeed, got %v", err)
	}
}
