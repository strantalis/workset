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

func (a *App) CreatePullRequest(input PullRequestCreateRequest) (worksetapi.PullRequestCreatedJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
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

func (a *App) GetPullRequestStatus(input PullRequestStatusRequest) (PullRequestStatusPayload, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
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

func (a *App) GetTrackedPullRequest(input PullRequestTrackedRequest) (worksetapi.PullRequestTrackedJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
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
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
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
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
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
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
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
	if err := a.StartWorkspaceTerminal(input.WorkspaceID); err != nil {
		return err
	}
	summary := formatReviewSummary(result.Comments)
	if summary == "" {
		return fmt.Errorf("no review comments to send")
	}
	return a.WriteWorkspaceTerminal(input.WorkspaceID, summary)
}

type CommitAndPushRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Message     string `json:"message,omitempty"`
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
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
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

func (a *App) GetRepoLocalStatus(input RepoLocalStatusRequest) (worksetapi.RepoLocalStatusJSON, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
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
	if a.service == nil {
		a.service = worksetapi.NewService(worksetapi.Options{})
	}
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
