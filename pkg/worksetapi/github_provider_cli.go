package worksetapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

type githubCLIProvider struct{}

func NewGitHubCLIProvider() GitHubProvider {
	return &githubCLIProvider{}
}

func (p *githubCLIProvider) AuthStatus(ctx context.Context) (GitHubAuthStatusJSON, error) {
	_ = ensureGitHubCLIPath()
	client, err := api.NewRESTClient(api.ClientOptions{Host: defaultGitHubHost})
	if err != nil {
		if isAuthMissing(err) {
			return GitHubAuthStatusJSON{Authenticated: false}, nil
		}
		return GitHubAuthStatusJSON{}, err
	}
	var user ghUserResponse
	if err := client.DoWithContext(ctx, http.MethodGet, "user", nil, &user); err != nil {
		if isAuthMissing(err) {
			return GitHubAuthStatusJSON{Authenticated: false}, nil
		}
		return GitHubAuthStatusJSON{}, err
	}
	return GitHubAuthStatusJSON{
		Authenticated: true,
		Login:         user.Login,
		Name:          user.Name,
		Scopes:        nil,
		TokenSource:   "cli",
	}, nil
}

func (p *githubCLIProvider) SetToken(_ context.Context, _ string) (GitHubAuthStatusJSON, error) {
	return GitHubAuthStatusJSON{}, ValidationError{Message: "GitHub CLI auth does not accept tokens"}
}

func (p *githubCLIProvider) ClearAuth(_ context.Context) error {
	return nil
}

func (p *githubCLIProvider) Client(ctx context.Context, host string) (GitHubClient, error) {
	if host == "" {
		host = defaultGitHubHost
	}
	_ = ensureGitHubCLIPath()
	rest, err := api.NewRESTClient(api.ClientOptions{Host: host})
	if err != nil {
		return nil, wrapAuthError(err)
	}
	graph, err := api.NewGraphQLClient(api.ClientOptions{Host: host})
	if err != nil {
		return nil, wrapAuthError(err)
	}
	return &githubCLIClient{rest: rest, graph: graph}, nil
}

type githubCLIClient struct {
	rest  *api.RESTClient
	graph *api.GraphQLClient
}

func (c *githubCLIClient) CreatePullRequest(ctx context.Context, owner, repo string, pr GitHubNewPullRequest) (GitHubPullRequest, error) {
	payload := map[string]any{
		"title": pr.Title,
		"head":  pr.Head,
		"base":  pr.Base,
		"body":  strings.TrimSpace(pr.Body),
		"draft": pr.Draft,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return GitHubPullRequest{}, err
	}
	var response pullRequestREST
	path := fmt.Sprintf("repos/%s/%s/pulls", owner, repo)
	if err := c.rest.DoWithContext(ctx, http.MethodPost, path, bytes.NewReader(body), &response); err != nil {
		return GitHubPullRequest{}, wrapAuthError(err)
	}
	return mapPullRequestREST(response), nil
}

func (c *githubCLIClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (GitHubPullRequest, error) {
	var response pullRequestREST
	path := fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, number)
	if err := c.rest.DoWithContext(ctx, http.MethodGet, path, nil, &response); err != nil {
		return GitHubPullRequest{}, wrapAuthError(err)
	}
	return mapPullRequestREST(response), nil
}

func (c *githubCLIClient) ListPullRequests(ctx context.Context, owner, repo, head, state string, page, perPage int) ([]GitHubPullRequest, int, error) {
	query := url.Values{}
	if state != "" {
		query.Set("state", state)
	}
	if head != "" {
		query.Set("head", head)
	}
	if perPage > 0 {
		query.Set("per_page", strconv.Itoa(perPage))
	}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	path := fmt.Sprintf("repos/%s/%s/pulls?%s", owner, repo, query.Encode())
	resp, err := c.rest.RequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, 0, wrapAuthError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var items []pullRequestREST
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, 0, err
	}
	next := nextPageFromLink(resp.Header.Get("Link"))
	out := make([]GitHubPullRequest, 0, len(items))
	for _, pr := range items {
		out = append(out, mapPullRequestREST(pr))
	}
	return out, next, nil
}

