package ops

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/workspace"
)

type RepoBranchSafety struct {
	Branch         string
	Path           string
	Dirty          bool
	Missing        bool
	Unmerged       bool
	UnmergedErr    string
	UnmergedReason string
	Unpushed       bool
	UnpushedErr    string
	StatusErr      string
	FetchRemoteErr string
}

type RepoSafetyReport struct {
	RepoName      string
	Remote        string
	DefaultBranch string
	Branches      []RepoBranchSafety
}

type RepoDefaults struct {
	Remote        string
	DefaultBranch string
}

type RepoSafetyInput struct {
	WorkspaceRoot string
	Repo          config.RepoConfig
	Defaults      config.Defaults
	RepoDefaults  RepoDefaults
	Git           git.Client
	FetchRemotes  bool
}

func CheckRepoSafety(ctx context.Context, input RepoSafetyInput) (RepoSafetyReport, error) {
	if input.WorkspaceRoot == "" {
		return RepoSafetyReport{}, errors.New("workspace root required")
	}
	if input.Git == nil {
		return RepoSafetyReport{}, errors.New("git client required")
	}
	repo := input.Repo
	config.ApplyRepoDefaults(&repo, input.Defaults)

	branches, err := listBranches(input.WorkspaceRoot, input.Defaults)
	if err != nil {
		return RepoSafetyReport{}, err
	}

	remote := strings.TrimSpace(input.RepoDefaults.Remote)
	if remote == "" {
		remote = input.Defaults.Remote
	}
	defaultBranch := strings.TrimSpace(input.RepoDefaults.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = input.Defaults.BaseBranch
	}

	report := RepoSafetyReport{
		RepoName:      repo.Name,
		Remote:        remote,
		DefaultBranch: defaultBranch,
	}
	repoPathForRefs := repo.LocalPath

	for _, branch := range branches {
		worktreePath := workspace.RepoWorktreePath(input.WorkspaceRoot, branch, repo.RepoDir)
		if _, err := os.Stat(worktreePath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return RepoSafetyReport{}, err
		}

		status, err := input.Git.Status(worktreePath)
		entry := RepoBranchSafety{
			Branch:  branch,
			Path:    worktreePath,
			Dirty:   status.Dirty,
			Missing: status.Missing,
		}
		if err != nil {
			entry.StatusErr = err.Error()
		}
		report.Branches = append(report.Branches, entry)
		if repoPathForRefs == "" {
			repoPathForRefs = worktreePath
		}
	}

	if repoPathForRefs == "" {
		return report, nil
	}

	remoteExists := false
	if remote != "" {
		exists, err := input.Git.RemoteExists(repoPathForRefs, remote)
		if err != nil {
			return RepoSafetyReport{}, err
		}
		remoteExists = exists
	}
	if remote == "" {
		return RepoSafetyReport{}, errors.New("remote required for safety checks")
	}
	if !remoteExists {
		return RepoSafetyReport{}, fmt.Errorf("remote %q not found", remote)
	}

	if input.FetchRemotes {
		if remoteExists {
			if err := input.Git.Fetch(ctx, repoPathForRefs, remote); err != nil {
				for i := range report.Branches {
					report.Branches[i].FetchRemoteErr = err.Error()
				}
			}
		}
	}

	baseRef := ""
	baseRefExists := false
	var baseRefErr error
	localBaseRef := ""
	localBaseExists := false
	var localBaseErr error
	baseCheckAvailable := false
	if defaultBranch != "" {
		localBaseRef = fmt.Sprintf("refs/heads/%s", defaultBranch)
		localBaseExists, localBaseErr = input.Git.ReferenceExists(repoPathForRefs, localBaseRef)
	}
	if remoteExists && defaultBranch != "" {
		baseRef = fmt.Sprintf("refs/remotes/%s/%s", remote, defaultBranch)
		baseRefExists, baseRefErr = input.Git.ReferenceExists(repoPathForRefs, baseRef)
	}
	if defaultBranch != "" {
		if remoteExists && baseRefExists && baseRefErr == nil {
			baseCheckAvailable = true
		} else if localBaseExists && localBaseErr == nil {
			baseCheckAvailable = true
		}
	}

	for i := range report.Branches {
		entry := &report.Branches[i]
		if entry.Missing {
			continue
		}

		branchRef := fmt.Sprintf("refs/heads/%s", entry.Branch)

		if remoteExists && defaultBranch != "" && baseRefExists && baseRefErr == nil {
			merged, err := input.Git.IsAncestor(repoPathForRefs, branchRef, baseRef)
			if err != nil {
				entry.UnmergedErr = err.Error()
			} else if !merged {
				contentMerged, err := input.Git.IsContentMerged(repoPathForRefs, branchRef, baseRef)
				if err != nil {
					entry.UnmergedErr = err.Error()
					entry.Unmerged = true
					entry.UnmergedReason = fmt.Sprintf("unmerged check failed for %s/%s", remote, defaultBranch)
				} else if !contentMerged {
					localMerged := false
					if entry.FetchRemoteErr != "" {
						if localBaseExists && localBaseErr == nil {
							if mergedLocal, err := input.Git.IsContentMerged(repoPathForRefs, branchRef, localBaseRef); err == nil {
								localMerged = mergedLocal
							}
						}
					}
					if !localMerged {
						entry.Unmerged = true
						if entry.FetchRemoteErr != "" {
							entry.UnmergedReason = fmt.Sprintf("branch content not found in %s/%s history (remote fetch failed; local %s differs)", remote, defaultBranch, defaultBranch)
						} else {
							entry.UnmergedReason = fmt.Sprintf("branch content not found in %s/%s history", remote, defaultBranch)
						}
					}
				}
			}
		} else if localBaseExists && localBaseErr == nil && defaultBranch != "" {
			contentMerged, err := input.Git.IsContentMerged(repoPathForRefs, branchRef, localBaseRef)
			if err != nil {
				entry.UnmergedErr = err.Error()
				entry.Unmerged = true
				entry.UnmergedReason = fmt.Sprintf("unmerged check failed for local %s", defaultBranch)
			} else if !contentMerged {
				entry.Unmerged = true
				entry.UnmergedReason = fmt.Sprintf("branch content not found in local %s history", defaultBranch)
			}
		} else if baseRefErr != nil {
			entry.UnmergedErr = baseRefErr.Error()
		} else if localBaseErr != nil {
			entry.UnmergedErr = localBaseErr.Error()
		}

		if remoteExists {
			remoteRef := fmt.Sprintf("refs/remotes/%s/%s", remote, entry.Branch)
			remoteRefExists, err := input.Git.ReferenceExists(repoPathForRefs, remoteRef)
			if err != nil {
				entry.UnpushedErr = err.Error()
			} else if !remoteRefExists {
				if entry.Unmerged || !baseCheckAvailable {
					entry.Unpushed = true
				}
			} else {
				pushed, err := input.Git.IsAncestor(repoPathForRefs, remoteRef, branchRef)
				if err != nil {
					entry.UnpushedErr = err.Error()
				} else if !pushed {
					entry.Unpushed = true
				}
			}
		}
	}

	return report, nil
}

