package worksetapi

import (
	"fmt"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/ops"
)

// WorkspaceSelector identifies a workspace by name or path.
// Require indicates whether missing selection should be treated as an error.
type WorkspaceSelector struct {
	Value   string
	Require bool
}

// WorkspaceRefJSON is the JSON-friendly representation of a registered workspace.
type WorkspaceRefJSON struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	CreatedAt      string `json:"created_at,omitempty"`
	LastUsed       string `json:"last_used,omitempty"`
	ArchivedAt     string `json:"archived_at,omitempty"`
	ArchivedReason string `json:"archived_reason,omitempty"`
	Archived       bool   `json:"archived"`
}

// WorkspaceListResult returns registered workspaces with config load metadata.
type WorkspaceListResult struct {
	Workspaces []WorkspaceRefJSON
	Config     config.GlobalConfigLoadInfo
}

// WorkspaceListOptions controls workspace listing behavior.
type WorkspaceListOptions struct {
	IncludeArchived bool
}

// WorkspaceCreatedJSON describes the JSON payload for a created workspace.
type WorkspaceCreatedJSON struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Workset string `json:"workset"`
	Branch  string `json:"branch"`
	Next    string `json:"next"`
}

// WorkspaceCreateResult wraps the create payload with warnings and config metadata.
type WorkspaceCreateResult struct {
	Workspace    WorkspaceCreatedJSON
	Warnings     []string
	PendingHooks []HookPending
	Config       config.GlobalConfigLoadInfo
}

// WorkspaceDeleteResultJSON is the JSON payload for workspace deletion.
type WorkspaceDeleteResultJSON struct {
	Status       string `json:"status"`
	Name         string `json:"name,omitempty"`
	Path         string `json:"path"`
	DeletedFiles bool   `json:"deleted_files"`
}

// WorkspaceDeleteResult includes safety details and config metadata.
type WorkspaceDeleteResult struct {
	Payload  WorkspaceDeleteResultJSON
	Warnings []string
	Unpushed []string
	Safety   ops.WorkspaceSafetyReport
	Config   config.GlobalConfigLoadInfo
}

// RepoJSON is the JSON-friendly view of a workspace repo entry.
type RepoJSON struct {
	Name      string `json:"name"`
	LocalPath string `json:"local_path"`
	Managed   bool   `json:"managed"`
	RepoDir   string `json:"repo_dir"`
	Base      string `json:"base"`
	Write     string `json:"write"`
}

// RepoListResult returns repos for a workspace with config metadata.
type RepoListResult struct {
	Repos  []RepoJSON
	Config config.GlobalConfigLoadInfo
}

// RepoAddResultJSON is the JSON payload for repo add operations.
type RepoAddResultJSON struct {
	Status       string            `json:"status"`
	Workspace    string            `json:"workspace"`
	Repo         string            `json:"repo"`
	LocalPath    string            `json:"local_path"`
	Managed      bool              `json:"managed"`
	PendingHooks []HookPendingJSON `json:"pending_hooks,omitempty"`
}

// RepoAddResult includes worktree details and config metadata.
type RepoAddResult struct {
	Payload      RepoAddResultJSON
	WorktreePath string
	RepoDir      string
	Warnings     []string
	PendingHooks []HookPending
	Config       config.GlobalConfigLoadInfo
}

// HookPendingJSON describes hooks waiting for approval/trust.
type HookPendingJSON struct {
	Event  string        `json:"event"`
	Repo   string        `json:"repo"`
	Hooks  []string      `json:"hooks"`
	Status HookRunStatus `json:"status,omitempty"`
	Reason string        `json:"reason,omitempty"`
}

// HookPending provides structured hook approval details.
type HookPending struct {
	Event  string
	Repo   string
	Hooks  []string
	Status HookRunStatus
	Reason string
}

// HooksRunResult describes hook execution results.
type HooksRunResult struct {
	Event   string
	Repo    string
	Results []HookRunJSON
	Config  config.GlobalConfigLoadInfo
}

// HookRunJSON reports individual hook execution results.
type HookRunJSON struct {
	ID      string        `json:"id"`
	Status  HookRunStatus `json:"status"`
	LogPath string        `json:"log_path,omitempty"`
}

type HookRunStatus string

const (
	HookRunStatusOK      HookRunStatus = "ok"
	HookRunStatusFailed  HookRunStatus = "failed"
	HookRunStatusSkipped HookRunStatus = "skipped"
)

// RepoRemotesUpdateResultJSON is the JSON payload for remote updates.
type RepoRemotesUpdateResultJSON struct {
	Status    string `json:"status"`
	Workspace string `json:"workspace"`
	Repo      string `json:"repo"`
	Base      string `json:"base"`
	Write     string `json:"write"`
}

