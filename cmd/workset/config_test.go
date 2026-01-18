package main

import (
	"testing"

	"github.com/strantalis/workset/internal/config"
)

func TestSetGlobalDefaultSessionBackend(t *testing.T) {
	cfg := config.DefaultConfig()
	if err := setGlobalDefault(&cfg, "defaults.session_backend", "Tmux"); err != nil {
		t.Fatalf("setGlobalDefault: %v", err)
	}
	if cfg.Defaults.SessionBackend != "tmux" {
		t.Fatalf("expected normalized backend, got %q", cfg.Defaults.SessionBackend)
	}

	if err := setGlobalDefault(&cfg, "defaults.session_backend", "bogus"); err == nil {
		t.Fatalf("expected error for invalid backend")
	}
}
