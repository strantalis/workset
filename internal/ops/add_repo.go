package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/workspace"
)

type AddRepoInput struct {
	WorkspaceRoot string
	Name          string
	URL           string
	SourcePath    string
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
	if input.URL == "" && input.SourcePath == "" {
		return config.WorkspaceConfig{}, errors.New("repo url or local path required")
	}
	if input.Git == nil {
		return config.WorkspaceConfig{}, errors.New("git client required")
	}

	if input.SourcePath == "" && looksLikeLocalPath(input.URL) {
		input.SourcePath = input.URL
		input.URL = ""
	}
	if input.SourcePath != "" {
		resolved, err := resolveLocalPath(input.SourcePath)
		if err != nil {
			return config.WorkspaceConfig{}, err
		}
		input.SourcePath = resolved
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
		Name:    input.Name,
		RepoDir: input.RepoDir,
		Remotes: input.Remotes,
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

	var gitDirPath string
	if input.SourcePath != "" {
		if ok, err := input.Git.IsRepo(input.SourcePath); err != nil {
			return config.WorkspaceConfig{}, err
		} else if !ok {
			return config.WorkspaceConfig{}, fmt.Errorf("local repo not found at %s", input.SourcePath)
		}
		repo.LocalPath = input.SourcePath
		gitDirPath = input.SourcePath
	} else {
		if input.Defaults.RepoStoreRoot == "" {
			return config.WorkspaceConfig{}, errors.New("defaults.repo_store_root required for URL clones")
		}
		target := filepath.Join(input.Defaults.RepoStoreRoot, repo.Name)
		target, err := filepath.Abs(target)
		if err != nil {
			return config.WorkspaceConfig{}, err
		}
		if ok, err := input.Git.IsRepo(target); err != nil {
			return config.WorkspaceConfig{}, err
		} else if !ok {
			if err := input.Git.Clone(ctx, input.URL, target, repo.Remotes.Write.Name); err != nil {
				return config.WorkspaceConfig{}, fmt.Errorf("clone %s: %w", input.URL, err)
			}
		}
		repo.LocalPath = target
		repo.Managed = true
		gitDirPath = target
	}
	resolvedGitDir, err := resolveGitDirPath(gitDirPath)
	if err != nil {
		return config.WorkspaceConfig{}, err
	}
	gitDirPath = resolvedGitDir

	if repo.Remotes.Base.Name != repo.Remotes.Write.Name {
		if input.URL != "" {
			if err := input.Git.AddRemote(gitDirPath, repo.Remotes.Base.Name, input.URL); err != nil {
				return config.WorkspaceConfig{}, fmt.Errorf("add base remote: %w", err)
			}
		}
	}

	targetBranch := ws.State.CurrentBranch
	if targetBranch == "" {
		targetBranch = repo.Remotes.Base.DefaultBranch
	}

	if targetBranch != "" {
		branch, ok, err := input.Git.CurrentBranch(gitDirPath)
		if err != nil {
			return config.WorkspaceConfig{}, err
		}
		if ok && branch == targetBranch {
			return config.WorkspaceConfig{}, fmt.Errorf(
				"branch %q already checked out in %s; git only allows a branch in one worktree",
				targetBranch,
				repo.LocalPath,
			)
		}
	}

	useBranchDirs := workspace.UseBranchDirs(input.WorkspaceRoot)
	if useBranchDirs {
		branchPath := workspace.WorktreeBranchPath(input.WorkspaceRoot, targetBranch)
		if err := os.MkdirAll(branchPath, 0o755); err != nil {
			return config.WorkspaceConfig{}, err
		}
		if err := workspace.WriteBranchMeta(input.WorkspaceRoot, targetBranch); err != nil {
			return config.WorkspaceConfig{}, err
		}
	}

	startRemote := repo.Remotes.Base.Name
	if startRemote != "" {
		exists, err := input.Git.RemoteExists(gitDirPath, startRemote)
		if err != nil {
			return config.WorkspaceConfig{}, err
		}
		if !exists {
			startRemote = ""
		}
	}
	if startRemote == "" && repo.Remotes.Write.Name != "" {
		exists, err := input.Git.RemoteExists(gitDirPath, repo.Remotes.Write.Name)
		if err != nil {
			return config.WorkspaceConfig{}, err
		}
		if exists {
			startRemote = repo.Remotes.Write.Name
		}
	}

	worktreePath := workspace.RepoWorktreePath(input.WorkspaceRoot, targetBranch, repo.RepoDir)
	worktreeName := workspace.WorktreeName(targetBranch)
	if err := input.Git.WorktreeAdd(ctx, git.WorktreeAddOptions{
		RepoPath:     gitDirPath,
		WorktreePath: worktreePath,
		WorktreeName: worktreeName,
		BranchName:   targetBranch,
		StartRemote:  startRemote,
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

func resolveGitDirPath(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path %q is not a directory", path)
	}

	dotGit := filepath.Join(path, ".git")
	if stat, err := os.Stat(dotGit); err == nil {
		if stat.IsDir() {
			return dotGit, nil
		}
		data, err := os.ReadFile(dotGit)
		if err != nil {
			return "", err
		}
		line := strings.TrimSpace(string(data))
		const prefix = "gitdir:"
		if strings.HasPrefix(line, prefix) {
			gitDir := strings.TrimSpace(line[len(prefix):])
			if !filepath.IsAbs(gitDir) {
				gitDir = filepath.Join(path, gitDir)
			}
			return gitDir, nil
		}
		return "", fmt.Errorf("invalid .git file in %q", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if looksLikeBareRepo(path) {
		return path, nil
	}

	return "", fmt.Errorf("unable to locate git dir for %q", path)
}

func resolveLocalPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("local path required")
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func looksLikeBareRepo(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "HEAD")); err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(path, "objects"))
	if err != nil {
		return false
	}
	return info.IsDir()
}

func looksLikeLocalPath(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "~") || strings.HasPrefix(value, ".") {
		return true
	}
	return filepath.IsAbs(value)
}
