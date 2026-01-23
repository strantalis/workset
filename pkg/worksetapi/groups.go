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
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return GroupJSON{}, info, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return GroupJSON{}, info, ValidationError{Message: "group name required"}
	}
	if err := groups.Upsert(&cfg, name, input.Description); err != nil {
		return GroupJSON{}, info, err
	}
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return GroupJSON{}, info, err
	}
	group, _ := groups.Get(cfg, name)
	return GroupJSON{Name: name, Description: group.Description, Members: group.Members}, info, nil
}

// UpdateGroup updates an existing group definition.
func (s *Service) UpdateGroup(ctx context.Context, input GroupUpsertInput) (GroupJSON, config.GlobalConfigLoadInfo, error) {
	return s.CreateGroup(ctx, input)
}

// DeleteGroup removes a group by name.
func (s *Service) DeleteGroup(ctx context.Context, name string) (AliasMutationResultJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return AliasMutationResultJSON{}, info, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return AliasMutationResultJSON{}, info, ValidationError{Message: "group name required"}
	}
	if err := groups.Delete(&cfg, name); err != nil {
		return AliasMutationResultJSON{}, info, err
	}
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return AliasMutationResultJSON{}, info, err
	}
	return AliasMutationResultJSON{Status: "ok", Name: name}, info, nil
}

// AddGroupMember adds a repo to a group.
func (s *Service) AddGroupMember(ctx context.Context, input GroupMemberInput) (GroupJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return GroupJSON{}, info, err
	}
	groupName := strings.TrimSpace(input.GroupName)
	repoName := strings.TrimSpace(input.RepoName)
	if groupName == "" || repoName == "" {
		return GroupJSON{}, info, ValidationError{Message: "group and repo name required"}
	}
	member := config.GroupMember{
		Repo: repoName,
	}
	if err := groups.AddMember(&cfg, groupName, member); err != nil {
		return GroupJSON{}, info, err
	}
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return GroupJSON{}, info, err
	}
	group, _ := groups.Get(cfg, groupName)
	return GroupJSON{Name: groupName, Description: group.Description, Members: group.Members}, info, nil
}

// RemoveGroupMember removes a repo from a group.
func (s *Service) RemoveGroupMember(ctx context.Context, input GroupMemberInput) (GroupJSON, config.GlobalConfigLoadInfo, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return GroupJSON{}, info, err
	}
	groupName := strings.TrimSpace(input.GroupName)
	repoName := strings.TrimSpace(input.RepoName)
	if groupName == "" || repoName == "" {
		return GroupJSON{}, info, ValidationError{Message: "group and repo name required"}
	}
	if err := groups.RemoveMember(&cfg, groupName, repoName); err != nil {
		return GroupJSON{}, info, err
	}
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return GroupJSON{}, info, err
	}
	group, _ := groups.Get(cfg, groupName)
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
			if alias.Remote == "" && resolvedRemote != "" {
				alias.Remote = resolvedRemote
				aliasUpdated = true
			}
			if alias.DefaultBranch == "" && plan.DefaultBranch != "" {
				alias.DefaultBranch = plan.DefaultBranch
				aliasUpdated = true
			}
			if aliasUpdated {
				cfg.Repos[plan.Name] = alias
			}
		}
	}

	registerWorkspace(&cfg, wsConfig.Name, wsRoot, s.clock())
	if err := s.configs.Save(ctx, info.Path, cfg); err != nil {
		return GroupApplyResultJSON{}, info, err
	}

	return GroupApplyResultJSON{
		Status:    "ok",
		Template:  name,
		Workspace: wsConfig.Name,
	}, info, nil
}
