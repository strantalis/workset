package session

import (
	"context"
	"strings"

	"github.com/strantalis/workset/internal/config"
)

const ThemeWorkset = "workset"

const (
	defaultTmuxStatusStyle  = "bg=colour235,fg=colour250"
	defaultTmuxStatusLeft   = " #[fg=colour39]workset #[fg=colour250]#S "
	defaultTmuxStatusRight  = " #[fg=colour244]%Y-%m-%d %H:%M "
	defaultScreenHardstatus = "alwayslastline workset %n %t %=%H:%M %d-%b-%y"
)

type Theme struct {
	Name             string
	TmuxStyle        string
	TmuxLeft         string
	TmuxRight        string
	ScreenHardstatus string
	Enabled          bool
}

func ResolveTheme(defaults config.Defaults) Theme {
	name := strings.TrimSpace(defaults.SessionTheme)
	if name == "" {
		return Theme{}
	}
	theme := Theme{
		Name:    name,
		Enabled: true,
	}
	if strings.EqualFold(name, ThemeWorkset) {
		theme.Name = ThemeWorkset
		theme.TmuxStyle = defaultTmuxStatusStyle
		theme.TmuxLeft = defaultTmuxStatusLeft
		theme.TmuxRight = defaultTmuxStatusRight
		theme.ScreenHardstatus = defaultScreenHardstatus
	}
	if defaults.SessionTmuxStyle != "" {
		theme.TmuxStyle = defaults.SessionTmuxStyle
	}
	if defaults.SessionTmuxLeft != "" {
		theme.TmuxLeft = defaults.SessionTmuxLeft
	}
	if defaults.SessionTmuxRight != "" {
		theme.TmuxRight = defaults.SessionTmuxRight
	}
	if defaults.SessionScreenHard != "" {
		theme.ScreenHardstatus = defaults.SessionScreenHard
	}
	return theme
}

func ThemeNotice(theme Theme, backend Backend) (label string, hint string) {
	if backend != BackendTmux && backend != BackendScreen {
		return "", ""
	}
	if !theme.Enabled {
		return "", "set defaults.session_theme=workset to enable the workset status line"
	}
	return theme.Name, ""
}

func ApplyTheme(ctx context.Context, runner Runner, backend Backend, name string, theme Theme) error {
	if !theme.Enabled {
		return nil
	}
	switch backend {
	case BackendTmux:
		if err := applyTmuxTheme(ctx, runner, name, theme); err != nil {
			return err
		}
	case BackendScreen:
		if err := applyScreenTheme(ctx, runner, name, theme); err != nil {
			return err
		}
	}
	return nil
}

func applyTmuxTheme(ctx context.Context, runner Runner, name string, theme Theme) error {
	if theme.TmuxStyle != "" {
		if _, err := runner.Run(ctx, CommandSpec{
			Name: "tmux",
			Args: []string{"set-option", "-t", name, "status-style", theme.TmuxStyle},
		}); err != nil {
			return err
		}
	}
	if theme.TmuxLeft != "" {
		if _, err := runner.Run(ctx, CommandSpec{
			Name: "tmux",
			Args: []string{"set-option", "-t", name, "status-left", theme.TmuxLeft},
		}); err != nil {
			return err
		}
	}
	if theme.TmuxRight != "" {
		if _, err := runner.Run(ctx, CommandSpec{
			Name: "tmux",
			Args: []string{"set-option", "-t", name, "status-right", theme.TmuxRight},
		}); err != nil {
			return err
		}
	}
	return nil
}

func applyScreenTheme(ctx context.Context, runner Runner, name string, theme Theme) error {
	if theme.ScreenHardstatus == "" {
		return nil
	}
	args := []string{"-S", name, "-X", "hardstatus"}
	args = append(args, strings.Fields(theme.ScreenHardstatus)...)
	_, err := runner.Run(ctx, CommandSpec{Name: "screen", Args: args})
	return err
}
