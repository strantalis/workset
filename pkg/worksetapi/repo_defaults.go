package worksetapi

import (
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/ops"
)

func resolveRepoDefaults(cfg config.GlobalConfig, repoName string) ops.RepoDefaults {
	alias, ok := cfg.Repos[repoName]
	if !ok {
		return ops.RepoDefaults{
			Remote:        cfg.Defaults.Remote,
			DefaultBranch: cfg.Defaults.BaseBranch,
		}
	}
	remote := strings.TrimSpace(alias.Remote)
	if remote == "" {
		remote = cfg.Defaults.Remote
	}
	branch := strings.TrimSpace(alias.DefaultBranch)
	if branch == "" {
		branch = cfg.Defaults.BaseBranch
	}
	return ops.RepoDefaults{
		Remote:        remote,
		DefaultBranch: branch,
	}
}

func repoDefaultBranches(ws config.WorkspaceConfig, cfg config.GlobalConfig) map[string]string {
	branches := make(map[string]string, len(ws.Repos))
	for _, repo := range ws.Repos {
		branches[repo.Name] = resolveRepoDefaults(cfg, repo.Name).DefaultBranch
	}
	return branches
}

func repoDefaultsMap(ws config.WorkspaceConfig, cfg config.GlobalConfig) map[string]ops.RepoDefaults {
	defaults := make(map[string]ops.RepoDefaults, len(ws.Repos))
	for _, repo := range ws.Repos {
		defaults[repo.Name] = resolveRepoDefaults(cfg, repo.Name)
	}
	return defaults
}
