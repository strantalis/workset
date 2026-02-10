package worksetapi

import (
	"context"
	"os"
	"sort"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/ops"
	"github.com/strantalis/workset/internal/workspace"
)

// ListWorkspaceSnapshots returns workspace snapshots with optional repo status.
func (s *Service) ListWorkspaceSnapshots(ctx context.Context, opts WorkspaceSnapshotOptions) (WorkspaceSnapshotResult, error) {
	cfg, info, err := s.loadGlobal(ctx)
	if err != nil {
		return WorkspaceSnapshotResult{}, err
	}
	if len(cfg.Workspaces) == 0 {
		return WorkspaceSnapshotResult{Workspaces: []WorkspaceSnapshotJSON{}, Config: info}, nil
	}

	names := make([]string, 0, len(cfg.Workspaces))
	for name := range cfg.Workspaces {
		names = append(names, name)
	}
	sort.Strings(names)

	snapshots := make([]WorkspaceSnapshotJSON, 0, len(names))
	for _, name := range names {
		ref := cfg.Workspaces[name]
		if !opts.IncludeArchived && ref.ArchivedAt != "" {
			continue
		}
		root := ref.Path
		if root == "" {
			if s.logf != nil {
				s.logf("workspace snapshots: skipping %q (path missing)", name)
			}
			continue
		}

		wsConfig, err := s.workspaces.LoadConfig(ctx, root)
		hasWorkspaceConfig := true
		if err != nil {
			if os.IsNotExist(err) {
				if s.logf != nil {
					s.logf("workspace snapshots: workspace config missing for %q at %s", name, worksetFilePath(root))
				}
				hasWorkspaceConfig = false
				wsConfig = config.WorkspaceConfig{}
			} else {
				if s.logf != nil {
					s.logf("workspace snapshots: skipping %q (load config: %v)", name, err)
				}
				continue
			}
		}
		if hasWorkspaceConfig {
			if err := s.migrateLegacyWorkspaceRemotes(ctx, &cfg, info.Path, root, &wsConfig); err != nil {
				return WorkspaceSnapshotResult{}, err
			}
		}
		var state workspace.State
		if hasWorkspaceConfig {
			state, err = s.workspaces.LoadState(ctx, root)
			if err != nil && !os.IsNotExist(err) {
				if s.logf != nil {
					s.logf("workspace snapshots: state unavailable for %q: %v", name, err)
				}
				state = workspace.State{}
			}
		}

		repos := make([]RepoSnapshotJSON, 0, len(wsConfig.Repos))
		repoDefaults := make(map[string]string, len(wsConfig.Repos))
		for _, repo := range wsConfig.Repos {
			config.ApplyRepoDefaults(&repo, cfg.Defaults)
			defaults := resolveRepoDefaults(cfg, repo.Name)
			repoDefaults[repo.Name] = defaults.DefaultBranch
			var trackedPR *TrackedPullRequestSnapshotJSON
			if pr, ok := state.PullRequests[repo.Name]; ok {
				trackedPR = &TrackedPullRequestSnapshotJSON{
					Repo:       pr.Repo,
					Number:     pr.Number,
					URL:        pr.URL,
					Title:      pr.Title,
					Body:       pr.Body,
					State:      pr.State,
					Draft:      pr.Draft,
					BaseRepo:   pr.BaseRepo,
					BaseBranch: pr.BaseBranch,
					HeadRepo:   pr.HeadRepo,
					HeadBranch: pr.HeadBranch,
					UpdatedAt:  pr.UpdatedAt,
				}
			}
			repos = append(repos, RepoSnapshotJSON{
				Name:               repo.Name,
				LocalPath:          repo.LocalPath,
				Managed:            repo.Managed,
				RepoDir:            repo.RepoDir,
				Remote:             defaults.Remote,
				DefaultBranch:      defaults.DefaultBranch,
				Dirty:              false,
				Missing:            false,
				StatusKnown:        false,
				TrackedPullRequest: trackedPR,
			})
		}

		if opts.IncludeStatus && hasWorkspaceConfig {
			statuses, err := ops.Status(ctx, ops.StatusInput{
				WorkspaceRoot:       root,
				Defaults:            cfg.Defaults,
				RepoDefaultBranches: repoDefaults,
				Git:                 s.git,
			})
			if err != nil {
				if s.logf != nil {
					s.logf("workspace snapshots: status unavailable for %q: %v", name, err)
				}
			} else {
				byName := map[string]ops.RepoStatus{}
				for _, status := range statuses {
					byName[status.Name] = status
				}
				for i := range repos {
					if status, ok := byName[repos[i].Name]; ok && status.Err == nil {
						repos[i].Dirty = status.Dirty
						repos[i].Missing = status.Missing
						repos[i].StatusKnown = true
					}
				}
			}
		}

		snapshots = append(snapshots, WorkspaceSnapshotJSON{
			Name:           name,
			Path:           ref.Path,
			CreatedAt:      ref.CreatedAt,
			LastUsed:       ref.LastUsed,
			ArchivedAt:     ref.ArchivedAt,
			ArchivedReason: ref.ArchivedReason,
			Archived:       ref.ArchivedAt != "",
			Pinned:         ref.Pinned,
			PinOrder:       ref.PinOrder,
			Color:          ref.Color,
			Description:    ref.Description,
			Expanded:       ref.Expanded,
			Repos:          repos,
		})
	}

	return WorkspaceSnapshotResult{Workspaces: snapshots, Config: info}, nil
}
