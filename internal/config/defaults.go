package config

func DefaultConfig() GlobalConfig {
	return GlobalConfig{
		Defaults: Defaults{
			BaseBranch: "main",
			Remotes: RemoteNames{
				Base:  "upstream",
				Write: "origin",
			},
			Parallelism: 8,
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
}

func ApplyRepoDefaults(repo *RepoConfig, defaults Defaults) {
	if repo.RepoDir == "" {
		repo.RepoDir = repo.Name
	}
	if repo.Remotes.Base.Name == "" {
		repo.Remotes.Base.Name = defaults.Remotes.Base
	}
	if repo.Remotes.Base.DefaultBranch == "" {
		repo.Remotes.Base.DefaultBranch = defaults.BaseBranch
	}
	if repo.Remotes.Write.Name == "" {
		repo.Remotes.Write.Name = defaults.Remotes.Write
	}
	if repo.Remotes.Write.DefaultBranch == "" {
		repo.Remotes.Write.DefaultBranch = defaults.BaseBranch
	}
}