// RepoRemoveDeletedJSON captures what was removed for a repo delete.
type RepoRemoveDeletedJSON struct {
	Worktrees bool `json:"worktrees"`
	Local     bool `json:"local"`
}

// RepoRemoveResultJSON is the JSON payload for repo removal operations.
type RepoRemoveResultJSON struct {
	Status    string                `json:"status"`
	Workspace string                `json:"workspace"`
	Repo      string                `json:"repo"`
	Deleted   RepoRemoveDeletedJSON `json:"deleted"`
}

// RepoRemoveResult includes safety details and config metadata.
type RepoRemoveResult struct {
	Payload  RepoRemoveResultJSON
	Warnings []string
	Unpushed []string
	Safety   ops.RepoSafetyReport
	Config   config.GlobalConfigLoadInfo
}

// RepoStatusJSON is the JSON payload for repo status reporting.
type RepoStatusJSON struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	State   string `json:"state"`
	Dirty   bool   `json:"dirty,omitempty"`
	Missing bool   `json:"missing,omitempty"`
	Error   string `json:"error,omitempty"`
}

// WorkspaceStatusResult returns per-repo status with config metadata.
type WorkspaceStatusResult struct {
	Statuses []RepoStatusJSON
	Config   config.GlobalConfigLoadInfo
}

// AliasJSON is the JSON-friendly view of an alias entry.
type AliasJSON struct {
	Name          string `json:"name"`
	URL           string `json:"url,omitempty"`
	Path          string `json:"path,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

// AliasListResult returns aliases with config metadata.
type AliasListResult struct {
	Aliases []AliasJSON
	Config  config.GlobalConfigLoadInfo
}

// AliasMutationResultJSON is the JSON payload for alias create/update/delete.
type AliasMutationResultJSON struct {
	Status string `json:"status"`
	Name   string `json:"name"`
}

// GroupSummaryJSON is the JSON summary view of a group.
type GroupSummaryJSON struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	RepoCount   int    `json:"repo_count"`
}

// GroupListResult returns group summaries with config metadata.
type GroupListResult struct {
	Groups []GroupSummaryJSON
	Config config.GlobalConfigLoadInfo
}

// GroupJSON is the JSON-friendly view of a group with members.
type GroupJSON struct {
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Members     []config.GroupMember `json:"members"`
}

// GroupApplyResultJSON is the JSON payload for group apply operations.
type GroupApplyResultJSON struct {
	Status    string `json:"status"`
	Template  string `json:"template"`
	Workspace string `json:"workspace"`
}

// SessionRecordJSON describes a session entry for list/show calls.
type SessionRecordJSON struct {
	Name         string   `json:"name"`
	Backend      string   `json:"backend"`
	Command      []string `json:"command,omitempty"`
	StartedAt    string   `json:"started_at,omitempty"`
	LastAttached string   `json:"last_attached,omitempty"`
	Running      bool     `json:"running"`
}

// SessionListResult returns sessions with config metadata.
type SessionListResult struct {
	Sessions []SessionRecordJSON
	Config   config.GlobalConfigLoadInfo
}

// SessionNotice provides user-facing guidance for session operations.
type SessionNotice struct {
	Title         string
	Workspace     string
	Session       string
	Backend       string
	ThemeLabel    string
	ThemeHint     string
	AttachCommand string
	AttachNote    string
	DetachHint    string
	NameNotice    string
}

// SessionStartResult captures notices and attachment state for StartSession.
type SessionStartResult struct {
	Notice   SessionNotice
	Attached bool
	Config   config.GlobalConfigLoadInfo
}

// SessionActionResult captures notices for attach/stop actions.
type SessionActionResult struct {
	Notice SessionNotice
	Config config.GlobalConfigLoadInfo
}

// ConfigSetResultJSON is the JSON payload for config set operations.
type ConfigSetResultJSON struct {
	Status string `json:"status"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

// ValidationError indicates invalid input or state.
type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }

// NotFoundError indicates a missing resource.
type NotFoundError struct{ Message string }

func (e NotFoundError) Error() string { return e.Message }

// ConflictError indicates a conflicting resource already exists.
type ConflictError struct{ Message string }

func (e ConflictError) Error() string { return e.Message }

// ConfirmationRequired indicates an operation needs explicit confirmation.
type ConfirmationRequired struct{ Message string }

func (e ConfirmationRequired) Error() string { return e.Message }

// UnsafeOperation describes safety concerns for potentially destructive actions.
type UnsafeOperation struct {
	Message  string
	Dirty    []string
	Unmerged []string
	Unpushed []string
	Warnings []string
}

func (e UnsafeOperation) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("unsafe operation: dirty=%d unmerged=%d unpushed=%d", len(e.Dirty), len(e.Unmerged), len(e.Unpushed))
}
