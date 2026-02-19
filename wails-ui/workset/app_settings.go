package main

import (
	"context"

	"github.com/strantalis/workset/pkg/worksetapi"
)

type SettingsDefaults struct {
	Remote               string              `json:"remote"`
	BaseBranch           string              `json:"baseBranch"`
	Workspace            string              `json:"workspace"`
	WorkspaceRoot        string              `json:"workspaceRoot"`
	RepoStoreRoot        string              `json:"repoStoreRoot"`
	SessionBackend       string              `json:"sessionBackend"`
	SessionNameFormat    string              `json:"sessionNameFormat"`
	SessionTheme         string              `json:"sessionTheme"`
	SessionTmuxStyle     string              `json:"sessionTmuxStyle"`
	SessionTmuxLeft      string              `json:"sessionTmuxLeft"`
	SessionTmuxRight     string              `json:"sessionTmuxRight"`
	SessionScreenHard    string              `json:"sessionScreenHard"`
	Agent                string              `json:"agent"`
	AgentModel           string              `json:"agentModel"`
	TerminalIdleTimeout  string              `json:"terminalIdleTimeout"`
	TerminalProtocolLog  string              `json:"terminalProtocolLog"`
	TerminalDebugOverlay string              `json:"terminalDebugOverlay"`
	TerminalKeybindings  map[string][]string `json:"terminalKeybindings"`
}

type SettingsSnapshot struct {
	Defaults   SettingsDefaults `json:"defaults"`
	ConfigPath string           `json:"configPath"`
}

type AgentCheckRequest struct {
	Agent string `json:"agent"`
}

type AgentCLIPathRequest struct {
	Agent string `json:"agent"`
	Path  string `json:"path"`
}

func (a *App) GetSettings() (SettingsSnapshot, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()

	cfg, info, err := a.service.GetConfig(ctx)
	if err != nil {
		return SettingsSnapshot{}, err
	}

	return SettingsSnapshot{
		ConfigPath: info.Path,
		Defaults: SettingsDefaults{
			Remote:               cfg.Defaults.Remote,
			BaseBranch:           cfg.Defaults.BaseBranch,
			Workspace:            cfg.Defaults.Workspace,
			WorkspaceRoot:        cfg.Defaults.WorkspaceRoot,
			RepoStoreRoot:        cfg.Defaults.RepoStoreRoot,
			SessionBackend:       cfg.Defaults.SessionBackend,
			SessionNameFormat:    cfg.Defaults.SessionNameFormat,
			SessionTheme:         cfg.Defaults.SessionTheme,
			SessionTmuxStyle:     cfg.Defaults.SessionTmuxStyle,
			SessionTmuxLeft:      cfg.Defaults.SessionTmuxLeft,
			SessionTmuxRight:     cfg.Defaults.SessionTmuxRight,
			SessionScreenHard:    cfg.Defaults.SessionScreenHard,
			Agent:                cfg.Defaults.Agent,
			AgentModel:           cfg.Defaults.AgentModel,
			TerminalIdleTimeout:  cfg.Defaults.TerminalIdleTimeout,
			TerminalProtocolLog:  cfg.Defaults.TerminalProtocolLog,
			TerminalDebugOverlay: cfg.Defaults.TerminalDebugOverlay,
			TerminalKeybindings:  cfg.Defaults.TerminalKeybindings,
		},
	}, nil
}

func (a *App) SetDefaultSetting(key, value string) (worksetapi.ConfigSetResultJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	result, _, err := a.service.SetDefault(ctx, key, value)
	return result, err
}

func (a *App) CheckAgentStatus(input AgentCheckRequest) (worksetapi.AgentCLIStatusJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.GetAgentCLIStatus(ctx, input.Agent)
}

func (a *App) SetAgentCLIPath(input AgentCLIPathRequest) (worksetapi.AgentCLIStatusJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.SetAgentCLIPath(ctx, input.Agent, input.Path)
}

func (a *App) ReloadLoginEnv() (worksetapi.EnvSnapshotResultJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.ReloadLoginEnv(ctx)
}
