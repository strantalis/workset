package worksetapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v75/github"
)

const githubTokenSourcePAT = "pat"

type githubPATProvider struct {
	store TokenStore
}

func NewGitHubPATProvider(store TokenStore) GitHubProvider {
	if store == nil {
		store = KeyringTokenStore{}
	}
	return &githubPATProvider{store: store}
}

func (p *githubPATProvider) AuthStatus(ctx context.Context) (GitHubAuthStatusJSON, error) {
	token, err := p.getToken(ctx)
	if err != nil {
		if IsAuthRequiredError(err) {
			return GitHubAuthStatusJSON{Authenticated: false}, nil
		}
		return GitHubAuthStatusJSON{}, err
	}
	client, err := newGitHubClientWithToken(token, defaultGitHubHost)
	if err != nil {
		return GitHubAuthStatusJSON{}, err
	}
	user, resp, err := client.Users.Get(ctx, "")
	if err != nil {
		return GitHubAuthStatusJSON{}, ValidationError{Message: formatGitHubAPIError(err)}
	}
	scopes := parseGitHubScopes(resp)
	return GitHubAuthStatusJSON{
		Authenticated: true,
		Login:         user.GetLogin(),
		Name:          user.GetName(),
		Scopes:        scopes,
		TokenSource:   githubTokenSourcePAT,
	}, nil
}

func (p *githubPATProvider) SetToken(ctx context.Context, token string) (GitHubAuthStatusJSON, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return GitHubAuthStatusJSON{}, ValidationError{Message: "GitHub token required"}
	}
	client, err := newGitHubClientWithToken(token, defaultGitHubHost)
	if err != nil {
		return GitHubAuthStatusJSON{}, err
	}
	user, resp, err := client.Users.Get(ctx, "")
	if err != nil {
		return GitHubAuthStatusJSON{}, ValidationError{Message: formatGitHubAPIError(err)}
	}
	if err := p.store.Set(ctx, tokenStoreKey, token); err != nil {
		return GitHubAuthStatusJSON{}, err
	}
	if err := p.store.Set(ctx, tokenSourceKey, githubTokenSourcePAT); err != nil {
		_ = p.store.Delete(ctx, tokenStoreKey)
		return GitHubAuthStatusJSON{}, err
	}
	scopes := parseGitHubScopes(resp)
	return GitHubAuthStatusJSON{
		Authenticated: true,
		Login:         user.GetLogin(),
		Name:          user.GetName(),
		Scopes:        scopes,
		TokenSource:   githubTokenSourcePAT,
	}, nil
}

func (p *githubPATProvider) ClearAuth(ctx context.Context) error {
	if p.store == nil {
		return nil
	}
	if err := p.store.Delete(ctx, tokenStoreKey); err != nil {
		return err
	}
	if err := p.store.Delete(ctx, tokenSourceKey); err != nil {
		return err
	}
	return nil
}

func (p *githubPATProvider) Client(ctx context.Context, host string) (GitHubClient, error) {
	token, err := p.getToken(ctx)
	if err != nil {
		return nil, err
	}
	client, err := newGitHubClientWithToken(token, host)
	if err != nil {
		return nil, err
	}
	return &githubPATClient{
		token:  token,
		host:   host,
		client: client,
	}, nil
}

func (p *githubPATProvider) getToken(ctx context.Context) (string, error) {
	if p.store == nil {
		return "", AuthRequiredError{Message: "GitHub authentication required"}
	}
	token, err := p.store.Get(ctx, tokenStoreKey)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return "", AuthRequiredError{Message: "GitHub authentication required"}
		}
		return "", err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", AuthRequiredError{Message: "GitHub authentication required"}
	}
	return token, nil
}

type githubPATClient struct {
	token  string
	host   string
	client *github.Client
}

func (c *githubPATClient) CreatePullRequest(ctx context.Context, owner, repo string, pr GitHubNewPullRequest) (GitHubPullRequest, error) {
	newPR := &github.NewPullRequest{
		Title: github.Ptr(pr.Title),
		Head:  github.Ptr(pr.Head),
		Base:  github.Ptr(pr.Base),
		Body:  github.Ptr(strings.TrimSpace(pr.Body)),
		Draft: github.Ptr(pr.Draft),
	}
	created, _, err := c.client.PullRequests.Create(ctx, owner, repo, newPR)
	if err != nil {
		return GitHubPullRequest{}, err
	}
	return mapPullRequest(created), nil
}

