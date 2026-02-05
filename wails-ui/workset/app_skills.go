package main

import (
	"context"

	"github.com/strantalis/workset/pkg/worksetapi"
)

type ListSkillsRequest struct {
	WorkspaceID string `json:"workspaceId,omitempty"`
}

type GetSkillRequest struct {
	WorkspaceID string `json:"workspaceId,omitempty"`
	Scope       string `json:"scope"`
	DirName     string `json:"dirName"`
	Tool        string `json:"tool"`
}

type SaveSkillRequest struct {
	WorkspaceID string `json:"workspaceId,omitempty"`
	Scope       string `json:"scope"`
	DirName     string `json:"dirName"`
	Tool        string `json:"tool"`
	Content     string `json:"content"`
}

type DeleteSkillRequest struct {
	WorkspaceID string `json:"workspaceId,omitempty"`
	Scope       string `json:"scope"`
	DirName     string `json:"dirName"`
	Tool        string `json:"tool"`
}

type SyncSkillRequest struct {
	WorkspaceID string   `json:"workspaceId,omitempty"`
	Scope       string   `json:"scope"`
	DirName     string   `json:"dirName"`
	FromTool    string   `json:"fromTool"`
	ToTools     []string `json:"toTools"`
}

func (a *App) resolveProjectRoot(ctx context.Context, workspaceID string) string {
	if workspaceID == "" {
		return ""
	}
	path, err := a.resolveWorkspacePath(ctx, workspaceID)
	if err != nil {
		return ""
	}
	return path
}

func (a *App) ListSkills(input ListSkillsRequest) ([]worksetapi.SkillInfo, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.ListSkills(ctx, a.resolveProjectRoot(ctx, input.WorkspaceID))
}

func (a *App) GetSkill(input GetSkillRequest) (worksetapi.SkillContent, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.GetSkillWithRoot(ctx, input.Scope, input.DirName, input.Tool, a.resolveProjectRoot(ctx, input.WorkspaceID))
}

func (a *App) SaveSkill(input SaveSkillRequest) error {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.SaveSkillWithRoot(ctx, input.Scope, input.DirName, input.Tool, input.Content, a.resolveProjectRoot(ctx, input.WorkspaceID))
}

func (a *App) DeleteSkill(input DeleteSkillRequest) error {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.DeleteSkillWithRoot(ctx, input.Scope, input.DirName, input.Tool, a.resolveProjectRoot(ctx, input.WorkspaceID))
}

func (a *App) SyncSkill(input SyncSkillRequest) error {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.SyncSkillWithRoot(ctx, input.Scope, input.DirName, input.FromTool, input.ToTools, a.resolveProjectRoot(ctx, input.WorkspaceID))
}
