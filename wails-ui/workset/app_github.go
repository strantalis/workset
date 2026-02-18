package main

import (
	"context"
	"errors"
	"fmt"
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
	TerminalID  string `json:"terminalId,omitempty"`
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
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.PullRequestCreatedJSON{}, err
	}
	result, err := a.service.CreatePullRequest(ctx, worksetapi.PullRequestCreateInput{
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
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()

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
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return PullRequestStatusPayload{}, err
	}
	result, err := a.service.GetPullRequestStatus(ctx, worksetapi.PullRequestStatusInput{
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
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.GetCheckAnnotations(ctx, worksetapi.GetCheckAnnotationsInput{
		Owner:      input.Owner,
		Repo:       input.Repo,
		CheckRunID: input.CheckRunID,
	})
}

func (a *App) GetTrackedPullRequest(input PullRequestTrackedRequest) (worksetapi.PullRequestTrackedJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.PullRequestTrackedJSON{}, err
	}
	result, err := a.service.GetTrackedPullRequest(ctx, worksetapi.PullRequestTrackedInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return worksetapi.PullRequestTrackedJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) GetPullRequestReviews(input PullRequestReviewsRequest) (PullRequestReviewCommentsPayload, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return PullRequestReviewCommentsPayload{}, err
	}
	result, err := a.service.ListPullRequestReviewComments(ctx, worksetapi.PullRequestReviewsInput{
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
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.PullRequestGeneratedJSON{}, err
	}
	result, err := a.service.GeneratePullRequestText(ctx, worksetapi.PullRequestGenerateInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return worksetapi.PullRequestGeneratedJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) SendPullRequestReviewsToTerminal(input PullRequestReviewsRequest) error {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return err
	}
	result, err := a.service.ListPullRequestReviewComments(ctx, worksetapi.PullRequestReviewsInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		Number:    input.Number,
		Branch:    input.Branch,
	})
	if err != nil {
		return err
	}
	summary := formatReviewSummary(result.Comments)
	if summary == "" {
		return fmt.Errorf("no review comments to send")
	}
	terminalID := strings.TrimSpace(input.TerminalID)
	if terminalID == "" {
		if latest := a.latestTerminalForWorkspace(input.WorkspaceID); latest != nil {
			terminalID = latest.terminalID
		}
	}
	if terminalID == "" {
		created, err := a.CreateWorkspaceTerminal(input.WorkspaceID)
		if err != nil {
			return err
		}
		terminalID = created.TerminalID
	}
	windowName := a.workspaceTerminalOwner(input.WorkspaceID)
	if err := a.StartWorkspaceTerminalForWindowName(ctx, input.WorkspaceID, terminalID, windowName); err != nil {
		return err
	}
	return a.WriteWorkspaceTerminalForWindowName(ctx, input.WorkspaceID, terminalID, summary, windowName)
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
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.CommitAndPushResultJSON{}, err
	}
	result, err := a.service.CommitAndPush(ctx, worksetapi.CommitAndPushInput{
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
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()

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
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.RepoLocalStatusJSON{}, err
	}
	result, err := a.service.GetRepoLocalStatus(ctx, worksetapi.RepoLocalStatusInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return worksetapi.RepoLocalStatusJSON{}, err
	}
	return result.Payload, nil
}

func (a *App) ListRemotes(input ListRemotesRequest) ([]worksetapi.RemoteInfoJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return nil, err
	}
	result, err := a.service.ListRemotes(ctx, worksetapi.ListRemotesInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return nil, err
	}
	return result.Remotes, nil
}

func (a *App) GetCurrentGitHubUser(input GitHubUserRequest) (worksetapi.GitHubUserJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.GitHubUserJSON{}, err
	}
	result, err := a.service.GetCurrentGitHubUser(ctx, worksetapi.GitHubUserInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
	})
	if err != nil {
		return worksetapi.GitHubUserJSON{}, err
	}
	return result.User, nil
}

func (a *App) GetGitHubAuthStatus() (worksetapi.GitHubAuthStatusJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.GetGitHubAuthStatus(ctx)
}

func (a *App) GetGitHubAuthInfo() (worksetapi.GitHubAuthInfoJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.GetGitHubAuthInfo(ctx)
}

func (a *App) SetGitHubToken(input GitHubTokenRequest) (worksetapi.GitHubAuthStatusJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.SetGitHubToken(ctx, worksetapi.GitHubTokenInput{
		Token:  input.Token,
		Source: input.Source,
	})
}

func (a *App) SetGitHubAuthMode(input GitHubAuthModeRequest) (worksetapi.GitHubAuthInfoJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.SetGitHubAuthMode(ctx, input.Mode)
}

func (a *App) SetGitHubCLIPath(input GitHubCLIPathRequest) (worksetapi.GitHubAuthInfoJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.SetGitHubCLIPath(ctx, input.Path)
}

func (a *App) DisconnectGitHub() error {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	return a.service.ClearGitHubAuth(ctx)
}

func (a *App) ReplyToReviewComment(input ReplyToReviewCommentRequest) (worksetapi.PullRequestReviewCommentJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.PullRequestReviewCommentJSON{}, err
	}
	result, err := a.service.ReplyToReviewComment(ctx, worksetapi.ReplyToReviewCommentInput{
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
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return worksetapi.PullRequestReviewCommentJSON{}, err
	}
	result, err := a.service.EditReviewComment(ctx, worksetapi.EditReviewCommentInput{
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
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return err
	}
	_, err = a.service.DeleteReviewComment(ctx, worksetapi.DeleteReviewCommentInput{
		Workspace: worksetapi.WorkspaceSelector{Value: input.WorkspaceID},
		Repo:      repoName,
		CommentID: input.CommentID,
	})
	return err
}

func (a *App) ResolveReviewThread(input ResolveReviewThreadRequest) (bool, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	repoName, err := resolveRepoAlias(input.WorkspaceID, input.RepoID)
	if err != nil {
		return false, err
	}
	result, err := a.service.ResolveReviewThread(ctx, worksetapi.ResolveReviewThreadInput{
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

func formatReviewSummary(comments []worksetapi.PullRequestReviewCommentJSON) string {
	if len(comments) == 0 {
		return ""
	}
	builder := strings.Builder{}
	builder.WriteString("Pull request review feedback:\n")
	for _, comment := range comments {
		location := comment.Path
		if comment.Line > 0 {
			location = fmt.Sprintf("%s:%d", location, comment.Line)
		}
		body := strings.TrimSpace(comment.Body)
		if body == "" {
			continue
		}
		builder.WriteString(fmt.Sprintf("- %s (%s): %s\n", location, comment.Author, body))
	}
	builder.WriteString("\n")
	return builder.String()
}
