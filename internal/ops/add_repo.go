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
	Remote        string
	DefaultBranch string
	AllowFallback bool
	Git           git.Client
}

func AddRepo(ctx context.Context, input AddRepoInput) (config.WorkspaceConfig, string, []string, error) {
	if input.WorkspaceRoot == "" {
		return config.WorkspaceConfig{}, "", nil, errors.New("workspace root required")
	}
	if input.Name == "" {
		return config.WorkspaceConfig{}, "", nil, errors.New("repo name required")
	}
	if input.URL == "" && input.SourcePath == "" {
		return config.WorkspaceConfig{}, "", nil, errors.New("repo url or local path required")
	}
	if input.Git == nil {
		return config.WorkspaceConfig{}, "", nil, errors.New("git client required")
	}

	if input.SourcePath == "" && looksLikeLocalPath(input.URL) {
		input.SourcePath = input.URL
		input.URL = ""
	}
	if input.SourcePath != "" {
		resolved, err := resolveLocalPath(input.SourcePath)
		if err != nil {
			return config.WorkspaceConfig{}, "", nil, err
		}
		input.SourcePath = resolved
	}

	ws, err := workspace.Load(input.WorkspaceRoot, input.Defaults)
	if err != nil {
		return config.WorkspaceConfig{}, "", nil, err
	}

	for _, repo := range ws.Config.Repos {
		if repo.Name == input.Name {
			return config.WorkspaceConfig{}, "", nil, fmt.Errorf("repo %q already exists in workspace", input.Name)
		}
	}

	repo := config.RepoConfig{
		Name:    input.Name,
		RepoDir: input.RepoDir,
	}
	if repo.RepoDir == "" {
		repo.RepoDir = repo.Name
	}
	defaultBranch := strings.TrimSpace(input.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = input.Defaults.BaseBranch
	}
	remote := strings.TrimSpace(input.Remote)
	if remote == "" {
		remote = input.Defaults.Remote
	}

	var gitDirPath string
	if input.SourcePath != "" {
		if ok, err := input.Git.IsRepo(input.SourcePath); err != nil {
			return config.WorkspaceConfig{}, "", nil, err
		} else if !ok {
			return config.WorkspaceConfig{}, "", nil, fmt.Errorf("local repo not found at %s", input.SourcePath)
		}
		repo.LocalPath = input.SourcePath
		gitDirPath = input.SourcePath
	} else {
		if input.Defaults.RepoStoreRoot == "" {
			return config.WorkspaceConfig{}, "", nil, errors.New("defaults.repo_store_root required for URL clones")
		}
		target := filepath.Join(input.Defaults.RepoStoreRoot, repo.Name)
		target, err := filepath.Abs(target)
		if err != nil {
			return config.WorkspaceConfig{}, "", nil, err
		}
		if ok, err := input.Git.IsRepo(target); err != nil {
			return config.WorkspaceConfig{}, "", nil, err
		} else if !ok {
			if err := input.Git.Clone(ctx, input.URL, target, remote); err != nil {
				return config.WorkspaceConfig{}, "", nil, fmt.Errorf("clone %s: %w", input.URL, err)
			}
		}
		repo.LocalPath = target
		repo.Managed = true
		gitDirPath = target
	}
	resolvedGitDir, err := resolveGitDirPath(gitDirPath)
	if err != nil {
		return config.WorkspaceConfig{}, "", nil, err
	}
	gitDirPath = resolvedGitDir

	warnings := []string{}
	if input.SourcePath != "" {
		resolvedRemote, warn, err := resolveRemoteForLocalRepo(gitDirPath, input.Git, remote, input.AllowFallback)
		if err != nil {
			return config.WorkspaceConfig{}, "", nil, err
		}
		remote = resolvedRemote
		if warn != "" {
			warnings = append(warnings, warn)
		}
	}

	targetBranch := ws.State.CurrentBranch
	if targetBranch == "" {
		targetBranch = defaultBranch
	}

	worktreePath := workspace.RepoWorktreePath(input.WorkspaceRoot, targetBranch, repo.RepoDir)
	worktreeExists := false
	if _, err := os.Stat(worktreePath); err == nil {
		ok, err := input.Git.IsRepo(worktreePath)
		if err != nil {
			return config.WorkspaceConfig{}, "", nil, err
		}
		if !ok {
			return config.WorkspaceConfig{}, "", nil, fmt.Errorf("worktree path %q exists but is not a git repo", worktreePath)
		}
		worktreeExists = true
	}

	if !worktreeExists && targetBranch != "" {
		branch, ok, err := input.Git.CurrentBranch(gitDirPath)
		if err != nil {
			return config.WorkspaceConfig{}, "", nil, err
		}
		if ok && branch == targetBranch {
			return config.WorkspaceConfig{}, "", nil, fmt.Errorf(
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
			return config.WorkspaceConfig{}, "", nil, err
		}
		if err := workspace.WriteBranchMeta(input.WorkspaceRoot, targetBranch); err != nil {
			return config.WorkspaceConfig{}, "", nil, err
		}
	}

	startRemote := ""
	if remote != "" {
		exists, err := input.Git.RemoteExists(gitDirPath, remote)
		if err != nil {
			return config.WorkspaceConfig{}, "", nil, err
		}
		if exists {
			startRemote = remote
		}
	}

	if !worktreeExists {
		if startRemote != "" && defaultBranch != "" {
			if err := input.Git.Fetch(ctx, gitDirPath, startRemote); err != nil {
				warnings = append(warnings, fmt.Sprintf("fetch %s failed: %v", startRemote, err))
			} else {
				localRef := fmt.Sprintf("refs/heads/%s", defaultBranch)
				remoteRef := fmt.Sprintf("refs/remotes/%s/%s", startRemote, defaultBranch)
				localExists, err := input.Git.ReferenceExists(gitDirPath, localRef)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("check local base branch %s: %v", defaultBranch, err))
				}
				remoteExists, err := input.Git.ReferenceExists(gitDirPath, remoteRef)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("check remote base branch %s/%s: %v", startRemote, defaultBranch, err))
				}
				skipUpdate := false
				if repo.LocalPath != "" && !looksLikeBareRepo(repo.LocalPath) {
					if branch, ok, err := input.Git.CurrentBranch(repo.LocalPath); err != nil {
						warnings = append(warnings, fmt.Sprintf("check current branch for %s: %v", repo.LocalPath, err))
					} else if ok && branch == defaultBranch {
						skipUpdate = true
					}
				}

				if localExists && remoteExists && !skipUpdate {
					ancestor, err := input.Git.IsAncestor(gitDirPath, localRef, remoteRef)
					if err != nil {
						warnings = append(warnings, fmt.Sprintf("compare base branch %s to %s/%s: %v", defaultBranch, startRemote, defaultBranch, err))
					} else if ancestor {
						if err := input.Git.UpdateBranch(ctx, gitDirPath, defaultBranch, fmt.Sprintf("%s/%s", startRemote, defaultBranch)); err != nil {
							warnings = append(warnings, fmt.Sprintf("fast-forward %s to %s/%s failed: %v", defaultBranch, startRemote, defaultBranch, err))
						}
					} else {
						warnings = append(warnings, fmt.Sprintf("base branch %s does not fast-forward to %s/%s; leaving local branch unchanged", defaultBranch, startRemote, defaultBranch))
					}
				}
			}
		}

		worktreeName := workspace.WorktreeName(targetBranch)
		if err := input.Git.WorktreeAdd(ctx, git.WorktreeAddOptions{
			RepoPath:     gitDirPath,
			WorktreePath: worktreePath,
			WorktreeName: worktreeName,
			BranchName:   targetBranch,
			StartRemote:  startRemote,
			StartBranch:  defaultBranch,
		}); err != nil {
			return config.WorkspaceConfig{}, "", nil, fmt.Errorf("add worktree: %w", err)
		}
	}

	ws.Config.Repos = append(ws.Config.Repos, repo)
	if err := config.SaveWorkspace(workspace.WorksetFile(input.WorkspaceRoot), ws.Config); err != nil {
		return config.WorkspaceConfig{}, "", nil, err
	}
	if err := workspace.UpdateAgentsFile(input.WorkspaceRoot, ws.Config, ws.State); err != nil {
		return config.WorkspaceConfig{}, "", nil, fmt.Errorf("update agents: %w", err)
	}
	return ws.Config, remote, warnings, nil
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

func resolveRemoteForLocalRepo(repoPath string, gitClient git.Client, preferred string, allowFallback bool) (string, string, error) {
	remotes, err := gitClient.RemoteNames(repoPath)
	if err != nil {
		return "", "", err
	}
	if preferred != "" {
		for _, name := range remotes {
			if name == preferred {
				return preferred, "", nil
			}
		}
		if !allowFallback {
			return "", "", fmt.Errorf("remote %q not found in repo; set an alias remote", preferred)
		}
	}
	if len(remotes) == 1 {
		sort.Strings(remotes)
		if preferred == "" {
			return remotes[0], fmt.Sprintf("no remote configured; using %q", remotes[0]), nil
		}
		return remotes[0], fmt.Sprintf("remote %q not found; using %q", preferred, remotes[0]), nil
	}
	if preferred == "" {
		return "", "", errors.New("remote required; set defaults.remote or repo alias remote")
	}
	if len(remotes) == 0 {
		return "", "", fmt.Errorf("remote %q not found and repo has no remotes", preferred)
	}
	return "", "", fmt.Errorf("remote %q not found; repo has multiple remotes", preferred)
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
