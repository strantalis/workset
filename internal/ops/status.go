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
	WorkspaceRoot       string
	Defaults            config.Defaults
	RepoDefaultBranches map[string]string
	Git                 git.Client
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
		branch := ws.State.CurrentBranch
		if branch == "" {
			if input.RepoDefaultBranches != nil {
				branch = input.RepoDefaultBranches[repo.Name]
			}
			if branch == "" {
				branch = input.Defaults.BaseBranch
			}
		}
		path := workspace.RepoWorktreePath(ws.Root, branch, repo.RepoDir)
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
