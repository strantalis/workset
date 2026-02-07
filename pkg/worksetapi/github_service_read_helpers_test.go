package worksetapi

import (
	"context"
	"errors"
	"testing"

	"github.com/strantalis/workset/internal/ops"
)

type readHelpersPRCall struct {
	owner   string
	repo    string
	head    string
	state   string
	page    int
	perPage int
}

type readHelpersCheckRunCall struct {
	owner   string
	repo    string
	ref     string
	page    int
	perPage int
}

type readHelpersGitHubClient struct {
	listPullRequestsCalls []readHelpersPRCall
	listCheckRunsCalls    []readHelpersCheckRunCall

	listPullRequestsFunc func(ctx context.Context, owner, repo, head, state string, page, perPage int) ([]GitHubPullRequest, int, error)
	listCheckRunsFunc    func(ctx context.Context, owner, repo, ref string, page, perPage int) ([]PullRequestCheckJSON, int, error)
}

func (c *readHelpersGitHubClient) CreatePullRequest(_ context.Context, _ string, _ string, _ GitHubNewPullRequest) (GitHubPullRequest, error) {
	return GitHubPullRequest{}, nil
}

func (c *readHelpersGitHubClient) GetPullRequest(_ context.Context, _ string, _ string, _ int) (GitHubPullRequest, error) {
	return GitHubPullRequest{}, nil
}

func (c *readHelpersGitHubClient) ListPullRequests(ctx context.Context, owner, repo, head, state string, page, perPage int) ([]GitHubPullRequest, int, error) {
	c.listPullRequestsCalls = append(c.listPullRequestsCalls, readHelpersPRCall{
		owner:   owner,
		repo:    repo,
		head:    head,
		state:   state,
		page:    page,
		perPage: perPage,
	})
	if c.listPullRequestsFunc == nil {
		return nil, 0, nil
	}
	return c.listPullRequestsFunc(ctx, owner, repo, head, state, page, perPage)
}

func (c *readHelpersGitHubClient) ListReviewComments(_ context.Context, _ string, _ string, _ int, _ int, _ int) ([]PullRequestReviewCommentJSON, int, error) {
	return nil, 0, nil
}

func (c *readHelpersGitHubClient) CreateReplyComment(_ context.Context, _ string, _ string, _ int, _ int64, _ string) (PullRequestReviewCommentJSON, error) {
	return PullRequestReviewCommentJSON{}, nil
}

func (c *readHelpersGitHubClient) EditReviewComment(_ context.Context, _ string, _ string, _ int64, _ string) (PullRequestReviewCommentJSON, error) {
	return PullRequestReviewCommentJSON{}, nil
}

func (c *readHelpersGitHubClient) DeleteReviewComment(_ context.Context, _ string, _ string, _ int64) error {
	return nil
}

func (c *readHelpersGitHubClient) ListCheckRuns(ctx context.Context, owner, repo, ref string, page, perPage int) ([]PullRequestCheckJSON, int, error) {
	c.listCheckRunsCalls = append(c.listCheckRunsCalls, readHelpersCheckRunCall{
		owner:   owner,
		repo:    repo,
		ref:     ref,
		page:    page,
		perPage: perPage,
	})
	if c.listCheckRunsFunc == nil {
		return nil, 0, nil
	}
	return c.listCheckRunsFunc(ctx, owner, repo, ref, page, perPage)
}

func (c *readHelpersGitHubClient) GetCheckRunAnnotations(_ context.Context, _ string, _ string, _ int64) ([]CheckAnnotationJSON, error) {
	return nil, nil
}

