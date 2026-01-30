package git

import (
	"context"
	"errors"
)

// ErrWorktreeNotFound indicates a worktree entry was missing.
var ErrWorktreeNotFound = errors.New("worktree not found")

type StatusSummary struct {
	Dirty   bool
	Missing bool
}

type WorktreeAddOptions struct {
	RepoPath      string
	WorktreePath  string
	WorktreeName  string
	BranchName    string
	StartRemote   string
	StartBranch   string
	ForceCheckout bool
}

type WorktreeRemoveOptions struct {
	RepoPath     string
	WorktreeName string
	Force        bool
}

type Client interface {
	Clone(ctx context.Context, url, path, remoteName string) error
	CloneBare(ctx context.Context, url, path, remoteName string) error
	AddRemote(path, name, url string) error
	RemoteNames(repoPath string) ([]string, error)
	RemoteURLs(repoPath, remoteName string) ([]string, error)
	ReferenceExists(repoPath, ref string) (bool, error)
	Fetch(ctx context.Context, repoPath, remoteName string) error
	Status(path string) (StatusSummary, error)
	IsRepo(path string) (bool, error)
	IsAncestor(repoPath, ancestorRef, descendantRef string) (bool, error)
	IsContentMerged(repoPath, branchRef, baseRef string) (bool, error)
	CurrentBranch(repoPath string) (string, bool, error)
	RemoteExists(repoPath, remoteName string) (bool, error)
	WorktreeAdd(ctx context.Context, opts WorktreeAddOptions) error
	WorktreeRemove(opts WorktreeRemoveOptions) error
	WorktreeList(repoPath string) ([]string, error)
}
