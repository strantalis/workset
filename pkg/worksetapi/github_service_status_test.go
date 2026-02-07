package worksetapi

import (
	"context"
	"testing"
)

func TestGetCheckAnnotationsReturnsClientPayload(t *testing.T) {
	want := []CheckAnnotationJSON{
		{
			Path:      "main.go",
			StartLine: 10,
			EndLine:   10,
			Level:     "failure",
			Message:   "unused value",
		},
	}
	client := &readHelpersGitHubClient{
		getCheckRunAnnotationsFunc: func(_ context.Context, owner, repo string, checkRunID int64) ([]CheckAnnotationJSON, error) {
			if owner != "acme" || repo != "widgets" || checkRunID != 99 {
				t.Fatalf("unexpected annotation lookup: owner=%s repo=%s check=%d", owner, repo, checkRunID)
			}
			return want, nil
		},
	}
	provider := &readHelpersGitHubProvider{client: client}
	svc := &Service{github: provider}

	result, err := svc.GetCheckAnnotations(context.Background(), GetCheckAnnotationsInput{
		Owner:      "acme",
		Repo:       "widgets",
		CheckRunID: 99,
	})
	if err != nil {
		t.Fatalf("GetCheckAnnotations: %v", err)
	}
	if len(result.Annotations) != 1 || result.Annotations[0].Message != "unused value" {
		t.Fatalf("unexpected annotations: %+v", result.Annotations)
	}
	if len(provider.clientHosts) != 1 || provider.clientHosts[0] != "" {
		t.Fatalf("unexpected client host calls: %+v", provider.clientHosts)
	}
}

func TestGetTrackedPullRequestReturnsNotFoundWhenStateMissing(t *testing.T) {
	env, root, _ := setupGitHubServiceRepo(t)

	result, err := env.svc.GetTrackedPullRequest(context.Background(), PullRequestTrackedInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
	})
	if err != nil {
		t.Fatalf("GetTrackedPullRequest: %v", err)
	}
	if result.Payload.Found {
		t.Fatalf("expected no tracked pull request: %+v", result.Payload)
	}
}

func TestGetTrackedPullRequestReturnsRecordedEntry(t *testing.T) {
	env, root, _ := setupGitHubServiceRepo(t)
	ctx := context.Background()
	resolution, err := env.svc.resolveRepo(ctx, RepoSelectionInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
	})
	if err != nil {
		t.Fatalf("resolveRepo: %v", err)
	}

	env.svc.recordPullRequest(ctx, resolution, PullRequestCreatedJSON{
		Repo:       "repo-a",
		Number:     41,
		URL:        "https://github.com/base-org/base-repo/pull/41",
		Title:      "Improve coverage",
		Body:       "Adds tranche 2 tests",
		Draft:      true,
		State:      "open",
		BaseRepo:   "base-org/base-repo",
		BaseBranch: "main",
		HeadRepo:   "head-org/head-repo",
		HeadBranch: "feature/tests",
	})

	result, err := env.svc.GetTrackedPullRequest(ctx, PullRequestTrackedInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
	})
	if err != nil {
		t.Fatalf("GetTrackedPullRequest: %v", err)
	}
	if !result.Payload.Found {
		t.Fatalf("expected tracked pull request")
	}
	if result.Payload.PullRequest.Number != 41 || result.Payload.PullRequest.Title != "Improve coverage" {
		t.Fatalf("unexpected tracked pull request: %+v", result.Payload.PullRequest)
	}
}

func TestListRemotesSkipsInvalidRemoteURLs(t *testing.T) {
	env, root, repoPath := setupGitHubServiceRepo(t)
	env.git.remotes[repoPath] = []string{"origin", "fork", "invalid"}
	env.git.remoteURLs[repoPath] = map[string][]string{
		"origin":  {"git@github.com:acme/repo.git"},
		"fork":    {"https://github.com/sean/repo"},
		"invalid": {"not-a-valid-remote-url"},
	}

	result, err := env.svc.ListRemotes(context.Background(), ListRemotesInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
	})
	if err != nil {
		t.Fatalf("ListRemotes: %v", err)
	}
	if len(result.Remotes) != 2 {
		t.Fatalf("expected 2 parsed remotes, got %d (%+v)", len(result.Remotes), result.Remotes)
	}
	if result.Remotes[0].Name != "origin" || result.Remotes[0].Owner != "acme" || result.Remotes[0].Repo != "repo" {
		t.Fatalf("unexpected origin remote: %+v", result.Remotes[0])
	}
	if result.Remotes[1].Name != "fork" || result.Remotes[1].Owner != "sean" || result.Remotes[1].Repo != "repo" {
		t.Fatalf("unexpected fork remote: %+v", result.Remotes[1])
	}
}
