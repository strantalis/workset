package main

import (
	"errors"
	"strings"

	"github.com/strantalis/workset/pkg/worksetapi"
)

type PullRequestCreateRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Base        string `json:"base,omitempty"`
	Head        string `json:"head,omitempty"`
	BaseRemote  string `json:"baseRemote,omitempty"`
	Draft       bool   `json:"draft"`
	AutoCommit  bool   `json:"autoCommit"`
	AutoPush    bool   `json:"autoPush"`
}

type StartCreatePullRequestAsyncRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Base        string `json:"base,omitempty"`
	Head        string `json:"head,omitempty"`
	BaseRemote  string `json:"baseRemote,omitempty"`
	Draft       bool   `json:"draft"`
}

type ListRemotesRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
}

type PullRequestStatusRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Number      int    `json:"number,omitempty"`
	Branch      string `json:"branch,omitempty"`
}

type PullRequestStatusPayload struct {
	PullRequest worksetapi.PullRequestStatusJSON  `json:"pullRequest"`
	Checks      []worksetapi.PullRequestCheckJSON `json:"checks"`
}

type GetCheckAnnotationsRequest struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	CheckRunID int64  `json:"checkRunId"`
}

type PullRequestTrackedRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
}

type PullRequestReviewsRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Number      int    `json:"number,omitempty"`
	Branch      string `json:"branch,omitempty"`
}

type PullRequestReviewCommentsPayload struct {
	Comments []worksetapi.PullRequestReviewCommentJSON `json:"comments"`
}

type PullRequestGenerateRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
}

type GitHubUserRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
}

type GitHubTokenRequest struct {
	Token  string `json:"token"`
	Source string `json:"source,omitempty"`
}

type GitHubAuthModeRequest struct {
	Mode string `json:"mode"`
}

type GitHubCLIPathRequest struct {
	Path string `json:"path"`
}

func (a *App) CreatePullRequest(input PullRequestCreateRequest) (worksetapi.PullRequestCreatedJSON, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.PullRequestCreatedJSON{}, err
	}
	result, err := svc.CreatePullRequest(ctx, worksetapi.PullRequestCreateInput{
		Workspace:  worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:       repoName,
		Base:       input.Base,
		Head:       input.Head,
		BaseRemote: input.BaseRemote,
		Title:      input.Title,
		Body:       input.Body,
		Draft:      input.Draft,
		AutoCommit: input.AutoCommit,
		AutoPush:   input.AutoPush,
	})
	if err != nil {
		return worksetapi.PullRequestCreatedJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) StartCreatePullRequestAsync(input StartCreatePullRequestAsyncRequest) (GitHubOperationStatusPayload, error) {
	ctx, _ := a.serviceContext()

	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return GitHubOperationStatusPayload{}, err
	}

	manager := a.ensureGitHubOperationManager()
	key, status, err := manager.start(input.WorkspaceID, input.RepoID, GitHubOperationTypeCreatePR)
	if err != nil {
		return GitHubOperationStatusPayload{}, err
	}
	a.emitGitHubOperation(status)

	go a.runCreatePullRequestAsync(ctx, key, repoName, input)
	return status, nil
}

func (a *App) GetPullRequestStatus(input PullRequestStatusRequest) (PullRequestStatusPayload, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return PullRequestStatusPayload{}, err
	}
	result, err := svc.GetPullRequestStatus(ctx, worksetapi.PullRequestStatusInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		Number:    input.Number,
		Branch:    input.Branch,
	})
	if err != nil {
		return PullRequestStatusPayload{}, err
	}
	return PullRequestStatusPayload{
		PullRequest: result.PullRequest,
		Checks:      result.Checks,
	}, nil
}

func (a *App) GetCheckAnnotations(input GetCheckAnnotationsRequest) (worksetapi.CheckAnnotationsResult, error) {
	ctx, svc := a.serviceContext()
	return svc.GetCheckAnnotations(ctx, worksetapi.GetCheckAnnotationsInput{
		Owner:      input.Owner,
		Repo:       input.Repo,
		CheckRunID: input.CheckRunID,
	})
}

func (a *App) GetTrackedPullRequest(input PullRequestTrackedRequest) (worksetapi.PullRequestTrackedJSON, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.PullRequestTrackedJSON{}, err
	}
	result, err := svc.GetTrackedPullRequest(ctx, worksetapi.PullRequestTrackedInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return worksetapi.PullRequestTrackedJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) GetPullRequestReviews(input PullRequestReviewsRequest) (PullRequestReviewCommentsPayload, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return PullRequestReviewCommentsPayload{}, err
	}
	result, err := svc.ListPullRequestReviewComments(ctx, worksetapi.PullRequestReviewsInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		Number:    input.Number,
		Branch:    input.Branch,
	})
	if err != nil {
		return PullRequestReviewCommentsPayload{}, err
	}
	return PullRequestReviewCommentsPayload{Comments: result.Comments}, nil
}