func (c *githubCLIClient) ListReviewComments(ctx context.Context, owner, repo string, number, page, perPage int) ([]PullRequestReviewCommentJSON, int, error) {
	query := url.Values{}
	if perPage > 0 {
		query.Set("per_page", strconv.Itoa(perPage))
	}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/comments?%s", owner, repo, number, query.Encode())
	resp, err := c.rest.RequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, 0, wrapAuthError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var items []reviewCommentREST
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, 0, err
	}
	next := nextPageFromLink(resp.Header.Get("Link"))
	out := make([]PullRequestReviewCommentJSON, 0, len(items))
	for _, comment := range items {
		out = append(out, mapReviewCommentREST(comment))
	}
	return out, next, nil
}

func (c *githubCLIClient) CreateReplyComment(ctx context.Context, owner, repo string, number int, commentID int64, body string) (PullRequestReviewCommentJSON, error) {
	payload := map[string]any{
		"body":        strings.TrimSpace(body),
		"in_reply_to": commentID,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return PullRequestReviewCommentJSON{}, err
	}
	var response reviewCommentREST
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/comments", owner, repo, number)
	if err := c.rest.DoWithContext(ctx, http.MethodPost, path, bytes.NewReader(raw), &response); err != nil {
		return PullRequestReviewCommentJSON{}, wrapAuthError(err)
	}
	return mapReviewCommentREST(response), nil
}

func (c *githubCLIClient) EditReviewComment(ctx context.Context, owner, repo string, commentID int64, body string) (PullRequestReviewCommentJSON, error) {
	payload := map[string]any{
		"body": strings.TrimSpace(body),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return PullRequestReviewCommentJSON{}, err
	}
	var response reviewCommentREST
	path := fmt.Sprintf("repos/%s/%s/pulls/comments/%d", owner, repo, commentID)
	if err := c.rest.DoWithContext(ctx, http.MethodPatch, path, bytes.NewReader(raw), &response); err != nil {
		return PullRequestReviewCommentJSON{}, wrapAuthError(err)
	}
	return mapReviewCommentREST(response), nil
}

func (c *githubCLIClient) DeleteReviewComment(ctx context.Context, owner, repo string, commentID int64) error {
	path := fmt.Sprintf("repos/%s/%s/pulls/comments/%d", owner, repo, commentID)
	return wrapAuthError(c.rest.DoWithContext(ctx, http.MethodDelete, path, nil, &struct{}{}))
}

func (c *githubCLIClient) ListCheckRuns(ctx context.Context, owner, repo, ref string, page, perPage int) ([]PullRequestCheckJSON, int, error) {
	query := url.Values{}
	if perPage > 0 {
		query.Set("per_page", strconv.Itoa(perPage))
	}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	path := fmt.Sprintf("repos/%s/%s/commits/%s/check-runs?%s", owner, repo, ref, query.Encode())
	resp, err := c.rest.RequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, 0, wrapAuthError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var response checkRunsREST
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, 0, err
	}
	next := nextPageFromLink(resp.Header.Get("Link"))
	checks := make([]PullRequestCheckJSON, 0, len(response.CheckRuns))
	for _, run := range response.CheckRuns {
		checks = append(checks, PullRequestCheckJSON{
			Name:        run.Name,
			Status:      run.Status,
			Conclusion:  run.Conclusion,
			DetailsURL:  run.DetailsURL,
			StartedAt:   formatTime(run.StartedAt),
			CompletedAt: formatTime(run.CompletedAt),
			CheckRunID:  run.ID,
		})
	}
	return checks, next, nil
}

