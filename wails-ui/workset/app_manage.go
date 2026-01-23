package main

import (
	"context"
	"strings"

	"github.com/strantalis/workset/pkg/worksetapi"
)

type WorkspaceCreateRequest struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Repos  []string `json:"repos,omitempty"`
	Groups []string `json:"groups,omitempty"`
}

type WorkspaceCreateResponse struct {
	Workspace    worksetapi.WorkspaceCreatedJSON `json:"workspace"`
	Warnings     []string                        `json:"warnings,omitempty"`
	PendingHooks []worksetapi.HookPendingJSON    `json:"pendingHooks,omitempty"`
}

type RepoAddRequest struct {
	WorkspaceID string `json:"workspaceId"`
	Source      string `json:"source"`
	Name        string `json:"name,omitempty"`
	RepoDir     string `json:"repoDir,omitempty"`
}

type RepoAddResponse struct {
	Payload      worksetapi.RepoAddResultJSON `json:"payload"`
	Warnings     []string                     `json:"warnings,omitempty"`
	PendingHooks []worksetapi.HookPendingJSON `json:"pendingHooks,omitempty"`
}

type RepoRemoveRequest struct {
	WorkspaceID    string `json:"workspaceId"`
	RepoName       string `json:"repoName"`
	DeleteWorktree bool   `json:"deleteWorktree"`
	DeleteLocal    bool   `json:"deleteLocal"`
}

type AliasUpsertRequest struct {
	Name          string `json:"name"`
	Source        string `json:"source"`
	Remote        string `json:"remote"`
	DefaultBranch string `json:"defaultBranch"`
}

type GroupUpsertRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type GroupMemberRequest struct {
	GroupName string `json:"groupName"`
	RepoName  string `json:"repoName"`
}

func (a *App) CreateWorkspace(input WorkspaceCreateRequest) (WorkspaceCreateResponse, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	result, err := a.service.CreateWorkspace(ctx, worksetapi.WorkspaceCreateInput{
		Name:   input.Name,
		Path:   input.Path,
		Repos:  input.Repos,
		Groups: input.Groups,
	})
	if err != nil {
		return WorkspaceCreateResponse{}, err
	}

	response := WorkspaceCreateResponse{
		Workspace: result.Workspace,
		Warnings:  result.Warnings,
	}
	if len(result.PendingHooks) > 0 {
		response.PendingHooks = make([]worksetapi.HookPendingJSON, 0, len(result.PendingHooks))
		for _, pending := range result.PendingHooks {
			response.PendingHooks = append(response.PendingHooks, worksetapi.HookPendingJSON(pending))
		}
	}
	return response, nil
}

func (a *App) ArchiveWorkspace(workspaceID, reason string) (worksetapi.WorkspaceRefJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	result, _, err := a.service.ArchiveWorkspace(ctx, worksetapi.WorkspaceSelector{Value: workspaceID}, reason)
	return result, err
}

func (a *App) UnarchiveWorkspace(workspaceID string) (worksetapi.WorkspaceRefJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	result, _, err := a.service.UnarchiveWorkspace(ctx, worksetapi.WorkspaceSelector{Value: workspaceID})
	return result, err
}

func (a *App) RemoveWorkspace(workspaceID string) (worksetapi.WorkspaceDeleteResultJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	result, err := a.service.DeleteWorkspace(ctx, worksetapi.WorkspaceDeleteInput{
		Selector:    worksetapi.WorkspaceSelector{Value: workspaceID},
		DeleteFiles: false,
		Confirmed:   true,
	})
	if err != nil {
		return worksetapi.WorkspaceDeleteResultJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) RenameWorkspace(workspaceID, newName string) (worksetapi.WorkspaceRefJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	result, err := a.service.RenameWorkspace(ctx, worksetapi.WorkspaceRenameInput{
		Selector: worksetapi.WorkspaceSelector{Value: workspaceID},
		NewName:  newName,
	})
	return result, err
}

func (a *App) AddRepo(input RepoAddRequest) (RepoAddResponse, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	name := input.Name
	nameSet := false
	if name != "" {
		nameSet = true
	}

	result, err := a.service.AddRepo(ctx, worksetapi.RepoAddInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Source:    input.Source,
		Name:      name,
		NameSet:   nameSet,
		RepoDir:   input.RepoDir,
	})
	if err != nil {
		return RepoAddResponse{}, err
	}

	response := RepoAddResponse{
		Payload:  result.Payload,
		Warnings: result.Warnings,
	}
	if len(result.PendingHooks) > 0 {
		response.PendingHooks = make([]worksetapi.HookPendingJSON, 0, len(result.PendingHooks))
		for _, pending := range result.PendingHooks {
			response.PendingHooks = append(response.PendingHooks, worksetapi.HookPendingJSON(pending))
		}
	}
	return response, nil
}

