package worksetapi

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/ops"
	"github.com/strantalis/workset/internal/workspace"
)

type resolvedWorksetRepoSource struct {
	name       string
	url        string
	sourcePath string
}

// ListRepos lists repos configured for a workspace.
func (s *Service) ListRepos(ctx context.Context, selector WorkspaceSelector) (RepoListResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return RepoListResult{}, err
	}
	wsRoot, wsConfig, err := s.resolveWorkspace(ctx, &cfg, info.Path, selector)
	if err != nil {
		return RepoListResult{}, err
	}

	rows := make([]RepoJSON, 0, len(wsConfig.Repos))
	for _, repo := range wsConfig.Repos {
		config.ApplyRepoDefaults(&repo, cfg.Defaults)
		repoDefaults := resolveRepoDefaults(cfg, repo.Name)
		rows = append(rows, RepoJSON{
			Name:          repo.Name,
			LocalPath:     repo.LocalPath,
			Managed:       repo.Managed,
			RepoDir:       repo.RepoDir,
			Remote:        repoDefaults.Remote,
			DefaultBranch: repoDefaults.DefaultBranch,
		})
	}

	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		registerWorkspace(cfg, wsConfig.Name, wsRoot, s.clock(), "")
		s.rebuildWorksetRepoModel(ctx, cfg)
		return nil
	}); err != nil {
		return RepoListResult{}, err
	}

	return RepoListResult{Repos: rows, Config: info}, nil
}

// AddRepo adds a repo to a workspace (clone or attach).
func (s *Service) AddRepo(ctx context.Context, input RepoAddInput) (RepoAddResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return RepoAddResult{}, err
	}
	wsRoot, wsConfig, err := s.resolveWorkspace(ctx, &cfg, info.Path, input.Workspace)
	if err != nil {
		return RepoAddResult{}, err
	}

	name := strings.TrimSpace(input.Name)
	nameProvided := input.NameSet
	url := strings.TrimSpace(input.URL)
	sourcePath := strings.TrimSpace(input.SourcePath)
	source := strings.TrimSpace(input.Source)

	if url == "" && sourcePath == "" {
		if source == "" {
			return RepoAddResult{}, ValidationError{Message: "repo alias or source required"}
		}
		if alias, ok := cfg.Repos[source]; ok {
			url = alias.URL
			name = source
			sourcePath = alias.Path
			if sourcePath == "" && looksLikeLocalPath(url) {
				sourcePath = url
				url = ""
			}
		} else if looksLikeURL(source) {
			url = source
		} else {
			sourcePath = source
		}
	}

	if sourcePath == "" && url != "" && looksLikeLocalPath(url) {
		sourcePath = url
		url = ""
	}
	if sourcePath != "" {
		resolved, err := resolveLocalPathInput(sourcePath)
		if err != nil {
			return RepoAddResult{}, err
		}
		sourcePath = resolved
		if !nameProvided && name == "" {
			name = filepath.Base(sourcePath)
		}
	}
	if name == "" {
		name = ops.DeriveRepoNameFromURL(url)
	}
	if nameProvided {
		name = strings.TrimSpace(input.Name)
	}

	alias, aliasExists := cfg.Repos[name]
	defaultBranch := cfg.Defaults.BaseBranch
	if aliasExists && alias.DefaultBranch != "" {
		defaultBranch = alias.DefaultBranch
	}
	remote := cfg.Defaults.Remote
	if aliasExists && alias.Remote != "" {
		remote = alias.Remote
	}
	_, resolvedRemote, repoWarnings, err := ops.AddRepo(ctx, ops.AddRepoInput{
		WorkspaceRoot: wsRoot,
		Name:          name,
		URL:           url,
		SourcePath:    sourcePath,
		RepoDir:       input.RepoDir,
		Defaults:      cfg.Defaults,
		Remote:        remote,
		DefaultBranch: defaultBranch,
		AllowFallback: false,
		Git:           s.git,
	})
	if err != nil {
		return RepoAddResult{}, err
	}

	warnings := []string{}
	pendingHooks := []HookPending{}
	hookRuns := []HookExecutionJSON{}
	if len(repoWarnings) > 0 {
		warnings = append(warnings, repoWarnings...)
	}

	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		alias, aliasExists := cfg.Repos[name]
		if sourcePath != "" && aliasExists && alias.Path != sourcePath {
			alias.Path = sourcePath
			alias.URL = ""
		}
		if alias.Path == "" && alias.URL == "" {
			if sourcePath != "" {
				alias.Path = sourcePath
				alias.URL = ""
			} else if url != "" {
				alias.URL = url
				alias.Path = ""
			}
		}
		if alias.DefaultBranch == "" && defaultBranch != "" {
			alias.DefaultBranch = defaultBranch
		}
		if alias.Remote == "" && resolvedRemote != "" {
			alias.Remote = resolvedRemote
		}
		if cfg.Repos == nil {
			cfg.Repos = map[string]config.RegisteredRepo{}
		}
		cfg.Repos[name] = alias
		registerWorkspace(cfg, wsConfig.Name, wsRoot, s.clock(), "")
		s.rebuildWorksetRepoModel(ctx, cfg)
		return nil
	}); err != nil {
		return RepoAddResult{}, err
	}

	localPath := sourcePath
	managed := false
	if localPath == "" {
		localPath = filepath.Join(cfg.Defaults.RepoStoreRoot, name)
		managed = true
	}

	repoDir := input.RepoDir
	if repoDir == "" {
		repoDir = name
	}
	branch := cfg.Defaults.BaseBranch
	if loaded, err := s.workspaces.Load(ctx, wsRoot, cfg.Defaults); err == nil {
		branch = loaded.State.CurrentBranch
	}
	if branch == "" {
		branch = defaultBranch
	}
	worktreePath := workspace.RepoWorktreePath(wsRoot, branch, repoDir)

	wsName := wsConfig.Name
	if wsName == "" {
		wsName = threadNameByPath(&cfg, wsRoot)
	}
	if wsName == "" {
		wsName = filepath.Base(wsRoot)
	}
	pending, runs, hookWarnings, err := s.runWorktreeCreatedHooks(ctx, cfg, wsRoot, wsName, config.RepoConfig{
		Name:    name,
		RepoDir: repoDir,
	}, worktreePath, branch, "repo.add")
	if err != nil {
		return RepoAddResult{}, err
	}
	if len(runs) > 0 {
		hookRuns = append(hookRuns, runs...)
	}
	if len(hookWarnings) > 0 {
		warnings = append(warnings, hookWarnings...)
	}
	if len(pending.Hooks) > 0 {
		pendingHooks = append(pendingHooks, pending)
	}

	payload := RepoAddResultJSON{
		Status:    "ok",
		Workspace: wsConfig.Name,
		Repo:      name,
		LocalPath: localPath,
		Managed:   managed,
	}
	if len(pendingHooks) > 0 {
		payload.PendingHooks = make([]HookPendingJSON, 0, len(pendingHooks))
		for _, pending := range pendingHooks {
			payload.PendingHooks = append(payload.PendingHooks, HookPendingJSON(pending))
		}
	}
	return RepoAddResult{
		Payload:      payload,
		WorktreePath: worktreePath,
		RepoDir:      repoDir,
		Warnings:     warnings,
		PendingHooks: pendingHooks,
		HookRuns:     hookRuns,
		Config:       info,
	}, nil
}

