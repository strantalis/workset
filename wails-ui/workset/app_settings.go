package main

import "github.com/strantalis/workset/pkg/worksetapi"

type SettingsDefaults struct {
	Remote               string              `json:"remote"`
	BaseBranch           string              `json:"baseBranch"`
	Thread               string              `json:"thread"`
	WorksetRoot          string              `json:"worksetRoot"`
	RepoStoreRoot        string              `json:"repoStoreRoot"`
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
	ctx, svc := a.serviceContext()
	cfg, info, err := svc.GetConfig(ctx)
	if err != nil {
		return SettingsSnapshot{}, err
	}

	return SettingsSnapshot{
		ConfigPath: info.Path,
		Defaults: SettingsDefaults{
			Remote:               cfg.Defaults.Remote,
			BaseBranch:           cfg.Defaults.BaseBranch,
			Thread:               cfg.Defaults.Thread,
			WorksetRoot:          cfg.Defaults.WorksetRoot,
			RepoStoreRoot:        cfg.Defaults.RepoStoreRoot,
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
	ctx, svc := a.serviceContext()
	result, _, err := svc.SetDefault(ctx, key, value)
	return result, err
}

func (a *App) CheckAgentStatus(input AgentCheckRequest) (worksetapi.AgentCLIStatusJSON, error) {
	ctx, svc := a.serviceContext()
	return svc.GetAgentCLIStatus(ctx, input.Agent)
}

func (a *App) SetAgentCLIPath(input AgentCLIPathRequest) (worksetapi.AgentCLIStatusJSON, error) {
	ctx, svc := a.serviceContext()
	return svc.SetAgentCLIPath(ctx, input.Agent, input.Path)
}

func (a *App) ReloadLoginEnv() (worksetapi.EnvSnapshotResultJSON, error) {
	ctx, svc := a.serviceContext()
	return svc.ReloadLoginEnv(ctx)
}
