package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/workspace"
)

type AddRepoInput struct {
	WorkspaceRoot string
	Name          string
	URL           string
	Editable      bool
	RepoDir       string
	Defaults      config.Defaults
	Remotes       config.Remotes
	Git           git.Client
}

func AddRepo(ctx context.Context, input AddRepoInput) (config.WorkspaceConfig, error) {
	if input.WorkspaceRoot == "" {
		return config.WorkspaceConfig{}, errors.New("workspace root required")
	}
	if input.Name == "" {
		return config.WorkspaceConfig{}, errors.New("repo name required")
	}
	if input.URL == "" {
		return config.WorkspaceConfig{}, errors.New("repo url required")
	}
	if input.Git == nil {
		return config.WorkspaceConfig{}, errors.New("git client required")
	}

	ws, err := workspace.Load(input.WorkspaceRoot, input.Defaults)
	if err != nil {
		return config.WorkspaceConfig{}, err
	}

	for _, repo := range ws.Config.Repos {
		if repo.Name == input.Name {
			return config.WorkspaceConfig{}, fmt.Errorf("repo %q already exists in workspace", input.Name)
		}
	}

	repo := config.RepoConfig{
		Name:     input.Name,
		RepoDir:  input.RepoDir,
		Editable: input.Editable,
		Remotes:  input.Remotes,
	}
	if repo.RepoDir == "" {
		repo.RepoDir = repo.Name
	}
	if repo.Remotes.Base.Name == "" {
		repo.Remotes.Base.Name = input.Defaults.Remotes.Base
	}
	if repo.Remotes.Base.DefaultBranch == "" {
		repo.Remotes.Base.DefaultBranch = input.Defaults.BaseBranch
	}
	if repo.Remotes.Write.Name == "" {
		repo.Remotes.Write.Name = input.Defaults.Remotes.Write
	}
	if repo.Remotes.Write.DefaultBranch == "" {
		repo.Remotes.Write.DefaultBranch = input.Defaults.BaseBranch
	}

	gitDirPath := workspace.RepoGitDirPath(input.WorkspaceRoot, repo.Name)
	exists, err := input.Git.IsRepo(gitDirPath)
	if err != nil {
		return config.WorkspaceConfig{}, err
	}
	if !exists {
		if err := input.Git.CloneBare(ctx, input.URL, gitDirPath, repo.Remotes.Write.Name); err != nil {
			return config.WorkspaceConfig{}, fmt.Errorf("clone %s: %w", input.URL, err)
		}
	}

	if repo.Remotes.Base.Name != repo.Remotes.Write.Name {
		if err := input.Git.AddRemote(gitDirPath, repo.Remotes.Base.Name, input.URL); err != nil {
			return config.WorkspaceConfig{}, fmt.Errorf("add base remote: %w", err)
		}
	}

	targetBranch := ws.State.CurrentBranch
	if !repo.Editable {
		targetBranch = repo.Remotes.Base.DefaultBranch
	}

	if err := os.MkdirAll(workspace.BranchPath(input.WorkspaceRoot, targetBranch), 0o755); err != nil {
		return config.WorkspaceConfig{}, err
	}

	worktreePath := workspace.RepoWorktreePath(input.WorkspaceRoot, targetBranch, repo.RepoDir)
	worktreeName := workspace.WorktreeName(targetBranch)
	if err := input.Git.WorktreeAdd(ctx, git.WorktreeAddOptions{
		RepoPath:     gitDirPath,
		WorktreePath: worktreePath,
		WorktreeName: worktreeName,
		BranchName:   targetBranch,
		StartRemote:  repo.Remotes.Base.Name,
		StartBranch:  repo.Remotes.Base.DefaultBranch,
	}); err != nil {
		return config.WorkspaceConfig{}, fmt.Errorf("add worktree: %w", err)
	}

	ws.Config.Repos = append(ws.Config.Repos, repo)
	if err := config.SaveWorkspace(workspace.WorksetFile(input.WorkspaceRoot), ws.Config); err != nil {
		return config.WorkspaceConfig{}, err
	}
	return ws.Config, nil
}

func DeriveRepoNameFromURL(url string) string {
	trimmed := strings.TrimSuffix(url, ".git")
	trimmed = strings.TrimRight(trimmed, "/")
	if idx := strings.LastIndex(trimmed, "/"); idx != -1 {
		return trimmed[idx+1:]
	}
	if idx := strings.LastIndex(trimmed, ":"); idx != -1 {
		return trimmed[idx+1:]
	}
	return trimmed
}
