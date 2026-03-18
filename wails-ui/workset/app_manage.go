package main

import (
	"strings"

	"github.com/strantalis/workset/pkg/worksetapi"
)

type WorkspaceCreateRequest struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Workset     string   `json:"workset,omitempty"`
	WorksetOnly bool     `json:"worksetOnly,omitempty"`
	Repos       []string `json:"repos,omitempty"`
}

type WorkspaceCreateResponse struct {
	Workspace    worksetapi.WorkspaceCreatedJSON `json:"workspace"`
	Warnings     []string                        `json:"warnings,omitempty"`
	PendingHooks []worksetapi.HookPendingJSON    `json:"pendingHooks,omitempty"`
	HookRuns     []worksetapi.HookExecutionJSON  `json:"hookRuns,omitempty"`
}

type RepoAddRequest struct {
	WorkspaceID string `json:"workspaceId"`
	Source      string `json:"source"`
	Name        string `json:"name,omitempty"`
	RepoDir     string `json:"repoDir,omitempty"`
}

type RepoAddResponse struct {
	Payload      worksetapi.RepoAddResultJSON   `json:"payload"`
	Warnings     []string                       `json:"warnings,omitempty"`
	PendingHooks []worksetapi.HookPendingJSON   `json:"pendingHooks,omitempty"`
	HookRuns     []worksetapi.HookExecutionJSON `json:"hookRuns,omitempty"`
}

type WorksetRepoAddRequest struct {
	Workset string   `json:"workset"`
	Sources []string `json:"sources"`
}

type WorksetRepoAddResponse struct {
	Payload  worksetapi.WorksetRepoAddResultJSON `json:"payload"`
	Warnings []string                            `json:"warnings,omitempty"`
}