func (c *githubCLIClient) GetCheckRunAnnotations(ctx context.Context, owner, repo string, checkRunID int64) ([]CheckAnnotationJSON, error) {
	path := fmt.Sprintf("repos/%s/%s/check-runs/%d/annotations", owner, repo, checkRunID)
	resp, err := c.rest.RequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, wrapAuthError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var response []checkAnnotationREST
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	annotations := make([]CheckAnnotationJSON, 0, len(response))
	for _, ann := range response {
		annotations = append(annotations, CheckAnnotationJSON{
			Path:      ann.Path,
			StartLine: ann.StartLine,
			EndLine:   ann.EndLine,
			Level:     ann.AnnotationLevel,
			Message:   ann.Message,
			Title:     ann.Title,
		})
	}
	return annotations, nil
}

func (c *githubCLIClient) GetRepoDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	var response struct {
		DefaultBranch string `json:"default_branch"`
	}
	path := fmt.Sprintf("repos/%s/%s", owner, repo)
	if err := c.rest.DoWithContext(ctx, http.MethodGet, path, nil, &response); err != nil {
		return "", wrapAuthError(err)
	}
	return response.DefaultBranch, nil
}

func (c *githubCLIClient) GetFileContent(ctx context.Context, owner, repo, path, ref string) ([]byte, bool, error) {
	query := url.Values{}
	if strings.TrimSpace(ref) != "" {
		query.Set("ref", strings.TrimSpace(ref))
	}

	requestPath := fmt.Sprintf("repos/%s/%s/contents/%s", owner, repo, strings.TrimPrefix(path, "/"))
	if encoded := query.Encode(); encoded != "" {
		requestPath += "?" + encoded
	}

	var response struct {
		Type     string `json:"type"`
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
	}
	if err := c.rest.DoWithContext(ctx, http.MethodGet, requestPath, nil, &response); err != nil {
		if isHTTPStatus(err, http.StatusNotFound) {
			return nil, false, nil
		}
		return nil, false, wrapAuthError(err)
	}
	if response.Type != "" && !strings.EqualFold(response.Type, "file") {
		return nil, false, nil
	}
	if strings.EqualFold(response.Encoding, "base64") {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(response.Content, "\n", ""))
		if err != nil {
			return nil, false, err
		}
		return decoded, true, nil
	}
	return []byte(response.Content), true, nil
}

func (c *githubCLIClient) GetCurrentUser(ctx context.Context) (GitHubUserJSON, []string, error) {
	var user ghUserResponse
	if err := c.rest.DoWithContext(ctx, http.MethodGet, "user", nil, &user); err != nil {
		return GitHubUserJSON{}, nil, wrapAuthError(err)
	}
	return GitHubUserJSON(user), nil, nil
}

