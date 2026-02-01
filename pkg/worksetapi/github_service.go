package worksetapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v75/github"
	"github.com/strantalis/workset/internal/workspace"
)

const (
	defaultGitHubHost = "github.com"
	defaultDiffLimit  = 120000
)

// CreatePullRequest opens a pull request against the resolved upstream repo.
func (s *Service) CreatePullRequest(ctx context.Context, input PullRequestCreateInput) (PullRequestCreateResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return PullRequestCreateResult{}, err
	}
	if strings.TrimSpace(input.Title) == "" {
		return PullRequestCreateResult{}, ValidationError{Message: "title required"}
	}

	headInfo, baseInfo, err := s.resolveRemoteInfo(ctx, resolution, input.BaseRemote)
	if err != nil {
		return PullRequestCreateResult{}, err
	}
	client, err := s.githubClient(ctx, baseInfo.Host)
	if err != nil {
		return PullRequestCreateResult{}, err
	}

	baseBranch := strings.TrimSpace(input.Base)
	if baseBranch == "" {
		baseBranch, err = s.resolveDefaultBranch(ctx, client, baseInfo, resolution)
		if err != nil {
			return PullRequestCreateResult{}, err
		}
	}
	headBranch := strings.TrimSpace(input.Head)
	if headBranch == "" {
		headBranch, err = s.resolveCurrentBranch(resolution)
		if err != nil {
			return PullRequestCreateResult{}, err
		}
	}
	headBranch = strings.TrimPrefix(headBranch, "refs/heads/")
	if strings.Contains(headBranch, ":") {
		return PullRequestCreateResult{}, ValidationError{Message: "head should be a branch name without an owner prefix"}
	}
	if input.AutoCommit {
		if err := s.commitPullRequestChanges(ctx, resolution, headBranch); err != nil {
			return PullRequestCreateResult{}, err
		}
	}
	if input.AutoPush {
		if err := gitPushBranch(ctx, resolution.RepoPath, headInfo.Remote, headBranch, s.commands); err != nil {
			return PullRequestCreateResult{}, err
		}
	} else {
		exists, err := remoteBranchExists(ctx, resolution.RepoPath, headInfo.Remote, headBranch, s.commands)
		if err != nil {
			return PullRequestCreateResult{}, err
		}
		if !exists {
			return PullRequestCreateResult{}, ValidationError{Message: fmt.Sprintf("head branch %q not found on remote %q; push it first: git push -u %s %s", headBranch, headInfo.Remote, headInfo.Remote, headBranch)}
		}
	}

	headRef := fmt.Sprintf("%s:%s", headInfo.Owner, headBranch)
	pr, err := client.CreatePullRequest(ctx, baseInfo.Owner, baseInfo.Repo, GitHubNewPullRequest{
		Title: input.Title,
		Head:  headRef,
		Base:  baseBranch,
		Body:  strings.TrimSpace(input.Body),
		Draft: input.Draft,
	})
	if err != nil {
		if isInvalidHeadError(err) {
			return PullRequestCreateResult{}, ValidationError{Message: fmt.Sprintf("GitHub rejected head %q; ensure the branch exists on %s/%s and that remote %q points to your fork", headRef, headInfo.Owner, headInfo.Repo, headInfo.Remote)}
		}
		return PullRequestCreateResult{}, ValidationError{Message: formatGitHubAPIError(err)}
	}

	payload := PullRequestCreatedJSON{
		Repo:       resolution.Repo.Name,
		Number:     pr.Number,
		URL:        pr.URL,
		Title:      pr.Title,
		Body:       pr.Body,
		Draft:      pr.Draft,
		State:      pr.State,
		BaseRepo:   fmt.Sprintf("%s/%s", baseInfo.Owner, baseInfo.Repo),
		BaseBranch: baseBranch,
		HeadRepo:   fmt.Sprintf("%s/%s", headInfo.Owner, headInfo.Repo),
		HeadBranch: headBranch,
	}
	s.recordPullRequest(ctx, resolution, payload)
	return PullRequestCreateResult{Payload: payload, Config: resolution.ConfigInfo}, nil
}

// GetPullRequestStatus returns the PR summary and checks.
func (s *Service) GetPullRequestStatus(ctx context.Context, input PullRequestStatusInput) (PullRequestStatusResult, error) {
	pr, headInfo, baseInfo, client, resolution, err := s.resolvePullRequest(ctx, input)
	if err != nil {
		return PullRequestStatusResult{}, err
	}

	mergeable := ""
	if pr.Mergeable != nil {
		switch {
		case *pr.Mergeable:
			mergeable = "mergeable"
		default:
			mergeable = "conflicts"
		}
	}

	checks, err := s.listCheckRuns(ctx, client, baseInfo, pr)
	if err != nil {
		return PullRequestStatusResult{}, err
	}

	status := PullRequestStatusJSON{
		Repo:       resolution.Repo.Name,
		Number:     pr.Number,
		URL:        pr.URL,
		Title:      pr.Title,
		State:      pr.State,
		Draft:      pr.Draft,
		BaseRepo:   fmt.Sprintf("%s/%s", baseInfo.Owner, baseInfo.Repo),
		BaseBranch: pr.BaseRef,
		HeadRepo:   fmt.Sprintf("%s/%s", headInfo.Owner, headInfo.Repo),
		HeadBranch: pr.HeadRef,
		Mergeable:  mergeable,
	}

	return PullRequestStatusResult{
		PullRequest: status,
		Checks:      checks,
		Config:      resolution.ConfigInfo,
	}, nil
}

// GetTrackedPullRequest returns the last recorded PR for a repo.
func (s *Service) GetTrackedPullRequest(ctx context.Context, input PullRequestTrackedInput) (PullRequestTrackedResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput(input))
	if err != nil {
		return PullRequestTrackedResult{}, err
	}
	state, err := s.workspaces.LoadState(ctx, resolution.WorkspaceRoot)
	if err != nil {
		return PullRequestTrackedResult{}, err
	}
	pr, ok := state.PullRequests[resolution.Repo.Name]
	if !ok {
		return PullRequestTrackedResult{
			Payload: PullRequestTrackedJSON{Found: false},
			Config:  resolution.ConfigInfo,
		}, nil
	}
	return PullRequestTrackedResult{
		Payload: PullRequestTrackedJSON{
			Found: true,
			PullRequest: PullRequestCreatedJSON{
				Repo:       pr.Repo,
				Number:     pr.Number,
				URL:        pr.URL,
				Title:      pr.Title,
				Body:       pr.Body,
				Draft:      pr.Draft,
				State:      pr.State,
				BaseRepo:   pr.BaseRepo,
				BaseBranch: pr.BaseBranch,
				HeadRepo:   pr.HeadRepo,
				HeadBranch: pr.HeadBranch,
			},
		},
		Config: resolution.ConfigInfo,
	}, nil
}

