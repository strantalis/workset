package main

import (
	"context"
	"errors"
	"testing"
)

type fakeRunner struct {
	available map[string]bool
	results   []commandResult
	errs      []error
	commands  []commandSpec
}

func (f *fakeRunner) LookPath(name string) error {
	if f.available[name] {
		return nil
	}
	return errors.New("missing")
}

func (f *fakeRunner) Run(_ context.Context, spec commandSpec) (commandResult, error) {
	f.commands = append(f.commands, spec)
	var result commandResult
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

func TestResolveSessionBackendAuto(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{"tmux": true}}
	backend, err := resolveSessionBackend(sessionBackendAuto, runner)
	if err != nil {
		t.Fatalf("resolveSessionBackend: %v", err)
	}
	if backend != sessionBackendTmux {
		t.Fatalf("expected tmux, got %s", backend)
	}

	runner = &fakeRunner{available: map[string]bool{"screen": true}}
	backend, err = resolveSessionBackend(sessionBackendAuto, runner)
	if err != nil {
		t.Fatalf("resolveSessionBackend: %v", err)
	}
	if backend != sessionBackendScreen {
		t.Fatalf("expected screen, got %s", backend)
	}

	runner = &fakeRunner{available: map[string]bool{}}
	backend, err = resolveSessionBackend(sessionBackendAuto, runner)
	if err != nil {
		t.Fatalf("resolveSessionBackend: %v", err)
	}
	if backend != sessionBackendExec {
		t.Fatalf("expected exec, got %s", backend)
	}
}

func TestParseSessionBackend(t *testing.T) {
	backend, err := parseSessionBackend("")
	if err != nil {
		t.Fatalf("parseSessionBackend: %v", err)
	}
	if backend != sessionBackendAuto {
		t.Fatalf("expected auto, got %s", backend)
	}

	backend, err = parseSessionBackend("Screen")
	if err != nil {
		t.Fatalf("parseSessionBackend: %v", err)
	}
	if backend != sessionBackendScreen {
		t.Fatalf("expected screen, got %s", backend)
	}

	if _, err := parseSessionBackend("bogus"); err == nil {
		t.Fatalf("expected error for invalid backend")
	}
}

func TestStartSessionTmuxBuildsCommand(t *testing.T) {
	runner := &fakeRunner{}
	if err := startSession(context.Background(), runner, sessionBackendTmux, "/tmp/ws", "demo", []string{"zsh"}, nil, false); err != nil {
		t.Fatalf("startSession: %v", err)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.commands))
	}
	cmd := runner.commands[0]
	if cmd.Name != "tmux" {
		t.Fatalf("expected tmux, got %s", cmd.Name)
	}
	expected := []string{"new-session", "-d", "-s", "demo", "-c", "/tmp/ws", "zsh"}
	assertArgs(t, cmd.Args, expected)
}

func TestStartSessionScreenBuildsCommand(t *testing.T) {
	runner := &fakeRunner{}
	if err := startSession(context.Background(), runner, sessionBackendScreen, "/tmp/ws", "demo", []string{"bash"}, nil, false); err != nil {
		t.Fatalf("startSession: %v", err)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.commands))
	}
	cmd := runner.commands[0]
	if cmd.Name != "screen" {
		t.Fatalf("expected screen, got %s", cmd.Name)
	}
	expected := []string{"-dmS", "demo", "bash"}
	assertArgs(t, cmd.Args, expected)
	if cmd.Dir != "/tmp/ws" {
		t.Fatalf("expected dir /tmp/ws, got %s", cmd.Dir)
	}
}

func TestAttachSessionUsesSwitchWhenInTmux(t *testing.T) {
	t.Setenv("TMUX", "1")
	runner := &fakeRunner{}
	if err := attachSession(context.Background(), runner, sessionBackendTmux, "demo"); err != nil {
		t.Fatalf("attachSession: %v", err)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.commands))
	}
	assertArgs(t, runner.commands[0].Args, []string{"switch-client", "-t", "demo"})
}

func TestAttachSessionUsesScreenXWhenInScreen(t *testing.T) {
	t.Setenv("STY", "1")
	runner := &fakeRunner{}
	if err := attachSession(context.Background(), runner, sessionBackendScreen, "demo"); err != nil {
		t.Fatalf("attachSession: %v", err)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.commands))
	}
	assertArgs(t, runner.commands[0].Args, []string{"-x", "demo"})
}

func TestSessionExistsTmux(t *testing.T) {
	runner := &fakeRunner{
		results: []commandResult{{ExitCode: 0}},
	}
	exists, err := sessionExists(context.Background(), runner, sessionBackendTmux, "demo")
	if err != nil {
		t.Fatalf("sessionExists: %v", err)
	}
	if !exists {
		t.Fatalf("expected session to exist")
	}

	runner = &fakeRunner{
		results: []commandResult{{ExitCode: 1}},
		errs:    []error{errors.New("exit status 1")},
	}
	exists, err = sessionExists(context.Background(), runner, sessionBackendTmux, "demo")
	if err != nil {
		t.Fatalf("sessionExists: %v", err)
	}
	if exists {
		t.Fatalf("expected session to be missing")
	}
}

func TestScreenHasSession(t *testing.T) {
	output := "There is a screen on:\n\t1234.demo\t(Detached)\n1 Socket in /tmp.\n"
	if !screenHasSession(output, "demo") {
		t.Fatalf("expected screen session to be found")
	}
	if screenHasSession(output, "other") {
		t.Fatalf("did not expect session to be found")
	}
}

func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d args, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected arg %d to be %q, got %q", i, want[i], got[i])
		}
	}
}