type RemoveRepoInput struct {
	WorkspaceRoot   string
	Name            string
	Defaults        config.Defaults
	Git             git.Client
	DeleteWorktrees bool
	DeleteLocal     bool
	Force           bool
	Logf            func(format string, args ...any)
}

func RemoveRepo(ctx context.Context, input RemoveRepoInput) (config.WorkspaceConfig, error) {
	if input.WorkspaceRoot == "" {
		return config.WorkspaceConfig{}, errors.New("workspace root required")
	}
	if input.Name == "" {
		return config.WorkspaceConfig{}, errors.New("repo name required")
	}
	if input.Git == nil {
		return config.WorkspaceConfig{}, errors.New("git client required")
	}

	ws, err := workspace.Load(input.WorkspaceRoot, input.Defaults)
	if err != nil {
		return config.WorkspaceConfig{}, err
	}

	repoIndex := -1
	var repo config.RepoConfig
	for i, cfg := range ws.Config.Repos {
		if cfg.Name == input.Name {
			repoIndex = i
			repo = cfg
			break
		}
	}
	if repoIndex == -1 {
		return config.WorkspaceConfig{}, fmt.Errorf("repo %q not found in workspace", input.Name)
	}
	config.ApplyRepoDefaults(&repo, input.Defaults)
	if input.Logf != nil {
		input.Logf("repo remove: %s local_path=%q repo_dir=%q", input.Name, repo.LocalPath, repo.RepoDir)
	}

	if input.DeleteWorktrees {
		branches, err := listBranches(input.WorkspaceRoot, input.Defaults)
		if err != nil {
			return config.WorkspaceConfig{}, err
		}
		if input.Logf != nil {
			input.Logf("repo remove: branches=%v", branches)
		}
		repoGitDir := ""
		if repo.LocalPath != "" {
			resolved, err := resolveGitDirPath(repo.LocalPath)
			if err != nil {
				return config.WorkspaceConfig{}, err
			}
			repoGitDir = resolved
			if input.Logf != nil {
				input.Logf("repo remove: repo git dir=%s", repoGitDir)
			}
		}
		for _, branch := range branches {
			worktreePath := workspace.RepoWorktreePath(input.WorkspaceRoot, branch, repo.RepoDir)
			pathExists := true
			if _, err := os.Stat(worktreePath); err != nil {
				if os.IsNotExist(err) {
					if input.Logf != nil {
						input.Logf("repo remove: worktree missing at %s", worktreePath)
					}
					pathExists = false
				} else {
					return config.WorkspaceConfig{}, err
				}
			}
			worktreeName := workspace.WorktreeName(branch)
			removeGitDir := repoGitDir
			if resolvedDir, resolvedName, ok, err := worktreeAdminFromPath(worktreePath); err == nil && ok {
				removeGitDir = resolvedDir
				worktreeName = resolvedName
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				return config.WorkspaceConfig{}, err
			}
			if removeGitDir == "" {
				removeGitDir = repoGitDir
			}
			if input.Logf != nil {
				input.Logf("repo remove: removing worktree %s (gitdir %s, path %s)", worktreeName, removeGitDir, worktreePath)
			}
			if removeGitDir != "" {
				if err := input.Git.WorktreeRemove(git.WorktreeRemoveOptions{
					RepoPath:     removeGitDir,
					WorktreeName: worktreeName,
					Force:        input.Force,
				}); err != nil {
					if !errors.Is(err, git.ErrWorktreeNotFound) {
						if input.Logf != nil {
							input.Logf("repo remove: worktree remove failed for %s (%v)", worktreeName, err)
						}
						return config.WorkspaceConfig{}, err
					}
					if input.Logf != nil {
						input.Logf("repo remove: worktree %s not found in %s", worktreeName, removeGitDir)
					}
					if resolvedName, ok, err := findWorktreeNameByPath(removeGitDir, worktreePath); err == nil && ok && resolvedName != worktreeName {
						if input.Logf != nil {
							input.Logf("repo remove: retry remove as %s (matched by gitdir)", resolvedName)
						}
						if err := input.Git.WorktreeRemove(git.WorktreeRemoveOptions{
							RepoPath:     removeGitDir,
							WorktreeName: resolvedName,
							Force:        input.Force,
						}); err != nil {
							if !errors.Is(err, git.ErrWorktreeNotFound) {
								if input.Logf != nil {
									input.Logf("repo remove: retry remove failed for %s (%v)", resolvedName, err)
								}
								return config.WorkspaceConfig{}, err
							}
							if input.Logf != nil {
								input.Logf("repo remove: retry worktree %s not found in %s", resolvedName, removeGitDir)
							}
						} else if input.Logf != nil {
							input.Logf("repo remove: removed %s via retry", resolvedName)
						}
					} else if err != nil && !errors.Is(err, os.ErrNotExist) {
						return config.WorkspaceConfig{}, err
					}
				}
			}
			if pathExists {
				if err := os.RemoveAll(worktreePath); err != nil {
					return config.WorkspaceConfig{}, err
				}
				if err := removeIfEmpty(filepath.Dir(worktreePath)); err != nil {
					return config.WorkspaceConfig{}, err
				}
			}
		}
	}

	if input.DeleteLocal {
		if !repo.Managed {
			return config.WorkspaceConfig{}, fmt.Errorf("refusing to delete unmanaged repo at %s", repo.LocalPath)
		}
		if repo.LocalPath == "" {
			return config.WorkspaceConfig{}, errors.New("local_path missing for repo")
		}
		if err := os.RemoveAll(repo.LocalPath); err != nil {
			return config.WorkspaceConfig{}, err
		}
	}

	ws.Config.Repos = append(ws.Config.Repos[:repoIndex], ws.Config.Repos[repoIndex+1:]...)
	if err := config.SaveWorkspace(workspace.WorksetFile(input.WorkspaceRoot), ws.Config); err != nil {
		return config.WorkspaceConfig{}, err
	}
	return ws.Config, nil
}