func (a *App) RemoveRepo(input RepoRemoveRequest) (worksetapi.RepoRemoveResultJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}

	result, err := a.service.RemoveRepo(ctx, worksetapi.RepoRemoveInput{
		Workspace:       worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Name:            input.RepoName,
		DeleteWorktrees: input.DeleteWorktree,
		DeleteLocal:     input.DeleteLocal,
		Confirmed:       true,
	})
	if err != nil {
		return worksetapi.RepoRemoveResultJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) ListAliases() ([]worksetapi.AliasJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, err := a.service.ListAliases(ctx)
	if err != nil {
		return nil, err
	}
	return result.Aliases, nil
}

func (a *App) CreateAlias(input AliasUpsertRequest) (worksetapi.AliasMutationResultJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, _, err := a.service.CreateAlias(ctx, worksetapi.AliasUpsertInput{
		Name:             input.Name,
		Source:           input.Source,
		Remote:           input.Remote,
		DefaultBranch:    input.DefaultBranch,
		SourceSet:        strings.TrimSpace(input.Source) != "",
		RemoteSet:        strings.TrimSpace(input.Remote) != "",
		DefaultBranchSet: strings.TrimSpace(input.DefaultBranch) != "",
	})
	return result, err
}

func (a *App) UpdateAlias(input AliasUpsertRequest) (worksetapi.AliasMutationResultJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, _, err := a.service.UpdateAlias(ctx, worksetapi.AliasUpsertInput{
		Name:             input.Name,
		Source:           input.Source,
		Remote:           input.Remote,
		DefaultBranch:    input.DefaultBranch,
		SourceSet:        strings.TrimSpace(input.Source) != "",
		RemoteSet:        strings.TrimSpace(input.Remote) != "",
		DefaultBranchSet: strings.TrimSpace(input.DefaultBranch) != "",
	})
	return result, err
}

func (a *App) DeleteAlias(name string) (worksetapi.AliasMutationResultJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, _, err := a.service.DeleteAlias(ctx, name)
	return result, err
}

func (a *App) ListGroups() ([]worksetapi.GroupSummaryJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, err := a.service.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	return result.Groups, nil
}

func (a *App) GetGroup(name string) (worksetapi.GroupJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, _, err := a.service.GetGroup(ctx, name)
	return result, err
}

func (a *App) CreateGroup(input GroupUpsertRequest) (worksetapi.GroupJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, _, err := a.service.CreateGroup(ctx, worksetapi.GroupUpsertInput{
		Name:        input.Name,
		Description: input.Description,
	})
	return result, err
}

func (a *App) UpdateGroup(input GroupUpsertRequest) (worksetapi.GroupJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, _, err := a.service.UpdateGroup(ctx, worksetapi.GroupUpsertInput{
		Name:        input.Name,
		Description: input.Description,
	})
	return result, err
}

func (a *App) DeleteGroup(name string) (worksetapi.AliasMutationResultJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, _, err := a.service.DeleteGroup(ctx, name)
	return result, err
}

func (a *App) AddGroupMember(input GroupMemberRequest) (worksetapi.GroupJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, _, err := a.service.AddGroupMember(ctx, worksetapi.GroupMemberInput{
		GroupName: input.GroupName,
		RepoName:  input.RepoName,
	})
	return result, err
}

func (a *App) RemoveGroupMember(input GroupMemberRequest) (worksetapi.GroupJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, _, err := a.service.RemoveGroupMember(ctx, worksetapi.GroupMemberInput{
		GroupName: input.GroupName,
		RepoName:  input.RepoName,
	})
	return result, err
}

func (a *App) ApplyGroup(workspaceID, groupName string) (worksetapi.GroupApplyResultJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
	result, _, err := a.service.ApplyGroup(ctx, worksetapi.GroupApplyInput{
		Workspace: worksetapi.WorkspaceSelector{Value: workspaceID},
		Name:      groupName,
	})
	return result, err
}