// AddReposToWorkset adds repos to an existing workset without requiring any thread.
func (s *Service) AddReposToWorkset(
	ctx context.Context,
	input WorksetRepoAddInput,
) (WorksetRepoAddResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return WorksetRepoAddResult{}, err
	}
	worksetName, sources, err := validateWorksetRepoAddInput(cfg, input)
	if err != nil {
		return WorksetRepoAddResult{}, err
	}
	resolved, err := resolveWorksetRepoSources(cfg, sources)
	if err != nil {
		return WorksetRepoAddResult{}, err
	}

	added := []string{}
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		var applyErr error
		added, applyErr = applyResolvedWorksetRepos(cfg, worksetName, resolved)
		return applyErr
	}); err != nil {
		return WorksetRepoAddResult{}, err
	}

	return WorksetRepoAddResult{
		Payload: WorksetRepoAddResultJSON{
			Status:  "ok",
			Workset: worksetName,
			Added:   added,
		},
		Config: info,
	}, nil
}

func validateWorksetRepoAddInput(
	cfg config.GlobalConfig,
	input WorksetRepoAddInput,
) (string, []string, error) {
	worksetName := strings.TrimSpace(input.Workset)
	if worksetName == "" {
		return "", nil, ValidationError{Message: "workset required"}
	}
	if !worksetExists(cfg, worksetName) {
		return "", nil, NotFoundError{Message: fmt.Sprintf("workset not found: %q", worksetName)}
	}
	sources := normalizeRepoNames(input.Sources)
	if len(sources) == 0 {
		return "", nil, ValidationError{Message: "at least one repo source required"}
	}
	return worksetName, sources, nil
}