type WorkspaceSafetyReport struct {
	Root  string
	Repos []RepoSafetyReport
}

type CleanupWorkspaceWorktreesInput struct {
	WorkspaceRoot string
	Git           git.Client
	Force         bool
	Logf          func(format string, args ...any)
}

func CleanupWorkspaceWorktrees(input CleanupWorkspaceWorktreesInput) error {
	if input.WorkspaceRoot == "" {
		return errors.New("workspace root required")
	}
	if input.Git == nil {
		return errors.New("git client required")
	}
	paths, err := findWorktreePaths(input.WorkspaceRoot)
	if err != nil {
		return err
	}
	if input.Logf != nil && len(paths) == 0 {
		input.Logf("worktree cleanup: no worktree paths found under %s", input.WorkspaceRoot)
	}
	for _, path := range paths {
		if input.Logf != nil {
			input.Logf("worktree cleanup: found %s", path)
		}
		commonDir, worktreeName, ok, err := worktreeAdminFromPath(path)
		if err != nil {
			if input.Logf != nil {
				input.Logf("worktree cleanup: parse failed for %s (%v)", path, err)
			}
			if input.Force {
				continue
			}
			return err
		}
		if !ok {
			if input.Logf != nil {
				input.Logf("worktree cleanup: no linked worktree metadata for %s", path)
			}
			continue
		}
		if input.Logf != nil {
			input.Logf("worktree cleanup: removing %s (gitdir %s)", worktreeName, commonDir)
		}
		if err := input.Git.WorktreeRemove(git.WorktreeRemoveOptions{
			RepoPath:     commonDir,
			WorktreeName: worktreeName,
			Force:        input.Force,
		}); err != nil {
			if errors.Is(err, git.ErrWorktreeNotFound) {
				if input.Logf != nil {
					input.Logf("worktree cleanup: %s not found in %s", worktreeName, commonDir)
				}
				continue
			}
			if input.Logf != nil {
				input.Logf("worktree cleanup: failed removing %s in %s (%v)", worktreeName, commonDir, err)
			}
			if input.Force {
				continue
			}
			return err
		}
		if input.Logf != nil {
			input.Logf("worktree cleanup: removed %s", worktreeName)
		}
	}
	return nil
}

