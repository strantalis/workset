package worksetapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/ops"
	"github.com/strantalis/workset/internal/session"
)

func (s *Service) stopWorkspaceSessions(ctx context.Context, root string, force bool) error {
	state, err := s.workspaces.LoadState(ctx, root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if force {
			if s.logf != nil {
				s.logf("warning: failed to read workspace session state: %v", err)
			}
			return nil
		}
		return err
	}
	if len(state.Sessions) == 0 {
		return nil
	}
	for name, entry := range state.Sessions {
		sessionName := name
		if strings.TrimSpace(entry.Name) != "" {
			sessionName = entry.Name
		}
		backendValue := strings.TrimSpace(entry.Backend)
		if backendValue == "" {
			if force {
				if s.logf != nil {
					s.logf("warning: session %s missing backend; skipping", sessionName)
				}
				continue
			}
			return fmt.Errorf("session %s missing backend; use --force to skip", sessionName)
		}
		backend, err := session.ParseBackend(backendValue)
		if err != nil {
			if force {
				if s.logf != nil {
					s.logf("warning: session %s has invalid backend %q: %v", sessionName, backendValue, err)
				}
				continue
			}
			return err
		}
		if backend == session.BackendAuto || backend == session.BackendExec {
			if force {
				if s.logf != nil {
					s.logf("warning: session %s uses unsupported backend %q; skipping", sessionName, backend)
				}
				continue
			}
			return fmt.Errorf("session %s uses unsupported backend %q; use --force to skip", sessionName, backend)
		}
		if err := s.runner.LookPath(string(backend)); err != nil {
			if force {
				if s.logf != nil {
					s.logf("warning: %s not available to stop session %s: %v", backend, sessionName, err)
				}
				continue
			}
			return fmt.Errorf("%s not available to stop session %s", backend, sessionName)
		}
		exists, err := session.Exists(ctx, s.runner, backend, sessionName)
		if err != nil {
			if force {
				if s.logf != nil {
					s.logf("warning: failed to check session %s: %v", sessionName, err)
				}
				continue
			}
			return err
		}
		if !exists {
			continue
		}
		if err := session.Stop(ctx, s.runner, backend, sessionName); err != nil {
			if force {
				if s.logf != nil {
					s.logf("warning: failed to stop session %s: %v", sessionName, err)
				}
				continue
			}
			return err
		}
	}
	return nil
}

func (s *Service) removeWorkspaceRepoWorktrees(ctx context.Context, root string, defaults config.Defaults, force bool) error {
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