func (a *App) GeneratePullRequestText(input PullRequestGenerateRequest) (worksetapi.PullRequestGeneratedJSON, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.PullRequestGeneratedJSON{}, err
	}
	result, err := svc.GeneratePullRequestText(ctx, worksetapi.PullRequestGenerateInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return worksetapi.PullRequestGeneratedJSON{}, err
	}
	return result.Payload, nil
}

type CommitAndPushRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Message     string `json:"message,omitempty"`
}

type StartCommitAndPushAsyncRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Message     string `json:"message,omitempty"`
}

type LocalMergeRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Base        string `json:"base,omitempty"`
	Message     string `json:"message,omitempty"`
}

type StartLocalMergeAsyncRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Base        string `json:"base,omitempty"`
	Message     string `json:"message,omitempty"`
}

type PushBranchRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Branch      string `json:"branch"`
}

type GitHubOperationStatusRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Type        string `json:"type"`
}

type ReplyToReviewCommentRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Number      int    `json:"number,omitempty"`
	Branch      string `json:"branch,omitempty"`
	CommentID   int64  `json:"commentId"`
	Body        string `json:"body"`
}

type EditReviewCommentRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	CommentID   int64  `json:"commentId"`
	Body        string `json:"body"`
}

type DeleteReviewCommentRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	CommentID   int64  `json:"commentId"`
}

type ResolveReviewThreadRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	ThreadID    string `json:"threadId"`
	Resolve     bool   `json:"resolve"`
}

type RepoLocalStatusRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
}

func (a *App) CommitAndPush(input CommitAndPushRequest) (worksetapi.CommitAndPushResultJSON, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.CommitAndPushResultJSON{}, err
	}
	result, err := svc.CommitAndPush(ctx, worksetapi.CommitAndPushInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		Message:   input.Message,
	})
	if err != nil {
		return worksetapi.CommitAndPushResultJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) StartCommitAndPushAsync(input StartCommitAndPushAsyncRequest) (GitHubOperationStatusPayload, error) {
	ctx, _ := a.serviceContext()

	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return GitHubOperationStatusPayload{}, err
	}

	manager := a.ensureGitHubOperationManager()
	key, status, err := manager.start(input.WorkspaceID, input.RepoID, GitHubOperationTypeCommitPush)
	if err != nil {
		return GitHubOperationStatusPayload{}, err
	}
	a.emitGitHubOperation(status)

	go a.runCommitAndPushAsync(ctx, key, repoName, input)
	return status, nil
}

func (a *App) LocalMerge(input LocalMergeRequest) (worksetapi.LocalMergeResultJSON, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.LocalMergeResultJSON{}, err
	}
	result, err := svc.LocalMerge(ctx, worksetapi.LocalMergeInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		Base:      input.Base,
		Message:   input.Message,
	})
	if err != nil {
		return worksetapi.LocalMergeResultJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) StartLocalMergeAsync(input StartLocalMergeAsyncRequest) (GitHubOperationStatusPayload, error) {
	ctx, _ := a.serviceContext()

	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return GitHubOperationStatusPayload{}, err
	}

	manager := a.ensureGitHubOperationManager()
	key, status, err := manager.start(input.WorkspaceID, input.RepoID, GitHubOperationTypeLocalMerge)
	if err != nil {
		return GitHubOperationStatusPayload{}, err
	}
	a.emitGitHubOperation(status)

	go a.runLocalMergeAsync(ctx, key, repoName, input)
	return status, nil
}

func (a *App) GetGitHubOperationStatus(input GitHubOperationStatusRequest) (GitHubOperationStatusPayload, error) {
	opType, err := parseGitHubOperationType(input.Type)
	if err != nil {
		return GitHubOperationStatusPayload{}, err
	}
	key := githubOperationKey{
		workspaceID: input.WorkspaceID,
		repoID:      input.RepoID,
		opType:      opType,
	}
	status, ok := a.ensureGitHubOperationManager().get(key)
	if !ok {
		// No active operation is a valid state; return empty payload without error.
		return GitHubOperationStatusPayload{}, nil
	}
	return status, nil
}