type WorkspaceSafetyInput struct {
	WorkspaceRoot string
	Defaults      config.Defaults
	RepoDefaults  map[string]RepoDefaults
	Git           git.Client
	FetchRemotes  bool
}

func CheckWorkspaceSafety(ctx context.Context, input WorkspaceSafetyInput) (WorkspaceSafetyReport, error) {
	if input.WorkspaceRoot == "" {
		return WorkspaceSafetyReport{}, errors.New("workspace root required")
	}
	ws, err := workspace.Load(input.WorkspaceRoot, input.Defaults)
	if err != nil {
		return WorkspaceSafetyReport{}, err
	}

	report := WorkspaceSafetyReport{Root: input.WorkspaceRoot}
	for _, repo := range ws.Config.Repos {
		repoDefaults := RepoDefaults{}
		if input.RepoDefaults != nil {
			repoDefaults = input.RepoDefaults[repo.Name]
		}
		repoReport, err := CheckRepoSafety(ctx, RepoSafetyInput{
			WorkspaceRoot: input.WorkspaceRoot,
			Repo:          repo,
			Defaults:      input.Defaults,
			RepoDefaults:  repoDefaults,
			Git:           input.Git,
			FetchRemotes:  input.FetchRemotes,
		})
		if err != nil {
			return WorkspaceSafetyReport{}, err
		}
		report.Repos = append(report.Repos, repoReport)
	}
	return report, nil
}