func (c *githubCLIClient) ReviewThreadMap(ctx context.Context, owner, repo string, number int) (map[string]threadInfo, error) {
	threadMap := make(map[string]threadInfo)
	var cursor *string

	query := `query($owner: String!, $repo: String!, $number: Int!, $after: String) {
		repository(owner: $owner, name: $repo) {
			pullRequest(number: $number) {
				reviewThreads(first: 100, after: $after) {
					pageInfo {
						hasNextPage
						endCursor
					}
					nodes {
						id
						isResolved
						comments(first: 100) {
							nodes {
								id
							}
						}
					}
				}
			}
		}
	}`

	for {
		var result struct {
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
						Nodes []struct {
							ID         string `json:"id"`
							IsResolved bool   `json:"isResolved"`
							Comments   struct {
								Nodes []struct {
									ID string `json:"id"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}
		variables := map[string]any{
			"owner":  owner,
			"repo":   repo,
			"number": number,
			"after":  cursor,
		}

		if err := c.graph.DoWithContext(ctx, query, variables, &result); err != nil {
			return nil, wrapAuthError(err)
		}
		for _, thread := range result.Repository.PullRequest.ReviewThreads.Nodes {
			for _, comment := range thread.Comments.Nodes {
				if comment.ID != "" && thread.ID != "" {
					threadMap[comment.ID] = threadInfo{
						ThreadID:   thread.ID,
						IsResolved: thread.IsResolved,
					}
				}
			}
		}
		if !result.Repository.PullRequest.ReviewThreads.PageInfo.HasNextPage {
			break
		}
		next := strings.TrimSpace(result.Repository.PullRequest.ReviewThreads.PageInfo.EndCursor)
		if next == "" {
			break
		}
		cursor = &next
	}

	return threadMap, nil
}

func (c *githubCLIClient) GetReviewThreadID(ctx context.Context, commentNodeID string) (string, error) {
	query := `query($id: ID!) {
		node(id: $id) {
			... on PullRequestReviewComment {
				id
				pullRequest {
					reviewThreads(first: 100) {
						nodes {
							id
							comments(first: 100) {
								nodes {
									id
								}
							}
						}
					}
				}
			}
		}
	}`

	var result struct {
		Node struct {
			ID          string `json:"id"`
			PullRequest struct {
				ReviewThreads struct {
					Nodes []struct {
						ID       string `json:"id"`
						Comments struct {
							Nodes []struct {
								ID string `json:"id"`
							} `json:"nodes"`
						} `json:"comments"`
					} `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"node"`
	}

	if err := c.graph.DoWithContext(ctx, query, map[string]any{"id": commentNodeID}, &result); err != nil {
		return "", wrapAuthError(err)
	}

	for _, thread := range result.Node.PullRequest.ReviewThreads.Nodes {
		for _, comment := range thread.Comments.Nodes {
			if comment.ID == commentNodeID {
				return thread.ID, nil
			}
		}
	}
	return "", ValidationError{Message: "could not find thread for comment"}
}

func (c *githubCLIClient) ResolveReviewThread(ctx context.Context, threadID string, resolve bool) (bool, error) {
	mutation := "resolveReviewThread"
	if !resolve {
		mutation = "unresolveReviewThread"
	}

	query := fmt.Sprintf(`mutation($threadId: ID!) {
		%s(input: {threadId: $threadId}) {
			thread {
				isResolved
			}
		}
	}`, mutation)

	var result struct {
		ResolveReviewThread struct {
			Thread struct {
				IsResolved bool `json:"isResolved"`
			} `json:"thread"`
		} `json:"resolveReviewThread"`
		UnresolveReviewThread struct {
			Thread struct {
				IsResolved bool `json:"isResolved"`
			} `json:"thread"`
		} `json:"unresolveReviewThread"`
	}

	if err := c.graph.DoWithContext(ctx, query, map[string]any{"threadId": threadID}, &result); err != nil {
		return false, wrapAuthError(err)
	}

	if resolve {
		return result.ResolveReviewThread.Thread.IsResolved, nil
	}
	return result.UnresolveReviewThread.Thread.IsResolved, nil
}

type pullRequestREST struct {
	Number    int    `json:"number"`
	HTMLURL   string `json:"html_url"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Draft     bool   `json:"draft"`
	State     string `json:"state"`
	Mergeable *bool  `json:"mergeable"`
	Base      struct {
		Ref string `json:"ref"`
	} `json:"base"`
	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
}

type ghUserResponse struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type checkRunsREST struct {
	CheckRuns []struct {
		ID          int64      `json:"id"`
		Name        string     `json:"name"`
		Status      string     `json:"status"`
		Conclusion  string     `json:"conclusion"`
		DetailsURL  string     `json:"details_url"`
		StartedAt   *time.Time `json:"started_at"`
		CompletedAt *time.Time `json:"completed_at"`
	} `json:"check_runs"`
}

// checkAnnotationREST represents a single check annotation from GitHub API
type checkAnnotationREST struct {
	Path            string `json:"path"`
	StartLine       int    `json:"start_line"`
	EndLine         int    `json:"end_line"`
	AnnotationLevel string `json:"annotation_level"`
	Message         string `json:"message"`
	Title           string `json:"title"`
}

type reviewCommentREST struct {
	ID                int64     `json:"id"`
	NodeID            string    `json:"node_id"`
	PullRequestReview int64     `json:"pull_request_review_id"`
	User              *ghUser   `json:"user"`
	Body              string    `json:"body"`
	Path              string    `json:"path"`
	Line              int       `json:"line"`
	Side              string    `json:"side"`
	CommitID          string    `json:"commit_id"`
	OriginalCommitID  string    `json:"original_commit_id"`
	OriginalLine      int       `json:"original_line"`
	OriginalStartLine int       `json:"original_start_line"`
	Position          int       `json:"position"`
	OriginalPosition  int       `json:"original_position"`
	HTMLURL           string    `json:"html_url"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	InReplyToID       int64     `json:"in_reply_to_id"`
}

type ghUser struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

func mapPullRequestREST(pr pullRequestREST) GitHubPullRequest {
	return GitHubPullRequest{
		Number:    pr.Number,
		URL:       pr.HTMLURL,
		Title:     pr.Title,
		Body:      pr.Body,
		Draft:     pr.Draft,
		State:     pr.State,
		BaseRef:   pr.Base.Ref,
		HeadRef:   pr.Head.Ref,
		HeadSHA:   pr.Head.SHA,
		Mergeable: pr.Mergeable,
	}
}

func mapReviewCommentREST(comment reviewCommentREST) PullRequestReviewCommentJSON {
	outdated := comment.Position == 0 && comment.OriginalPosition != 0
	author := ""
	var authorID int64
	if comment.User != nil {
		author = comment.User.Login
		authorID = comment.User.ID
	}
	return PullRequestReviewCommentJSON{
		ID:             comment.ID,
		NodeID:         comment.NodeID,
		ReviewID:       comment.PullRequestReview,
		Author:         author,
		AuthorID:       authorID,
		Body:           comment.Body,
		Path:           comment.Path,
		Line:           comment.Line,
		Side:           comment.Side,
		CommitID:       comment.CommitID,
		OriginalCommit: comment.OriginalCommitID,
		OriginalLine:   comment.OriginalLine,
		OriginalStart:  comment.OriginalStartLine,
		Outdated:       outdated,
		URL:            comment.HTMLURL,
		CreatedAt:      formatTime(&comment.CreatedAt),
		UpdatedAt:      formatTime(&comment.UpdatedAt),
		InReplyTo:      comment.InReplyToID,
		ReplyToComment: comment.InReplyToID != 0,
	}
}

func nextPageFromLink(link string) int {
	if link == "" {
		return 0
	}
	for part := range strings.SplitSeq(link, ",") {
		section := strings.Split(strings.TrimSpace(part), ";")
		if len(section) < 2 {
			continue
		}
		if strings.TrimSpace(section[1]) != `rel="next"` {
			continue
		}
		urlPart := strings.Trim(section[0], " <>")
		parsed, err := url.Parse(urlPart)
		if err != nil {
			return 0
		}
		page := parsed.Query().Get("page")
		if page == "" {
			return 0
		}
		value, err := strconv.Atoi(page)
		if err != nil {
			return 0
		}
		return value
	}
	return 0
}

func formatTime(ts *time.Time) string {
	if ts == nil || ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339)
}

func isAuthMissing(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden {
			return true
		}
	}
	return strings.Contains(err.Error(), "authentication token not found")
}

func isHTTPStatus(err error, status int) bool {
	if err == nil {
		return false
	}
	var httpErr *api.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == status
}

func wrapAuthError(err error) error {
	if err == nil {
		return nil
	}
	if isAuthMissing(err) {
		return AuthRequiredError{Message: "GitHub authentication required"}
	}
	return err
}
