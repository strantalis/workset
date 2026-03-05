package worksetapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/strantalis/workset/internal/config"
)

const (
	migrationIDWorkspacesToWorksets  = "2026-02-workspaces-to-worksets"
	migrationIDGroupRemotesToAliases = "2026-02-group-remotes-to-aliases"
)

type globalConfigMigration struct {
	id            string
	summary       string
	removeAfter   string
	targetVersion int
	runWhen       func(*config.GlobalConfig, config.GlobalConfigLoadInfo) bool
	run           func(context.Context, *config.GlobalConfig, string, config.GlobalConfigLoadInfo) error
}

type globalConfigMigrationDescriptor struct {
	ID            string
	Summary       string
	RemoveAfter   string
	TargetVersion int
}

func (s *Service) globalConfigMigrations(
	info config.GlobalConfigLoadInfo,
) []globalConfigMigration {
	return []globalConfigMigration{
		{
			id:            migrationIDWorkspacesToWorksets,
			summary:       "backfill workset labels and persist canonical worksets key",
			removeAfter:   "drop once all active configs have no legacy workspaces/template/group usage for two minor releases",
			targetVersion: config.CurrentGlobalConfigVersion,
			runWhen: func(cfg *config.GlobalConfig, info config.GlobalConfigLoadInfo) bool {
				return normalizedGlobalConfigVersion(cfg.ConfigVersion) < config.CurrentGlobalConfigVersion ||
					info.UsedLegacyWorkspacesKey ||
					info.UsedFlatWorksetsShape ||
					info.UsedLegacyWorksetCatalog ||
					needsWorkspaceWorksetNormalization(cfg) ||
					needsWorksetRepoModelNormalization(cfg)
			},
			run: func(ctx context.Context, cfg *config.GlobalConfig, configPath string, info config.GlobalConfigLoadInfo) error {
				forcePersist := info.UsedLegacyWorkspacesKey ||
					info.UsedFlatWorksetsShape ||
					info.UsedLegacyWorksetCatalog ||
					normalizedGlobalConfigVersion(cfg.ConfigVersion) < config.CurrentGlobalConfigVersion
				return s.migrateWorkspaceWorksets(ctx, cfg, configPath, forcePersist)
			},
		},
		{
			id:            migrationIDGroupRemotesToAliases,
			summary:       "promote legacy group remotes to alias defaults and strip legacy remote blocks",
			removeAfter:   "drop after legacy remotes/group metadata has been absent for two minor releases",
			targetVersion: config.CurrentGlobalConfigVersion,
			runWhen: func(cfg *config.GlobalConfig, _ config.GlobalConfigLoadInfo) bool {
				return normalizedGlobalConfigVersion(cfg.ConfigVersion) < config.CurrentGlobalConfigVersion || hasLegacyGroupRemotes(cfg)
			},
			run: func(ctx context.Context, cfg *config.GlobalConfig, configPath string, info config.GlobalConfigLoadInfo) error {
				return s.migrateLegacyGroupRemotes(ctx, cfg, configPath)
			},
		},
	}
}

func (s *Service) globalConfigMigrationPlan(
	info config.GlobalConfigLoadInfo,
) []globalConfigMigrationDescriptor {
	migrations := s.globalConfigMigrations(info)
	plan := make([]globalConfigMigrationDescriptor, 0, len(migrations))
	for _, migration := range migrations {
		plan = append(plan, globalConfigMigrationDescriptor{
			ID:            migration.id,
			Summary:       migration.summary,
			RemoveAfter:   migration.removeAfter,
			TargetVersion: migration.targetVersion,
		})
	}
	return plan
}

