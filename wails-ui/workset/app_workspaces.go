package main

import (
	"context"

	"github.com/strantalis/workset/pkg/worksetapi"
)

type WorkspaceSnapshot struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Path           string         `json:"path"`
	Template       string         `json:"template,omitempty"`
	CreatedAt      string         `json:"createdAt,omitempty"`
	LastUsed       string         `json:"lastUsed,omitempty"`
	ArchivedAt     string         `json:"archivedAt,omitempty"`
	ArchivedReason string         `json:"archivedReason,omitempty"`
	Archived       bool           `json:"archived"`
	Pinned         bool           `json:"pinned"`
	PinOrder       int            `json:"pinOrder"`
	Color          string         `json:"color,omitempty"`
	Description    string         `json:"description,omitempty"`
	Expanded       bool           `json:"expanded"`
	Repos          []RepoSnapshot `json:"repos"`
}

type RepoSnapshot struct {
	ID                 string                 `json:"id"`
	Name               string                 `json:"name"`
	Path               string                 `json:"path"`
	Remote             string                 `json:"remote,omitempty"`
	DefaultBranch      string                 `json:"defaultBranch,omitempty"`
	CurrentBranch      string                 `json:"currentBranch,omitempty"`
	Ahead              int                    `json:"ahead"`
	Behind             int                    `json:"behind"`
	Dirty              bool                   `json:"dirty"`
	Missing            bool                   `json:"missing"`
	StatusKnown        bool                   `json:"statusKnown"`
	Diff               RepoDiffStatSnapshot   `json:"diff"`
	Files              []RepoDiffFileSnapshot `json:"files"`
	TrackedPullRequest *TrackedPullRequestRef `json:"trackedPullRequest,omitempty"`
}

type RepoDiffStatSnapshot struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

type RepoDiffFileSnapshot struct {
	Path    string `json:"path"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

type TrackedPullRequestRef struct {
	Repo       string `json:"repo"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	Body       string `json:"body,omitempty"`
	State      string `json:"state"`
	Draft      bool   `json:"draft"`
	Merged     bool   `json:"merged"`
	BaseRepo   string `json:"baseRepo"`
	BaseBranch string `json:"baseBranch"`
	HeadRepo   string `json:"headRepo"`
	HeadBranch string `json:"headBranch"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
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
			var tracked *TrackedPullRequestRef
			if repo.TrackedPullRequest != nil {
				tracked = &TrackedPullRequestRef{
					Repo:       repo.TrackedPullRequest.Repo,
					Number:     repo.TrackedPullRequest.Number,
					URL:        repo.TrackedPullRequest.URL,
					Title:      repo.TrackedPullRequest.Title,
					Body:       repo.TrackedPullRequest.Body,
					State:      repo.TrackedPullRequest.State,
					Draft:      repo.TrackedPullRequest.Draft,
					Merged:     repo.TrackedPullRequest.Merged,
					BaseRepo:   repo.TrackedPullRequest.BaseRepo,
					BaseBranch: repo.TrackedPullRequest.BaseBranch,
					HeadRepo:   repo.TrackedPullRequest.HeadRepo,
					HeadBranch: repo.TrackedPullRequest.HeadBranch,
					UpdatedAt:  repo.TrackedPullRequest.UpdatedAt,
				}
			}
			repoSnapshot := RepoSnapshot{
				ID:                 repoID,
				Name:               repo.Name,
				Path:               repo.LocalPath,
				Remote:             repo.Remote,
				DefaultBranch:      repo.DefaultBranch,
				Dirty:              repo.Dirty,
				Missing:            repo.Missing,
				StatusKnown:        repo.StatusKnown,
				Diff:               RepoDiffStatSnapshot{},
				Files:              []RepoDiffFileSnapshot{},
				TrackedPullRequest: tracked,
			}

			if input.IncludeStatus && !repo.Missing {
				localStatus, localErr := a.service.GetRepoLocalStatus(ctx, worksetapi.RepoLocalStatusInput{
					Workspace: worksetapi.WorkspaceSelector{Value: workspace.Name},
					Repo:      repo.Name,
				})
				if localErr == nil {
					repoSnapshot.Dirty = localStatus.Payload.HasUncommitted
					repoSnapshot.StatusKnown = true
					repoSnapshot.Ahead = localStatus.Payload.Ahead
					repoSnapshot.Behind = localStatus.Payload.Behind
					repoSnapshot.CurrentBranch = localStatus.Payload.CurrentBranch

					if localStatus.Payload.HasUncommitted {
						diffSummary, diffErr := a.GetRepoDiffSummary(workspace.Name, repoID)
						if diffErr == nil {
							repoSnapshot.Diff = RepoDiffStatSnapshot{
								Added:   diffSummary.TotalAdded,
								Removed: diffSummary.TotalRemoved,
							}
							files := make([]RepoDiffFileSnapshot, 0, len(diffSummary.Files))
							for _, file := range diffSummary.Files {
								files = append(files, RepoDiffFileSnapshot{
									Path:    file.Path,
									Added:   file.Added,
									Removed: file.Removed,
								})
							}
							repoSnapshot.Files = files
						}
					}
				}
			}

			repos = append(repos, repoSnapshot)
		}

		snapshots = append(snapshots, WorkspaceSnapshot{
			ID:             workspace.Name,
			Name:           workspace.Name,
			Path:           workspace.Path,
			Template:       workspace.Template,
			CreatedAt:      workspace.CreatedAt,
			LastUsed:       workspace.LastUsed,
			ArchivedAt:     workspace.ArchivedAt,
			ArchivedReason: workspace.ArchivedReason,
			Archived:       workspace.Archived,
			Pinned:         workspace.Pinned,
			PinOrder:       workspace.PinOrder,
			Color:          workspace.Color,
			Description:    workspace.Description,
			Expanded:       workspace.Expanded,
			Repos:          repos,
		})
	}

	return snapshots, nil
}
