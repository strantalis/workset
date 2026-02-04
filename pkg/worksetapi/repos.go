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
		registerWorkspace(cfg, wsConfig.Name, wsRoot, s.clock())
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
			cfg.Repos = map[string]config.RepoAlias{}
		}
		cfg.Repos[name] = alias
		registerWorkspace(cfg, wsConfig.Name, wsRoot, s.clock())
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
		wsName = workspaceNameByPath(&cfg, wsRoot)
	}
	if wsName == "" {
		wsName = filepath.Base(wsRoot)
	}
	pending, hookWarnings, err := s.runWorktreeCreatedHooks(ctx, cfg, wsRoot, wsName, config.RepoConfig{
		Name:    name,
		RepoDir: repoDir,
	}, worktreePath, branch, "repo.add")
	if err != nil {
		return RepoAddResult{}, err
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
		Config:       info,
	}, nil
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

	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		registerWorkspace(cfg, wsConfig.Name, wsRoot, s.clock())
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