// ListPullRequestReviewComments returns review comments for a PR.
func (s *Service) ListPullRequestReviewComments(ctx context.Context, input PullRequestReviewsInput) (PullRequestReviewCommentsResult, error) {
	statusInput := PullRequestStatusInput(input)
	pr, _, baseInfo, client, resolution, err := s.resolvePullRequest(ctx, statusInput)
	if err != nil {
		return PullRequestReviewCommentsResult{}, err
	}

	threadMap := map[string]threadInfo{}
	if client != nil {
		if mapResult, err := client.ReviewThreadMap(ctx, baseInfo.Owner, baseInfo.Repo, pr.Number); err == nil {
			threadMap = mapResult
		}
	}

	comments := make([]PullRequestReviewCommentJSON, 0)
	page := 1
	for {
		pageComments, nextPage, err := client.ListReviewComments(ctx, baseInfo.Owner, baseInfo.Repo, pr.Number, page, 100)
		if err != nil {
			return PullRequestReviewCommentsResult{}, err
		}
		for _, comment := range pageComments {
			mapped := comment
			if mapped.NodeID != "" {
				if info, ok := threadMap[mapped.NodeID]; ok {
					mapped.ThreadID = info.ThreadID
					mapped.Resolved = info.IsResolved
				}
			}
			comments = append(comments, mapped)
		}
		if nextPage == 0 {
			break
		}
		page = nextPage
	}

	return PullRequestReviewCommentsResult{
		Comments: comments,
		Config:   resolution.ConfigInfo,
	}, nil
}

// ReplyToReviewComment creates a reply to an existing review comment.
func (s *Service) ReplyToReviewComment(ctx context.Context, input ReplyToReviewCommentInput) (ReviewCommentResult, error) {
	if input.CommentID <= 0 {
		return ReviewCommentResult{}, ValidationError{Message: "comment ID required"}
	}
	if strings.TrimSpace(input.Body) == "" {
		return ReviewCommentResult{}, ValidationError{Message: "body required"}
	}

	statusInput := PullRequestStatusInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
		Number:    input.Number,
		Branch:    input.Branch,
	}
	pr, _, baseInfo, client, resolution, err := s.resolvePullRequest(ctx, statusInput)
	if err != nil {
		return ReviewCommentResult{}, err
	}

	comment, err := client.CreateReplyComment(
		ctx,
		baseInfo.Owner,
		baseInfo.Repo,
		pr.Number,
		input.CommentID,
		strings.TrimSpace(input.Body),
	)
	if err != nil {
		return ReviewCommentResult{}, ValidationError{Message: formatGitHubAPIError(err)}
	}

	return ReviewCommentResult{
		Comment: comment,
		Config:  resolution.ConfigInfo,
	}, nil
}

// EditReviewComment updates an existing review comment.
func (s *Service) EditReviewComment(ctx context.Context, input EditReviewCommentInput) (ReviewCommentResult, error) {
	if input.CommentID <= 0 {
		return ReviewCommentResult{}, ValidationError{Message: "comment ID required"}
	}
	if strings.TrimSpace(input.Body) == "" {
		return ReviewCommentResult{}, ValidationError{Message: "body required"}
	}

	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return ReviewCommentResult{}, err
	}

	_, baseInfo, err := s.resolveRemoteInfo(ctx, resolution, "")
	if err != nil {
		return ReviewCommentResult{}, err
	}

	client, err := s.githubClient(ctx, baseInfo.Host)
	if err != nil {
		return ReviewCommentResult{}, err
	}

	comment, err := client.EditReviewComment(
		ctx,
		baseInfo.Owner,
		baseInfo.Repo,
		input.CommentID,
		strings.TrimSpace(input.Body),
	)
	if err != nil {
		return ReviewCommentResult{}, ValidationError{Message: formatGitHubAPIError(err)}
	}

	return ReviewCommentResult{
		Comment: comment,
		Config:  resolution.ConfigInfo,
	}, nil
}

// DeleteReviewComment deletes a review comment.
func (s *Service) DeleteReviewComment(ctx context.Context, input DeleteReviewCommentInput) (DeleteReviewCommentResult, error) {
	if input.CommentID <= 0 {
		return DeleteReviewCommentResult{}, ValidationError{Message: "comment ID required"}
	}

	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return DeleteReviewCommentResult{}, err
	}

	_, baseInfo, err := s.resolveRemoteInfo(ctx, resolution, "")
	if err != nil {
		return DeleteReviewCommentResult{}, err
	}

	client, err := s.githubClient(ctx, baseInfo.Host)
	if err != nil {
		return DeleteReviewCommentResult{}, err
	}

	err = client.DeleteReviewComment(ctx, baseInfo.Owner, baseInfo.Repo, input.CommentID)
	if err != nil {
		return DeleteReviewCommentResult{}, ValidationError{Message: formatGitHubAPIError(err)}
	}

	return DeleteReviewCommentResult{
		Success: true,
		Config:  resolution.ConfigInfo,
	}, nil
}

// ResolveReviewThread resolves or unresolves a review thread using GraphQL.
func (s *Service) ResolveReviewThread(ctx context.Context, input ResolveReviewThreadInput) (ResolveReviewThreadResult, error) {
	threadID := strings.TrimSpace(input.ThreadID)
	if threadID == "" {
		return ResolveReviewThreadResult{}, ValidationError{Message: "thread ID required"}
	}

	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return ResolveReviewThreadResult{}, err
	}

	_, baseInfo, err := s.resolveRemoteInfo(ctx, resolution, "")
	if err != nil {
		return ResolveReviewThreadResult{}, err
	}

	client, err := s.githubClient(ctx, baseInfo.Host)
	if err != nil {
		return ResolveReviewThreadResult{}, err
	}

	if strings.HasPrefix(threadID, "PRRC_") {
		resolvedThreadID, err := client.GetReviewThreadID(ctx, threadID)
		if err != nil {
			return ResolveReviewThreadResult{}, err
		}
		threadID = resolvedThreadID
	}

	resolved, err := client.ResolveReviewThread(ctx, threadID, input.Resolve)
	if err != nil {
		return ResolveReviewThreadResult{}, err
	}

	return ResolveReviewThreadResult{
		Resolved: resolved,
		Config:   resolution.ConfigInfo,
	}, nil
}

