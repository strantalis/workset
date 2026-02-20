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

func TestSearchGitHubRepositoriesValidatesQueryLength(t *testing.T) {
	svc := &Service{github: &readHelpersGitHubProvider{client: &readHelpersGitHubClient{}}}
	_, err := svc.SearchGitHubRepositories(context.Background(), GitHubRepoSearchInput{
		Query: "a",
		Limit: 5,
	})
	if err == nil {
		t.Fatalf("expected query length validation error")
	}
}

func TestSearchGitHubRepositoriesReturnsMappedRepositories(t *testing.T) {
	client := &readHelpersGitHubClient{
		searchRepositoriesFunc: func(_ context.Context, query string, perPage int) ([]GitHubRepositorySearchResult, error) {
			if query != "workset" {
				t.Fatalf("unexpected query %q", query)
			}
			if perPage != 20 {
				t.Fatalf("expected clamped limit 20, got %d", perPage)
			}
			return []GitHubRepositorySearchResult{
				{
					Name:          "workset",
					FullName:      "strantalis/workset",
					Owner:         "strantalis",
					DefaultBranch: "main",
					CloneURL:      "https://github.com/strantalis/workset.git",
					SSHURL:        "git@github.com:strantalis/workset.git",
					Private:       false,
					Archived:      false,
				},
			}, nil
		},
	}
	provider := &readHelpersGitHubProvider{client: client}
	svc := &Service{github: provider}

	result, err := svc.SearchGitHubRepositories(context.Background(), GitHubRepoSearchInput{
		Query: "  workset ",
		Limit: 200,
	})
	if err != nil {
		t.Fatalf("SearchGitHubRepositories: %v", err)
	}
	if len(result.Repositories) != 1 {
		t.Fatalf("expected one repository, got %d", len(result.Repositories))
	}
	if result.Repositories[0].FullName != "strantalis/workset" {
		t.Fatalf("unexpected repository payload: %+v", result.Repositories[0])
	}
	if result.Repositories[0].Host != defaultGitHubHost {
		t.Fatalf("expected default host fallback, got %q", result.Repositories[0].Host)
	}
	if len(provider.clientHosts) != 1 || provider.clientHosts[0] != defaultGitHubHost {
		t.Fatalf("unexpected provider host calls: %+v", provider.clientHosts)
	}
	if len(client.searchRepositoriesCalls) != 1 {
		t.Fatalf("expected one search call, got %d", len(client.searchRepositoriesCalls))
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

func TestGetPullRequestStatusClearsTrackedPullRequestWhenNotOpen(t *testing.T) {
	env, root, repoPath := setupGitHubServiceRepo(t)
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
		URL:        "https://github.com/head-org/head-repo/pull/41",
		Title:      "Initial title",
		State:      "open",
		BaseRepo:   "head-org/head-repo",
		BaseBranch: "main",
		HeadRepo:   "head-org/head-repo",
		HeadBranch: "feature/topic",
	})
	env.git.remoteURLs[repoPath] = map[string][]string{
		"origin": {"git@github.com:head-org/head-repo.git"},
	}
	env.git.remoteExists[repoPath] = map[string]bool{
		"upstream": false,
	}
	client := &readHelpersGitHubClient{
		getPullRequestFunc: func(_ context.Context, owner, repo string, number int) (GitHubPullRequest, error) {
			if owner != "head-org" || repo != "head-repo" || number != 41 {
				t.Fatalf("unexpected pull request lookup: owner=%s repo=%s number=%d", owner, repo, number)
			}
			return GitHubPullRequest{
				Number:  41,
				URL:     "https://github.com/head-org/head-repo/pull/41",
				Title:   "Merged title",
				State:   "closed",
				BaseRef: "main",
				HeadRef: "feature/topic",
			}, nil
		},
	}
	env.svc.github = &readHelpersGitHubProvider{client: client}

	result, err := env.svc.GetPullRequestStatus(ctx, PullRequestStatusInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
		Number:    41,
	})
	if err != nil {
		t.Fatalf("GetPullRequestStatus: %v", err)
	}
	if result.PullRequest.State != "closed" {
		t.Fatalf("expected closed pull request state, got %q", result.PullRequest.State)
	}

	tracked, err := env.svc.GetTrackedPullRequest(ctx, PullRequestTrackedInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
	})
	if err != nil {
		t.Fatalf("GetTrackedPullRequest: %v", err)
	}
	if tracked.Payload.Found {
		t.Fatalf("expected tracked pull request to be cleared: %+v", tracked.Payload.PullRequest)
	}
}

