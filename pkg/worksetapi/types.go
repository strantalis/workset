package worksetapi

import (
	"errors"
	"fmt"
	"strings"

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
	Pinned         bool   `json:"pinned"`
	PinOrder       int    `json:"pin_order"`
	Color          string `json:"color,omitempty"`
	Description    string `json:"description,omitempty"`
	Expanded       bool   `json:"expanded"`
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

// WorkspaceSnapshotOptions controls workspace snapshot behavior.
type WorkspaceSnapshotOptions struct {
	IncludeArchived bool `json:"include_archived"`
	IncludeStatus   bool `json:"include_status"`
}

// WorkspaceSnapshotResult returns workspace snapshots with config metadata.
type WorkspaceSnapshotResult struct {
	Workspaces []WorkspaceSnapshotJSON
	Config     config.GlobalConfigLoadInfo
}

// WorkspaceSnapshotJSON describes a workspace and its repos.
type WorkspaceSnapshotJSON struct {
	Name           string             `json:"name"`
	Path           string             `json:"path"`
	CreatedAt      string             `json:"created_at,omitempty"`
	LastUsed       string             `json:"last_used,omitempty"`
	ArchivedAt     string             `json:"archived_at,omitempty"`
	ArchivedReason string             `json:"archived_reason,omitempty"`
	Archived       bool               `json:"archived"`
	Pinned         bool               `json:"pinned"`
	PinOrder       int                `json:"pin_order"`
	Color          string             `json:"color,omitempty"`
	Description    string             `json:"description,omitempty"`
	Expanded       bool               `json:"expanded"`
	Repos          []RepoSnapshotJSON `json:"repos"`
}

// RepoSnapshotJSON summarizes a workspace repo and optional status.
type RepoSnapshotJSON struct {
	Name               string                          `json:"name"`
	LocalPath          string                          `json:"local_path"`
	Managed            bool                            `json:"managed"`
	RepoDir            string                          `json:"repo_dir"`
	Remote             string                          `json:"remote"`
	DefaultBranch      string                          `json:"default_branch"`
	Dirty              bool                            `json:"dirty"`
	Missing            bool                            `json:"missing"`
	StatusKnown        bool                            `json:"status_known"`
	TrackedPullRequest *TrackedPullRequestSnapshotJSON `json:"tracked_pull_request,omitempty"`
}

// TrackedPullRequestSnapshotJSON summarizes tracked PR metadata stored in workspace state.
type TrackedPullRequestSnapshotJSON struct {
	Repo       string `json:"repo"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	Body       string `json:"body,omitempty"`
	State      string `json:"state"`
	Draft      bool   `json:"draft"`
	BaseRepo   string `json:"base_repo"`
	BaseBranch string `json:"base_branch"`
	HeadRepo   string `json:"head_repo"`
	HeadBranch string `json:"head_branch"`
	UpdatedAt  string `json:"updated_at,omitempty"`
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
	HookRuns     []HookExecutionJSON
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
	Name          string `json:"name"`
	LocalPath     string `json:"local_path"`
	Managed       bool   `json:"managed"`
	RepoDir       string `json:"repo_dir"`
	Remote        string `json:"remote"`
	DefaultBranch string `json:"default_branch"`
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
	HookRuns     []HookExecutionJSON
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

// RepoHookPreviewJSON describes a single hook definition discovered from source.
type RepoHookPreviewJSON struct {
	ID      string   `json:"id"`
	On      []string `json:"on,omitempty"`
	Run     []string `json:"run,omitempty"`
	Cwd     string   `json:"cwd,omitempty"`
	OnError string   `json:"on_error,omitempty"`
}

// RepoHooksPreviewJSON is the JSON payload for pre-clone hook discovery.
type RepoHooksPreviewJSON struct {
	Source         string                `json:"source"`
	ResolvedSource string                `json:"resolved_source,omitempty"`
	Host           string                `json:"host,omitempty"`
	Owner          string                `json:"owner,omitempty"`
	Repo           string                `json:"repo,omitempty"`
	Ref            string                `json:"ref,omitempty"`
	Exists         bool                  `json:"exists"`
	Hooks          []RepoHookPreviewJSON `json:"hooks,omitempty"`
}

// RepoHooksPreviewResult wraps hook preview payload with config metadata.
type RepoHooksPreviewResult struct {
	Payload RepoHooksPreviewJSON
	Config  config.GlobalConfigLoadInfo
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

// HookExecutionJSON reports hook execution results with repo and event context.
type HookExecutionJSON struct {
	Event   string        `json:"event"`
	Repo    string        `json:"repo"`
	ID      string        `json:"id"`
	Status  HookRunStatus `json:"status"`
	LogPath string        `json:"log_path,omitempty"`
}

// HookProgress describes lifecycle updates emitted while hooks run.
type HookProgress struct {
	Phase     string        `json:"phase"`
	Event     string        `json:"event"`
	Repo      string        `json:"repo"`
	Workspace string        `json:"workspace,omitempty"`
	HookID    string        `json:"hook_id"`
	Reason    string        `json:"reason,omitempty"`
	Status    HookRunStatus `json:"status,omitempty"`
	LogPath   string        `json:"log_path,omitempty"`
	Error     string        `json:"error,omitempty"`
}

// HookProgressObserver receives live hook lifecycle updates.
type HookProgressObserver interface {
	OnHookProgress(progress HookProgress)
}

type HookRunStatus string

const (
	HookRunStatusOK      HookRunStatus = "ok"
	HookRunStatusFailed  HookRunStatus = "failed"
	HookRunStatusSkipped HookRunStatus = "skipped"
)

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

// RegisteredRepoJSON is the JSON-friendly view of a registered repo entry.
type RegisteredRepoJSON struct {
	Name          string `json:"name"`
	URL           string `json:"url,omitempty"`
	Path          string `json:"path,omitempty"`
	Remote        string `json:"remote,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

// RegisteredRepoListResult returns registered repos with config metadata.
type RegisteredRepoListResult struct {
	Repos  []RegisteredRepoJSON
	Config config.GlobalConfigLoadInfo
}

// RegisteredRepoMutationResultJSON is the JSON payload for repo registry create/update/delete.
type RegisteredRepoMutationResultJSON struct {
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

const authRequiredPrefix = "AUTH_REQUIRED: "

// AuthRequiredError indicates a missing or invalid authentication token.
type AuthRequiredError struct{ Message string }

func (e AuthRequiredError) Error() string {
	if e.Message == "" {
		return authRequiredPrefix + "authentication required"
	}
	return authRequiredPrefix + e.Message
}

// IsAuthRequiredError reports whether the error indicates missing authentication.
func IsAuthRequiredError(err error) bool {
	if err == nil {
		return false
	}
	var authErr AuthRequiredError
	if errors.As(err, &authErr) {
		return true
	}
	return strings.HasPrefix(err.Error(), authRequiredPrefix)
}

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