type HooksRunRequest struct {
	WorkspaceID string `json:"workspaceId"`
	Repo        string `json:"repo"`
	Event       string `json:"event,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type RepoHooksPreviewRequest struct {
	Source string `json:"source"`
	Ref    string `json:"ref,omitempty"`
}

type HooksRunResponse struct {
	Event   string                   `json:"event"`
	Repo    string                   `json:"repo"`
	Results []worksetapi.HookRunJSON `json:"results"`
}

type RepoRemoveRequest struct {
	WorkspaceID    string `json:"workspaceId"`
	RepoName       string `json:"repoName"`
	DeleteWorktree bool   `json:"deleteWorktree"`
	DeleteLocal    bool   `json:"deleteLocal"`
}

type WorkspaceRemoveRequest struct {
	WorkspaceID  string `json:"workspaceId"`
	DeleteFiles  bool   `json:"deleteFiles"`
	Force        bool   `json:"force"`
	FetchRemotes bool   `json:"fetchRemotes"`
}

type AliasUpsertRequest struct {
	Name          string `json:"name"`
	Source        string `json:"source"`
	Remote        string `json:"remote"`
	DefaultBranch string `json:"defaultBranch"`
}

func (a *App) CreateWorkspace(input WorkspaceCreateRequest) (WorkspaceCreateResponse, error) {
	ctx, svc := a.serviceContext()

	result, err := svc.CreateWorkspace(ctx, worksetapi.WorkspaceCreateInput{
		Name:        input.Name,
		Path:        input.Path,
		Workset:     input.Workset,
		WorksetOnly: input.WorksetOnly,
		Repos:       input.Repos,
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
	if len(result.HookRuns) > 0 {
		response.HookRuns = append(response.HookRuns, result.HookRuns...)
	}
	return response, nil
}

func (a *App) ArchiveWorkspace(workspaceID, reason string) (worksetapi.WorkspaceRefJSON, error) {
	ctx, svc := a.serviceContext()
	result, _, err := svc.ArchiveWorkspace(ctx, worksetapi.WorkspaceSelector{Value: workspaceID}, reason)
	return result, err
}

func (a *App) UnarchiveWorkspace(workspaceID string) (worksetapi.WorkspaceRefJSON, error) {
	ctx, svc := a.serviceContext()
	result, _, err := svc.UnarchiveWorkspace(ctx, worksetapi.WorkspaceSelector{Value: workspaceID})
	return result, err
}

func (a *App) RemoveWorkspace(input WorkspaceRemoveRequest) (worksetapi.WorkspaceDeleteResultJSON, error) {
	ctx, svc := a.serviceContext()

	result, err := svc.DeleteWorkspace(ctx, worksetapi.WorkspaceDeleteInput{
		Selector:     worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		DeleteFiles:  input.DeleteFiles,
		Force:        input.Force,
		Confirmed:    true,
		FetchRemotes: input.FetchRemotes,
	})
	if err != nil {
		return worksetapi.WorkspaceDeleteResultJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) RenameWorkspace(workspaceID, newName string) (worksetapi.WorkspaceRefJSON, error) {
	ctx, svc := a.serviceContext()
	result, err := svc.RenameWorkspace(ctx, worksetapi.WorkspaceRenameInput{
		Selector: worksetapi.WorkspaceSelector{Value: workspaceID},
		NewName:  newName,
	})
	return result, err
}

func (a *App) AddRepo(input RepoAddRequest) (RepoAddResponse, error) {
	ctx, svc := a.serviceContext()

	name := input.Name
	nameSet := false
	if name != "" {
		nameSet = true
	}

	result, err := svc.AddRepo(ctx, worksetapi.RepoAddInput{
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
	if len(result.HookRuns) > 0 {
		response.HookRuns = append(response.HookRuns, result.HookRuns...)
	}
	return response, nil
}

func (a *App) AddReposToWorkset(input WorksetRepoAddRequest) (WorksetRepoAddResponse, error) {
	ctx, svc := a.serviceContext()
	result, err := svc.AddReposToWorkset(ctx, worksetapi.WorksetRepoAddInput{
		Workset: input.Workset,
		Sources: input.Sources,
	})
	if err != nil {
		return WorksetRepoAddResponse{}, err
	}
	return WorksetRepoAddResponse{
		Payload:  result.Payload,
		Warnings: result.Warnings,
	}, nil
}

func (a *App) RunHooks(input HooksRunRequest) (HooksRunResponse, error) {
	ctx, svc := a.serviceContext()
	result, err := svc.RunHooks(ctx, worksetapi.HooksRunInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      input.Repo,
		Event:     input.Event,
		Reason:    input.Reason,
	})
	if err != nil {
		return HooksRunResponse{}, err
	}
	return HooksRunResponse{
		Event:   result.Event,
		Repo:    result.Repo,
		Results: result.Results,
	}, nil
}

func (a *App) TrustRepoHooks(repoName string) error {
	ctx, svc := a.serviceContext()
	_, err := svc.TrustRepoHooks(ctx, repoName)
	return err
}

func (a *App) PreviewRepoHooks(input RepoHooksPreviewRequest) (worksetapi.RepoHooksPreviewJSON, error) {
	ctx, svc := a.serviceContext()
	result, err := svc.PreviewRepoHooks(ctx, worksetapi.RepoHooksPreviewInput{
		Source: input.Source,
		Ref:    input.Ref,
	})
	if err != nil {
		return worksetapi.RepoHooksPreviewJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) RemoveRepo(input RepoRemoveRequest) (worksetapi.RepoRemoveResultJSON, error) {
	ctx, svc := a.serviceContext()
	result, err := svc.RemoveRepo(ctx, worksetapi.RepoRemoveInput{
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

func (a *App) ListRegisteredRepos() ([]worksetapi.RegisteredRepoJSON, error) {
	ctx, svc := a.serviceContext()
	result, err := svc.ListRegisteredRepos(ctx)
	if err != nil {
		return nil, err
	}
	return result.Repos, nil
}

func (a *App) RegisterRepo(input AliasUpsertRequest) (worksetapi.RegisteredRepoMutationResultJSON, error) {
	ctx, svc := a.serviceContext()
	result, _, err := svc.RegisterRepo(ctx, worksetapi.RepoRegistryInput{
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

func (a *App) UpdateRegisteredRepo(input AliasUpsertRequest) (worksetapi.RegisteredRepoMutationResultJSON, error) {
	ctx, svc := a.serviceContext()
	result, _, err := svc.UpdateRegisteredRepo(ctx, worksetapi.RepoRegistryInput{
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

func (a *App) UnregisterRepo(name string) (worksetapi.RegisteredRepoMutationResultJSON, error) {
	ctx, svc := a.serviceContext()
	result, _, err := svc.UnregisterRepo(ctx, name)
	return result, err
}

func (a *App) PinWorkspace(workspaceID string, pin bool) (worksetapi.WorkspaceRefJSON, error) {
	ctx, svc := a.serviceContext()
	result, _, err := svc.PinWorkspace(ctx, worksetapi.WorkspaceSelector{Value: workspaceID}, pin)
	return result, err
}

func (a *App) SetWorkspaceColor(workspaceID, color string) (worksetapi.WorkspaceRefJSON, error) {
	ctx, svc := a.serviceContext()
	result, _, err := svc.SetWorkspaceColor(ctx, worksetapi.WorkspaceSelector{Value: workspaceID}, color)
	return result, err
}

func (a *App) SetWorkspaceDescription(workspaceID, description string) (worksetapi.WorkspaceRefJSON, error) {
	ctx, svc := a.serviceContext()
	result, _, err := svc.SetWorkspaceDescription(ctx, worksetapi.WorkspaceSelector{Value: workspaceID}, description)
	return result, err
}

func (a *App) SetWorkspaceExpanded(workspaceID string, expanded bool) (worksetapi.WorkspaceRefJSON, error) {
	ctx, svc := a.serviceContext()
	result, _, err := svc.SetWorkspaceExpanded(ctx, worksetapi.WorkspaceSelector{Value: workspaceID}, expanded)
	return result, err
}

func (a *App) UpdateWorkspaceLastUsed(workspaceID string) (worksetapi.WorkspaceRefJSON, error) {
	ctx, svc := a.serviceContext()
	result, _, err := svc.UpdateWorkspaceLastUsed(ctx, worksetapi.WorkspaceSelector{Value: workspaceID})
	return result, err
}

type ReorderWorkspacesRequest struct {
	Orders map[string]int `json:"orders"`
}

func (a *App) ReorderWorkspaces(input ReorderWorkspacesRequest) ([]worksetapi.WorkspaceRefJSON, error) {
	ctx, svc := a.serviceContext()
	result, _, err := svc.ReorderWorkspaces(ctx, input.Orders)
	return result, err
}
