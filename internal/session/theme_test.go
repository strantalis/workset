package session

import (
	"context"
	"testing"

	"github.com/strantalis/workset/internal/config"
)

func TestResolveThemeDisabled(t *testing.T) {
	theme := ResolveTheme(config.Defaults{})
	if theme.Enabled {
		t.Fatalf("expected theme disabled")
	}
}

func TestResolveThemeWorksetDefaults(t *testing.T) {
	theme := ResolveTheme(config.Defaults{SessionTheme: "workset"})
	if !theme.Enabled {
		t.Fatalf("expected theme enabled")
	}
	if theme.Name != ThemeWorkset {
		t.Fatalf("expected theme name %q, got %q", ThemeWorkset, theme.Name)
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

func TestResolveThemeOverrides(t *testing.T) {
	theme := ResolveTheme(config.Defaults{
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

func TestApplyThemeTmux(t *testing.T) {
	runner := &fakeRunner{}
	theme := Theme{
		Name:      "custom",
		Enabled:   true,
		TmuxStyle: "bg=black,fg=white",
		TmuxLeft:  "left",
		TmuxRight: "right",
	}
	if err := ApplyTheme(context.Background(), runner, BackendTmux, "demo", theme); err != nil {
		t.Fatalf("ApplyTheme: %v", err)
	}
	if len(runner.commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(runner.commands))
	}
	assertArgs(t, runner.commands[0].Args, []string{"set-option", "-t", "demo", "status-style", "bg=black,fg=white"})
	assertArgs(t, runner.commands[1].Args, []string{"set-option", "-t", "demo", "status-left", "left"})
	assertArgs(t, runner.commands[2].Args, []string{"set-option", "-t", "demo", "status-right", "right"})
}

func TestApplyThemeScreen(t *testing.T) {
	runner := &fakeRunner{}
	theme := Theme{
		Name:             "custom",
		Enabled:          true,
		ScreenHardstatus: "alwayslastline workset %n %t",
	}
	if err := ApplyTheme(context.Background(), runner, BackendScreen, "demo", theme); err != nil {
		t.Fatalf("ApplyTheme: %v", err)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.commands))
	}
	assertArgs(t, runner.commands[0].Args, []string{"-S", "demo", "-X", "hardstatus", "alwayslastline", "workset", "%n", "%t"})
}
