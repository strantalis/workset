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

		repos := make([]RepoSnapshotJSON, 0, len(wsConfig.Repos))
		for _, repo := range wsConfig.Repos {
			config.ApplyRepoDefaults(&repo, cfg.Defaults)
			base := repo.Remotes.Base.Name
			if repo.Remotes.Base.DefaultBranch != "" {
				base = fmt.Sprintf("%s/%s", base, repo.Remotes.Base.DefaultBranch)
			}
			write := repo.Remotes.Write.Name
			if repo.Remotes.Write.DefaultBranch != "" {
				write = fmt.Sprintf("%s/%s", write, repo.Remotes.Write.DefaultBranch)
			}
			repos = append(repos, RepoSnapshotJSON{
				Name:        repo.Name,
				LocalPath:   repo.LocalPath,
				Managed:     repo.Managed,
				RepoDir:     repo.RepoDir,
				Base:        base,
				Write:       write,
				Dirty:       false,
				Missing:     false,
				StatusKnown: false,
			})
		}

		if opts.IncludeStatus {
			statuses, err := ops.Status(ctx, ops.StatusInput{
				WorkspaceRoot: root,
				Defaults:      cfg.Defaults,
				Git:           s.git,
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
