package git

import "context"

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

type Client interface {
	Clone(ctx context.Context, url, path, remoteName string) error
	CloneBare(ctx context.Context, url, path, remoteName string) error
	AddRemote(path, name, url string) error
	Status(path string) (StatusSummary, error)
	IsRepo(path string) (bool, error)
	WorktreeAdd(ctx context.Context, opts WorktreeAddOptions) error
	WorktreeRemove(repoPath, worktreeName string) error
	WorktreeList(repoPath string) ([]string, error)
}