func TestGetPullRequestStatusKeepsTrackedPullRequestWhenClosedStatusForDifferentNumber(t *testing.T) {
	env, root, repoPath := setupGitHubServiceRepo(t)
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
		URL:        "https://github.com/head-org/head-repo/pull/41",
		Title:      "Still open",
		State:      "open",
		BaseRepo:   "head-org/head-repo",
		BaseBranch: "main",
		HeadRepo:   "head-org/head-repo",
		HeadBranch: "feature/topic",
	})
	env.git.remoteURLs[repoPath] = map[string][]string{
		"origin": {"git@github.com:head-org/head-repo.git"},
	}
	env.git.remoteExists[repoPath] = map[string]bool{
		"upstream": false,
	}
	client := &readHelpersGitHubClient{
		getPullRequestFunc: func(_ context.Context, owner, repo string, number int) (GitHubPullRequest, error) {
			if owner != "head-org" || repo != "head-repo" || number != 42 {
				t.Fatalf("unexpected pull request lookup: owner=%s repo=%s number=%d", owner, repo, number)
			}
			return GitHubPullRequest{
				Number:  42,
				URL:     "https://github.com/head-org/head-repo/pull/42",
				Title:   "Merged other PR",
				State:   "closed",
				BaseRef: "main",
				HeadRef: "feature/another",
			}, nil
		},
	}
	env.svc.github = &readHelpersGitHubProvider{client: client}

	result, err := env.svc.GetPullRequestStatus(ctx, PullRequestStatusInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
		Number:    42,
	})
	if err != nil {
		t.Fatalf("GetPullRequestStatus: %v", err)
	}
	if result.PullRequest.Number != 42 || result.PullRequest.State != "closed" {
		t.Fatalf("unexpected pull request status: %+v", result.PullRequest)
	}

	tracked, err := env.svc.GetTrackedPullRequest(ctx, PullRequestTrackedInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
	})
	if err != nil {
		t.Fatalf("GetTrackedPullRequest: %v", err)
	}
	if !tracked.Payload.Found {
		t.Fatalf("expected tracked pull request to remain")
	}
	if tracked.Payload.PullRequest.Number != 41 {
		t.Fatalf("expected tracked PR #41 to remain, got #%d", tracked.Payload.PullRequest.Number)
	}
}

func TestGetPullRequestStatusRefreshesTrackedPullRequestWhenOpen(t *testing.T) {
	env, root, repoPath := setupGitHubServiceRepo(t)
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
		URL:        "https://github.com/head-org/head-repo/pull/41",
		Title:      "Stale title",
		State:      "open",
		BaseRepo:   "head-org/head-repo",
		BaseBranch: "main",
		HeadRepo:   "head-org/head-repo",
		HeadBranch: "feature/topic",
	})
	env.git.remoteURLs[repoPath] = map[string][]string{
		"origin": {"git@github.com:head-org/head-repo.git"},
	}
	env.git.remoteExists[repoPath] = map[string]bool{
		"upstream": false,
	}
	client := &readHelpersGitHubClient{
		getPullRequestFunc: func(_ context.Context, owner, repo string, number int) (GitHubPullRequest, error) {
			if owner != "head-org" || repo != "head-repo" || number != 41 {
				t.Fatalf("unexpected pull request lookup: owner=%s repo=%s number=%d", owner, repo, number)
			}
			return GitHubPullRequest{
				Number:  41,
				URL:     "https://github.com/head-org/head-repo/pull/41",
				Title:   "Updated title",
				Body:    "Updated body",
				State:   "open",
				BaseRef: "main",
				HeadRef: "feature/topic",
			}, nil
		},
	}
	env.svc.github = &readHelpersGitHubProvider{client: client}

	_, err = env.svc.GetPullRequestStatus(ctx, PullRequestStatusInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
		Number:    41,
	})
	if err != nil {
		t.Fatalf("GetPullRequestStatus: %v", err)
	}

	tracked, err := env.svc.GetTrackedPullRequest(ctx, PullRequestTrackedInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
	})
	if err != nil {
		t.Fatalf("GetTrackedPullRequest: %v", err)
	}
	if !tracked.Payload.Found {
		t.Fatalf("expected tracked pull request to remain")
	}
	if tracked.Payload.PullRequest.Title != "Updated title" {
		t.Fatalf("expected tracked pull request title to refresh, got %q", tracked.Payload.PullRequest.Title)
	}
	if tracked.Payload.PullRequest.Body != "Updated body" {
		t.Fatalf("expected tracked pull request body to refresh, got %q", tracked.Payload.PullRequest.Body)
	}
}
