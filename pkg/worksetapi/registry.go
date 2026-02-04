package worksetapi

import (
	"context"
	"sort"
	"strings"

	"github.com/strantalis/workset/internal/config"
)

// ListRegisteredRepos returns all registered repos from config.
func (s *Service) ListRegisteredRepos(ctx context.Context) (RegisteredRepoListResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return RegisteredRepoListResult{}, err
	}
	if len(cfg.Repos) == 0 {
		return RegisteredRepoListResult{Repos: []RegisteredRepoJSON{}, Config: info}, nil
	}
	names := make([]string, 0, len(cfg.Repos))
	for name := range cfg.Repos {
		names = append(names, name)
	}
	sort.Strings(names)
	rows := make([]RegisteredRepoJSON, 0, len(names))
	for _, name := range names {
		repo := cfg.Repos[name]
		rows = append(rows, RegisteredRepoJSON{
			Name:          name,
			URL:           repo.URL,
			Path:          repo.Path,
			Remote:        repo.Remote,
			DefaultBranch: repo.DefaultBranch,
		})
	}
	return RegisteredRepoListResult{Repos: rows, Config: info}, nil
}

// GetRegisteredRepo returns a single registered repo by name.
func (s *Service) GetRegisteredRepo(ctx context.Context, name string) (RegisteredRepoJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return RegisteredRepoJSON{}, info, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return RegisteredRepoJSON{}, info, ValidationError{Message: "repo name required"}
	}
	repo, ok := cfg.Repos[name]
	if !ok {
		return RegisteredRepoJSON{}, info, NotFoundError{Message: "registered repo not found"}
	}
	return RegisteredRepoJSON{
		Name:          name,
		URL:           repo.URL,
		Path:          repo.Path,
		Remote:        repo.Remote,
		DefaultBranch: repo.DefaultBranch,
	}, info, nil
}

// RegisterRepo adds a new repo to the registry.
func (s *Service) RegisterRepo(ctx context.Context, input RepoRegistryInput) (RegisteredRepoMutationResultJSON, config.GlobalConfigLoadInfo, error) {
	var (
		info config.GlobalConfigLoadInfo
		name string
	)
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		name = strings.TrimSpace(input.Name)
		if name == "" {
			return ValidationError{Message: "repo name required"}
		}
		if cfg.Repos != nil {
			if _, ok := cfg.Repos[name]; ok {
				return ConflictError{Message: "repo already registered"}
			}
		}
		if strings.TrimSpace(input.Source) == "" {
			return ValidationError{Message: "source required to register repo"}
		}

		url := ""
		path := ""
		if looksLikeURL(input.Source) {
			url = strings.TrimSpace(input.Source)
		} else {
			resolved, err := resolveLocalPathInput(input.Source)
			if err != nil {
				return err
			}
			path = resolved
		}
		if cfg.Repos == nil {
			cfg.Repos = map[string]config.RegisteredRepo{}
		}
		defaultBranch := strings.TrimSpace(input.DefaultBranch)
		if defaultBranch == "" {
			defaultBranch = cfg.Defaults.BaseBranch
		}
		remote := strings.TrimSpace(input.Remote)
		if remote == "" {
			remote = cfg.Defaults.Remote
		}
		cfg.Repos[name] = config.RegisteredRepo{
			URL:           url,
			Path:          path,
			Remote:        remote,
			DefaultBranch: defaultBranch,
		}
		return nil
	}); err != nil {
		return RegisteredRepoMutationResultJSON{}, info, err
	}
	return RegisteredRepoMutationResultJSON{Status: "ok", Name: name}, info, nil
}