func listBranches(root string, defaults config.Defaults) ([]string, error) {
	entries, err := os.ReadDir(workspace.WorktreesPath(root))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		ws, loadErr := workspace.Load(root, defaults)
		if loadErr != nil {
			return nil, loadErr
		}
		if ws.State.CurrentBranch == "" {
			return nil, nil
		}
		return []string{ws.State.CurrentBranch}, nil
	}
	branches := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dir := filepath.Join(workspace.WorktreesPath(root), entry.Name())
			if name, ok, err := workspace.ReadBranchMeta(dir); err != nil {
				return nil, err
			} else if ok {
				branches = append(branches, name)
			} else {
				branches = append(branches, workspace.BranchNameFromDir(entry.Name()))
			}
		}
	}
	return branches, nil
}

func removeIfEmpty(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return os.Remove(path)
	}
	return nil
}

func worktreeAdminFromPath(path string) (commonDir string, worktreeName string, ok bool, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", "", false, err
	}
	if !info.IsDir() {
		return "", "", false, fmt.Errorf("path %q is not a directory", path)
	}

	dotGit := filepath.Join(path, ".git")
	stat, err := os.Stat(dotGit)
	if err != nil {
		return "", "", false, err
	}
	if stat.IsDir() {
		return "", "", false, nil
	}
	data, err := os.ReadFile(dotGit)
	if err != nil {
		return "", "", false, err
	}
	line := strings.TrimSpace(string(data))
	const prefix = "gitdir:"
	if !strings.HasPrefix(line, prefix) {
		return "", "", false, fmt.Errorf("invalid .git file in %q", path)
	}
	gitDir := strings.TrimSpace(line[len(prefix):])
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(path, gitDir)
	}
	gitDir = filepath.Clean(gitDir)
	worktreeName = filepath.Base(gitDir)
	commonDirPath := filepath.Join(gitDir, "commondir")
	commonData, err := os.ReadFile(commonDirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return gitDir, worktreeName, true, nil
		}
		return "", "", false, err
	}
	commonDir = strings.TrimSpace(string(commonData))
	if commonDir == "" {
		return gitDir, worktreeName, true, nil
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(gitDir, commonDir)
	}
	return filepath.Clean(commonDir), worktreeName, true, nil
}

func findWorktreePaths(root string) ([]string, error) {
	root = filepath.Clean(root)
	paths := make([]string, 0)
	maxDepth := 3
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path == root {
			return nil
		}
		if entry.Name() == ".workset" {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(root, path)
		if err == nil {
			depth := strings.Count(rel, string(os.PathSeparator)) + 1
			if depth > maxDepth {
				return filepath.SkipDir
			}
		}
		dotGit := filepath.Join(path, ".git")
		if stat, err := os.Stat(dotGit); err == nil && !stat.IsDir() {
			paths = append(paths, path)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

func findWorktreeNameByPath(commonGitDir, worktreePath string) (string, bool, error) {
	if commonGitDir == "" || worktreePath == "" {
		return "", false, errors.New("git dir and worktree path required")
	}
	worktreesDir := filepath.Join(commonGitDir, "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	target := filepath.Clean(worktreePath)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		gitdirPath := filepath.Join(worktreesDir, entry.Name(), "gitdir")
		data, err := os.ReadFile(gitdirPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", false, err
		}
		gitdir := strings.TrimSpace(string(data))
		if gitdir == "" {
			continue
		}
		if !filepath.IsAbs(gitdir) {
			gitdir = filepath.Join(commonGitDir, gitdir)
		}
		if filepath.Clean(filepath.Dir(gitdir)) == target {
			return entry.Name(), true, nil
		}
	}
	return "", false, nil
}
