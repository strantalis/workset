package worksetapi

import "github.com/strantalis/workset/internal/config"

// RepoSelectionInput selects a repo within a workspace.
type RepoSelectionInput struct {
	Workspace WorkspaceSelector
	Repo      string
}

// PullRequestCreateInput describes inputs for CreatePullRequest.
type PullRequestCreateInput struct {
	Workspace  WorkspaceSelector
	Repo       string
	Base       string
	Head       string
	BaseRemote string // Optional: override auto-detected base remote
	Title      string
	Body       string
	Draft      bool
	AutoCommit bool
	AutoPush   bool
}

// ListRemotesInput describes inputs for listing repo remotes.
type ListRemotesInput struct {
	Workspace WorkspaceSelector
	Repo      string
}

// RemoteInfoJSON describes a git remote.
type RemoteInfoJSON struct {
	Name  string `json:"name"`
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

// ListRemotesResult wraps the remotes list.
type ListRemotesResult struct {
	Remotes []RemoteInfoJSON
}

// PullRequestCreatedJSON describes a created pull request.
type PullRequestCreatedJSON struct {
	Repo       string `json:"repo"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	Body       string `json:"body,omitempty"`
	Draft      bool   `json:"draft"`
	State      string `json:"state"`
	BaseRepo   string `json:"base_repo"`
	BaseBranch string `json:"base_branch"`
	HeadRepo   string `json:"head_repo"`
	HeadBranch string `json:"head_branch"`
}

// PullRequestCreateResult wraps PR creation payload with config metadata.
type PullRequestCreateResult struct {
	Payload PullRequestCreatedJSON
	Config  config.GlobalConfigLoadInfo
}

// PullRequestStatusInput describes inputs for PR status/checks.
type PullRequestStatusInput struct {
	Workspace WorkspaceSelector
	Repo      string
	Number    int
	Branch    string
}

// PullRequestStatusJSON summarizes a pull request.
type PullRequestStatusJSON struct {
	Repo       string `json:"repo"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	State      string `json:"state"`
	Draft      bool   `json:"draft"`
	BaseRepo   string `json:"base_repo"`
	BaseBranch string `json:"base_branch"`
	HeadRepo   string `json:"head_repo"`
	HeadBranch string `json:"head_branch"`
	Mergeable  string `json:"mergeable,omitempty"`
}

// PullRequestCheckJSON describes a single check run.
type PullRequestCheckJSON struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Conclusion  string `json:"conclusion,omitempty"`
	DetailsURL  string `json:"details_url,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// PullRequestStatusResult wraps status payload with check runs.
type PullRequestStatusResult struct {
	PullRequest PullRequestStatusJSON
	Checks      []PullRequestCheckJSON
	Config      config.GlobalConfigLoadInfo
}

// PullRequestTrackedInput describes inputs for a tracked PR lookup.
type PullRequestTrackedInput struct {
	Workspace WorkspaceSelector
	Repo      string
}

// PullRequestTrackedJSON describes a locally tracked PR reference.
type PullRequestTrackedJSON struct {
	Found       bool                   `json:"found"`
	PullRequest PullRequestCreatedJSON `json:"pull_request,omitempty"`
}

// PullRequestTrackedResult wraps a tracked PR lookup.
type PullRequestTrackedResult struct {
	Payload PullRequestTrackedJSON
	Config  config.GlobalConfigLoadInfo
}

// PullRequestReviewsInput describes inputs for listing review comments.
type PullRequestReviewsInput struct {
	Workspace WorkspaceSelector
	Repo      string
	Number    int
	Branch    string
}

// PullRequestReviewCommentJSON describes a review comment.
type PullRequestReviewCommentJSON struct {
	ID             int64  `json:"id"`
	ReviewID       int64  `json:"review_id,omitempty"`
	Author         string `json:"author,omitempty"`
	Body           string `json:"body"`
	Path           string `json:"path"`
	Line           int    `json:"line,omitempty"`
	Side           string `json:"side,omitempty"`
	CommitID       string `json:"commit_id,omitempty"`
	OriginalCommit string `json:"original_commit_id,omitempty"`
	OriginalLine   int    `json:"original_line,omitempty"`
	OriginalStart  int    `json:"original_start_line,omitempty"`
	Outdated       bool   `json:"outdated"`
	URL            string `json:"url,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	InReplyTo      int64  `json:"in_reply_to,omitempty"`
	Resolved       bool   `json:"resolved,omitempty"`
	ReplyToComment bool   `json:"reply,omitempty"`
}

// PullRequestReviewCommentsResult wraps review comments with config metadata.
type PullRequestReviewCommentsResult struct {
	Comments []PullRequestReviewCommentJSON
	Config   config.GlobalConfigLoadInfo
}

// PullRequestGenerateInput describes inputs for AI PR generation.
type PullRequestGenerateInput struct {
	Workspace    WorkspaceSelector
	Repo         string
	Base         string
	Head         string
	MaxDiffBytes int
}

// PullRequestGeneratedJSON is the AI-generated title/body.
type PullRequestGeneratedJSON struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// PullRequestGenerateResult wraps generated text with config metadata.
type PullRequestGenerateResult struct {
	Payload PullRequestGeneratedJSON
	Config  config.GlobalConfigLoadInfo
}

// CommitAndPushInput describes inputs for committing and pushing changes.
type CommitAndPushInput struct {
	Workspace WorkspaceSelector
	Repo      string
	Message   string // Empty = auto-generate via agent
}

// CommitAndPushResultJSON describes the result of a commit and push operation.
type CommitAndPushResultJSON struct {
	Committed bool   `json:"committed"`
	Pushed    bool   `json:"pushed"`
	Message   string `json:"message"`
	SHA       string `json:"sha,omitempty"`
}

// CommitAndPushResult wraps the commit/push result with config metadata.
type CommitAndPushResult struct {
	Payload CommitAndPushResultJSON
	Config  config.GlobalConfigLoadInfo
}

// RepoLocalStatusInput describes inputs for checking local repo status.
type RepoLocalStatusInput struct {
	Workspace WorkspaceSelector
	Repo      string
}

// RepoLocalStatusJSON describes the local status of a repo.
type RepoLocalStatusJSON struct {
	HasUncommitted bool   `json:"hasUncommitted"`
	Ahead          int    `json:"ahead"`
	Behind         int    `json:"behind"`
	CurrentBranch  string `json:"currentBranch"`
}

// RepoLocalStatusResult wraps local status with config metadata.
type RepoLocalStatusResult struct {
	Payload RepoLocalStatusJSON
	Config  config.GlobalConfigLoadInfo
}