func (c *githubPATClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (GitHubPullRequest, error) {
	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return GitHubPullRequest{}, err
	}
	return mapPullRequest(pr), nil
}

func (c *githubPATClient) ListPullRequests(ctx context.Context, owner, repo, head, state string, page, perPage int) ([]GitHubPullRequest, int, error) {
	opts := &github.PullRequestListOptions{
		State:       state,
		Head:        head,
		ListOptions: github.ListOptions{PerPage: perPage, Page: page},
	}
	prs, resp, err := c.client.PullRequests.List(ctx, owner, repo, opts)
	if err != nil {
		return nil, 0, err
	}
	out := make([]GitHubPullRequest, 0, len(prs))
	for _, pr := range prs {
		out = append(out, mapPullRequest(pr))
	}
	next := 0
	if resp != nil {
		next = resp.NextPage
	}
	return out, next, nil
}

func (c *githubPATClient) ListReviewComments(ctx context.Context, owner, repo string, number, page, perPage int) ([]PullRequestReviewCommentJSON, int, error) {
	opts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: perPage, Page: page},
	}
	comments, resp, err := c.client.PullRequests.ListComments(ctx, owner, repo, number, opts)
	if err != nil {
		return nil, 0, err
	}
	out := make([]PullRequestReviewCommentJSON, 0, len(comments))
	for _, comment := range comments {
		out = append(out, mapReviewComment(comment))
	}
	next := 0
	if resp != nil {
		next = resp.NextPage
	}
	return out, next, nil
}

func (c *githubPATClient) CreateReplyComment(ctx context.Context, owner, repo string, number int, commentID int64, body string) (PullRequestReviewCommentJSON, error) {
	comment, _, err := c.client.PullRequests.CreateCommentInReplyTo(ctx, owner, repo, number, strings.TrimSpace(body), commentID)
	if err != nil {
		return PullRequestReviewCommentJSON{}, err
	}
	return mapReviewComment(comment), nil
}

func (c *githubPATClient) EditReviewComment(ctx context.Context, owner, repo string, commentID int64, body string) (PullRequestReviewCommentJSON, error) {
	comment, _, err := c.client.PullRequests.EditComment(ctx, owner, repo, commentID, &github.PullRequestComment{
		Body: github.Ptr(strings.TrimSpace(body)),
	})
	if err != nil {
		return PullRequestReviewCommentJSON{}, err
	}
	return mapReviewComment(comment), nil
}

func (c *githubPATClient) DeleteReviewComment(ctx context.Context, owner, repo string, commentID int64) error {
	_, err := c.client.PullRequests.DeleteComment(ctx, owner, repo, commentID)
	return err
}

func (c *githubPATClient) ListCheckRuns(ctx context.Context, owner, repo, ref string, page, perPage int) ([]PullRequestCheckJSON, int, error) {
	opts := &github.ListCheckRunsOptions{ListOptions: github.ListOptions{PerPage: perPage, Page: page}}
	result, resp, err := c.client.Checks.ListCheckRunsForRef(ctx, owner, repo, ref, opts)
	if err != nil {
		return nil, 0, err
	}
	checks := make([]PullRequestCheckJSON, 0, len(result.CheckRuns))
	for _, run := range result.CheckRuns {
		checks = append(checks, PullRequestCheckJSON{
			Name:        run.GetName(),
			Status:      run.GetStatus(),
			Conclusion:  run.GetConclusion(),
			DetailsURL:  run.GetDetailsURL(),
			StartedAt:   formatGitHubTimestamp(run.StartedAt),
			CompletedAt: formatGitHubTimestamp(run.CompletedAt),
		})
	}
	next := 0
	if resp != nil {
		next = resp.NextPage
	}
	return checks, next, nil
}

func (c *githubPATClient) GetRepoDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	repository, _, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return "", err
	}
	return repository.GetDefaultBranch(), nil
}

