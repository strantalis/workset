package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	if repo.Remotes.Base.DefaultBranch == "" {
		repo.Remotes.Base.DefaultBranch = input.Defaults.BaseBranch
	}
	if repo.Remotes.Write.DefaultBranch == "" {
		repo.Remotes.Write.DefaultBranch = repo.Remotes.Base.DefaultBranch
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

	if input.SourcePath != "" {
		derived, ok, err := deriveRemotesFromLocalRepo(gitDirPath, input.Git)
		if err != nil {
			return config.WorkspaceConfig{}, err
		}
		if ok {
			if repo.Remotes.Base.Name == "" {
				repo.Remotes.Base.Name = derived.Base.Name
			}
			if repo.Remotes.Write.Name == "" {
				repo.Remotes.Write.Name = derived.Write.Name
			}
		}
	} else {
		if repo.Remotes.Base.Name == "" {
			repo.Remotes.Base.Name = "origin"
		}
		if repo.Remotes.Write.Name == "" {
			repo.Remotes.Write.Name = "origin"
		}
	}

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

	worktreePath := workspace.RepoWorktreePath(input.WorkspaceRoot, targetBranch, repo.RepoDir)
	worktreeExists := false
	if _, err := os.Stat(worktreePath); err == nil {
		ok, err := input.Git.IsRepo(worktreePath)
		if err != nil {
			return config.WorkspaceConfig{}, err
		}
		if !ok {
			return config.WorkspaceConfig{}, fmt.Errorf("worktree path %q exists but is not a git repo", worktreePath)
		}
		worktreeExists = true
	}

	if !worktreeExists && targetBranch != "" {
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

	if !worktreeExists {
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

func deriveRemotesFromLocalRepo(repoPath string, gitClient git.Client) (config.Remotes, bool, error) {
	remotes, err := gitClient.RemoteNames(repoPath)
	if err != nil {
		return config.Remotes{}, false, err
	}
	if len(remotes) == 0 {
		return config.Remotes{}, false, nil
	}
	base, write := deriveRemoteNames(remotes)
	return config.Remotes{
		Base:  config.RemoteConfig{Name: base},
		Write: config.RemoteConfig{Name: write},
	}, true, nil
}

func deriveRemoteNames(remotes []string) (string, string) {
	if len(remotes) == 0 {
		return "", ""
	}
	sort.Strings(remotes)
	hasOrigin := false
	hasUpstream := false
	for _, name := range remotes {
		switch name {
		case "origin":
			hasOrigin = true
		case "upstream":
			hasUpstream = true
		}
	}
	base := ""
	write := ""
	if hasUpstream {
		base = "upstream"
	}
	if hasOrigin {
		if base == "" {
			base = "origin"
		}
		write = "origin"
	}
	if base == "" {
		base = remotes[0]
	}
	if write == "" {
		write = base
	}
	return base, write
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