func (s *Service) runGlobalConfigMigrations(
	ctx context.Context,
	cfg *config.GlobalConfig,
	configPath string,
	info config.GlobalConfigLoadInfo,
) error {
	initialVersion := normalizedGlobalConfigVersion(cfg.ConfigVersion)
	cfg.ConfigVersion = initialVersion

	for _, migration := range s.globalConfigMigrations(info) {
		if migration.runWhen != nil && !migration.runWhen(cfg, info) {
			continue
		}
		if err := migration.run(ctx, cfg, configPath, info); err != nil {
			return fmt.Errorf("migration %s failed: %w", migration.id, err)
		}
		if cfg.ConfigVersion < migration.targetVersion {
			cfg.ConfigVersion = migration.targetVersion
		}
	}
	if cfg.ConfigVersion < config.CurrentGlobalConfigVersion {
		cfg.ConfigVersion = config.CurrentGlobalConfigVersion
	}
	if info.Exists && cfg.ConfigVersion > initialVersion {
		if err := s.persistGlobalConfigVersion(ctx, configPath, cfg.ConfigVersion); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) persistGlobalConfigVersion(ctx context.Context, configPath string, version int) error {
	if version <= 0 {
		return nil
	}
	if updater, ok := s.configs.(ConfigUpdater); ok {
		_, err := updater.Update(ctx, configPath, func(target *config.GlobalConfig, _ config.GlobalConfigLoadInfo) error {
			if target.ConfigVersion < version {
				target.ConfigVersion = version
			}
			return nil
		})
		return err
	}
	loaded, _, err := s.configs.Load(ctx, configPath)
	if err != nil {
		return err
	}
	if loaded.ConfigVersion >= version {
		return nil
	}
	loaded.ConfigVersion = version
	return s.configs.Save(ctx, configPath, loaded)
}

func normalizedGlobalConfigVersion(value int) int {
	if value < config.LegacyGlobalConfigVersion {
		return config.LegacyGlobalConfigVersion
	}
	return value
}

func hasLegacyGroupRemotes(cfg *config.GlobalConfig) bool {
	if cfg == nil || len(cfg.Groups) == 0 {
		return false
	}
	for _, group := range cfg.Groups {
		for _, member := range group.Members {
			if member.LegacyRemotes != nil {
				return true
			}
		}
	}
	return false
}

func needsWorkspaceWorksetNormalization(cfg *config.GlobalConfig) bool {
	if cfg == nil {
		return false
	}
	if len(cfg.Workspaces) == 0 {
		return false
	}
	for _, ref := range cfg.Workspaces {
		if strings.TrimSpace(ref.Workset) == "" ||
			strings.TrimSpace(ref.Template) != "" ||
			len(normalizeRepoNames(ref.RepoOverrides)) != len(ref.RepoOverrides) {
			return true
		}
	}
	return false
}

func needsWorksetRepoModelNormalization(cfg *config.GlobalConfig) bool {
	if cfg == nil {
		return false
	}
	if len(cfg.Workspaces) == 0 {
		return len(cfg.WorksetRepos) > 0
	}
	worksetThreads := map[string]struct{}{}
	for workspaceName, ref := range cfg.Workspaces {
		worksetName := strings.TrimSpace(workspaceRefWorkset(ref))
		if worksetName == "" {
			worksetName = strings.TrimSpace(workspaceName)
		}
		if worksetName == "" {
			continue
		}
		worksetThreads[worksetName] = struct{}{}
	}
	for worksetName, repos := range cfg.WorksetRepos {
		trimmed := strings.TrimSpace(worksetName)
		if trimmed == "" || trimmed != worksetName {
			return true
		}
		if len(normalizeRepoNames(repos)) != len(repos) {
			return true
		}
		if len(normalizeRepoNames(repos)) == 0 {
			return true
		}
		delete(worksetThreads, worksetName)
	}
	return len(worksetThreads) > 0
}

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

func (s *Service) migrateWorkspaceWorksets(ctx context.Context, cfg *config.GlobalConfig, configPath string, forcePersist bool) error {
	if len(cfg.Workspaces) == 0 {
		return s.persistEmptyWorkspaceWorksetMigration(ctx, cfg, configPath, forcePersist)
	}

	state := s.collectWorkspaceWorksetMigrationState(ctx, cfg, forcePersist)
	if forcePersist && len(cfg.Groups) > 0 {
		cfg.Groups = map[string]config.Group{}
		state.changed = true
	}
	if !state.changed {
		return nil
	}

	return s.persistWorkspaceWorksetMigration(ctx, cfg, configPath, forcePersist, state.updates)
}

type workspaceWorksetMigrationState struct {
	changed bool
	updates map[string]config.WorkspaceRef
}

func (s *Service) persistEmptyWorkspaceWorksetMigration(
	ctx context.Context,
	cfg *config.GlobalConfig,
	configPath string,
	forcePersist bool,
) error {
	changed := forcePersist
	if len(cfg.WorksetRepos) > 0 {
		cfg.WorksetRepos = map[string][]string{}
		changed = true
	}
	if forcePersist && len(cfg.Groups) > 0 {
		cfg.Groups = map[string]config.Group{}
		changed = true
	}
	if !changed {
		return nil
	}

	if updater, ok := s.configs.(ConfigUpdater); ok {
		_, err := updater.Update(ctx, configPath, func(target *config.GlobalConfig, _ config.GlobalConfigLoadInfo) error {
			target.WorksetRepos = map[string][]string{}
			if forcePersist {
				target.Groups = map[string]config.Group{}
			}
			return nil
		})
		return err
	}
	return s.configs.Save(ctx, configPath, *cfg)
}

func (s *Service) collectWorkspaceWorksetMigrationState(
	ctx context.Context,
	cfg *config.GlobalConfig,
	forcePersist bool,
) workspaceWorksetMigrationState {
	state := workspaceWorksetMigrationState{
		changed: forcePersist,
		updates: map[string]config.WorkspaceRef{},
	}

	for name, ref := range cfg.Workspaces {
		normalized, refChanged := s.normalizeWorkspaceRef(ctx, name, ref)
		if refChanged {
			state.updates[name] = normalized
			cfg.Workspaces[name] = normalized
			state.changed = true
		}

	}
	if s.rebuildWorksetRepoModel(ctx, cfg) {
		state.changed = true
		for name, ref := range cfg.Workspaces {
			state.updates[name] = ref
		}
	}

	return state
}

func (s *Service) normalizeWorkspaceRef(
	ctx context.Context,
	workspaceName string,
	ref config.WorkspaceRef,
) (config.WorkspaceRef, bool) {
	changed := false
	legacyTemplate := strings.TrimSpace(ref.Template)
	if strings.TrimSpace(ref.Workset) == "" {
		workset := legacyTemplate
		if workset == "" {
			workset = s.deriveWorkspaceWorkset(ctx, workspaceName, ref.Path)
		}
		if workset != "" {
			ref.Workset = workset
			changed = true
		}
	}
	if legacyTemplate != "" {
		ref.Template = ""
		changed = true
	}
	normalizedOverrides := normalizeRepoNames(ref.RepoOverrides)
	if !sameRepoNames(ref.RepoOverrides, normalizedOverrides) {
		ref.RepoOverrides = normalizedOverrides
		changed = true
	}
	return ref, changed
}

func (s *Service) persistWorkspaceWorksetMigration(
	ctx context.Context,
	cfg *config.GlobalConfig,
	configPath string,
	forcePersist bool,
	updates map[string]config.WorkspaceRef,
) error {
	if updater, ok := s.configs.(ConfigUpdater); ok {
		_, err := updater.Update(ctx, configPath, func(target *config.GlobalConfig, _ config.GlobalConfigLoadInfo) error {
			if target.Workspaces == nil {
				return nil
			}
			for name, migrated := range updates {
				ref, ok := target.Workspaces[name]
				if !ok {
					continue
				}
				ref.Workset = migrated.Workset
				ref.Template = ""
				ref.RepoOverrides = append([]string(nil), migrated.RepoOverrides...)
				target.Workspaces[name] = ref
			}
			target.WorksetRepos = map[string][]string{}
			for worksetName, repos := range cfg.WorksetRepos {
				target.WorksetRepos[worksetName] = append([]string(nil), repos...)
			}
			if forcePersist {
				target.Groups = map[string]config.Group{}
			}
			return nil
		})
		return err
	}

	return s.configs.Save(ctx, configPath, *cfg)
}

func (s *Service) deriveWorkspaceWorkset(ctx context.Context, workspaceName, root string) string {
	trimmedWorkspaceName := strings.TrimSpace(workspaceName)
	if strings.TrimSpace(root) == "" {
		return trimmedWorkspaceName
	}
	wsConfig, err := s.workspaces.LoadConfig(ctx, root)
	if err != nil {
		return trimmedWorkspaceName
	}
	repos := make([]RepoSnapshotJSON, 0, len(wsConfig.Repos))
	for _, repo := range wsConfig.Repos {
		repoName := strings.TrimSpace(repo.Name)
		if repoName == "" {
			continue
		}
		repos = append(repos, RepoSnapshotJSON{Name: repoName})
	}
	repoNames := make([]string, 0, len(repos))
	for _, repo := range repos {
		repoNames = append(repoNames, repo.Name)
	}
	return deriveWorksetLabelFromRepoNames(trimmedWorkspaceName, repoNames)
}

func (s *Service) rebuildWorksetRepoModel(ctx context.Context, cfg *config.GlobalConfig) bool {
	if cfg == nil {
		return false
	}
	changed := false
	if cfg.WorksetRepos == nil {
		cfg.WorksetRepos = map[string][]string{}
		changed = true
	}
	groupThreads := map[string][]string{}
	threadRepos := map[string][]string{}
	for threadName, ref := range cfg.Workspaces {
		worksetName := strings.TrimSpace(workspaceRefWorkset(ref))
		if worksetName == "" {
			worksetName = strings.TrimSpace(threadName)
		}
		groupThreads[worksetName] = append(groupThreads[worksetName], threadName)
		repos, ok := s.loadThreadRepoNames(ctx, ref.Path)
		if !ok {
			continue
		}
		threadRepos[threadName] = repos
	}

	newWorksetRepos := map[string][]string{}
	for worksetName, threads := range groupThreads {
		base, hasBase := intersectThreadRepos(threads, threadRepos)
		if !hasBase {
			base = nil
		}
		newWorksetRepos[worksetName] = base
		for _, threadName := range threads {
			ref := cfg.Workspaces[threadName]
			current := normalizeRepoNames(ref.RepoOverrides)
			threadRepoSet, hasThreadRepos := threadRepos[threadName]
			next := current
			if hasThreadRepos {
				next = subtractRepoNames(threadRepoSet, base)
			} else if len(base) > 0 {
				next = subtractRepoNames(current, base)
			}
			if !sameRepoNames(current, next) {
				ref.RepoOverrides = next
				cfg.Workspaces[threadName] = ref
				changed = true
			}
		}
	}

	if !sameWorksetRepoMap(cfg.WorksetRepos, newWorksetRepos) {
		cfg.WorksetRepos = newWorksetRepos
		changed = true
	}
	return changed
}

func (s *Service) loadThreadRepoNames(ctx context.Context, root string) ([]string, bool) {
	if strings.TrimSpace(root) == "" {
		return nil, false
	}
	wsCfg, err := s.workspaces.LoadConfig(ctx, root)
	if err != nil {
		return nil, false
	}
	repos := make([]string, 0, len(wsCfg.Repos))
	for _, repo := range wsCfg.Repos {
		repoName := strings.TrimSpace(repo.Name)
		if repoName == "" {
			continue
		}
		repos = append(repos, repoName)
	}
	normalized := normalizeRepoNames(repos)
	return normalized, len(normalized) > 0
}

func intersectThreadRepos(threads []string, threadRepos map[string][]string) ([]string, bool) {
	base := []string(nil)
	initialized := false
	for _, threadName := range threads {
		repos, ok := threadRepos[threadName]
		if !ok {
			continue
		}
		if !initialized {
			base = append([]string(nil), repos...)
			initialized = true
			continue
		}
		base = intersectRepoNames(base, repos)
	}
	return base, initialized
}

func intersectRepoNames(base []string, other []string) []string {
	if len(base) == 0 || len(other) == 0 {
		return nil
	}
	otherSet := map[string]struct{}{}
	for _, repo := range normalizeRepoNames(other) {
		otherSet[strings.ToLower(repo)] = struct{}{}
	}
	out := make([]string, 0, len(base))
	for _, repo := range normalizeRepoNames(base) {
		if _, ok := otherSet[strings.ToLower(repo)]; ok {
			out = append(out, repo)
		}
	}
	return out
}

func subtractRepoNames(base []string, excluded []string) []string {
	base = normalizeRepoNames(base)
	if len(base) == 0 {
		return nil
	}
	excludedSet := map[string]struct{}{}
	for _, repo := range normalizeRepoNames(excluded) {
		excludedSet[strings.ToLower(repo)] = struct{}{}
	}
	out := make([]string, 0, len(base))
	for _, repo := range base {
		if _, ok := excludedSet[strings.ToLower(repo)]; ok {
			continue
		}
		out = append(out, repo)
	}
	return out
}

func normalizeRepoNames(repos []string) []string {
	if len(repos) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(repos))
	for _, repo := range repos {
		trimmed := strings.TrimSpace(repo)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func deriveWorksetLabelFromRepoNames(workspaceName string, repos []string) string {
	normalized := normalizeRepoNames(repos)
	if len(normalized) == 0 {
		return strings.TrimSpace(workspaceName)
	}
	return strings.Join(normalized, " + ")
}

func sameRepoNames(a []string, b []string) bool {
	a = normalizeRepoNames(a)
	b = normalizeRepoNames(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameWorksetRepoMap(a map[string][]string, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for worksetName, reposA := range a {
		reposB, ok := b[worksetName]
		if !ok {
			return false
		}
		if !sameRepoNames(reposA, reposB) {
			return false
		}
	}
	return true
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
		cfg.Repos = map[string]config.RegisteredRepo{}
	}
	alias, ok := cfg.Repos[name]
	if !ok {
		alias = config.RegisteredRepo{}
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
