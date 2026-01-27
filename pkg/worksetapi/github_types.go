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
	NodeID         string `json:"node_id,omitempty"`
	ThreadID       string `json:"thread_id,omitempty"`
	ReviewID       int64  `json:"review_id,omitempty"`
	Author         string `json:"author,omitempty"`
	AuthorID       int64  `json:"author_id,omitempty"`
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

// ReplyToReviewCommentInput describes inputs for replying to a review comment.
type ReplyToReviewCommentInput struct {
	Workspace WorkspaceSelector
	Repo      string
	Number    int    // PR number (0 = auto-detect)
	Branch    string // Branch to resolve PR if Number is 0
	CommentID int64
	Body      string
}

// ReviewCommentResult wraps a single review comment with config metadata.
type ReviewCommentResult struct {
	Comment PullRequestReviewCommentJSON
	Config  config.GlobalConfigLoadInfo
}

// EditReviewCommentInput describes inputs for editing a review comment.
type EditReviewCommentInput struct {
	Workspace WorkspaceSelector
	Repo      string
	CommentID int64
	Body      string
}

// DeleteReviewCommentInput describes inputs for deleting a review comment.
type DeleteReviewCommentInput struct {
	Workspace WorkspaceSelector
	Repo      string
	CommentID int64
}

// DeleteReviewCommentResult wraps the result of a delete operation.
type DeleteReviewCommentResult struct {
	Success bool
	Config  config.GlobalConfigLoadInfo
}

// ResolveReviewThreadInput describes inputs for resolving/unresolving a review thread.
type ResolveReviewThreadInput struct {
	Workspace WorkspaceSelector
	Repo      string
	ThreadID  string // GraphQL node ID
	Resolve   bool   // true = resolve, false = unresolve
}

// ResolveReviewThreadResult wraps the result of a resolve/unresolve operation.
type ResolveReviewThreadResult struct {
	Resolved bool
	Config   config.GlobalConfigLoadInfo
}

// GitHubUserInput describes inputs for getting the current GitHub user.
type GitHubUserInput struct {
	Workspace WorkspaceSelector
	Repo      string
}

// GitHubUserJSON describes a GitHub user.
type GitHubUserJSON struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// GitHubUserResult wraps the current user with config metadata.
type GitHubUserResult struct {
	User   GitHubUserJSON
	Config config.GlobalConfigLoadInfo
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
