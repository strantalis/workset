package worksetapi

import (
	"context"
	"errors"
	"os"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/ops"
)

func (s *Service) removeWorkspaceRepoWorktrees(
	ctx context.Context,
	root string,
	defaults config.Defaults,
	force bool,
) error {
	logf := s.logf
	ws, err := s.workspaces.Load(ctx, root, defaults)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if force {
			if logf != nil {
				logf("warning: failed to load workspace for worktree cleanup: %v", err)
			}
			return nil
		}
		return err
	}
	if len(ws.Config.Repos) == 0 && logf != nil {
		logf("worktree cleanup: no repos in %s; scanning for linked worktrees", root)
	}
	for _, repo := range ws.Config.Repos {
		if repo.Name == "" {
			continue
		}
		if _, err := ops.RemoveRepo(ctx, ops.RemoveRepoInput{
			WorkspaceRoot:   root,
			Name:            repo.Name,
			Defaults:        defaults,
			Git:             s.git,
			DeleteWorktrees: true,
			DeleteLocal:     false,
			Force:           force,
			Logf:            logf,
		}); err != nil {
			if force {
				if logf != nil {
					logf("warning: failed to remove worktrees for %s: %v", repo.Name, err)
				}
				continue
			}
			return err
		}
	}
	if err := ops.CleanupWorkspaceWorktrees(ops.CleanupWorkspaceWorktreesInput{
		WorkspaceRoot: root,
		Git:           s.git,
		Force:         force,
		Logf:          logf,
	}); err != nil {
		if force {
			if logf != nil {
				logf("warning: failed to clean up remaining worktrees: %v", err)
			}
			return nil
		}
		return err
	}
	return nil
}
