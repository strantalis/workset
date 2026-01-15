package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	xworktree "github.com/go-git/go-git/v6/x/plumbing/worktree"
	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/git"
	"github.com/strantalis/workset/internal/workspace"
)

type RepoBranchSafety struct {
	Branch        string
	Path          string
	Dirty         bool
	Missing       bool
	Unmerged      bool
	UnmergedErr   string
	Unpushed      bool
	UnpushedErr   string
	StatusErr     string
	FetchBaseErr  string
	FetchWriteErr string
}

type RepoSafetyReport struct {
	RepoName string
	Branches []RepoBranchSafety
}

type RepoSafetyInput struct {
	WorkspaceRoot string
	Repo          config.RepoConfig
	Defaults      config.Defaults
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

	branches, err := listBranches(input.WorkspaceRoot)
	if err != nil {
		return RepoSafetyReport{}, err
	}

	report := RepoSafetyReport{RepoName: repo.Name}
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

	baseRemote := repo.Remotes.Base.Name
	baseBranch := repo.Remotes.Base.DefaultBranch
	writeRemote := repo.Remotes.Write.Name

	if input.FetchRemotes {
		if baseRemote != "" {
			if err := input.Git.Fetch(ctx, repoPathForRefs, baseRemote); err != nil {
				for i := range report.Branches {
					report.Branches[i].FetchBaseErr = err.Error()
				}
			}
		}
		if writeRemote != "" && writeRemote != baseRemote {
			if err := input.Git.Fetch(ctx, repoPathForRefs, writeRemote); err != nil {
				for i := range report.Branches {
					report.Branches[i].FetchWriteErr = err.Error()
				}
			}
		}
	}

	for i := range report.Branches {
		entry := &report.Branches[i]
		if entry.Missing {
			continue
		}

		branchRef := fmt.Sprintf("refs/heads/%s", entry.Branch)

		if baseRemote != "" && baseBranch != "" {
			baseRef := fmt.Sprintf("refs/remotes/%s/%s", baseRemote, baseBranch)
			merged, err := input.Git.IsAncestor(repoPathForRefs, branchRef, baseRef)
			if err != nil {
				entry.UnmergedErr = err.Error()
			} else if !merged {
				entry.Unmerged = true
			}
		}

		if writeRemote != "" {
			writeRef := fmt.Sprintf("refs/remotes/%s/%s", writeRemote, entry.Branch)
			pushed, err := input.Git.IsAncestor(repoPathForRefs, writeRef, branchRef)
			if err != nil {
				entry.UnpushedErr = err.Error()
			} else if !pushed {
				entry.Unpushed = true
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

	if input.DeleteWorktrees {
		branches, err := listBranches(input.WorkspaceRoot)
		if err != nil {
			return config.WorkspaceConfig{}, err
		}
		repoGitDir := ""
		if repo.LocalPath != "" {
			resolved, err := resolveGitDirPath(repo.LocalPath)
			if err != nil {
				return config.WorkspaceConfig{}, err
			}
			repoGitDir = resolved
		}
		for _, branch := range branches {
			worktreePath := workspace.RepoWorktreePath(input.WorkspaceRoot, branch, repo.RepoDir)
			if _, err := os.Stat(worktreePath); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return config.WorkspaceConfig{}, err
			}
			if repoGitDir != "" {
				worktreeName := workspace.WorktreeName(branch)
				if err := input.Git.WorktreeRemove(repoGitDir, worktreeName); err != nil {
					if !errors.Is(err, xworktree.ErrWorktreeNotFound) {
						return config.WorkspaceConfig{}, err
					}
				}
			}
			if err := os.RemoveAll(worktreePath); err != nil {
				return config.WorkspaceConfig{}, err
			}
			if err := removeIfEmpty(filepath.Dir(worktreePath)); err != nil {
				return config.WorkspaceConfig{}, err
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

type WorkspaceSafetyInput struct {
	WorkspaceRoot string
	Defaults      config.Defaults
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
		repoReport, err := CheckRepoSafety(ctx, RepoSafetyInput{
			WorkspaceRoot: input.WorkspaceRoot,
			Repo:          repo,
			Defaults:      input.Defaults,
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

func listBranches(root string) ([]string, error) {
	entries, err := os.ReadDir(workspace.WorktreesPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
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
