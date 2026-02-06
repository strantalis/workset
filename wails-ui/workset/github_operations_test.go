package main

import (
	"errors"
	"testing"

	"github.com/strantalis/workset/pkg/worksetapi"
)

func TestGitHubOperationManagerRejectsConcurrentRepoOperations(t *testing.T) {
	manager := newGitHubOperationManager()

	if _, _, err := manager.start("ws-1", "repo-1", GitHubOperationTypeCreatePR); err != nil {
		t.Fatalf("start create_pr failed: %v", err)
	}

	_, _, err := manager.start("ws-1", "repo-1", GitHubOperationTypeCommitPush)
	if err == nil {
		t.Fatal("expected concurrent operation rejection")
	}
	var validationErr worksetapi.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %T", err)
	}
}

func TestGitHubOperationManagerCreatePullRequestLifecycle(t *testing.T) {
	manager := newGitHubOperationManager()

	key, status, err := manager.start("ws-1", "repo-1", GitHubOperationTypeCreatePR)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if status.Stage != GitHubOperationStageQueued || status.State != GitHubOperationStateRunning {
		t.Fatalf("unexpected initial status: %+v", status)
	}

	status, ok := manager.setStage(key, GitHubOperationStageGenerating)
	if !ok {
		t.Fatal("expected stage update to succeed")
	}
	if status.Stage != GitHubOperationStageGenerating || status.State != GitHubOperationStateRunning {
		t.Fatalf("unexpected generating status: %+v", status)
	}

	status, ok = manager.completeCreatePR(key, worksetapi.PullRequestCreatedJSON{
		Repo:       "repo-1",
		Number:     42,
		URL:        "https://github.com/org/repo/pull/42",
		Title:      "feat(repo): async PR creation",
		State:      "open",
		BaseRepo:   "org/repo",
		BaseBranch: "main",
		HeadRepo:   "org/repo",
		HeadBranch: "feature/async",
	})
	if !ok {
		t.Fatal("expected complete to succeed")
	}
	if status.State != GitHubOperationStateCompleted || status.Stage != GitHubOperationStageCompleted {
		t.Fatalf("unexpected completed status: %+v", status)
	}
	if status.PullRequest == nil || status.PullRequest.Number != 42 {
		t.Fatalf("unexpected pull request payload: %+v", status.PullRequest)
	}
}

func TestGitHubOperationManagerFailureLifecycle(t *testing.T) {
	manager := newGitHubOperationManager()
	key, _, err := manager.start("ws-1", "repo-1", GitHubOperationTypeCommitPush)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	status, ok := manager.fail(key, errors.New("push rejected"))
	if !ok {
		t.Fatal("expected fail update to succeed")
	}
	if status.State != GitHubOperationStateFailed || status.Stage != GitHubOperationStageFailed {
		t.Fatalf("unexpected failed status: %+v", status)
	}
	if status.Error != "push rejected" {
		t.Fatalf("unexpected error message: %q", status.Error)
	}
}

func TestGetGitHubOperationStatus(t *testing.T) {
	app := &App{
		githubOps: newGitHubOperationManager(),
	}
	key, started, err := app.githubOps.start("ws-1", "repo-1", GitHubOperationTypeCommitPush)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if _, ok := app.githubOps.setStage(key, GitHubOperationStageCommitting); !ok {
		t.Fatal("expected stage update")
	}

	status, err := app.GetGitHubOperationStatus(GitHubOperationStatusRequest{
		WorkspaceID: "ws-1",
		RepoID:      "repo-1",
		Type:        string(GitHubOperationTypeCommitPush),
	})
	if err != nil {
		t.Fatalf("GetGitHubOperationStatus failed: %v", err)
	}
	if status.OperationID != started.OperationID || status.Stage != GitHubOperationStageCommitting {
		t.Fatalf("unexpected status payload: %+v", status)
	}
}