// graphQLResolveThread calls the GitHub GraphQL API to resolve/unresolve a thread by node ID.
func graphQLResolveThread(ctx context.Context, token, host, threadID string, resolve bool) (bool, error) {
	endpoint := "https://api.github.com/graphql"
	if host != "" && host != defaultGitHubHost {
		endpoint = fmt.Sprintf("https://%s/api/graphql", host)
	}

	mutation := "resolveReviewThread"
	if !resolve {
		mutation = "unresolveReviewThread"
	}

	query := fmt.Sprintf(`mutation {
		%s(input: {threadId: %q}) {
			thread {
				isResolved
			}
		}
	}`, mutation, threadID)

	payload := map[string]string{"query": query}
	body, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	if resp.StatusCode != http.StatusOK {
		return false, ValidationError{Message: "GraphQL request failed: " + string(respBody)}
	}

	var result struct {
		Data struct {
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
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return false, err
	}

	if len(result.Errors) > 0 {
		return false, ValidationError{Message: result.Errors[0].Message}
	}

	if resolve {
		return result.Data.ResolveReviewThread.Thread.IsResolved, nil
	}
	return result.Data.UnresolveReviewThread.Thread.IsResolved, nil
}

// threadInfo holds thread ID and resolved state for a comment.
type threadInfo struct {
	ThreadID   string
	IsResolved bool
}

// graphQLReviewThreadMap fetches review threads for a PR and maps comment node IDs to thread info.
func graphQLReviewThreadMap(ctx context.Context, token, host, owner, repo string, number int) (map[string]threadInfo, error) {
	endpoint := "https://api.github.com/graphql"
	if host != "" && host != defaultGitHubHost {
		endpoint = fmt.Sprintf("https://%s/api/graphql", host)
	}

	threadMap := make(map[string]threadInfo)
	var cursor *string

	for {
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

		variables := map[string]any{
			"owner":  owner,
			"repo":   repo,
			"number": number,
			"after":  cursor,
		}

		payload := map[string]any{
			"query":     query,
			"variables": variables,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, ValidationError{Message: "GraphQL request failed: " + string(respBody)}
		}

		var result struct {
			Data struct {
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
			} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}

		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, err
		}

		if len(result.Errors) > 0 {
			return nil, ValidationError{Message: result.Errors[0].Message}
		}

		for _, thread := range result.Data.Repository.PullRequest.ReviewThreads.Nodes {
			for _, comment := range thread.Comments.Nodes {
				if comment.ID != "" && thread.ID != "" {
					threadMap[comment.ID] = threadInfo{
						ThreadID:   thread.ID,
						IsResolved: thread.IsResolved,
					}
				}
			}
		}

		if !result.Data.Repository.PullRequest.ReviewThreads.PageInfo.HasNextPage {
			break
		}
		next := result.Data.Repository.PullRequest.ReviewThreads.PageInfo.EndCursor
		if strings.TrimSpace(next) == "" {
			break
		}
		cursor = &next
	}

	return threadMap, nil
}

// graphQLGetThreadID fetches the thread ID for a comment node ID by querying through the PR.
func graphQLGetThreadID(ctx context.Context, endpoint, token, commentNodeID string) (string, error) {
	// Query the comment to get its PR, then find the thread containing this comment
	query := fmt.Sprintf(`query {
		node(id: %q) {
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
	}`, commentNodeID)

	payload := map[string]string{"query": query}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", ValidationError{Message: "GraphQL request failed: " + string(respBody)}
	}

	var result struct {
		Data struct {
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
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if len(result.Errors) > 0 {
		return "", ValidationError{Message: result.Errors[0].Message}
	}

	// Find the thread that contains this comment
	for _, thread := range result.Data.Node.PullRequest.ReviewThreads.Nodes {
		for _, comment := range thread.Comments.Nodes {
			if comment.ID == commentNodeID {
				return thread.ID, nil
			}
		}
	}

	return "", ValidationError{Message: "could not find thread for comment"}
}

// GetCurrentGitHubUser returns the authenticated GitHub user.
func (s *Service) GetCurrentGitHubUser(ctx context.Context, input GitHubUserInput) (GitHubUserResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput(input))
	if err != nil {
		return GitHubUserResult{}, err
	}

	_, baseInfo, err := s.resolveRemoteInfo(ctx, resolution, "")
	if err != nil {
		return GitHubUserResult{}, err
	}

	client, err := s.githubClient(ctx, baseInfo.Host)
	if err != nil {
		return GitHubUserResult{}, err
	}

	user, _, err := client.GetCurrentUser(ctx)
	if err != nil {
		return GitHubUserResult{}, ValidationError{Message: formatGitHubAPIError(err)}
	}

	return GitHubUserResult{
		User:   user,
		Config: resolution.ConfigInfo,
	}, nil
}

// GeneratePullRequestText runs the default agent to propose a title/body.
func (s *Service) GeneratePullRequestText(ctx context.Context, input PullRequestGenerateInput) (PullRequestGenerateResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return PullRequestGenerateResult{}, err
	}
	headBranch, err := s.resolveCurrentBranch(resolution)
	if err != nil {
		return PullRequestGenerateResult{}, err
	}

	diffLimit := input.MaxDiffBytes
	if diffLimit <= 0 {
		diffLimit = defaultDiffLimit
	}
	patch, err := buildRepoPatch(ctx, resolution.RepoPath, diffLimit, s.commands)
	if err != nil {
		return PullRequestGenerateResult{}, err
	}
	if strings.TrimSpace(patch) == "" {
		return PullRequestGenerateResult{}, ValidationError{Message: "no diff available to summarize"}
	}

	agent := strings.TrimSpace(resolution.Defaults.Agent)
	if agent == "" {
		return PullRequestGenerateResult{}, ValidationError{Message: "defaults.agent is not configured"}
	}

	prompt := formatPRPrompt(resolution.Repo.Name, headBranch, patch)
	result, err := s.runAgentPrompt(ctx, resolution.RepoPath, agent, prompt)
	if err != nil {
		return PullRequestGenerateResult{}, err
	}
	return PullRequestGenerateResult{Payload: result, Config: resolution.ConfigInfo}, nil
}

