package git

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-billy/v6/osfs"
	ggit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/cache"
	"github.com/go-git/go-git/v6/storage/filesystem"
	xworktree "github.com/go-git/go-git/v6/x/plumbing/worktree"
)

type GoGitClient struct{}

func NewGoGitClient() GoGitClient {
	return GoGitClient{}
}

func (c GoGitClient) Clone(ctx context.Context, url, path, remoteName string) error {
	if remoteName == "" {
		remoteName = "origin"
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	_, err := ggit.PlainCloneContext(ctx, path, &ggit.CloneOptions{
		URL:        url,
		RemoteName: remoteName,
		Bare:       false,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c GoGitClient) CloneBare(ctx context.Context, url, path, remoteName string) error {
	if remoteName == "" {
		remoteName = "origin"
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	_, err := ggit.PlainCloneContext(ctx, path, &ggit.CloneOptions{
		URL:        url,
		RemoteName: remoteName,
		Bare:       true,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c GoGitClient) AddRemote(path, name, url string) error {
	if name == "" {
		return errors.New("remote name required")
	}
	repo, err := ggit.PlainOpen(path)
	if err != nil {
		return err
	}
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: name,
		URLs: []string{url},
	})
	if errors.Is(err, ggit.ErrRemoteExists) {
		return nil
	}
	return err
}

func (c GoGitClient) Status(path string) (StatusSummary, error) {
	repo, err := ggit.PlainOpenWithOptions(path, &ggit.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		if errors.Is(err, ggit.ErrRepositoryNotExists) {
			return StatusSummary{Missing: true}, nil
		}
		return StatusSummary{}, err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return StatusSummary{}, err
	}
	status, err := worktree.Status()
	if err != nil {
		return StatusSummary{}, err
	}
	return StatusSummary{Dirty: !status.IsClean()}, nil
}

func (c GoGitClient) IsRepo(path string) (bool, error) {
	_, err := ggit.PlainOpenWithOptions(path, &ggit.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ggit.ErrRepositoryNotExists) {
		return false, nil
	}
	return false, err
}

func (c GoGitClient) WorktreeAdd(ctx context.Context, opts WorktreeAddOptions) error {
	if opts.RepoPath == "" {
		return errors.New("repo path required")
	}
	if opts.WorktreePath == "" {
		return errors.New("worktree path required")
	}
	if opts.WorktreeName == "" {
		return errors.New("worktree name required")
	}
	if opts.BranchName == "" {
		return errors.New("branch name required")
	}

	if err := os.MkdirAll(opts.WorktreePath, 0o755); err != nil {
		return err
	}

	storer := filesystem.NewStorage(osfs.New(opts.RepoPath), cache.NewObjectLRUDefault())
	repo, err := ggit.Open(storer, nil)
	if err != nil {
		return err
	}

	if opts.StartRemote != "" {
		if err := repo.FetchContext(ctx, &ggit.FetchOptions{RemoteName: opts.StartRemote}); err != nil && !errors.Is(err, ggit.NoErrAlreadyUpToDate) {
			return fmt.Errorf("fetch %s: %w", opts.StartRemote, err)
		}
	}

	startHash, err := resolveStartHash(repo, opts.StartRemote, opts.StartBranch)
	if err != nil {
		return err
	}

	manager, err := xworktree.New(storer)
	if err != nil {
		return err
	}
	wtFS := osfs.New(opts.WorktreePath)
	if err := manager.Add(wtFS, opts.WorktreeName, xworktree.WithCommit(startHash), xworktree.WithDetachedHead()); err != nil {
		return err
	}

	wtRepo, err := manager.Open(wtFS)
	if err != nil {
		return err
	}
	work, err := wtRepo.Worktree()
	if err != nil {
		return err
	}

	branchRef := plumbing.NewBranchReferenceName(opts.BranchName)
	create := false
	if _, err := repo.Reference(branchRef, true); err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			create = true
		} else {
			return err
		}
	}

	checkout := &ggit.CheckoutOptions{
		Branch: branchRef,
		Force:  opts.ForceCheckout,
	}
	if create {
		checkout.Create = true
		checkout.Hash = startHash
	}

	return work.Checkout(checkout)
}

func (c GoGitClient) WorktreeRemove(repoPath, worktreeName string) error {
	if repoPath == "" {
		return errors.New("repo path required")
	}
	if worktreeName == "" {
		return errors.New("worktree name required")
	}
	storer := filesystem.NewStorage(osfs.New(repoPath), cache.NewObjectLRUDefault())
	manager, err := xworktree.New(storer)
	if err != nil {
		return err
	}
	return manager.Remove(worktreeName)
}

func (c GoGitClient) WorktreeList(repoPath string) ([]string, error) {
	if repoPath == "" {
		return nil, errors.New("repo path required")
	}
	storer := filesystem.NewStorage(osfs.New(repoPath), cache.NewObjectLRUDefault())
	manager, err := xworktree.New(storer)
	if err != nil {
		return nil, err
	}
	return manager.List()
}

func resolveStartHash(repo *ggit.Repository, remoteName, branchName string) (plumbing.Hash, error) {
	if remoteName != "" && branchName != "" {
		refName := plumbing.NewRemoteReferenceName(remoteName, branchName)
		ref, err := repo.Reference(refName, true)
		if err == nil {
			return ref.Hash(), nil
		}
		if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			return plumbing.ZeroHash, fmt.Errorf("resolve %s: %w", refName.String(), err)
		}
	}
	ref, err := repo.Head()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return ref.Hash(), nil
}
