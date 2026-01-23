package config

import (
	"os"
	"path/filepath"
)

func DefaultConfig() GlobalConfig {
	return GlobalConfig{
		Defaults: Defaults{
			Remote:              "origin",
			BaseBranch:          "main",
			Workspace:           "",
			WorkspaceRoot:       defaultWorkspaceRoot(),
			RepoStoreRoot:       defaultRepoStoreRoot(),
			SessionBackend:      "auto",
			SessionNameFormat:   "workset-{workspace}",
			SessionTheme:        "",
			SessionTmuxStyle:    "",
			SessionTmuxLeft:     "",
			SessionTmuxRight:    "",
			SessionScreenHard:   "",
			Agent:               "codex",
			TerminalRenderer:    "auto",
			TerminalIdleTimeout: "30m",
		},
		Hooks: HooksConfig{
			Enabled: true,
			OnError: "fail",
			RepoHooks: RepoHooksConfig{
				TrustedRepos: []string{},
			},
			Items: []HookSpec{},
		},
		Repos:      map[string]RepoAlias{},
		Groups:     map[string]Group{},
		Workspaces: map[string]WorkspaceRef{},
	}
}

func (cfg *GlobalConfig) EnsureMaps() {
	if cfg.Repos == nil {
		cfg.Repos = map[string]RepoAlias{}
	}
	if cfg.Groups == nil {
		cfg.Groups = map[string]Group{}
	}
	if cfg.Workspaces == nil {
		cfg.Workspaces = map[string]WorkspaceRef{}
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

func defaultWorkspaceRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".workset", "workspaces")
}

func defaultRepoStoreRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".workset", "repos")
}