// GetRepoLocalStatus returns the local uncommitted/ahead/behind status for a repo.
func (s *Service) GetRepoLocalStatus(ctx context.Context, input RepoLocalStatusInput) (RepoLocalStatusResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput(input))
	if err != nil {
		return RepoLocalStatusResult{}, err
	}

	// Get current branch
	branch, err := s.resolveCurrentBranch(resolution)
	if err != nil {
		return RepoLocalStatusResult{}, err
	}

	// Check for uncommitted changes via git status --porcelain
	hasUncommitted, err := gitHasUncommittedChanges(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		return RepoLocalStatusResult{}, err
	}

	// Get ahead/behind counts
	ahead, behind, err := gitAheadBehind(ctx, resolution.RepoPath, branch, s.commands)
	if err != nil {
		// Non-fatal: upstream tracking may not be configured
		ahead, behind = 0, 0
	}

	return RepoLocalStatusResult{
		Payload: RepoLocalStatusJSON{
			HasUncommitted: hasUncommitted,
			Ahead:          ahead,
			Behind:         behind,
			CurrentBranch:  branch,
		},
		Config: resolution.ConfigInfo,
	}, nil
}

// ListRemotes returns the list of git remotes for a repo with owner/repo info.
func (s *Service) ListRemotes(ctx context.Context, input ListRemotesInput) (ListRemotesResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput(input))
	if err != nil {
		return ListRemotesResult{}, err
	}

	remoteNames, err := s.git.RemoteNames(resolution.RepoPath)
	if err != nil {
		return ListRemotesResult{}, err
	}

	remotes := make([]RemoteInfoJSON, 0, len(remoteNames))
	for _, name := range remoteNames {
		info, err := s.remoteInfoFor(resolution.RepoPath, name)
		if err != nil {
			// Skip remotes that can't be parsed (e.g., non-GitHub remotes)
			continue
		}
		remotes = append(remotes, RemoteInfoJSON{
			Name:  name,
			Owner: info.Owner,
			Repo:  info.Repo,
		})
	}

	return ListRemotesResult{Remotes: remotes}, nil
}

// CommitAndPush commits all changes and pushes to the remote.
func (s *Service) CommitAndPush(ctx context.Context, input CommitAndPushInput) (CommitAndPushResult, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	}) // Cannot use type conversion here due to Message field
	if err != nil {
		return CommitAndPushResult{}, err
	}

	branch, err := s.resolveCurrentBranch(resolution)
	if err != nil {
		return CommitAndPushResult{}, err
	}

	// Check for changes to commit
	hasUncommitted, err := gitHasUncommittedChanges(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		return CommitAndPushResult{}, err
	}
	if !hasUncommitted {
		return CommitAndPushResult{}, ValidationError{Message: "no changes to commit"}
	}

	// Generate commit message if not provided
	message := strings.TrimSpace(input.Message)
	if message == "" {
		agent := strings.TrimSpace(resolution.Defaults.Agent)
		if agent == "" {
			return CommitAndPushResult{}, ValidationError{Message: "defaults.agent is not configured; cannot auto-generate commit message"}
		}
		patch, err := buildRepoPatch(ctx, resolution.RepoPath, defaultDiffLimit, s.commands)
		if err != nil {
			return CommitAndPushResult{}, err
		}
		prompt := formatCommitPrompt(resolution.Repo.Name, branch, patch)
		schema, err := ensureCommitSchema()
		if err != nil {
			return CommitAndPushResult{}, err
		}
		output, err := s.runAgentPromptRaw(ctx, resolution.RepoPath, agent, prompt, schema)
		if err != nil {
			return CommitAndPushResult{}, err
		}
		message, err = parseCommitJSON(output)
		if err != nil {
			return CommitAndPushResult{}, err
		}
	}

	// Stage all changes
	if err := gitAddAll(ctx, resolution.RepoPath, s.commands); err != nil {
		return CommitAndPushResult{}, err
	}

	// Verify staged changes exist
	hasStaged, err := gitHasStagedChanges(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		return CommitAndPushResult{}, err
	}
	if !hasStaged {
		return CommitAndPushResult{}, ValidationError{Message: "no changes staged after git add"}
	}

	// Commit
	if err := gitCommitMessage(ctx, resolution.RepoPath, message, s.commands); err != nil {
		return CommitAndPushResult{}, err
	}

	// Get the new commit SHA
	sha, err := gitHeadSHA(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		sha = ""
	}

	// Resolve remote for push
	headInfo, _, err := s.resolveRemoteInfo(ctx, resolution, "")
	if err != nil {
		return CommitAndPushResult{
			Payload: CommitAndPushResultJSON{
				Committed: true,
				Pushed:    false,
				Message:   message,
				SHA:       sha,
			},
			Config: resolution.ConfigInfo,
		}, err
	}

	// Push
	if err := gitPushBranch(ctx, resolution.RepoPath, headInfo.Remote, branch, s.commands); err != nil {
		return CommitAndPushResult{
			Payload: CommitAndPushResultJSON{
				Committed: true,
				Pushed:    false,
				Message:   message,
				SHA:       sha,
			},
			Config: resolution.ConfigInfo,
		}, err
	}

	return CommitAndPushResult{
		Payload: CommitAndPushResultJSON{
			Committed: true,
			Pushed:    true,
			Message:   message,
			SHA:       sha,
		},
		Config: resolution.ConfigInfo,
	}, nil
}

func (s *Service) resolveRemoteInfo(ctx context.Context, resolution repoResolution, baseRemoteOverride string) (remoteInfo, remoteInfo, error) {
	headRemote := strings.TrimSpace(resolution.RepoDefaults.Remote)
	if headRemote == "" {
		headRemote = "origin"
	}
	headInfo, err := s.remoteInfoFor(resolution.RepoPath, headRemote)
	if err != nil {
		return remoteInfo{}, remoteInfo{}, err
	}
	baseRemote := strings.TrimSpace(baseRemoteOverride)
	if baseRemote == "" {
		// Auto-detect: use upstream if it exists, otherwise use head remote
		baseRemote = headRemote
		if exists, err := s.git.RemoteExists(resolution.RepoPath, "upstream"); err == nil && exists {
			baseRemote = "upstream"
		}
	}
	baseInfo, err := s.remoteInfoFor(resolution.RepoPath, baseRemote)
	if err != nil {
		return remoteInfo{}, remoteInfo{}, err
	}
	if headInfo.Host != baseInfo.Host {
		return remoteInfo{}, remoteInfo{}, ValidationError{Message: "head and base remotes must share the same GitHub host"}
	}
	if headInfo.Host != defaultGitHubHost {
		return remoteInfo{}, remoteInfo{}, ValidationError{Message: fmt.Sprintf("unsupported GitHub host %q: only github.com is supported in this release", headInfo.Host)}
	}
	return headInfo, baseInfo, nil
}

