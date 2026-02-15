package worksetapi

import (
	"context"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/groups"
	"github.com/strantalis/workset/internal/ops"
)

// ListGroups returns group summaries from global config.
func (s *Service) ListGroups(ctx context.Context) (GroupListResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return GroupListResult{}, err
	}
	names := groups.List(cfg)
	rows := make([]GroupSummaryJSON, 0, len(names))
	for _, name := range names {
		group, _ := groups.Get(cfg, name)
		rows = append(rows, GroupSummaryJSON{
			Name:        name,
			Description: group.Description,
			RepoCount:   len(group.Members),
		})
	}
	return GroupListResult{Groups: rows, Config: info}, nil
}

// GetGroup returns a single group by name.
func (s *Service) GetGroup(ctx context.Context, name string) (GroupJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return GroupJSON{}, info, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return GroupJSON{}, info, ValidationError{Message: "group name required"}
	}
	group, ok := groups.Get(cfg, name)
	if !ok {
		return GroupJSON{}, info, NotFoundError{Message: "group not found"}
	}
	return GroupJSON{
		Name:        name,
		Description: group.Description,
		Members:     group.Members,
	}, info, nil
}

// CreateGroup creates or updates a group definition.
func (s *Service) CreateGroup(ctx context.Context, input GroupUpsertInput) (GroupJSON, config.GlobalConfigLoadInfo, error) {
	var (
		info  config.GlobalConfigLoadInfo
		name  string
		group config.Group
	)
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		name = strings.TrimSpace(input.Name)
		if name == "" {
			return ValidationError{Message: "group name required"}
		}
		if err := groups.Upsert(cfg, name, input.Description); err != nil {
			return err
		}
		group, _ = groups.Get(*cfg, name)
		return nil
	}); err != nil {
		return GroupJSON{}, info, err
	}
	return GroupJSON{Name: name, Description: group.Description, Members: group.Members}, info, nil
}

// UpdateGroup updates an existing group definition.
func (s *Service) UpdateGroup(ctx context.Context, input GroupUpsertInput) (GroupJSON, config.GlobalConfigLoadInfo, error) {
	return s.CreateGroup(ctx, input)
}

// DeleteGroup removes a group by name.
func (s *Service) DeleteGroup(ctx context.Context, name string) (RegisteredRepoMutationResultJSON, config.GlobalConfigLoadInfo, error) {
	var info config.GlobalConfigLoadInfo
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		name = strings.TrimSpace(name)
		if name == "" {
			return ValidationError{Message: "group name required"}
		}
		return groups.Delete(cfg, name)
	}); err != nil {
		return RegisteredRepoMutationResultJSON{}, info, err
	}
	return RegisteredRepoMutationResultJSON{Status: "ok", Name: name}, info, nil
}

// AddGroupMember adds a repo to a group.
func (s *Service) AddGroupMember(ctx context.Context, input GroupMemberInput) (GroupJSON, config.GlobalConfigLoadInfo, error) {
	var (
		info      config.GlobalConfigLoadInfo
		groupName string
		group     config.Group
	)
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		groupName = strings.TrimSpace(input.GroupName)
		repoName := strings.TrimSpace(input.RepoName)
		if groupName == "" || repoName == "" {
			return ValidationError{Message: "group and repo name required"}
		}
		member := config.GroupMember{
			Repo: repoName,
		}
		if err := groups.AddMember(cfg, groupName, member); err != nil {
			return err
		}
		group, _ = groups.Get(*cfg, groupName)
		return nil
	}); err != nil {
		return GroupJSON{}, info, err
	}
	return GroupJSON{Name: groupName, Description: group.Description, Members: group.Members}, info, nil
}

// RemoveGroupMember removes a repo from a group.
func (s *Service) RemoveGroupMember(ctx context.Context, input GroupMemberInput) (GroupJSON, config.GlobalConfigLoadInfo, error) {
	var (
		info      config.GlobalConfigLoadInfo
		groupName string
		group     config.Group
	)
	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		groupName = strings.TrimSpace(input.GroupName)
		repoName := strings.TrimSpace(input.RepoName)
		if groupName == "" || repoName == "" {
			return ValidationError{Message: "group and repo name required"}
		}
		if err := groups.RemoveMember(cfg, groupName, repoName); err != nil {
			return err
		}
		group, _ = groups.Get(*cfg, groupName)
		return nil
	}); err != nil {
		return GroupJSON{}, info, err
	}
	return GroupJSON{Name: groupName, Description: group.Description, Members: group.Members}, info, nil
}

// ApplyGroup applies a group template to a workspace.
func (s *Service) ApplyGroup(ctx context.Context, input GroupApplyInput) (GroupApplyResultJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return GroupApplyResultJSON{}, info, err
	}
	wsRoot, wsConfig, err := s.resolveWorkspace(ctx, &cfg, info.Path, input.Workspace)
	if err != nil {
		return GroupApplyResultJSON{}, info, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return GroupApplyResultJSON{}, info, ValidationError{Message: "group name required"}
	}
	group, ok := groups.Get(cfg, name)
	if !ok {
		return GroupApplyResultJSON{}, info, NotFoundError{Message: "group not found"}
	}
	type aliasUpdate struct {
		remote string
		branch string
	}
	aliasUpdates := map[string]aliasUpdate{}
	for _, member := range group.Members {
		plan, err := resolveGroupMemberPlan(cfg, member)
		if err != nil {
			return GroupApplyResultJSON{}, info, err
		}
		_, resolvedRemote, repoWarnings, err := ops.AddRepo(ctx, ops.AddRepoInput{
			WorkspaceRoot: wsRoot,
			Name:          plan.Name,
			URL:           plan.URL,
			SourcePath:    plan.SourcePath,
			Defaults:      cfg.Defaults,
			Remote:        plan.Remote,
			DefaultBranch: plan.DefaultBranch,
			AllowFallback: false,
			Git:           s.git,
		})
		if err != nil {
			return GroupApplyResultJSON{}, info, err
		}
		if len(repoWarnings) > 0 && s.logf != nil {
			for _, warning := range repoWarnings {
				s.logf("warning: group %s repo %s: %s", name, plan.Name, warning)
			}
		}
		if alias, ok := cfg.Repos[plan.Name]; ok {
			aliasUpdated := false
			update := aliasUpdates[plan.Name]
			if alias.Remote == "" && resolvedRemote != "" {
				update.remote = resolvedRemote
				aliasUpdated = true
			}
			if alias.DefaultBranch == "" && plan.DefaultBranch != "" {
				update.branch = plan.DefaultBranch
				aliasUpdated = true
			}
			if aliasUpdated {
				aliasUpdates[plan.Name] = update
			}
		}
	}

	if _, err := s.updateGlobal(ctx, func(cfg *config.GlobalConfig, loadInfo config.GlobalConfigLoadInfo) error {
		info = loadInfo
		for name, update := range aliasUpdates {
			alias, ok := cfg.Repos[name]
			if !ok {
				continue
			}
			if alias.Remote == "" && update.remote != "" {
				alias.Remote = update.remote
			}
			if alias.DefaultBranch == "" && update.branch != "" {
				alias.DefaultBranch = update.branch
			}
			cfg.Repos[name] = alias
		}
		registerWorkspace(cfg, wsConfig.Name, wsRoot, s.clock(), "")
		return nil
	}); err != nil {
		return GroupApplyResultJSON{}, info, err
	}

	return GroupApplyResultJSON{
		Status:    "ok",
		Template:  name,
		Workspace: wsConfig.Name,
	}, info, nil
}
