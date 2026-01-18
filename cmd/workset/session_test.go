package main

import (
	"testing"

	"github.com/strantalis/workset/internal/workspace"
)

func TestResolveSessionTarget(t *testing.T) {
	state := workspace.State{
		CurrentBranch: "main",
		Sessions: map[string]workspace.SessionState{
			"workset:demo": {Backend: "tmux"},
			"custom":       {Backend: "screen"},
		},
	}

	name, entry, err := resolveSessionTarget(state, "custom", "workset:{workspace}", "demo")
	if err != nil {
		t.Fatalf("resolveSessionTarget: %v", err)
	}
	if name != "custom" || entry == nil || entry.Backend != "screen" {
		t.Fatalf("expected custom session, got %q %+v", name, entry)
	}

	name, entry, err = resolveSessionTarget(state, "", "workset:{workspace}", "demo")
	if err != nil {
		t.Fatalf("resolveSessionTarget: %v", err)
	}
	if name != "workset:demo" || entry == nil || entry.Backend != "tmux" {
		t.Fatalf("expected default session, got %q %+v", name, entry)
	}
}

func TestResolveSessionTargetErrors(t *testing.T) {
	state := workspace.State{CurrentBranch: "main"}
	if _, _, err := resolveSessionTarget(state, "", "workset:{workspace}", "demo"); err == nil {
		t.Fatalf("expected error for empty state")
	}

	state.Sessions = map[string]workspace.SessionState{
		"one": {Backend: "tmux"},
		"two": {Backend: "screen"},
	}
	if _, _, err := resolveSessionTarget(state, "", "workset:{workspace}", "demo"); err == nil {
		t.Fatalf("expected error for multiple sessions")
	}
}

func TestFormatSessionName(t *testing.T) {
	if got := formatSessionName("workset:{workspace}", "demo"); got != "workset:demo" {
		t.Fatalf("expected formatted name, got %q", got)
	}
	if got := formatSessionName("fixed", "demo"); got != "fixed" {
		t.Fatalf("expected fixed name, got %q", got)
	}
}