func (s *Service) remoteInfoFor(repoPath, remoteName string) (remoteInfo, error) {
	urls, err := s.git.RemoteURLs(repoPath, remoteName)
	if err != nil {
		return remoteInfo{}, err
	}
	if len(urls) == 0 {
		return remoteInfo{}, ValidationError{Message: fmt.Sprintf("remote %q has no URL configured", remoteName)}
	}
	info, err := parseGitHubRemoteURL(urls[0])
	if err != nil {
		return remoteInfo{}, err
	}
	info.Remote = remoteName
	info.URL = urls[0]
	if info.Host == "" {
		info.Host = defaultGitHubHost
	}
	return info, nil
}

func (s *Service) resolveCurrentBranch(resolution repoResolution) (string, error) {
	branch, ok, err := s.git.CurrentBranch(resolution.RepoPath)
	if err != nil {
		return "", err
	}
	if !ok || strings.TrimSpace(branch) == "" {
		if resolution.Branch != "" {
			return resolution.Branch, nil
		}
		return "", ValidationError{Message: "unable to resolve current branch"}
	}
	return branch, nil
}

func (s *Service) githubClient(ctx context.Context, host string) (GitHubClient, error) {
	if s.github == nil {
		return nil, AuthRequiredError{Message: "GitHub authentication required"}
	}
	if err := s.importGitHubPATFromEnv(ctx); err != nil {
		return nil, err
	}
	return s.github.Client(ctx, host)
}

func (s *Service) resolveDefaultBranch(ctx context.Context, client GitHubClient, base remoteInfo, resolution repoResolution) (string, error) {
	branch, err := client.GetRepoDefaultBranch(ctx, base.Owner, base.Repo)
	if err == nil && strings.TrimSpace(branch) != "" {
		return branch, nil
	}
	if resolution.RepoDefaults.DefaultBranch != "" {
		return resolution.RepoDefaults.DefaultBranch, nil
	}
	return "", ValidationError{Message: "base branch required"}
}

func (s *Service) resolvePullRequest(ctx context.Context, input PullRequestStatusInput) (GitHubPullRequest, remoteInfo, remoteInfo, GitHubClient, repoResolution, error) {
	resolution, err := s.resolveRepo(ctx, RepoSelectionInput{
		Workspace: input.Workspace,
		Repo:      input.Repo,
	})
	if err != nil {
		return GitHubPullRequest{}, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}
	headInfo, baseInfo, err := s.resolveRemoteInfo(ctx, resolution, "")
	if err != nil {
		return GitHubPullRequest{}, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}
	client, err := s.githubClient(ctx, baseInfo.Host)
	if err != nil {
		return GitHubPullRequest{}, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}

	number := input.Number
	if number == 0 {
		branch := strings.TrimSpace(input.Branch)
		if branch == "" {
			branch, err = s.resolveCurrentBranch(resolution)
			if err != nil {
				return GitHubPullRequest{}, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
			}
		}
		number, err = s.findPullRequestNumber(ctx, client, baseInfo, headInfo, branch)
		if err != nil {
			return GitHubPullRequest{}, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
		}
	}
	pr, err := client.GetPullRequest(ctx, baseInfo.Owner, baseInfo.Repo, number)
	if err != nil {
		return GitHubPullRequest{}, remoteInfo{}, remoteInfo{}, nil, repoResolution{}, err
	}
	return pr, headInfo, baseInfo, client, resolution, nil
}

func (s *Service) findPullRequestNumber(ctx context.Context, client GitHubClient, base remoteInfo, head remoteInfo, branch string) (int, error) {
	headRef := fmt.Sprintf("%s:%s", head.Owner, branch)
	page := 1
	for {
		prs, next, err := client.ListPullRequests(ctx, base.Owner, base.Repo, headRef, "open", page, 50)
		if err != nil {
			return 0, err
		}
		if len(prs) > 0 {
			return prs[0].Number, nil
		}
		if next == 0 {
			break
		}
		page = next
	}
	return 0, NotFoundError{Message: "pull request not found for current branch"}
}

func (s *Service) listCheckRuns(ctx context.Context, client GitHubClient, base remoteInfo, pr GitHubPullRequest) ([]PullRequestCheckJSON, error) {
	sha := pr.HeadSHA
	if sha == "" {
		return nil, nil
	}
	checks := make([]PullRequestCheckJSON, 0)
	page := 1
	for {
		pageChecks, next, err := client.ListCheckRuns(ctx, base.Owner, base.Repo, sha, page, 100)
		if err != nil {
			return nil, err
		}
		checks = append(checks, pageChecks...)
		if next == 0 {
			break
		}
		page = next
	}
	return checks, nil
}

func formatPRPrompt(repoName, branch, patch string) string {
	builder := strings.Builder{}
	builder.WriteString("Generate a pull request title and body based on this diff.\n")
	builder.WriteString("Return JSON only: {\"title\":\"...\",\"body\":\"...\"}.\n")
	builder.WriteString(fmt.Sprintf("Repo: %s\n", repoName))
	builder.WriteString(fmt.Sprintf("Branch: %s\n\n", branch))
	builder.WriteString("Diff:\n")
	builder.WriteString(patch)
	builder.WriteString("\n")
	return builder.String()
}

func formatCommitPrompt(repoName, branch, patch string) string {
	builder := strings.Builder{}
	builder.WriteString("Generate a conventional commit message for this diff.\n")
	builder.WriteString("Use format: type(scope): subject. Keep it concise.\n")
	builder.WriteString("Return JSON only: {\"message\":\"...\"}.\n")
	builder.WriteString(fmt.Sprintf("Repo: %s\n", repoName))
	builder.WriteString(fmt.Sprintf("Branch: %s\n\n", branch))
	builder.WriteString("Diff:\n")
	builder.WriteString(patch)
	builder.WriteString("\n")
	return builder.String()
}

