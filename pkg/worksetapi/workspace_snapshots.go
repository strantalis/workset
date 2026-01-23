package worksetapi

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/ops"
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
			return WorkspaceSnapshotResult{}, fmt.Errorf("workspace path missing for %q", name)
		}

		wsConfig, err := s.workspaces.LoadConfig(ctx, root)
		if err != nil {
			if os.IsNotExist(err) {
				return WorkspaceSnapshotResult{}, NotFoundError{Message: fmt.Sprintf("workset.yaml not found at %s", worksetFilePath(root))}
			}
			return WorkspaceSnapshotResult{}, err
		}
		if err := s.migrateLegacyWorkspaceRemotes(ctx, &cfg, info.Path, root, &wsConfig); err != nil {
			return WorkspaceSnapshotResult{}, err
		}

		repos := make([]RepoSnapshotJSON, 0, len(wsConfig.Repos))
		repoDefaults := make(map[string]string, len(wsConfig.Repos))
		for _, repo := range wsConfig.Repos {
			config.ApplyRepoDefaults(&repo, cfg.Defaults)
			defaults := resolveRepoDefaults(cfg, repo.Name)
			repoDefaults[repo.Name] = defaults.DefaultBranch
			repos = append(repos, RepoSnapshotJSON{
				Name:          repo.Name,
				LocalPath:     repo.LocalPath,
				Managed:       repo.Managed,
				RepoDir:       repo.RepoDir,
				Remote:        defaults.Remote,
				DefaultBranch: defaults.DefaultBranch,
				Dirty:         false,
				Missing:       false,
				StatusKnown:   false,
			})
		}

		if opts.IncludeStatus {
			statuses, err := ops.Status(ctx, ops.StatusInput{
				WorkspaceRoot:       root,
				Defaults:            cfg.Defaults,
				RepoDefaultBranches: repoDefaults,
				Git:                 s.git,
			})
			if err != nil {
				return WorkspaceSnapshotResult{}, err
			}
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

		snapshots = append(snapshots, WorkspaceSnapshotJSON{
			Name:           name,
			Path:           ref.Path,
			ArchivedAt:     ref.ArchivedAt,
			ArchivedReason: ref.ArchivedReason,
			Archived:       ref.ArchivedAt != "",
			Repos:          repos,
		})
	}

	return WorkspaceSnapshotResult{Workspaces: snapshots, Config: info}, nil
}
