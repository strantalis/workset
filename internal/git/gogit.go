package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	return wrapAuthError(err)
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
	return wrapAuthError(err)
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

func (c GoGitClient) RemoteNames(repoPath string) ([]string, error) {
	if repoPath == "" {
		return nil, errors.New("repo path required")
	}
	repo, err := ggit.PlainOpenWithOptions(repoPath, &ggit.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return nil, err
	}
	remotes, err := repo.Remotes()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(remotes))
	for _, remote := range remotes {
		if remote == nil {
			continue
		}
		cfg := remote.Config()
		if cfg == nil || cfg.Name == "" {
			continue
		}
		names = append(names, cfg.Name)
	}
	sort.Strings(names)
	return names, nil
}

func (c GoGitClient) RemoteURLs(repoPath, remoteName string) ([]string, error) {
	if repoPath == "" {
		return nil, errors.New("repo path required")
	}
	if remoteName == "" {
		return nil, errors.New("remote name required")
	}
	repo, err := ggit.PlainOpenWithOptions(repoPath, &ggit.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return nil, err
	}
	remote, err := repo.Remote(remoteName)
	if err != nil {
		return nil, err
	}
	cfg := remote.Config()
	if cfg == nil || len(cfg.URLs) == 0 {
		return nil, errors.New("remote has no URLs configured")
	}
	urls := append([]string{}, cfg.URLs...)
	sort.Strings(urls)
	return urls, nil
}

func (c GoGitClient) ReferenceExists(repoPath, ref string) (bool, error) {
	if repoPath == "" || ref == "" {
		return false, errors.New("repo path and ref required")
	}
	repo, err := ggit.PlainOpenWithOptions(repoPath, &ggit.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return false, err
	}
	if _, err := repo.Reference(plumbing.ReferenceName(ref), true); err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c GoGitClient) Fetch(ctx context.Context, repoPath, remoteName string) error {
	if remoteName == "" {
		return errors.New("remote name required")
	}
	repo, err := ggit.PlainOpenWithOptions(repoPath, &ggit.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return err
	}
	if err := repo.FetchContext(ctx, &ggit.FetchOptions{RemoteName: remoteName}); err != nil && !errors.Is(err, ggit.NoErrAlreadyUpToDate) {
		return wrapAuthError(err)
	}
	return nil
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

func (c GoGitClient) IsAncestor(repoPath, ancestorRef, descendantRef string) (bool, error) {
	if ancestorRef == "" || descendantRef == "" {
		return false, errors.New("ancestor and descendant refs required")
	}
	repo, err := ggit.PlainOpenWithOptions(repoPath, &ggit.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return false, err
	}
	ancestorRefName := plumbing.ReferenceName(ancestorRef)
	descRefName := plumbing.ReferenceName(descendantRef)
	ancestor, err := repo.Reference(ancestorRefName, true)
	if err != nil {
		return false, err
	}
	descendant, err := repo.Reference(descRefName, true)
	if err != nil {
		return false, err
	}
	ancestorCommit, err := repo.CommitObject(ancestor.Hash())
	if err != nil {
		return false, err
	}
	descendantCommit, err := repo.CommitObject(descendant.Hash())
	if err != nil {
		return false, err
	}
	return ancestorCommit.IsAncestor(descendantCommit)
}

func (c GoGitClient) CurrentBranch(repoPath string) (string, bool, error) {
	if repoPath == "" {
		return "", false, errors.New("repo path required")
	}
	repo, err := ggit.PlainOpenWithOptions(repoPath, &ggit.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return "", false, err
	}
	head, err := repo.Head()
	if err != nil {
		return "", false, err
	}
	name := head.Name()
	if !name.IsBranch() {
		return "", false, nil
	}
	return name.Short(), true, nil
}

func (c GoGitClient) RemoteExists(repoPath, remoteName string) (bool, error) {
	if repoPath == "" || remoteName == "" {
		return false, errors.New("repo path and remote name required")
	}
	repo, err := ggit.PlainOpenWithOptions(repoPath, &ggit.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return false, err
	}
	if _, err := repo.Remote(remoteName); err != nil {
		if errors.Is(err, ggit.ErrRemoteNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
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
			return fmt.Errorf("fetch %s: %w", opts.StartRemote, wrapAuthError(err))
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
	if err := manager.Remove(worktreeName); err != nil {
		if errors.Is(err, xworktree.ErrWorktreeNotFound) {
			if removed, removeErr := removeWorktreeMetadata(repoPath, worktreeName); removeErr == nil && removed {
				return nil
			}
		}
		return err
	}
	return nil
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

func removeWorktreeMetadata(repoPath, worktreeName string) (bool, error) {
	candidates := []string{
		filepath.Join(repoPath, "worktrees", worktreeName),
		filepath.Join(repoPath, ".git", "worktrees", worktreeName),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil {
			if !info.IsDir() {
				return false, fmt.Errorf("invalid worktree metadata at %s", candidate)
			}
			if err := os.RemoveAll(candidate); err != nil {
				return false, err
			}
			return true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
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

func wrapAuthError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "ssh: handshake failed") ||
		strings.Contains(message, "unable to authenticate") ||
		strings.Contains(message, "no supported methods remain") ||
		strings.Contains(message, "error creating ssh agent") ||
		strings.Contains(message, "authentication required") {
		hint := sshAuthHint()
		if hint == "" {
			hint = "ssh auth failed; check SSH_AUTH_SOCK/agent or unlock your SSH agent"
		}
		return fmt.Errorf("%w (%s)", err, hint)
	}
	return err
}

func sshAuthHint() string {
	sock := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
	if sock == "" {
		return "ssh auth failed; SSH_AUTH_SOCK is unset"
	}
	if !isSocket(sock) {
		return fmt.Sprintf("ssh auth failed; SSH_AUTH_SOCK is not a socket (%s)", sock)
	}
	return ""
}
