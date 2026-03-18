package worksetapi

// WorkspaceCreateInput describes inputs for CreateWorkspace.
type WorkspaceCreateInput struct {
	Name        string
	Path        string
	Workset     string
	WorksetOnly bool
	Repos       []string
}

// WorkspaceDeleteInput describes inputs for DeleteWorkspace.
type WorkspaceDeleteInput struct {
	Selector     WorkspaceSelector
	DeleteFiles  bool
	Force        bool
	Confirmed    bool
	FetchRemotes bool
}

// WorkspaceRenameInput describes inputs for RenameWorkspace.
type WorkspaceRenameInput struct {
	Selector WorkspaceSelector
	NewName  string
}

// RepoAddInput describes inputs for AddRepo.
type RepoAddInput struct {
	Workspace  WorkspaceSelector
	Source     string
	Name       string
	NameSet    bool
	RepoDir    string
	URL        string
	SourcePath string
}

// WorksetRepoAddInput describes inputs for adding repos directly to a workset.
type WorksetRepoAddInput struct {
	Workset string
	Sources []string
}

// RepoRemoveInput describes inputs for RemoveRepo.
type RepoRemoveInput struct {
	Workspace       WorkspaceSelector
	Name            string
	DeleteWorktrees bool
	DeleteLocal     bool
	Force           bool
	Confirmed       bool
	FetchRemotes    bool
}

// RepoRegistryInput describes inputs for RegisterRepo and UpdateRegisteredRepo.
type RepoRegistryInput struct {
	Name             string
	Source           string
	DefaultBranch    string
	Remote           string
	SourceSet        bool
	DefaultBranchSet bool
	RemoteSet        bool
}

// ExecInput describes inputs for Exec.
type ExecInput struct {
	Workspace WorkspaceSelector
	Command   []string
}

// HooksRunInput describes inputs for running hooks.
type HooksRunInput struct {
	Workspace WorkspaceSelector
	Repo      string
	Event     string
	Reason    string
	TrustRepo bool
}

// RepoHooksPreviewInput describes inputs for repo hook discovery without cloning.
type RepoHooksPreviewInput struct {
	Source string
	Ref    string
}