func resolveWorksetRepoSources(
	cfg config.GlobalConfig,
	sources []string,
) ([]resolvedWorksetRepoSource, error) {
	resolved := make([]resolvedWorksetRepoSource, 0, len(sources))
	for _, source := range sources {
		repoSource, err := resolveWorksetRepoSource(cfg, source)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, repoSource)
	}
	return resolved, nil
}

func applyResolvedWorksetRepos(
	cfg *config.GlobalConfig,
	worksetName string,
	resolved []resolvedWorksetRepoSource,
) ([]string, error) {
	if !worksetExists(*cfg, worksetName) {
		return nil, NotFoundError{Message: fmt.Sprintf("workset not found: %q", worksetName)}
	}
	ensureWorksetRepoMaps(cfg)
	baseRepos := normalizeRepoNames(cfg.WorksetRepos[worksetName])
	baseSet := repoNameSet(baseRepos)
	added := make([]string, 0, len(resolved))

	for _, repoSource := range resolved {
		cfg.Repos[repoSource.name] = mergeResolvedRepoAlias(
			cfg.Repos[repoSource.name],
			repoSource,
			cfg.Defaults,
		)
		key := strings.ToLower(repoSource.name)
		if _, exists := baseSet[key]; exists {
			continue
		}
		baseSet[key] = struct{}{}
		baseRepos = append(baseRepos, repoSource.name)
		added = append(added, repoSource.name)
	}

	cfg.WorksetRepos[worksetName] = normalizeRepoNames(baseRepos)
	return added, nil
}

func ensureWorksetRepoMaps(cfg *config.GlobalConfig) {
	if cfg.WorksetRepos == nil {
		cfg.WorksetRepos = map[string][]string{}
	}
	if cfg.Repos == nil {
		cfg.Repos = map[string]config.RegisteredRepo{}
	}
}

func repoNameSet(repoNames []string) map[string]struct{} {
	set := make(map[string]struct{}, len(repoNames))
	for _, repoName := range repoNames {
		set[strings.ToLower(repoName)] = struct{}{}
	}
	return set
}

func mergeResolvedRepoAlias(
	alias config.RegisteredRepo,
	repoSource resolvedWorksetRepoSource,
	defaults config.Defaults,
) config.RegisteredRepo {
	if alias.Path == "" && alias.URL == "" {
		if repoSource.sourcePath != "" {
			alias.Path = repoSource.sourcePath
			alias.URL = ""
		} else if repoSource.url != "" {
			alias.URL = repoSource.url
			alias.Path = ""
		}
	}
	if alias.DefaultBranch == "" {
		alias.DefaultBranch = defaults.BaseBranch
	}
	if alias.Remote == "" {
		alias.Remote = defaults.Remote
	}
	return alias
}

func resolveWorksetRepoSource(
	cfg config.GlobalConfig,
	source string,
) (resolvedWorksetRepoSource, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return resolvedWorksetRepoSource{}, ValidationError{Message: "repo source required"}
	}
	if alias, ok := cfg.Repos[trimmed]; ok {
		return resolvedWorksetRepoSource{
			name:       trimmed,
			url:        strings.TrimSpace(alias.URL),
			sourcePath: strings.TrimSpace(alias.Path),
		}, nil
	}
	if looksLikeURL(trimmed) {
		name := strings.TrimSpace(ops.DeriveRepoNameFromURL(trimmed))
		if name == "" {
			return resolvedWorksetRepoSource{}, ValidationError{Message: "unable to derive repo name from source"}
		}
		return resolvedWorksetRepoSource{name: name, url: trimmed}, nil
	}
	sourcePath, err := resolveLocalPathInput(trimmed)
	if err != nil {
		return resolvedWorksetRepoSource{}, err
	}
	name := strings.TrimSpace(filepath.Base(sourcePath))
	if name == "" {
		return resolvedWorksetRepoSource{}, ValidationError{Message: "unable to derive repo name from source"}
	}
	return resolvedWorksetRepoSource{name: name, sourcePath: sourcePath}, nil
}

func worksetExists(cfg config.GlobalConfig, worksetName string) bool {
	normalizedWorkset := strings.TrimSpace(worksetName)
	if normalizedWorkset == "" {
		return false
	}
	if cfg.WorksetRepos != nil {
		if _, ok := cfg.WorksetRepos[normalizedWorkset]; ok {
			return true
		}
	}
	for threadName, ref := range cfg.Workspaces {
		if worksetNameForThread(threadName, ref) == normalizedWorkset {
			return true
		}
	}
	return false
}

