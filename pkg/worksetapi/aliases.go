package worksetapi

import (
	"context"
	"sort"
	"strings"

	"github.com/strantalis/workset/internal/config"
)

// ListAliases returns all configured repo aliases.
func (s *Service) ListAliases(ctx context.Context) (AliasListResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return AliasListResult{}, err
	}
	if len(cfg.Repos) == 0 {
		return AliasListResult{Aliases: []AliasJSON{}, Config: info}, nil
	}
	names := make([]string, 0, len(cfg.Repos))
	for name := range cfg.Repos {
		names = append(names, name)
	}
	sort.Strings(names)
	rows := make([]AliasJSON, 0, len(names))
	for _, name := range names {
		alias := cfg.Repos[name]
		rows = append(rows, AliasJSON{
			Name:          name,
			URL:           alias.URL,
			Path:          alias.Path,
			Remote:        alias.Remote,
			DefaultBranch: alias.DefaultBranch,
		})
	}
	return AliasListResult{Aliases: rows, Config: info}, nil
}

// GetAlias returns a single alias by name.
func (s *Service) GetAlias(ctx context.Context, name string) (AliasJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return AliasJSON{}, info, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return AliasJSON{}, info, ValidationError{Message: "alias name required"}
	}
	alias, ok := cfg.Repos[name]
	if !ok {
		return AliasJSON{}, info, NotFoundError{Message: "repo alias not found"}
	}
	return AliasJSON{
		Name:          name,
		URL:           alias.URL,
		Path:          alias.Path,
		Remote:        alias.Remote,
		DefaultBranch: alias.DefaultBranch,
	}, info, nil
}

// CreateAlias adds a new repo alias.
func (s *Service) CreateAlias(ctx context.Context, input AliasUpsertInput) (AliasMutationResultJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return AliasMutationResultJSON{}, info, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return AliasMutationResultJSON{}, info, ValidationError{Message: "alias name required"}
	}
	if cfg.Repos != nil {
		if _, ok := cfg.Repos[name]; ok {
			return AliasMutationResultJSON{}, info, ConflictError{Message: "repo alias already exists"}
		}
	}
	if strings.TrimSpace(input.Source) == "" {
		return AliasMutationResultJSON{}, info, ValidationError{Message: "source required to create alias"}
	}

	url := ""
	path := ""
	if looksLikeURL(input.Source) {
		url = strings.TrimSpace(input.Source)
	} else {
		resolved, err := resolveLocalPathInput(input.Source)
		if err != nil {
			return AliasMutationResultJSON{}, info, err
		}
		path = resolved
	}
	if cfg.Repos == nil {
		cfg.Repos = map[string]config.RepoAlias{}
	}
	defaultBranch := strings.TrimSpace(input.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = cfg.Defaults.BaseBranch
	}
	remote := strings.TrimSpace(input.Remote)
	if remote == "" {
		remote = cfg.Defaults.Remote
	}
	cfg.Repos[name] = config.RepoAlias{
		URL:           url,
		Path:          path,
		Remote:        remote,
		DefaultBranch: defaultBranch,
	}
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return AliasMutationResultJSON{}, info, err
	}
	return AliasMutationResultJSON{Status: "ok", Name: name}, info, nil
}

// UpdateAlias updates an existing repo alias.
func (s *Service) UpdateAlias(ctx context.Context, input AliasUpsertInput) (AliasMutationResultJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return AliasMutationResultJSON{}, info, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return AliasMutationResultJSON{}, info, ValidationError{Message: "alias name required"}
	}
	alias, ok := cfg.Repos[name]
	if !ok {
		return AliasMutationResultJSON{}, info, NotFoundError{Message: "repo alias not found"}
	}
	updated := false
	if input.SourceSet {
		source := strings.TrimSpace(input.Source)
		if source != "" {
			if looksLikeURL(source) {
				alias.URL = source
				alias.Path = ""
			} else {
				resolved, err := resolveLocalPathInput(source)
				if err != nil {
					return AliasMutationResultJSON{}, info, err
				}
				alias.Path = resolved
				alias.URL = ""
			}
			updated = true
		}
	}
	if input.DefaultBranchSet {
		defaultBranch := strings.TrimSpace(input.DefaultBranch)
		if defaultBranch == "" {
			return AliasMutationResultJSON{}, info, ValidationError{Message: "default branch cannot be empty"}
		}
		alias.DefaultBranch = defaultBranch
		updated = true
	}
	if input.RemoteSet {
		remote := strings.TrimSpace(input.Remote)
		if remote == "" {
			return AliasMutationResultJSON{}, info, ValidationError{Message: "remote cannot be empty"}
		}
		alias.Remote = remote
		updated = true
	}
	if !updated {
		return AliasMutationResultJSON{}, info, ValidationError{Message: "no updates specified"}
	}
	cfg.Repos[name] = alias
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return AliasMutationResultJSON{}, info, err
	}
	return AliasMutationResultJSON{Status: "ok", Name: name}, info, nil
}

// DeleteAlias removes a repo alias by name.
func (s *Service) DeleteAlias(ctx context.Context, name string) (AliasMutationResultJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return AliasMutationResultJSON{}, info, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return AliasMutationResultJSON{}, info, ValidationError{Message: "alias name required"}
	}
	if _, ok := cfg.Repos[name]; !ok {
		return AliasMutationResultJSON{}, info, NotFoundError{Message: "repo alias not found"}
	}
	delete(cfg.Repos, name)
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return AliasMutationResultJSON{}, info, err
	}
	return AliasMutationResultJSON{Status: "ok", Name: name}, info, nil
}
