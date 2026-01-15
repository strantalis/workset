package ops

import (
	"context"
	"errors"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/workspace"
)

type RepoStatus struct {
	Name    string
	Path    string
	Dirty   bool
	Missing bool
	Err     error
}

type StatusInput struct {
	WorkspaceRoot string
	Defaults      config.Defaults
	Git           git.Client
}

func Status(ctx context.Context, input StatusInput) ([]RepoStatus, error) {
	if input.WorkspaceRoot == "" {
		return nil, errors.New("workspace root required")
	}
	if input.Git == nil {
		return nil, errors.New("git client required")
	}

	ws, err := workspace.Load(input.WorkspaceRoot, input.Defaults)
	if err != nil {
		return nil, err
	}

	var results []RepoStatus
	for _, repo := range ws.Config.Repos {
		config.ApplyRepoDefaults(&repo, input.Defaults)
		path := repo.LocalPath
		if ws.State.CurrentBranch != "" && ws.State.CurrentBranch != repo.Remotes.Base.DefaultBranch {
			path = workspace.RepoWorktreePath(ws.Root, ws.State.CurrentBranch, repo.RepoDir)
		}
		if path == "" {
			results = append(results, RepoStatus{
				Name: repo.Name,
				Err:  errors.New("local_path missing"),
			})
			continue
		}
		status, err := input.Git.Status(path)
		if err != nil && !status.Missing {
			results = append(results, RepoStatus{
				Name: repo.Name,
				Path: path,
				Err:  err,
			})
			continue
		}
		results = append(results, RepoStatus{
			Name:    repo.Name,
			Path:    path,
			Dirty:   status.Dirty,
			Missing: status.Missing,
		})
	}

	return results, nil
}
