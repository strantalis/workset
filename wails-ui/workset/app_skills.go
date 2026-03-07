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

type SearchMarketplaceSkillsRequest struct {
	WorkspaceID string `json:"workspaceId,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Query       string `json:"query"`
	Limit       int    `json:"limit,omitempty"`
}

type MarketplaceSkillRequest struct {
	WorkspaceID  string `json:"workspaceId,omitempty"`
	Provider     string `json:"provider"`
	ExternalID   string `json:"externalId,omitempty"`
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	SourceRepo   string `json:"sourceRepo,omitempty"`
	SourceURL    string `json:"sourceUrl,omitempty"`
	ListingURL   string `json:"listingUrl,omitempty"`
	RawSkillURL  string `json:"rawSkillUrl,omitempty"`
	InstallCount *int   `json:"installCount,omitempty"`
}

type InstallMarketplaceSkillRequest struct {
	WorkspaceID  string   `json:"workspaceId,omitempty"`
	Provider     string   `json:"provider"`
	ExternalID   string   `json:"externalId,omitempty"`
	Name         string   `json:"name,omitempty"`
	Description  string   `json:"description,omitempty"`
	SourceRepo   string   `json:"sourceRepo,omitempty"`
	SourceURL    string   `json:"sourceUrl,omitempty"`
	ListingURL   string   `json:"listingUrl,omitempty"`
	RawSkillURL  string   `json:"rawSkillUrl,omitempty"`
	InstallCount *int     `json:"installCount,omitempty"`
	Scope        string   `json:"scope"`
	DirName      string   `json:"dirName"`
	Tools        []string `json:"tools"`
}

type AttachSkillMarketplaceSourceRequest struct {
	WorkspaceID string   `json:"workspaceId,omitempty"`
	Scope       string   `json:"scope"`
	DirName     string   `json:"dirName"`
	Tools       []string `json:"tools"`
	Provider    string   `json:"provider"`
	ExternalID  string   `json:"externalId,omitempty"`
	SourceRepo  string   `json:"sourceRepo,omitempty"`
	SourceURL   string   `json:"sourceUrl,omitempty"`
	ListingURL  string   `json:"listingUrl,omitempty"`
	RawSkillURL string   `json:"rawSkillUrl,omitempty"`
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

func (a *App) SearchMarketplaceSkills(input SearchMarketplaceSkillsRequest) ([]worksetapi.MarketplaceSkill, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.SearchMarketplaceSkills(ctx, input.Provider, input.Query, input.Limit)
}

func (a *App) GetMarketplaceSkillContent(input MarketplaceSkillRequest) (worksetapi.MarketplaceSkillContent, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.GetMarketplaceSkillContent(ctx, worksetapi.MarketplaceSkillRequest{
		Provider:     input.Provider,
		ExternalID:   input.ExternalID,
		Name:         input.Name,
		Description:  input.Description,
		SourceRepo:   input.SourceRepo,
		SourceURL:    input.SourceURL,
		ListingURL:   input.ListingURL,
		RawSkillURL:  input.RawSkillURL,
		InstallCount: input.InstallCount,
	})
}

func (a *App) GetMarketplaceSkillMetadata(input MarketplaceSkillRequest) (worksetapi.MarketplaceSkill, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.GetMarketplaceSkillMetadata(ctx, worksetapi.MarketplaceSkillRequest{
		Provider:     input.Provider,
		ExternalID:   input.ExternalID,
		Name:         input.Name,
		Description:  input.Description,
		SourceRepo:   input.SourceRepo,
		SourceURL:    input.SourceURL,
		ListingURL:   input.ListingURL,
		RawSkillURL:  input.RawSkillURL,
		InstallCount: input.InstallCount,
	})
}

func (a *App) InstallMarketplaceSkill(input InstallMarketplaceSkillRequest) (worksetapi.SkillInfo, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.InstallMarketplaceSkill(ctx, worksetapi.InstallMarketplaceSkillInput{
		MarketplaceSkillRequest: worksetapi.MarketplaceSkillRequest{
			Provider:     input.Provider,
			ExternalID:   input.ExternalID,
			Name:         input.Name,
			Description:  input.Description,
			SourceRepo:   input.SourceRepo,
			SourceURL:    input.SourceURL,
			ListingURL:   input.ListingURL,
			RawSkillURL:  input.RawSkillURL,
			InstallCount: input.InstallCount,
		},
		Scope:   input.Scope,
		DirName: input.DirName,
		Tools:   input.Tools,
	}, a.resolveProjectRoot(ctx, input.WorkspaceID))
}

func (a *App) AttachSkillMarketplaceSource(input AttachSkillMarketplaceSourceRequest) error {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.AttachSkillMarketplaceSource(ctx, input.Scope, input.DirName, input.Tools, worksetapi.SkillMarketplaceSource{
		Provider:    input.Provider,
		ExternalID:  input.ExternalID,
		SourceRepo:  input.SourceRepo,
		SourceURL:   input.SourceURL,
		ListingURL:  input.ListingURL,
		RawSkillURL: input.RawSkillURL,
	}, a.resolveProjectRoot(ctx, input.WorkspaceID))
}