// UpdateRegisteredRepo updates an existing registered repo.
func (s *Service) UpdateRegisteredRepo(ctx context.Context, input RepoRegistryInput) (RegisteredRepoMutationResultJSON, config.GlobalConfigLoadInfo, error) {
	var (
		info config.GlobalConfigLoadInfo
		name string
	)
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		name = strings.TrimSpace(input.Name)
		if name == "" {
			return ValidationError{Message: "repo name required"}
		}
		repo, ok := cfg.Repos[name]
		if !ok {
			return NotFoundError{Message: "registered repo not found"}
		}
		updated := false
		if input.SourceSet {
			source := strings.TrimSpace(input.Source)
			if source != "" {
				if looksLikeURL(source) {
					repo.URL = source
					repo.Path = ""
				} else {
					resolved, err := resolveLocalPathInput(source)
					if err != nil {
						return err
					}
					repo.Path = resolved
					repo.URL = ""
				}
				updated = true
			}
		}
		if input.DefaultBranchSet {
			defaultBranch := strings.TrimSpace(input.DefaultBranch)
			if defaultBranch == "" {
				return ValidationError{Message: "default branch cannot be empty"}
			}
			repo.DefaultBranch = defaultBranch
			updated = true
		}
		if input.RemoteSet {
			remote := strings.TrimSpace(input.Remote)
			if remote == "" {
				return ValidationError{Message: "remote cannot be empty"}
			}
			repo.Remote = remote
			updated = true
		}
		if !updated {
			return ValidationError{Message: "no updates specified"}
		}
		cfg.Repos[name] = repo
		return nil
	}); err != nil {
		return RegisteredRepoMutationResultJSON{}, info, err
	}
	return RegisteredRepoMutationResultJSON{Status: "ok", Name: name}, info, nil
}

// UnregisterRepo removes a repo from the registry by name.
func (s *Service) UnregisterRepo(ctx context.Context, name string) (RegisteredRepoMutationResultJSON, config.GlobalConfigLoadInfo, error) {
	var info config.GlobalConfigLoadInfo
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		name = strings.TrimSpace(name)
		if name == "" {
			return ValidationError{Message: "repo name required"}
		}
		if _, ok := cfg.Repos[name]; !ok {
			return NotFoundError{Message: "registered repo not found"}
		}
		delete(cfg.Repos, name)
		return nil
	}); err != nil {
		return RegisteredRepoMutationResultJSON{}, info, err
	}
	return RegisteredRepoMutationResultJSON{Status: "ok", Name: name}, info, nil
}

// ListAliases is deprecated, use ListRegisteredRepos instead.
//
// Deprecated: Use ListRegisteredRepos instead. This will be removed in a future version.
func (s *Service) ListAliases(ctx context.Context) (RegisteredRepoListResult, error) {
	return s.ListRegisteredRepos(ctx)
}

// GetAlias is deprecated, use GetRegisteredRepo instead.
//
// Deprecated: Use GetRegisteredRepo instead. This will be removed in a future version.
func (s *Service) GetAlias(ctx context.Context, name string) (RegisteredRepoJSON, config.GlobalConfigLoadInfo, error) {
	return s.GetRegisteredRepo(ctx, name)
}

// CreateAlias is deprecated, use RegisterRepo instead.
//
// Deprecated: Use RegisterRepo instead. This will be removed in a future version.
func (s *Service) CreateAlias(ctx context.Context, input RepoRegistryInput) (RegisteredRepoMutationResultJSON, config.GlobalConfigLoadInfo, error) {
	return s.RegisterRepo(ctx, input)
}

// UpdateAlias is deprecated, use UpdateRegisteredRepo instead.
//
// Deprecated: Use UpdateRegisteredRepo instead. This will be removed in a future version.
func (s *Service) UpdateAlias(ctx context.Context, input RepoRegistryInput) (RegisteredRepoMutationResultJSON, config.GlobalConfigLoadInfo, error) {
	return s.UpdateRegisteredRepo(ctx, input)
}

// DeleteAlias is deprecated, use UnregisterRepo instead.
//
// Deprecated: Use UnregisterRepo instead. This will be removed in a future version.
func (s *Service) DeleteAlias(ctx context.Context, name string) (RegisteredRepoMutationResultJSON, config.GlobalConfigLoadInfo, error) {
	return s.UnregisterRepo(ctx, name)
}