func (c *githubPATClient) GetCurrentUser(ctx context.Context) (GitHubUserJSON, []string, error) {
	user, resp, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return GitHubUserJSON{}, nil, err
	}
	return GitHubUserJSON{
		ID:    user.GetID(),
		Login: user.GetLogin(),
		Name:  user.GetName(),
		Email: user.GetEmail(),
	}, parseGitHubScopes(resp), nil
}

func (c *githubPATClient) ReviewThreadMap(ctx context.Context, owner, repo string, number int) (map[string]threadInfo, error) {
	return graphQLReviewThreadMap(ctx, c.token, c.host, owner, repo, number)
}

func (c *githubPATClient) GetReviewThreadID(ctx context.Context, commentNodeID string) (string, error) {
	endpoint := "https://api.github.com/graphql"
	if c.host != "" && c.host != defaultGitHubHost {
		endpoint = fmt.Sprintf("https://%s/api/graphql", c.host)
	}
	return graphQLGetThreadID(ctx, endpoint, c.token, commentNodeID)
}

func (c *githubPATClient) ResolveReviewThread(ctx context.Context, threadID string, resolve bool) (bool, error) {
	return graphQLResolveThread(ctx, c.token, c.host, threadID, resolve)
}

func newGitHubClientWithToken(token, host string) (*github.Client, error) {
	if host == "" || host == defaultGitHubHost {
		return github.NewClient(nil).WithAuthToken(token), nil
	}
	baseURL := fmt.Sprintf("https://%s/api/v3/", host)
	uploadURL := fmt.Sprintf("https://%s/api/uploads/", host)
	client, err := github.NewClient(nil).WithEnterpriseURLs(baseURL, uploadURL)
	if err != nil {
		return nil, err
	}
	return client.WithAuthToken(token), nil
}

func mapPullRequest(pr *github.PullRequest) GitHubPullRequest {
	if pr == nil {
		return GitHubPullRequest{}
	}
	return GitHubPullRequest{
		Number:    pr.GetNumber(),
		URL:       pr.GetHTMLURL(),
		Title:     pr.GetTitle(),
		Body:      pr.GetBody(),
		Draft:     pr.GetDraft(),
		State:     pr.GetState(),
		BaseRef:   pr.GetBase().GetRef(),
		HeadRef:   pr.GetHead().GetRef(),
		HeadSHA:   pr.GetHead().GetSHA(),
		Mergeable: pr.Mergeable,
	}
}

func mapReviewComment(comment *github.PullRequestComment) PullRequestReviewCommentJSON {
	outdated := comment.GetPosition() == 0 && comment.GetOriginalPosition() != 0
	var authorID int64
	if comment.User != nil {
		authorID = comment.User.GetID()
	}
	return PullRequestReviewCommentJSON{
		ID:             comment.GetID(),
		NodeID:         comment.GetNodeID(),
		ReviewID:       comment.GetPullRequestReviewID(),
		Author:         comment.User.GetLogin(),
		AuthorID:       authorID,
		Body:           comment.GetBody(),
		Path:           comment.GetPath(),
		Line:           comment.GetLine(),
		Side:           comment.GetSide(),
		CommitID:       comment.GetCommitID(),
		OriginalCommit: comment.GetOriginalCommitID(),
		OriginalLine:   comment.GetOriginalLine(),
		OriginalStart:  comment.GetOriginalStartLine(),
		Outdated:       outdated,
		URL:            comment.GetHTMLURL(),
		CreatedAt:      formatGitHubTimestamp(comment.CreatedAt),
		UpdatedAt:      formatGitHubTimestamp(comment.UpdatedAt),
		InReplyTo:      comment.GetInReplyTo(),
		ReplyToComment: comment.GetInReplyTo() != 0,
	}
}

func parseGitHubScopes(resp *github.Response) []string {
	if resp == nil || resp.Response == nil {
		return nil
	}
	header := resp.Header.Get("X-OAuth-Scopes")
	if strings.TrimSpace(header) == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	scopes := make([]string, 0, len(parts))
	for _, part := range parts {
		scope := strings.TrimSpace(part)
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

func formatGitHubTimestamp(ts *github.Timestamp) string {
	if ts == nil || ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339)
}
