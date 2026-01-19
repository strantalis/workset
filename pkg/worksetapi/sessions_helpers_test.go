package worksetapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/strantalis/workset/internal/session"
	"github.com/strantalis/workset/internal/workspace"
)

type fakeRunner struct {
	results  []session.CommandResult
	errs     []error
	commands []session.CommandSpec
}

func (f *fakeRunner) LookPath(_ string) error {
	return nil
}

func (f *fakeRunner) Run(_ context.Context, spec session.CommandSpec) (session.CommandResult, error) {
	f.commands = append(f.commands, spec)
	var result session.CommandResult
	var err error
	if len(f.results) > 0 {
		result = f.results[0]
		f.results = f.results[1:]
	}
	if len(f.errs) > 0 {
		err = f.errs[0]
		f.errs = f.errs[1:]
	}
	return result, err
}

func TestResolveSessionTarget(t *testing.T) {
	state := workspace.State{
		CurrentBranch: "main",
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux"},
			"custom":       {Backend: "screen"},
		},
	}

	name, entry, err := resolveSessionTarget(state, "custom", "workset-{workspace}", "demo")
	if err != nil {
		t.Fatalf("resolveSessionTarget: %v", err)
	}
	if name != "custom" || entry == nil || entry.Backend != "screen" {
		t.Fatalf("expected custom session, got %q %+v", name, entry)
	}

	name, entry, err = resolveSessionTarget(state, "", "workset-{workspace}", "demo")
	if err != nil {
		t.Fatalf("resolveSessionTarget: %v", err)
	}
	if name != "workset-demo" || entry == nil || entry.Backend != "tmux" {
		t.Fatalf("expected default session, got %q %+v", name, entry)
	}
}

func TestResolveSessionTargetErrors(t *testing.T) {
	state := workspace.State{CurrentBranch: "main"}
	if _, _, err := resolveSessionTarget(state, "", "workset-{workspace}", "demo"); err == nil {
		t.Fatalf("expected error for empty state")
	}

	state.Sessions = map[string]workspace.SessionState{
		"one": {Backend: "tmux"},
		"two": {Backend: "screen"},
	}
	if _, _, err := resolveSessionTarget(state, "", "workset-{workspace}", "demo"); err == nil {
		t.Fatalf("expected error for multiple sessions")
	}
}

func TestFormatSessionName(t *testing.T) {
	if got := formatSessionName("workset-{workspace}", "demo"); got != "workset-demo" {
		t.Fatalf("expected formatted name, got %q", got)
	}
	if got := formatSessionName("fixed", "demo"); got != "fixed" {
		t.Fatalf("expected fixed name, got %q", got)
	}
}

func TestEnsureSessionNameAvailableCollision(t *testing.T) {
	state := workspace.State{
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "screen"},
		},
	}
	runner := &fakeRunner{}
	err := ensureSessionNameAvailable(context.Background(), runner, state, "workset-demo", session.BackendTmux)
	if err == nil {
		t.Fatalf("expected collision error")
	}
}

func TestEnsureSessionNameAvailableRunning(t *testing.T) {
	state := workspace.State{
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux"},
		},
	}
	runner := &fakeRunner{
		results: []session.CommandResult{{ExitCode: 0}},
	}
	err := ensureSessionNameAvailable(context.Background(), runner, state, "workset-demo", session.BackendTmux)
	if err == nil {
		t.Fatalf("expected running session error")
	}
}

func TestEnsureSessionNameAvailableStopped(t *testing.T) {
	state := workspace.State{
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux"},
		},
	}
	runner := &fakeRunner{
		results: []session.CommandResult{{ExitCode: 1}},
		errs:    []error{errors.New("exit status 1")},
	}
	err := ensureSessionNameAvailable(context.Background(), runner, state, "workset-demo", session.BackendTmux)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAllowAttachAfterStart(t *testing.T) {
	allowed, note := allowAttachAfterStart(session.BackendTmux, true)
	if !allowed || note != "" {
		t.Fatalf("expected attach allowed for tmux, got allowed=%v note=%q", allowed, note)
	}

	allowed, note = allowAttachAfterStart(session.BackendExec, true)
	if allowed || note == "" {
		t.Fatalf("expected attach ignored for exec, got allowed=%v note=%q", allowed, note)
	}

	allowed, note = allowAttachAfterStart(session.BackendTmux, false)
	if allowed || note != "" {
		t.Fatalf("expected attach disabled, got allowed=%v note=%q", allowed, note)
	}
}

func TestMarkSessionAttached(t *testing.T) {
	when := time.Date(2026, 1, 18, 12, 0, 0, 0, time.UTC)
	state := workspace.State{
		Sessions: map[string]workspace.SessionState{
			"workset-demo": {Backend: "tmux"},
		},
	}
	if !markSessionAttached(&state, "workset-demo", when) {
		t.Fatalf("expected session to be updated")
	}
	if state.Sessions["workset-demo"].LastAttached != when.Format(time.RFC3339) {
		t.Fatalf("expected last_attached to be updated, got %q", state.Sessions["workset-demo"].LastAttached)
	}
}

func TestMarkSessionAttachedMissing(t *testing.T) {
	state := workspace.State{}
	if markSessionAttached(&state, "missing", time.Now()) {
		t.Fatalf("expected no update for missing session")
	}
}