func (a *App) GetRepoLocalStatus(input RepoLocalStatusRequest) (worksetapi.RepoLocalStatusJSON, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.RepoLocalStatusJSON{}, err
	}
	result, err := svc.GetRepoLocalStatus(ctx, worksetapi.RepoLocalStatusInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return worksetapi.RepoLocalStatusJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) PushBranch(input PushBranchRequest) (worksetapi.PushBranchResultJSON, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.PushBranchResultJSON{}, err
	}
	result, err := svc.PushBranch(ctx, worksetapi.PushBranchInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		Branch:    input.Branch,
	})
	if err != nil {
		return worksetapi.PushBranchResultJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) ListRemotes(input ListRemotesRequest) ([]worksetapi.RemoteInfoJSON, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return nil, err
	}
	result, err := svc.ListRemotes(ctx, worksetapi.ListRemotesInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return nil, err
	}
	return result.Remotes, nil
}

func (a *App) GetCurrentGitHubUser(input GitHubUserRequest) (worksetapi.GitHubUserJSON, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.GitHubUserJSON{}, err
	}
	result, err := svc.GetCurrentGitHubUser(ctx, worksetapi.GitHubUserInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return worksetapi.GitHubUserJSON{}, err
	}
	return result.User, nil
}

func (a *App) SearchGitHubRepositories(query string, limit int) ([]worksetapi.GitHubRepoSearchItemJSON, error) {
	ctx, svc := a.serviceContext()
	result, err := svc.SearchGitHubRepositories(ctx, worksetapi.GitHubRepoSearchInput{
		Query: query,
		Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	return result.Repositories, nil
}

func (a *App) GetGitHubAuthStatus() (worksetapi.GitHubAuthStatusJSON, error) {
	ctx, svc := a.serviceContext()
	return svc.GetGitHubAuthStatus(ctx)
}

func (a *App) GetGitHubAuthInfo() (worksetapi.GitHubAuthInfoJSON, error) {
	ctx, svc := a.serviceContext()
	return svc.GetGitHubAuthInfo(ctx)
}

func (a *App) SetGitHubToken(input GitHubTokenRequest) (worksetapi.GitHubAuthStatusJSON, error) {
	ctx, svc := a.serviceContext()
	return svc.SetGitHubToken(ctx, worksetapi.GitHubTokenInput{
		Token:  input.Token,
		Source: input.Source,
	})
}

func (a *App) SetGitHubAuthMode(input GitHubAuthModeRequest) (worksetapi.GitHubAuthInfoJSON, error) {
	ctx, svc := a.serviceContext()
	return svc.SetGitHubAuthMode(ctx, input.Mode)
}

func (a *App) SetGitHubCLIPath(input GitHubCLIPathRequest) (worksetapi.GitHubAuthInfoJSON, error) {
	ctx, svc := a.serviceContext()
	return svc.SetGitHubCLIPath(ctx, input.Path)
}

func (a *App) DisconnectGitHub() error {
	ctx, svc := a.serviceContext()
	return svc.ClearGitHubAuth(ctx)
}

func (a *App) ReplyToReviewComment(input ReplyToReviewCommentRequest) (worksetapi.PullRequestReviewCommentJSON, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.PullRequestReviewCommentJSON{}, err
	}
	result, err := svc.ReplyToReviewComment(ctx, worksetapi.ReplyToReviewCommentInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		Number:    input.Number,
		Branch:    input.Branch,
		CommentID: input.CommentID,
		Body:      input.Body,
	})
	if err != nil {
		return worksetapi.PullRequestReviewCommentJSON{}, err
	}
	return result.Comment, nil
}

func (a *App) EditReviewComment(input EditReviewCommentRequest) (worksetapi.PullRequestReviewCommentJSON, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.PullRequestReviewCommentJSON{}, err
	}
	result, err := svc.EditReviewComment(ctx, worksetapi.EditReviewCommentInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		CommentID: input.CommentID,
		Body:      input.Body,
	})
	if err != nil {
		return worksetapi.PullRequestReviewCommentJSON{}, err
	}
	return result.Comment, nil
}

func (a *App) DeleteReviewComment(input DeleteReviewCommentRequest) error {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return err
	}
	_, err = svc.DeleteReviewComment(ctx, worksetapi.DeleteReviewCommentInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		CommentID: input.CommentID,
	})
	return err
}

func (a *App) ResolveReviewThread(input ResolveReviewThreadRequest) (bool, error) {
	ctx, svc := a.serviceContext()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return false, err
	}
	result, err := svc.ResolveReviewThread(ctx, worksetapi.ResolveReviewThreadInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		ThreadID:  input.ThreadID,
		Resolve:   input.Resolve,
	})
	if err != nil {
		return false, err
	}
	return result.Resolved, nil
}

func resolveRepoAlias(workspaceID, repoID string) (string, error) {
	repoID = strings.TrimSpace(repoID)
	if repoID == "" {
		return "", errors.New("repo is required")
	}
	if strings.Contains(repoID, "::") {
		parts := strings.SplitN(repoID, "::", 2)
		if len(parts) == 2 {
			return parts[1], nil
		}
	}
	return repoID, nil
}
