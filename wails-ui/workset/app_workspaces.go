package main

import "github.com/strantalis/workset/pkg/worksetapi"

type WorkspaceSnapshot struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Path           string         `json:"path"`
	Workset        string         `json:"workset,omitempty"`
	WorksetKey     string         `json:"worksetKey,omitempty"`
	WorksetLabel   string         `json:"worksetLabel,omitempty"`
	Placeholder    bool           `json:"placeholder,omitempty"`
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
	Repo                string `json:"repo"`
	Number              int    `json:"number"`
	URL                 string `json:"url"`
	Title               string `json:"title"`
	Body                string `json:"body,omitempty"`
	State               string `json:"state"`
	Draft               bool   `json:"draft"`
	Merged              bool   `json:"merged"`
	BaseRepo            string `json:"baseRepo"`
	BaseBranch          string `json:"baseBranch"`
	HeadRepo            string `json:"headRepo"`
	HeadBranch          string `json:"headBranch"`
	UpdatedAt           string `json:"updatedAt,omitempty"`
	Author              string `json:"author,omitempty"`
	CommentsCount       int    `json:"commentsCount,omitempty"`
	ReviewCommentsCount int    `json:"reviewCommentsCount,omitempty"`
}

type WorkspaceSnapshotRequest struct {
	IncludeArchived bool `json:"includeArchived"`
	IncludeStatus   bool `json:"includeStatus"`
}

// ListWorkspaceSnapshots returns workspaces and their repos for the UI.
func (a *App) ListWorkspaceSnapshots(input WorkspaceSnapshotRequest) ([]WorkspaceSnapshot, error) {
	ctx, svc := a.serviceContext()
	result, err := svc.ListWorkspaceSnapshots(ctx, worksetapi.WorkspaceSnapshotOptions{
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
			repoPath := repo.LocalPath
			if resolvedPath, resolveErr := resolveWorkspaceRepoRoot(workspace.Path, repo.RepoDir, repo.LocalPath); resolveErr == nil {
				repoPath = resolvedPath
			}
			var tracked *TrackedPullRequestRef
			if repo.TrackedPullRequest != nil {
				tracked = &TrackedPullRequestRef{
					Repo:                repo.TrackedPullRequest.Repo,
					Number:              repo.TrackedPullRequest.Number,
					URL:                 repo.TrackedPullRequest.URL,
					Title:               repo.TrackedPullRequest.Title,
					Body:                repo.TrackedPullRequest.Body,
					State:               repo.TrackedPullRequest.State,
					Draft:               repo.TrackedPullRequest.Draft,
					Merged:              repo.TrackedPullRequest.Merged,
					BaseRepo:            repo.TrackedPullRequest.BaseRepo,
					BaseBranch:          repo.TrackedPullRequest.BaseBranch,
					HeadRepo:            repo.TrackedPullRequest.HeadRepo,
					HeadBranch:          repo.TrackedPullRequest.HeadBranch,
					UpdatedAt:           repo.TrackedPullRequest.UpdatedAt,
					Author:              repo.TrackedPullRequest.Author,
					CommentsCount:       repo.TrackedPullRequest.CommentsCount,
					ReviewCommentsCount: repo.TrackedPullRequest.ReviewCommentsCount,
				}
			}
			repoSnapshot := RepoSnapshot{
				ID:                 repoID,
				Name:               repo.Name,
				Path:               repoPath,
				Remote:             repo.Remote,
				DefaultBranch:      repo.DefaultBranch,
				Dirty:              repo.Dirty,
				Missing:            repo.Missing,
				StatusKnown:        repo.StatusKnown,
				Diff:               RepoDiffStatSnapshot{},
				Files:              []RepoDiffFileSnapshot{},
				TrackedPullRequest: tracked,
			}

			if input.IncludeStatus && !workspace.Placeholder && !repo.Missing && repoPath != "" {
				localStatus, localErr := loadRepoLocalStatus(ctx, repoPath, repo.DefaultBranch)
				if localErr == nil {
					repoSnapshot.Dirty = localStatus.payload.HasUncommitted
					repoSnapshot.StatusKnown = true
					repoSnapshot.Ahead = localStatus.payload.Ahead
					repoSnapshot.Behind = localStatus.payload.Behind
					repoSnapshot.CurrentBranch = localStatus.payload.CurrentBranch

					if localStatus.payload.HasUncommitted {
						diffSummary, diffErr := a.cachedRepoDiffSummary(ctx, repoPath, localStatus)
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
			Workset:        workspace.Workset,
			WorksetKey:     workspace.WorksetKey,
			WorksetLabel:   workspace.WorksetLabel,
			Placeholder:    workspace.Placeholder,
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