func (c *readHelpersGitHubClient) GetRepoDefaultBranch(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

func (c *readHelpersGitHubClient) GetCurrentUser(_ context.Context) (GitHubUserJSON, []string, error) {
	return GitHubUserJSON{}, nil, nil
}

func (c *readHelpersGitHubClient) ReviewThreadMap(_ context.Context, _ string, _ string, _ int) (map[string]threadInfo, error) {
	return nil, nil
}

func (c *readHelpersGitHubClient) GetReviewThreadID(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (c *readHelpersGitHubClient) ResolveReviewThread(_ context.Context, _ string, _ bool) (bool, error) {
	return false, nil
}

func TestResolveRemoteInfoAutoDetectsUpstream(t *testing.T) {
	repoPath := t.TempDir()
	gitClient := newFakeGit()
	gitClient.remoteURLs[repoPath] = map[string][]string{
		"origin":   {"git@github.com:head-org/head-repo.git"},
		"upstream": {"git@github.com:base-org/base-repo.git"},
	}
	gitClient.remoteExists[repoPath] = map[string]bool{
		"upstream": true,
	}
	svc := &Service{git: gitClient}

	head, base, err := svc.resolveRemoteInfo(context.Background(), repoResolution{
		RepoPath:     repoPath,
		RepoDefaults: ops.RepoDefaults{},
	}, "")
	if err != nil {
		t.Fatalf("resolveRemoteInfo: %v", err)
	}
	if head.Remote != "origin" || head.Owner != "head-org" || head.Repo != "head-repo" {
		t.Fatalf("unexpected head info: %+v", head)
	}
	if base.Remote != "upstream" || base.Owner != "base-org" || base.Repo != "base-repo" {
		t.Fatalf("unexpected base info: %+v", base)
	}
}

func TestResolveRemoteInfoRejectsMismatchedHost(t *testing.T) {
	repoPath := t.TempDir()
	gitClient := newFakeGit()
	gitClient.remoteURLs[repoPath] = map[string][]string{
		"origin": {"git@github.com:head-org/head-repo.git"},
		"fork":   {"git@ghe.internal:head-org/head-repo.git"},
	}
	svc := &Service{git: gitClient}

	_, _, err := svc.resolveRemoteInfo(context.Background(), repoResolution{
		RepoPath:     repoPath,
		RepoDefaults: ops.RepoDefaults{Remote: "origin"},
	}, "fork")
	if err == nil {
		t.Fatalf("expected error")
	}
	validationErr := requireErrorType[ValidationError](t, err)
	if validationErr.Message != "head and base remotes must share the same GitHub host" {
		t.Fatalf("unexpected message: %q", validationErr.Message)
	}
}

func TestResolveRemoteInfoRejectsUnsupportedHost(t *testing.T) {
	repoPath := t.TempDir()
	gitClient := newFakeGit()
	gitClient.remoteURLs[repoPath] = map[string][]string{
		"origin": {"git@ghe.internal:head-org/head-repo.git"},
	}
	gitClient.remoteExists[repoPath] = map[string]bool{
		"upstream": false,
	}
	svc := &Service{git: gitClient}

	_, _, err := svc.resolveRemoteInfo(context.Background(), repoResolution{
		RepoPath:     repoPath,
		RepoDefaults: ops.RepoDefaults{Remote: "origin"},
	}, "")
	if err == nil {
		t.Fatalf("expected error")
	}
	validationErr := requireErrorType[ValidationError](t, err)
	if validationErr.Message == "" {
		t.Fatalf("expected non-empty error message")
	}
}

func TestResolveCurrentBranchPrefersGitBranch(t *testing.T) {
	repoPath := t.TempDir()
	gitClient := newFakeGit()
	gitClient.currentBranch[repoPath] = "feature/current"
	gitClient.currentOK[repoPath] = true
	svc := &Service{git: gitClient}

	branch, err := svc.resolveCurrentBranch(repoResolution{
		RepoPath: repoPath,
		Branch:   "fallback",
	})
	if err != nil {
		t.Fatalf("resolveCurrentBranch: %v", err)
	}
	if branch != "feature/current" {
		t.Fatalf("unexpected branch: %q", branch)
	}
}

func TestResolveCurrentBranchFallsBackToResolutionBranch(t *testing.T) {
	svc := &Service{git: newFakeGit()}
	branch, err := svc.resolveCurrentBranch(repoResolution{RepoPath: t.TempDir(), Branch: "fallback"})
	if err != nil {
		t.Fatalf("resolveCurrentBranch: %v", err)
	}
	if branch != "fallback" {
		t.Fatalf("unexpected branch: %q", branch)
	}
}

func TestResolveCurrentBranchReturnsValidationErrorWithoutFallback(t *testing.T) {
	svc := &Service{git: newFakeGit()}
	_, err := svc.resolveCurrentBranch(repoResolution{RepoPath: t.TempDir()})
	if err == nil {
		t.Fatalf("expected error")
	}
	validationErr := requireErrorType[ValidationError](t, err)
	if validationErr.Message != "unable to resolve current branch" {
		t.Fatalf("unexpected message: %q", validationErr.Message)
	}
}

func TestFindPullRequestNumberPaginatesAndUsesHeadRef(t *testing.T) {
	client := &readHelpersGitHubClient{
		listPullRequestsFunc: func(_ context.Context, _ string, _ string, _ string, _ string, page, _ int) ([]GitHubPullRequest, int, error) {
			if page == 1 {
				return nil, 2, nil
			}
			return []GitHubPullRequest{{Number: 42}}, 0, nil
		},
	}
	svc := &Service{}

	number, err := svc.findPullRequestNumber(context.Background(), client, remoteInfo{
		Owner: "base-org",
		Repo:  "base-repo",
	}, remoteInfo{
		Owner: "head-org",
	}, "feature/abc")
	if err != nil {
		t.Fatalf("findPullRequestNumber: %v", err)
	}
	if number != 42 {
		t.Fatalf("unexpected number: %d", number)
	}
	if len(client.listPullRequestsCalls) != 2 {
		t.Fatalf("expected 2 API calls, got %d", len(client.listPullRequestsCalls))
	}
	first := client.listPullRequestsCalls[0]
	if first.owner != "base-org" || first.repo != "base-repo" {
		t.Fatalf("unexpected repo params: %+v", first)
	}
	if first.head != "head-org:feature/abc" || first.state != "open" {
		t.Fatalf("unexpected PR query params: %+v", first)
	}
	if first.page != 1 || first.perPage != 50 {
		t.Fatalf("unexpected paging params: %+v", first)
	}
}

func TestFindPullRequestNumberReturnsNotFoundWhenNoResults(t *testing.T) {
	client := &readHelpersGitHubClient{
		listPullRequestsFunc: func(_ context.Context, _ string, _ string, _ string, _ string, _ int, _ int) ([]GitHubPullRequest, int, error) {
			return nil, 0, nil
		},
	}
	svc := &Service{}

	_, err := svc.findPullRequestNumber(context.Background(), client, remoteInfo{}, remoteInfo{}, "branch")
	if err == nil {
		t.Fatalf("expected error")
	}
	notFoundErr := requireErrorType[NotFoundError](t, err)
	if notFoundErr.Message != "pull request not found for current branch" {
		t.Fatalf("unexpected message: %q", notFoundErr.Message)
	}
}

func TestFindPullRequestNumberPropagatesClientError(t *testing.T) {
	wantErr := errors.New("github failure")
	client := &readHelpersGitHubClient{
		listPullRequestsFunc: func(_ context.Context, _ string, _ string, _ string, _ string, _ int, _ int) ([]GitHubPullRequest, int, error) {
			return nil, 0, wantErr
		},
	}
	svc := &Service{}

	_, err := svc.findPullRequestNumber(context.Background(), client, remoteInfo{}, remoteInfo{}, "branch")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
}

func TestListCheckRunsReturnsNilWhenHeadSHAIsEmpty(t *testing.T) {
	client := &readHelpersGitHubClient{}
	svc := &Service{}

	checks, err := svc.listCheckRuns(context.Background(), client, remoteInfo{}, GitHubPullRequest{})
	if err != nil {
		t.Fatalf("listCheckRuns: %v", err)
	}
	if checks != nil {
		t.Fatalf("expected nil checks when sha is empty, got %+v", checks)
	}
	if len(client.listCheckRunsCalls) != 0 {
		t.Fatalf("expected no API calls, got %d", len(client.listCheckRunsCalls))
	}
}

func TestListCheckRunsPaginatesAndAggregates(t *testing.T) {
	client := &readHelpersGitHubClient{
		listCheckRunsFunc: func(_ context.Context, _ string, _ string, _ string, page, _ int) ([]PullRequestCheckJSON, int, error) {
			if page == 1 {
				return []PullRequestCheckJSON{{Name: "build"}}, 2, nil
			}
			return []PullRequestCheckJSON{{Name: "test"}}, 0, nil
		},
	}
	svc := &Service{}

	checks, err := svc.listCheckRuns(context.Background(), client, remoteInfo{
		Owner: "base-org",
		Repo:  "base-repo",
	}, GitHubPullRequest{
		HeadSHA: "abc123",
	})
	if err != nil {
		t.Fatalf("listCheckRuns: %v", err)
	}
	if len(checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(checks))
	}
	if checks[0].Name != "build" || checks[1].Name != "test" {
		t.Fatalf("unexpected checks: %+v", checks)
	}
	if len(client.listCheckRunsCalls) != 2 {
		t.Fatalf("expected 2 API calls, got %d", len(client.listCheckRunsCalls))
	}
	first := client.listCheckRunsCalls[0]
	if first.owner != "base-org" || first.repo != "base-repo" || first.ref != "abc123" {
		t.Fatalf("unexpected check query params: %+v", first)
	}
	if first.page != 1 || first.perPage != 100 {
		t.Fatalf("unexpected check paging params: %+v", first)
	}
}

func TestListCheckRunsPropagatesClientError(t *testing.T) {
	wantErr := errors.New("check run failure")
	client := &readHelpersGitHubClient{
		listCheckRunsFunc: func(_ context.Context, _ string, _ string, _ string, _ int, _ int) ([]PullRequestCheckJSON, int, error) {
			return nil, 0, wantErr
		},
	}
	svc := &Service{}

	_, err := svc.listCheckRuns(context.Background(), client, remoteInfo{}, GitHubPullRequest{HeadSHA: "abc123"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
}