func (s *Service) runAgentPrompt(ctx context.Context, repoPath, agent, prompt string) (PullRequestGeneratedJSON, error) {
	schema, err := ensurePRSchema()
	if err != nil {
		return PullRequestGeneratedJSON{}, err
	}
	output, err := s.runAgentPromptRaw(ctx, repoPath, agent, prompt, schema)
	if err != nil {
		return PullRequestGeneratedJSON{}, err
	}
	return parseAgentJSON(output)
}

func (s *Service) runAgentPromptRaw(ctx context.Context, repoPath, agent, prompt, schema string) (string, error) {
	command := strings.Fields(agent)
	if len(command) == 0 {
		return "", ValidationError{Message: "agent command required"}
	}
	if configuredPath, err := s.agentCLIPathFromConfig(ctx); err != nil {
		return "", err
	} else if configuredPath != "" && isExecutableCandidate(configuredPath) {
		if filepath.Base(configuredPath) == filepath.Base(command[0]) {
			command[0] = configuredPath
		}
	}
	command, env, stdin, err := prepareAgentCommand(command, prompt, schema)
	if err != nil {
		return "", err
	}
	if wrapped, ok := wrapAgentCommandForShell(command); ok {
		command = wrapped
	}
	result, err := s.commands(ctx, repoPath, command, env, stdin)
	if err != nil || result.ExitCode != 0 {
		if shouldRetryWithPTY(err, result) {
			ptyResult, ptyErr := runCommandWithPTY(ctx, repoPath, command, env, stdin)
			if ptyErr == nil && ptyResult.ExitCode == 0 {
				return ptyResult.Stdout, nil
			}
			if ptyErr != nil && err == nil {
				err = ptyErr
			}
			if ptyResult.ExitCode != 0 && ptyResult.Stdout != "" {
				result = ptyResult
			}
		}
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			message = "agent command not found: " + command[0]
		} else if message == "" {
			message = "agent command failed"
		}
		return "", ValidationError{Message: message}
	}
	return result.Stdout, nil
}

func wrapAgentCommandForShell(command []string) ([]string, bool) {
	if runtime.GOOS == "windows" || len(command) == 0 {
		return command, false
	}
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}
	shellBase := strings.ToLower(filepath.Base(shell))
	commandLine := shellJoinArgs(command)
	args := shellArgsFor(shellBase, commandLine)
	return append([]string{shell}, args...), true
}

func shellArgsFor(shellBase, command string) []string {
	switch shellBase {
	case "fish", "csh", "tcsh":
		return []string{"-l", "-c", command}
	default:
		return []string{"-lc", command}
	}
}

func shellJoinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, shellEscape(arg))
	}
	return strings.Join(parts, " ")
}

func shellEscape(value string) string {
	if value == "" {
		return "''"
	}
	escaped := strings.ReplaceAll(value, "'", `'"'"'`)
	return "'" + escaped + "'"
}

func parseAgentJSON(output string) (PullRequestGeneratedJSON, error) {
	output = strings.TrimSpace(stripANSI(output))
	if output == "" {
		return PullRequestGeneratedJSON{}, ValidationError{Message: "agent returned empty output"}
	}
	payload, err := decodeJSON(output)
	if err != nil {
		return PullRequestGeneratedJSON{}, err
	}
	if strings.TrimSpace(payload.Title) == "" {
		return PullRequestGeneratedJSON{}, ValidationError{Message: "agent output missing title"}
	}
	return payload, nil
}

func parseCommitJSON(output string) (string, error) {
	output = strings.TrimSpace(stripANSI(output))
	if output == "" {
		return "", ValidationError{Message: "agent returned empty output"}
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		re := regexp.MustCompile(`\{[\s\S]*\}`)
		match := re.FindString(output)
		if match == "" {
			return "", ValidationError{Message: "unable to parse agent JSON output"}
		}
		if err := json.Unmarshal([]byte(match), &payload); err != nil {
			return "", ValidationError{Message: "invalid agent JSON output"}
		}
	}
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		return "", ValidationError{Message: "agent output missing commit message"}
	}
	return message, nil
}

func decodeJSON(output string) (PullRequestGeneratedJSON, error) {
	var payload PullRequestGeneratedJSON
	if err := json.Unmarshal([]byte(output), &payload); err == nil {
		return payload, nil
	}
	re := regexp.MustCompile(`\{[\s\S]*\}`)
	match := re.FindString(output)
	if match == "" {
		return PullRequestGeneratedJSON{}, ValidationError{Message: "unable to parse agent JSON output"}
	}
	if err := json.Unmarshal([]byte(match), &payload); err != nil {
		return PullRequestGeneratedJSON{}, ValidationError{Message: "invalid agent JSON output"}
	}
	return payload, nil
}

func stripANSI(value string) string {
	if value == "" {
		return value
	}
	re := regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	return re.ReplaceAllString(value, "")
}

func isStdinNotTerminal(err error, result CommandResult) bool {
	message := strings.ToLower(strings.TrimSpace(result.Stderr))
	if message == "" {
		message = strings.ToLower(strings.TrimSpace(result.Stdout))
	}
	if err != nil && message == "" {
		message = strings.ToLower(err.Error())
	}
	return strings.Contains(message, "stdin is not a terminal") || strings.Contains(message, "not a tty")
}

func shouldRetryWithPTY(err error, result CommandResult) bool {
	if isStdinNotTerminal(err, result) {
		return true
	}
	if err == nil || result.ExitCode == 0 {
		return false
	}
	output := strings.TrimSpace(result.Stdout + result.Stderr)
	return output == ""
}

func (s *Service) commitPullRequestChanges(ctx context.Context, resolution repoResolution, branch string) error {
	agent := strings.TrimSpace(resolution.Defaults.Agent)
	if agent == "" {
		return ValidationError{Message: "defaults.agent is not configured"}
	}
	diffLimit := defaultDiffLimit
	patch, err := buildRepoPatch(ctx, resolution.RepoPath, diffLimit, s.commands)
	if err != nil {
		return err
	}
	if strings.TrimSpace(patch) == "" {
		return nil
	}
	prompt := formatCommitPrompt(resolution.Repo.Name, branch, patch)
	schema, err := ensureCommitSchema()
	if err != nil {
		return err
	}
	output, err := s.runAgentPromptRaw(ctx, resolution.RepoPath, agent, prompt, schema)
	if err != nil {
		return err
	}
	message, err := parseCommitJSON(output)
	if err != nil {
		return err
	}
	if err := gitAddAll(ctx, resolution.RepoPath, s.commands); err != nil {
		return err
	}
	hasStaged, err := gitHasStagedChanges(ctx, resolution.RepoPath, s.commands)
	if err != nil {
		return err
	}
	if !hasStaged {
		return ValidationError{Message: "no changes to commit"}
	}
	if err := gitCommitMessage(ctx, resolution.RepoPath, message, s.commands); err != nil {
		return err
	}
	return nil
}

