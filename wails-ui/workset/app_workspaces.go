package main

import (
	"context"

	"github.com/strantalis/workset/pkg/worksetapi"
)

type WorkspaceSnapshot struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Path           string         `json:"path"`
	ArchivedAt     string         `json:"archivedAt,omitempty"`
	ArchivedReason string         `json:"archivedReason,omitempty"`
	Archived       bool           `json:"archived"`
	Repos          []RepoSnapshot `json:"repos"`
}

type RepoSnapshot struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Remote        string `json:"remote,omitempty"`
	DefaultBranch string `json:"defaultBranch,omitempty"`
	Dirty         bool   `json:"dirty"`
	Missing       bool   `json:"missing"`
	StatusKnown   bool   `json:"statusKnown"`
}

type WorkspaceSnapshotRequest struct {
	IncludeArchived bool `json:"includeArchived"`
	IncludeStatus   bool `json:"includeStatus"`
}

// ListWorkspaceSnapshots returns workspaces and their repos for the UI.
func (a *App) ListWorkspaceSnapshots(input WorkspaceSnapshotRequest) ([]WorkspaceSnapshot, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()

	result, err := a.service.ListWorkspaceSnapshots(ctx, worksetapi.WorkspaceSnapshotOptions{
		IncludeArchived: input.IncludeArchived,
		IncludeStatus:   input.IncludeStatus,
	})
	if err != nil {
		return nil, err
	}

	snapshots := make([]WorkspaceSnapshot, 0, len(result.Workspaces))
	for _, workspace := range result.Workspaces {
		repos := make([]RepoSnapshot, 0, len(workspace.Repos))
		for _, repo := range workspace.Repos {
			repoID := workspace.Name + "::" + repo.Name
			repos = append(repos, RepoSnapshot{
				ID:            repoID,
				Name:          repo.Name,
				Path:          repo.LocalPath,
				Remote:        repo.Remote,
				DefaultBranch: repo.DefaultBranch,
				Dirty:         repo.Dirty,
				Missing:       repo.Missing,
				StatusKnown:   repo.StatusKnown,
			})
		}

		snapshots = append(snapshots, WorkspaceSnapshot{
			ID:             workspace.Name,
			Name:           workspace.Name,
			Path:           workspace.Path,
			ArchivedAt:     workspace.ArchivedAt,
			ArchivedReason: workspace.ArchivedReason,
			Archived:       workspace.Archived,
			Repos:          repos,
		})
	}

	return snapshots, nil
}
