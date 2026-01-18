package main

import (
	"context"
	"testing"

	"github.com/strantalis/workset/internal/config"
)

func TestResolveSessionThemeDisabled(t *testing.T) {
	theme := resolveSessionTheme(config.Defaults{})
	if theme.Enabled {
		t.Fatalf("expected theme disabled")
	}
}

func TestResolveSessionThemeWorksetDefaults(t *testing.T) {
	theme := resolveSessionTheme(config.Defaults{SessionTheme: "workset"})
	if !theme.Enabled {
		t.Fatalf("expected theme enabled")
	}
	if theme.Name != sessionThemeWorkset {
		t.Fatalf("expected theme name %q, got %q", sessionThemeWorkset, theme.Name)
	}
	if theme.TmuxStyle != defaultTmuxStatusStyle {
		t.Fatalf("expected tmux style %q, got %q", defaultTmuxStatusStyle, theme.TmuxStyle)
	}
	if theme.TmuxLeft != defaultTmuxStatusLeft {
		t.Fatalf("expected tmux left %q, got %q", defaultTmuxStatusLeft, theme.TmuxLeft)
	}
	if theme.TmuxRight != defaultTmuxStatusRight {
		t.Fatalf("expected tmux right %q, got %q", defaultTmuxStatusRight, theme.TmuxRight)
	}
	if theme.ScreenHardstatus != defaultScreenHardstatus {
		t.Fatalf("expected screen hardstatus %q, got %q", defaultScreenHardstatus, theme.ScreenHardstatus)
	}
}

func TestResolveSessionThemeOverrides(t *testing.T) {
	theme := resolveSessionTheme(config.Defaults{
		SessionTheme:      "workset",
		SessionTmuxStyle:  "bg=red,fg=white",
		SessionTmuxLeft:   "left",
		SessionTmuxRight:  "right",
		SessionScreenHard: "alwayslastline custom",
	})
	if theme.TmuxStyle != "bg=red,fg=white" {
		t.Fatalf("expected tmux style override, got %q", theme.TmuxStyle)
	}
	if theme.TmuxLeft != "left" {
		t.Fatalf("expected tmux left override, got %q", theme.TmuxLeft)
	}
	if theme.TmuxRight != "right" {
		t.Fatalf("expected tmux right override, got %q", theme.TmuxRight)
	}
	if theme.ScreenHardstatus != "alwayslastline custom" {
		t.Fatalf("expected screen hardstatus override, got %q", theme.ScreenHardstatus)
	}
}

func TestApplySessionThemeTmux(t *testing.T) {
	runner := &fakeRunner{}
	theme := sessionTheme{
		Name:      "custom",
		Enabled:   true,
		TmuxStyle: "bg=black,fg=white",
		TmuxLeft:  "left",
		TmuxRight: "right",
	}
	if err := applySessionTheme(context.Background(), runner, sessionBackendTmux, "demo", theme); err != nil {
		t.Fatalf("applySessionTheme: %v", err)
	}
	if len(runner.commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(runner.commands))
	}
	assertArgs(t, runner.commands[0].Args, []string{"set-option", "-t", "demo", "status-style", "bg=black,fg=white"})
	assertArgs(t, runner.commands[1].Args, []string{"set-option", "-t", "demo", "status-left", "left"})
	assertArgs(t, runner.commands[2].Args, []string{"set-option", "-t", "demo", "status-right", "right"})
}

func TestApplySessionThemeScreen(t *testing.T) {
	runner := &fakeRunner{}
	theme := sessionTheme{
		Name:             "custom",
		Enabled:          true,
		ScreenHardstatus: "alwayslastline workset %n %t",
	}
	if err := applySessionTheme(context.Background(), runner, sessionBackendScreen, "demo", theme); err != nil {
		t.Fatalf("applySessionTheme: %v", err)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.commands))
	}
	assertArgs(t, runner.commands[0].Args, []string{"-S", "demo", "-X", "hardstatus", "alwayslastline", "workset", "%n", "%t"})
}