func isInvalidHeadError(err error) bool {
	var ghErr *github.ErrorResponse
	if !errors.As(err, &ghErr) {
		return false
	}
	for _, entry := range ghErr.Errors {
		if strings.EqualFold(entry.Resource, "PullRequest") && strings.EqualFold(entry.Field, "head") && strings.EqualFold(entry.Code, "invalid") {
			return true
		}
	}
	return false
}

func formatGitHubAPIError(err error) string {
	var ghErr *github.ErrorResponse
	if !errors.As(err, &ghErr) {
		if err != nil {
			return err.Error()
		}
		return "GitHub API error"
	}
	details := make([]string, 0, len(ghErr.Errors))
	for _, entry := range ghErr.Errors {
		detail := strings.TrimSpace(entry.Message)
		if detail == "" {
			parts := []string{}
			if entry.Resource != "" {
				parts = append(parts, entry.Resource)
			}
			if entry.Field != "" {
				parts = append(parts, entry.Field)
			}
			if entry.Code != "" {
				parts = append(parts, entry.Code)
			}
			detail = strings.TrimSpace(strings.Join(parts, " "))
		}
		if detail != "" {
			details = append(details, detail)
		}
	}
	message := strings.TrimSpace(ghErr.Message)
	if len(details) == 0 {
		if message != "" {
			return message
		}
		if err != nil {
			return err.Error()
		}
		return "GitHub API error"
	}
	if message == "" {
		return strings.Join(details, "; ")
	}
	return fmt.Sprintf("%s (%s)", message, strings.Join(details, "; "))
}

var (
	prSchemaOnce     sync.Once
	prSchemaPath     string
	errPRSchema      error
	commitSchemaOnce sync.Once
	commitSchemaPath string
	errCommitSchema  error
)

func prepareAgentCommand(command []string, prompt string, schema string) ([]string, []string, string, error) {
	env := append(os.Environ(),
		"WORKSET_PR_PROMPT="+prompt,
		"WORKSET_PR_JSON=1",
	)
	if len(command) == 0 {
		return nil, nil, "", errors.New("agent command required")
	}
	if resolved := resolveCLIPath(command[0]); resolved != "" {
		command[0] = resolved
	}
	if filepath.Base(command[0]) != "codex" {
		return command, env, prompt, nil
	}
	if schema == "" {
		return nil, nil, "", errors.New("agent schema required")
	}

	args := command[1:]
	switch {
	case len(args) == 0 || strings.HasPrefix(args[0], "-"):
		args = append([]string{"exec"}, args...)
	case args[0] == "exec" || args[0] == "e":
		// ok
	default:
		// Any other subcommand should pass through unchanged.
		return command, env, prompt, nil
	}

	promptProvided := hasPromptArg(args)
	if !hasFlag(args, "--color") {
		args = append(args, "--color", "never")
	}
	if !hasFlag(args, "--output-schema") {
		args = append(args, "--output-schema", schema)
	}
	// In non-interactive mode, read the prompt from stdin.
	if !promptProvided {
		args = append(args, "-")
	}
	return append([]string{"codex"}, args...), env, prompt, nil
}

func hasFlag(args []string, name string) bool {
	for i := range args {
		arg := args[i]
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

func hasPromptArg(args []string) bool {
	sawExec := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if !sawExec && (arg == "exec" || arg == "e") {
			sawExec = true
			continue
		}
		if arg == "-" {
			return true
		}
		// first non-flag arg is prompt; treat it as present
		return true
	}
	return false
}

func ensurePRSchema() (string, error) {
	prSchemaOnce.Do(func() {
		path := filepath.Join(os.TempDir(), "workset-pr-schema.json")
		payload := `{"type":"object","properties":{"title":{"type":"string"},"body":{"type":"string"}},"required":["title","body"],"additionalProperties":false}`
		errPRSchema = os.WriteFile(path, []byte(payload), 0o644)
		if errPRSchema == nil {
			prSchemaPath = path
		}
	})
	return prSchemaPath, errPRSchema
}

func ensureCommitSchema() (string, error) {
	commitSchemaOnce.Do(func() {
		path := filepath.Join(os.TempDir(), "workset-commit-schema.json")
		payload := `{"type":"object","properties":{"message":{"type":"string"}},"required":["message"],"additionalProperties":false}`
		errCommitSchema = os.WriteFile(path, []byte(payload), 0o644)
		if errCommitSchema == nil {
			commitSchemaPath = path
		}
	})
	return commitSchemaPath, errCommitSchema
}

func gitAddAll(ctx context.Context, repoPath string, runner CommandRunner) error {
	result, err := runner(ctx, repoPath, []string{"git", "add", "-A"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "git add failed"
		}
		return ValidationError{Message: message}
	}
	return nil
}

func gitHasStagedChanges(ctx context.Context, repoPath string, runner CommandRunner) (bool, error) {
	result, err := runner(ctx, repoPath, []string{"git", "diff", "--cached", "--name-only"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "unable to check staged changes"
		}
		return false, ValidationError{Message: message}
	}
	return strings.TrimSpace(result.Stdout) != "", nil
}

func gitCommitMessage(ctx context.Context, repoPath, message string, runner CommandRunner) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return ValidationError{Message: "commit message required"}
	}
	result, err := runner(ctx, repoPath, []string{"git", "commit", "-m", message}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		msg := strings.TrimSpace(result.Stderr)
		if msg == "" && err != nil {
			msg = err.Error()
		}
		if msg == "" {
			msg = "git commit failed"
		}
		return ValidationError{Message: msg}
	}
	return nil
}

func gitPushBranch(ctx context.Context, repoPath, remote, branch string, runner CommandRunner) error {
	if strings.TrimSpace(remote) == "" {
		return ValidationError{Message: "remote name required to push head branch"}
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return ValidationError{Message: "head branch required"}
	}
	result, err := runner(ctx, repoPath, []string{"git", "push", "-u", remote, branch}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "git push failed"
		}
		return ValidationError{Message: message}
	}
	return nil
}

