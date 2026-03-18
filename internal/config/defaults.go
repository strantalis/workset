package config

import (
	"os"
	"path/filepath"
)

func DefaultConfig() GlobalConfig {
	return GlobalConfig{
		ConfigVersion: CurrentGlobalConfigVersion,
		Defaults: Defaults{
			Remote:               "origin",
			BaseBranch:           "main",
			Thread:               "",
			WorksetRoot:          defaultWorksetRoot(),
			RepoStoreRoot:        defaultRepoStoreRoot(),
			Agent:                "codex",
			AgentModel:           "",
			TerminalIdleTimeout:  "0",
			TerminalProtocolLog:  "off",
			TerminalDebugOverlay: "off",
			TerminalKeybindings:  map[string][]string{},
		},
		GitHub: GitHubConfig{
			CLIPath: "",
		},
		Agent: AgentConfig{
			CLIPath: "",
		},
		Hooks: HooksConfig{
			Enabled: true,
			OnError: "fail",
			RepoHooks: RepoHooksConfig{
				TrustedRepos: []string{},
			},
			Items: []HookSpec{},
		},
		Repos:        map[string]RegisteredRepo{},
		Workspaces:   map[string]WorkspaceRef{},
		WorksetRepos: map[string][]string{},
	}
}

func (cfg *GlobalConfig) EnsureMaps() {
	if cfg.Repos == nil {
		cfg.Repos = map[string]RegisteredRepo{}
	}
	if cfg.Workspaces == nil {
		cfg.Workspaces = map[string]WorkspaceRef{}
	}
	if cfg.WorksetRepos == nil {
		cfg.WorksetRepos = map[string][]string{}
	}
	if cfg.Hooks.RepoHooks.TrustedRepos == nil {
		cfg.Hooks.RepoHooks.TrustedRepos = []string{}
	}
	if cfg.Hooks.Items == nil {
		cfg.Hooks.Items = []HookSpec{}
	}
}

func ApplyRepoDefaults(repo *RepoConfig, defaults Defaults) {
	if repo.RepoDir == "" {
		repo.RepoDir = repo.Name
	}
}

func defaultWorksetRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".workset")
}

func defaultRepoStoreRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".workset", "repos")
}
