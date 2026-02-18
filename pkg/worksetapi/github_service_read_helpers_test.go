package worksetapi

import (
	"context"
	"errors"
	"testing"

	"github.com/strantalis/workset/internal/ops"
)

type readHelpersGetPullRequestCall struct {
	owner  string
	repo   string
	number int
}

type readHelpersGetRepoDefaultBranchCall struct {
	owner string
	repo  string
}

type readHelpersGetCheckRunAnnotationsCall struct {
	owner      string
	repo       string
	checkRunID int64
}

type readHelpersGetFileContentCall struct {
	owner string
	repo  string
	path  string
	ref   string
}

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
	getPullRequestCalls       []readHelpersGetPullRequestCall
	getRepoDefaultBranchCalls []readHelpersGetRepoDefaultBranchCall
	getCheckAnnotationsCalls  []readHelpersGetCheckRunAnnotationsCall
	getFileContentCalls       []readHelpersGetFileContentCall
	listPullRequestsCalls     []readHelpersPRCall
	listCheckRunsCalls        []readHelpersCheckRunCall

	getPullRequestFunc         func(ctx context.Context, owner, repo string, number int) (GitHubPullRequest, error)
	getRepoDefaultBranchFunc   func(ctx context.Context, owner, repo string) (string, error)
	getCheckRunAnnotationsFunc func(ctx context.Context, owner, repo string, checkRunID int64) ([]CheckAnnotationJSON, error)
	getFileContentFunc         func(ctx context.Context, owner, repo, path, ref string) ([]byte, bool, error)
	listPullRequestsFunc       func(ctx context.Context, owner, repo, head, state string, page, perPage int) ([]GitHubPullRequest, int, error)
	listCheckRunsFunc          func(ctx context.Context, owner, repo, ref string, page, perPage int) ([]PullRequestCheckJSON, int, error)
}

func (c *readHelpersGitHubClient) CreatePullRequest(_ context.Context, _ string, _ string, _ GitHubNewPullRequest) (GitHubPullRequest, error) {
	return GitHubPullRequest{}, nil
}

