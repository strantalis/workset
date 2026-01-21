package main

import (
	"context"

	"github.com/strantalis/workset/pkg/worksetapi"
)

type SettingsDefaults struct {
	BaseBranch        string `json:"baseBranch"`
	Workspace         string `json:"workspace"`
	WorkspaceRoot     string `json:"workspaceRoot"`
	RepoStoreRoot     string `json:"repoStoreRoot"`
	SessionBackend    string `json:"sessionBackend"`
	SessionNameFormat string `json:"sessionNameFormat"`
	SessionTheme      string `json:"sessionTheme"`
	SessionTmuxStyle  string `json:"sessionTmuxStyle"`
	SessionTmuxLeft   string `json:"sessionTmuxLeft"`
	SessionTmuxRight  string `json:"sessionTmuxRight"`
	SessionScreenHard string `json:"sessionScreenHard"`
	Agent             string `json:"agent"`
}

type SettingsSnapshot struct {
	Defaults   SettingsDefaults `json:"defaults"`
	ConfigPath string           `json:"configPath"`
}

func (a *App) GetSettings() (SettingsSnapshot, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	cfg, info, err := a.service.GetConfig(ctx)
	if err != nil {
		return SettingsSnapshot{}, err
	}

	return SettingsSnapshot{
		ConfigPath: info.Path,
		Defaults: SettingsDefaults{
			BaseBranch:        cfg.Defaults.BaseBranch,
			Workspace:         cfg.Defaults.Workspace,
			WorkspaceRoot:     cfg.Defaults.WorkspaceRoot,
			RepoStoreRoot:     cfg.Defaults.RepoStoreRoot,
			SessionBackend:    cfg.Defaults.SessionBackend,
			SessionNameFormat: cfg.Defaults.SessionNameFormat,
			SessionTheme:      cfg.Defaults.SessionTheme,
			SessionTmuxStyle:  cfg.Defaults.SessionTmuxStyle,
			SessionTmuxLeft:   cfg.Defaults.SessionTmuxLeft,
			SessionTmuxRight:  cfg.Defaults.SessionTmuxRight,
			SessionScreenHard: cfg.Defaults.SessionScreenHard,
			Agent:             cfg.Defaults.Agent,
		},
	}, nil
}

func (a *App) SetDefaultSetting(key, value string) (worksetapi.ConfigSetResultJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, _, err := a.service.SetDefault(ctx, key, value)
	return result, err
}
