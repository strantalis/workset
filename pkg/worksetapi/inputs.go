package worksetapi

// WorkspaceCreateInput describes inputs for CreateWorkspace.
type WorkspaceCreateInput struct {
	Name     string
	Path     string
	Template string
	Groups   []string
	Repos    []string
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

// GroupUpsertInput describes inputs for CreateGroup and UpdateGroup.
type GroupUpsertInput struct {
	Name        string
	Description string
}

// GroupMemberInput describes inputs for AddGroupMember and RemoveGroupMember.
type GroupMemberInput struct {
	GroupName string
	RepoName  string
}

// GroupApplyInput describes inputs for ApplyGroup.
type GroupApplyInput struct {
	Workspace WorkspaceSelector
	Name      string
}

// SessionStartInput describes inputs for StartSession.
type SessionStartInput struct {
	Workspace   WorkspaceSelector
	Backend     string
	Attach      bool
	Interactive bool
	Name        string
	Command     []string
	Confirmed   bool
}

// SessionAttachInput describes inputs for AttachSession.
type SessionAttachInput struct {
	Workspace WorkspaceSelector
	Backend   string
	Name      string
	Confirmed bool
}

// SessionStopInput describes inputs for StopSession.
type SessionStopInput struct {
	Workspace WorkspaceSelector
	Backend   string
	Name      string
	Confirmed bool
}

// SessionShowInput describes inputs for ShowSession.
type SessionShowInput struct {
	Workspace WorkspaceSelector
	Backend   string
	Name      string
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