func (s *Service) recordPullRequest(ctx context.Context, resolution repoResolution, payload PullRequestCreatedJSON) {
	state, err := s.workspaces.LoadState(ctx, resolution.WorkspaceRoot)
	if err != nil {
		if s.logf != nil {
			s.logf("workset: unable to load workspace state for PR tracking: %v", err)
		}
		return
	}
	if state.PullRequests == nil {
		state.PullRequests = map[string]workspace.PullRequestState{}
	}
	state.PullRequests[resolution.Repo.Name] = workspace.PullRequestState{
		Repo:       payload.Repo,
		Number:     payload.Number,
		URL:        payload.URL,
		Title:      payload.Title,
		Body:       payload.Body,
		Draft:      payload.Draft,
		State:      payload.State,
		BaseRepo:   payload.BaseRepo,
		BaseBranch: payload.BaseBranch,
		HeadRepo:   payload.HeadRepo,
		HeadBranch: payload.HeadBranch,
		UpdatedAt:  s.clock().Format(time.RFC3339),
	}
	if err := s.workspaces.SaveState(ctx, resolution.WorkspaceRoot, state); err != nil && s.logf != nil {
		s.logf("workset: unable to save workspace state for PR tracking: %v", err)
	}
}

func remoteBranchExists(ctx context.Context, repoPath, remote, branch string, runner CommandRunner) (bool, error) {
	if strings.TrimSpace(repoPath) == "" {
		return false, errors.New("repo path required")
	}
	if strings.TrimSpace(remote) == "" {
		return false, ValidationError{Message: "remote name required to verify head branch"}
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return false, ValidationError{Message: "head branch required"}
	}
	result, err := runner(ctx, repoPath, []string{"git", "ls-remote", "--heads", remote, branch}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "unable to verify remote head branch"
		}
		return false, ValidationError{Message: message}
	}
	return strings.TrimSpace(result.Stdout) != "", nil
}

func buildRepoPatch(ctx context.Context, repoPath string, limit int, runner CommandRunner) (string, error) {
	if repoPath == "" {
		return "", errors.New("repo path required")
	}
	parts := []string{}
	staged, err := runGitDiff(ctx, repoPath, runner, true, "")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(staged) != "" {
		parts = append(parts, staged)
	}
	unstaged, err := runGitDiff(ctx, repoPath, runner, false, "")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(unstaged) != "" {
		parts = append(parts, unstaged)
	}
	untracked, err := gitUntracked(ctx, repoPath, runner)
	if err != nil {
		return "", err
	}
	for _, file := range untracked {
		diff, err := gitDiffNoIndex(ctx, repoPath, runner, file)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(diff) != "" {
			parts = append(parts, diff)
		}
	}
	patch := strings.Join(parts, "\n")
	if limit > 0 && len(patch) > limit {
		patch = patch[:limit] + "\n... (diff truncated)\n"
	}
	return patch, nil
}

func runGitDiff(ctx context.Context, repoPath string, runner CommandRunner, staged bool, file string) (string, error) {
	args := []string{"git", "diff"}
	if staged {
		args = append(args, "--cached")
	}
	if file != "" {
		args = append(args, "--", file)
	}
	result, err := runner(ctx, repoPath, args, os.Environ(), "")
	if err != nil && result.ExitCode != 1 {
		return "", err
	}
	return result.Stdout, nil
}

func gitDiffNoIndex(ctx context.Context, repoPath string, runner CommandRunner, file string) (string, error) {
	if strings.TrimSpace(file) == "" {
		return "", nil
	}
	args := []string{"git", "diff", "--no-index", "--", "/dev/null", file}
	result, err := runner(ctx, repoPath, args, os.Environ(), "")
	if err != nil && result.ExitCode != 1 {
		return "", err
	}
	return result.Stdout, nil
}

func gitUntracked(ctx context.Context, repoPath string, runner CommandRunner) ([]string, error) {
	result, err := runner(ctx, repoPath, []string{"git", "ls-files", "--others", "--exclude-standard"}, os.Environ(), "")
	if err != nil && result.ExitCode != 1 {
		return nil, err
	}
	lines := strings.Split(result.Stdout, "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, line)
	}
	return files, nil
}

func gitHasUncommittedChanges(ctx context.Context, repoPath string, runner CommandRunner) (bool, error) {
	result, err := runner(ctx, repoPath, []string{"git", "status", "--porcelain"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "unable to check uncommitted changes"
		}
		return false, ValidationError{Message: message}
	}
	return strings.TrimSpace(result.Stdout) != "", nil
}

func gitAheadBehind(ctx context.Context, repoPath, branch string, runner CommandRunner) (int, int, error) {
	// Get upstream tracking branch
	upstreamResult, err := runner(ctx, repoPath, []string{"git", "rev-parse", "--abbrev-ref", branch + "@{upstream}"}, os.Environ(), "")
	if err != nil || upstreamResult.ExitCode != 0 {
		return 0, 0, ValidationError{Message: "no upstream tracking branch configured"}
	}
	upstream := strings.TrimSpace(upstreamResult.Stdout)
	if upstream == "" {
		return 0, 0, ValidationError{Message: "no upstream tracking branch configured"}
	}

	// Get ahead count
	aheadResult, err := runner(ctx, repoPath, []string{"git", "rev-list", "--count", upstream + ".." + branch}, os.Environ(), "")
	ahead := 0
	if err == nil && aheadResult.ExitCode == 0 {
		if parsed, parseErr := parseCount(aheadResult.Stdout); parseErr == nil {
			ahead = parsed
		}
	}

	// Get behind count
	behindResult, err := runner(ctx, repoPath, []string{"git", "rev-list", "--count", branch + ".." + upstream}, os.Environ(), "")
	behind := 0
	if err == nil && behindResult.ExitCode == 0 {
		if parsed, parseErr := parseCount(behindResult.Stdout); parseErr == nil {
			behind = parsed
		}
	}

	return ahead, behind, nil
}

func parseCount(output string) (int, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return 0, errors.New("empty output")
	}
	var count int
	_, err := fmt.Sscanf(output, "%d", &count)
	return count, err
}

func gitHeadSHA(ctx context.Context, repoPath string, runner CommandRunner) (string, error) {
	result, err := runner(ctx, repoPath, []string{"git", "rev-parse", "HEAD"}, os.Environ(), "")
	if err != nil || result.ExitCode != 0 {
		return "", errors.New("unable to get HEAD SHA")
	}
	return strings.TrimSpace(result.Stdout), nil
}