// RemoveRepo removes a repo from a workspace and optionally deletes files.
func (s *Service) RemoveRepo(ctx context.Context, input RepoRemoveInput) (RepoRemoveResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return RepoRemoveResult{}, err
	}
	wsRoot, wsConfig, err := s.resolveWorkspace(ctx, &cfg, info.Path, input.Workspace)
	if err != nil {
		return RepoRemoveResult{}, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return RepoRemoveResult{}, ValidationError{Message: "repo name required"}
	}
	repoCfg, ok := findRepo(wsConfig, name)
	if !ok {
		return RepoRemoveResult{}, NotFoundError{Message: "repo not found in workspace"}
	}

	repoDefaults := resolveRepoDefaults(cfg, repoCfg.Name)
	report, err := ops.CheckRepoSafety(ctx, ops.RepoSafetyInput{
		WorkspaceRoot: wsRoot,
		Repo:          repoCfg,
		Defaults:      cfg.Defaults,
		RepoDefaults:  repoDefaults,
		Git:           s.git,
		FetchRemotes:  input.FetchRemotes,
	})
	if err != nil {
		return RepoRemoveResult{}, err
	}

	dirty, unmerged, unpushed, warnings := summarizeRepoSafety(report)

	if (input.DeleteWorktrees || input.DeleteLocal) && !input.Force {
		if len(dirty) > 0 {
			return RepoRemoveResult{}, UnsafeOperation{
				Message:  fmt.Sprintf("refusing to delete: dirty worktrees: %s (use --force)", strings.Join(dirty, ", ")),
				Dirty:    dirty,
				Warnings: warnings,
			}
		}
		if len(unmerged) > 0 {
			warnings = append(warnings, unmergedRepoDetails(report)...)
			return RepoRemoveResult{}, UnsafeOperation{
				Message:  fmt.Sprintf("refusing to delete: unmerged branches: %s (use --force)", strings.Join(unmerged, ", ")),
				Unmerged: unmerged,
				Warnings: warnings,
			}
		}
		if input.DeleteLocal && !repoCfg.Managed {
			return RepoRemoveResult{}, UnsafeOperation{Message: fmt.Sprintf("refusing to delete unmanaged repo at %s (use --force to override)", repoCfg.LocalPath)}
		}
	}

	if (input.DeleteWorktrees || input.DeleteLocal) && !input.Confirmed {
		return RepoRemoveResult{}, ConfirmationRequired{Message: "remove repo " + name}
	}

	if _, err := ops.RemoveRepo(ctx, ops.RemoveRepoInput{
		WorkspaceRoot:   wsRoot,
		Name:            name,
		Defaults:        cfg.Defaults,
		Git:             s.git,
		DeleteWorktrees: input.DeleteWorktrees,
		DeleteLocal:     input.DeleteLocal,
		Force:           input.Force,
		Logf:            s.logf,
	}); err != nil {
		return RepoRemoveResult{}, err
	}
	state, err := s.workspaces.LoadState(ctx, wsRoot)
	if err == nil && len(state.PullRequests) > 0 {
		if _, tracked := state.PullRequests[name]; tracked {
			delete(state.PullRequests, name)
			if err := s.workspaces.SaveState(ctx, wsRoot, state); err != nil {
				return RepoRemoveResult{}, err
			}
		}
	}

	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		registerWorkspace(cfg, wsConfig.Name, wsRoot, s.clock(), "")
		s.rebuildWorksetRepoModel(ctx, cfg)
		return nil
	}); err != nil {
		return RepoRemoveResult{}, err
	}

	payload := RepoRemoveResultJSON{
		Status:    "ok",
		Workspace: wsConfig.Name,
		Repo:      name,
		Deleted: RepoRemoveDeletedJSON{
			Worktrees: input.DeleteWorktrees,
			Local:     input.DeleteLocal,
		},
	}

	return RepoRemoveResult{Payload: payload, Warnings: warnings, Unpushed: unpushed, Safety: report, Config: info}, nil
}

func findRepo(cfg config.WorkspaceConfig, name string) (config.RepoConfig, bool) {
	for _, repo := range cfg.Repos {
		if repo.Name == name {
			return repo, true
		}
	}
	return config.RepoConfig{}, false
}