func (c *readHelpersGitHubClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (GitHubPullRequest, error) {
	c.getPullRequestCalls = append(c.getPullRequestCalls, readHelpersGetPullRequestCall{
		owner:  owner,
		repo:   repo,
		number: number,
	})
	if c.getPullRequestFunc == nil {
		return GitHubPullRequest{}, nil
	}
	return c.getPullRequestFunc(ctx, owner, repo, number)
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

func (c *readHelpersGitHubClient) GetCheckRunAnnotations(ctx context.Context, owner, repo string, checkRunID int64) ([]CheckAnnotationJSON, error) {
	c.getCheckAnnotationsCalls = append(c.getCheckAnnotationsCalls, readHelpersGetCheckRunAnnotationsCall{
		owner:      owner,
		repo:       repo,
		checkRunID: checkRunID,
	})
	if c.getCheckRunAnnotationsFunc == nil {
		return nil, nil
	}
	return c.getCheckRunAnnotationsFunc(ctx, owner, repo, checkRunID)
}

func (c *readHelpersGitHubClient) GetRepoDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	c.getRepoDefaultBranchCalls = append(c.getRepoDefaultBranchCalls, readHelpersGetRepoDefaultBranchCall{
		owner: owner,
		repo:  repo,
	})
	if c.getRepoDefaultBranchFunc == nil {
		return "", nil
	}
	return c.getRepoDefaultBranchFunc(ctx, owner, repo)
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

func (c *readHelpersGitHubClient) GetFileContent(ctx context.Context, owner, repo, path, ref string) ([]byte, bool, error) {
	c.getFileContentCalls = append(c.getFileContentCalls, readHelpersGetFileContentCall{
		owner: owner,
		repo:  repo,
		path:  path,
		ref:   ref,
	})
	if c.getFileContentFunc == nil {
		return nil, false, nil
	}
	return c.getFileContentFunc(ctx, owner, repo, path, ref)
}

type readHelpersGitHubProvider struct {
	client      GitHubClient
	clientErr   error
	importErr   error
	clientHosts []string
	importCalls int
}

func (p *readHelpersGitHubProvider) AuthStatus(_ context.Context) (GitHubAuthStatusJSON, error) {
	return GitHubAuthStatusJSON{}, nil
}

func (p *readHelpersGitHubProvider) SetToken(_ context.Context, _ string) (GitHubAuthStatusJSON, error) {
	return GitHubAuthStatusJSON{}, nil
}

func (p *readHelpersGitHubProvider) ClearAuth(_ context.Context) error {
	return nil
}

func (p *readHelpersGitHubProvider) Client(_ context.Context, host string) (GitHubClient, error) {
	p.clientHosts = append(p.clientHosts, host)
	if p.clientErr != nil {
		return nil, p.clientErr
	}
	return p.client, nil
}

func (p *readHelpersGitHubProvider) ImportPATFromEnv(_ context.Context) (bool, error) {
	p.importCalls++
	if p.importErr != nil {
		return false, p.importErr
	}
	return false, nil
}

func setupGitHubServiceRepo(t *testing.T) (*testEnv, string, string) {
	t.Helper()
	ctx := context.Background()
	env := newTestEnv(t)
	root := env.createWorkspace(ctx, "demo")
	local := env.createLocalRepo("repo-a")
	result, err := env.svc.AddRepo(ctx, RepoAddInput{
		Workspace:  WorkspaceSelector{Value: root},
		Name:       "repo-a",
		NameSet:    true,
		SourcePath: local,
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	return env, root, result.WorktreePath
}

func TestGitHubClientRequiresAuthWhenProviderMissing(t *testing.T) {
	svc := &Service{}

	_, err := svc.githubClient(context.Background(), defaultGitHubHost)
	if err == nil {
		t.Fatalf("expected auth error")
	}
	authErr := requireErrorType[AuthRequiredError](t, err)
	if authErr.Message != "GitHub authentication required" {
		t.Fatalf("unexpected message: %q", authErr.Message)
	}
}

func TestGitHubClientPropagatesPATImportError(t *testing.T) {
	wantErr := errors.New("import failed")
	svc := &Service{
		github: &readHelpersGitHubProvider{importErr: wantErr},
	}

	_, err := svc.githubClient(context.Background(), defaultGitHubHost)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestGitHubClientReturnsClientForHost(t *testing.T) {
	client := &readHelpersGitHubClient{}
	provider := &readHelpersGitHubProvider{client: client}
	svc := &Service{github: provider}

	got, err := svc.githubClient(context.Background(), "ghe.example.com")
	if err != nil {
		t.Fatalf("githubClient: %v", err)
	}
	if got != client {
		t.Fatalf("unexpected client: %#v", got)
	}
	if provider.importCalls != 1 {
		t.Fatalf("expected one PAT import attempt, got %d", provider.importCalls)
	}
	if len(provider.clientHosts) != 1 || provider.clientHosts[0] != "ghe.example.com" {
		t.Fatalf("unexpected host calls: %+v", provider.clientHosts)
	}
}

func TestResolveDefaultBranchUsesGitHubDefault(t *testing.T) {
	client := &readHelpersGitHubClient{
		getRepoDefaultBranchFunc: func(_ context.Context, owner, repo string) (string, error) {
			if owner != "base-org" || repo != "base-repo" {
				t.Fatalf("unexpected repo lookup: %s/%s", owner, repo)
			}
			return "main", nil
		},
	}
	svc := &Service{}

	branch, err := svc.resolveDefaultBranch(context.Background(), client, remoteInfo{Owner: "base-org", Repo: "base-repo"}, repoResolution{})
	if err != nil {
		t.Fatalf("resolveDefaultBranch: %v", err)
	}
	if branch != "main" {
		t.Fatalf("unexpected branch: %q", branch)
	}
}

func TestResolveDefaultBranchFallsBackToRepoDefaults(t *testing.T) {
	client := &readHelpersGitHubClient{
		getRepoDefaultBranchFunc: func(_ context.Context, _ string, _ string) (string, error) {
			return "", errors.New("api unavailable")
		},
	}
	svc := &Service{}

	branch, err := svc.resolveDefaultBranch(context.Background(), client, remoteInfo{}, repoResolution{
		RepoDefaults: ops.RepoDefaults{DefaultBranch: "develop"},
	})
	if err != nil {
		t.Fatalf("resolveDefaultBranch: %v", err)
	}
	if branch != "develop" {
		t.Fatalf("unexpected branch: %q", branch)
	}
}

func TestResolveDefaultBranchRequiresFallbackWhenGitHubUnavailable(t *testing.T) {
	client := &readHelpersGitHubClient{
		getRepoDefaultBranchFunc: func(_ context.Context, _ string, _ string) (string, error) {
			return "", errors.New("api unavailable")
		},
	}
	svc := &Service{}

	_, err := svc.resolveDefaultBranch(context.Background(), client, remoteInfo{}, repoResolution{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	validationErr := requireErrorType[ValidationError](t, err)
	if validationErr.Message != "base branch required" {
		t.Fatalf("unexpected message: %q", validationErr.Message)
	}
}

func TestResolvePullRequestUsesProvidedNumber(t *testing.T) {
	env, root, repoPath := setupGitHubServiceRepo(t)
	env.git.remoteURLs[repoPath] = map[string][]string{
		"origin": {"git@github.com:head-org/head-repo.git"},
	}
	env.git.remoteExists[repoPath] = map[string]bool{"upstream": false}

	client := &readHelpersGitHubClient{
		getPullRequestFunc: func(_ context.Context, owner, repo string, number int) (GitHubPullRequest, error) {
			if owner != "head-org" || repo != "head-repo" || number != 27 {
				t.Fatalf("unexpected pull request lookup: owner=%s repo=%s number=%d", owner, repo, number)
			}
			return GitHubPullRequest{
				Number: 27,
				Title:  "Add tests",
			}, nil
		},
	}
	provider := &readHelpersGitHubProvider{client: client}
	env.svc.github = provider

	pr, headInfo, baseInfo, gotClient, resolution, err := env.svc.resolvePullRequest(context.Background(), PullRequestStatusInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
		Number:    27,
	})
	if err != nil {
		t.Fatalf("resolvePullRequest: %v", err)
	}
	if pr.Number != 27 || pr.Title != "Add tests" {
		t.Fatalf("unexpected pull request: %+v", pr)
	}
	if headInfo.Owner != "head-org" || baseInfo.Owner != "head-org" {
		t.Fatalf("unexpected remote info: head=%+v base=%+v", headInfo, baseInfo)
	}
	if gotClient != client {
		t.Fatalf("unexpected github client: %#v", gotClient)
	}
	if resolution.Repo.Name != "repo-a" {
		t.Fatalf("unexpected resolution: %+v", resolution)
	}
	if len(client.listPullRequestsCalls) != 0 {
		t.Fatalf("expected no ListPullRequests calls, got %d", len(client.listPullRequestsCalls))
	}
	if len(client.getPullRequestCalls) != 1 {
		t.Fatalf("expected one GetPullRequest call, got %d", len(client.getPullRequestCalls))
	}
}

func TestResolvePullRequestFindsNumberFromBranch(t *testing.T) {
	env, root, repoPath := setupGitHubServiceRepo(t)
	env.git.remoteURLs[repoPath] = map[string][]string{
		"origin": {"git@github.com:head-org/head-repo.git"},
	}
	env.git.remoteExists[repoPath] = map[string]bool{"upstream": false}

	client := &readHelpersGitHubClient{
		listPullRequestsFunc: func(_ context.Context, owner, repo, head, state string, page, perPage int) ([]GitHubPullRequest, int, error) {
			if owner != "head-org" || repo != "head-repo" {
				t.Fatalf("unexpected list repo: %s/%s", owner, repo)
			}
			if head != "head-org:feature/topic" || state != "open" {
				t.Fatalf("unexpected list query: head=%q state=%q", head, state)
			}
			if page != 1 || perPage != 50 {
				t.Fatalf("unexpected pagination: page=%d perPage=%d", page, perPage)
			}
			return []GitHubPullRequest{{Number: 34}}, 0, nil
		},
		getPullRequestFunc: func(_ context.Context, _ string, _ string, number int) (GitHubPullRequest, error) {
			return GitHubPullRequest{Number: number, Title: "Topic PR"}, nil
		},
	}
	env.svc.github = &readHelpersGitHubProvider{client: client}

	pr, _, _, _, _, err := env.svc.resolvePullRequest(context.Background(), PullRequestStatusInput{
		Workspace: WorkspaceSelector{Value: root},
		Repo:      "repo-a",
		Branch:    "feature/topic",
	})
	if err != nil {
		t.Fatalf("resolvePullRequest: %v", err)
	}
	if pr.Number != 34 {
		t.Fatalf("unexpected pull request: %+v", pr)
	}
	if len(client.listPullRequestsCalls) != 1 {
		t.Fatalf("expected one ListPullRequests call, got %d", len(client.listPullRequestsCalls))
	}
	if len(client.getPullRequestCalls) != 1 || client.getPullRequestCalls[0].number != 34 {
		t.Fatalf("unexpected GetPullRequest calls: %+v", client.getPullRequestCalls)
	}
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
