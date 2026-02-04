package worksetapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/strantalis/workset/internal/config"
)

func (s *Service) migrateLegacyGroupRemotes(ctx context.Context, cfg *config.GlobalConfig, configPath string) error {
	if changed := s.applyLegacyGroupRemotesWithWarnings(cfg, true); changed {
		if updater, ok := s.configs.(ConfigUpdater); ok {
			_, err := updater.Update(ctx, configPath, func(target *config.GlobalConfig, info config.GlobalConfigLoadInfo) error {
				s.applyLegacyGroupRemotesWithWarnings(target, false)
				return nil
			})
			return err
		}
		if err := s.configs.Save(ctx, configPath, *cfg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) applyLegacyGroupRemotes(cfg *config.GlobalConfig) bool {
	return s.applyLegacyGroupRemotesWithWarnings(cfg, true)
}

func (s *Service) applyLegacyGroupRemotesWithWarnings(cfg *config.GlobalConfig, logWarnings bool) bool {
	changed := false
	for groupName, group := range cfg.Groups {
		if len(group.Members) == 0 {
			continue
		}
		groupChanged := false
		for i := range group.Members {
			member := group.Members[i]
			if member.LegacyRemotes == nil {
				continue
			}
			remote, branch, warnings := resolveLegacyRemoteDefaults(cfg.Defaults, member.LegacyRemotes)
			if logWarnings {
				for _, warning := range warnings {
					if s.logf != nil {
						s.logf("warning: group %s repo %s: %s", groupName, member.Repo, warning)
					}
				}
			}
			aliasUpdated, aliasWarnings := applyLegacyAliasDefaults(cfg, member.Repo, "", remote, branch)
			if logWarnings {
				for _, warning := range aliasWarnings {
					if s.logf != nil {
						s.logf("warning: group %s repo %s: %s", groupName, member.Repo, warning)
					}
				}
			}
			if aliasUpdated {
				changed = true
			}
			group.Members[i].LegacyRemotes = nil
			groupChanged = true
		}
		if groupChanged {
			cfg.Groups[groupName] = group
			changed = true
		}
	}
	return changed
}

func (s *Service) migrateLegacyWorkspaceRemotes(ctx context.Context, cfg *config.GlobalConfig, configPath, wsRoot string, wsConfig *config.WorkspaceConfig) error {
	type legacyWorkspaceRepo struct {
		name      string
		localPath string
		remotes   *config.Remotes
	}

	legacyRepos := []legacyWorkspaceRepo{}
	workspaceChanged := false
	for i := range wsConfig.Repos {
		repo := wsConfig.Repos[i]
		if repo.LegacyRemotes == nil {
			continue
		}
		legacyRepos = append(legacyRepos, legacyWorkspaceRepo{
			name:      repo.Name,
			localPath: repo.LocalPath,
			remotes:   repo.LegacyRemotes,
		})
		wsConfig.Repos[i].LegacyRemotes = nil
		workspaceChanged = true
	}

	applyToConfig := func(target *config.GlobalConfig, logWarnings bool) bool {
		configChanged := false
		for _, repo := range legacyRepos {
			remote, branch, warnings := resolveLegacyRemoteDefaults(target.Defaults, repo.remotes)
			if logWarnings {
				for _, warning := range warnings {
					if s.logf != nil {
						s.logf("warning: workspace repo %s: %s", repo.name, warning)
					}
				}
			}
			aliasUpdated, aliasWarnings := applyLegacyAliasDefaults(target, repo.name, repo.localPath, remote, branch)
			if logWarnings {
				for _, warning := range aliasWarnings {
					if s.logf != nil {
						s.logf("warning: workspace repo %s: %s", repo.name, warning)
					}
				}
			}
			if aliasUpdated {
				configChanged = true
			}
		}
		return configChanged
	}

	if len(legacyRepos) > 0 {
		configChanged := applyToConfig(cfg, true)
		if configChanged {
			if updater, ok := s.configs.(ConfigUpdater); ok {
				_, err := updater.Update(ctx, configPath, func(target *config.GlobalConfig, info config.GlobalConfigLoadInfo) error {
					applyToConfig(target, false)
					return nil
				})
				if err != nil {
					return err
				}
			} else {
				if err := s.configs.Save(ctx, configPath, *cfg); err != nil {
					return err
				}
			}
		}
	}
	if workspaceChanged {
		if err := s.workspaces.SaveConfig(ctx, wsRoot, *wsConfig); err != nil {
			return err
		}
	}
	return nil
}

func resolveLegacyRemoteDefaults(defaults config.Defaults, remotes *config.Remotes) (string, string, []string) {
	if remotes == nil {
		return "", "", nil
	}
	base := strings.TrimSpace(remotes.Base.Name)
	write := strings.TrimSpace(remotes.Write.Name)
	var warnings []string
	if base != "" && write != "" && base != write {
		warnings = append(warnings, fmt.Sprintf("base remote %q differs from write remote %q; using %q", base, write, base))
	}
	remote := base
	if remote == "" {
		remote = write
	}
	if remote == "" {
		remote = defaults.Remote
	}
	branch := strings.TrimSpace(remotes.Base.DefaultBranch)
	if branch == "" {
		branch = strings.TrimSpace(remotes.Write.DefaultBranch)
	}
	if branch == "" {
		branch = defaults.BaseBranch
	}
	return remote, branch, warnings
}

func applyLegacyAliasDefaults(cfg *config.GlobalConfig, name, localPath, remote, branch string) (bool, []string) {
	if name == "" {
		return false, nil
	}
	if cfg.Repos == nil {
		cfg.Repos = map[string]config.RepoAlias{}
	}
	alias, ok := cfg.Repos[name]
	if !ok {
		alias = config.RepoAlias{}
	}
	updated := false
	var warnings []string
	if alias.URL == "" && alias.Path == "" && localPath != "" {
		alias.Path = localPath
		updated = true
	}
	if alias.Remote == "" && remote != "" {
		alias.Remote = remote
		updated = true
	} else if alias.Remote != "" && remote != "" && alias.Remote != remote {
		warnings = append(warnings, fmt.Sprintf("alias remote %q differs from legacy %q; keeping %q", alias.Remote, remote, alias.Remote))
	}
	if alias.DefaultBranch == "" && branch != "" {
		alias.DefaultBranch = branch
		updated = true
	} else if alias.DefaultBranch != "" && branch != "" && alias.DefaultBranch != branch {
		warnings = append(warnings, fmt.Sprintf("alias default_branch %q differs from legacy %q; keeping %q", alias.DefaultBranch, branch, alias.DefaultBranch))
	}
	if !ok || updated {
		cfg.Repos[name] = alias
		return true, warnings
	}
	return false, warnings
}
