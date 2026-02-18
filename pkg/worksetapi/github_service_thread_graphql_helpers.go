package worksetapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// threadInfo holds thread ID and resolved state for a comment.
type threadInfo struct {
	ThreadID   string
	IsResolved bool
}

type graphQLReviewThreadsResponse struct {
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

const reviewThreadsQuery = `query($owner: String!, $repo: String!, $number: Int!, $after: String) {
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

// graphQLResolveThread calls the GitHub GraphQL API to resolve/unresolve a thread by node ID.
func graphQLResolveThread(ctx context.Context, token, host, threadID string, resolve bool) (bool, error) {
	endpoint := graphQLEndpoint(host)

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

// graphQLReviewThreadMap fetches review threads for a PR and maps comment node IDs to thread info.
func graphQLReviewThreadMap(ctx context.Context, token, host, owner, repo string, number int) (map[string]threadInfo, error) {
	endpoint := graphQLEndpoint(host)

	threadMap := make(map[string]threadInfo)
	var cursor *string

	for {
		page, err := fetchReviewThreadsPage(ctx, endpoint, token, owner, repo, number, cursor)
		if err != nil {
			return nil, err
		}

		addThreadsToCommentMap(threadMap, page)

		next, ok := nextReviewThreadCursor(page)
		if !ok {
			break
		}
		cursor = &next
	}

	return threadMap, nil
}

func graphQLEndpoint(host string) string {
	endpoint := "https://api.github.com/graphql"
	if host != "" && host != defaultGitHubHost {
		endpoint = fmt.Sprintf("https://%s/api/graphql", host)
	}
	return endpoint
}

func fetchReviewThreadsPage(ctx context.Context, endpoint, token, owner, repo string, number int, cursor *string) (graphQLReviewThreadsResponse, error) {
	payload := map[string]any{
		"query": reviewThreadsQuery,
		"variables": map[string]any{
			"owner":  owner,
			"repo":   repo,
			"number": number,
			"after":  cursor,
		},
	}
	body, err := doGraphQLRequest(ctx, endpoint, token, payload)
	if err != nil {
		return graphQLReviewThreadsResponse{}, err
	}

	var page graphQLReviewThreadsResponse
	if err := json.Unmarshal(body, &page); err != nil {
		return graphQLReviewThreadsResponse{}, err
	}
	if len(page.Errors) > 0 {
		return graphQLReviewThreadsResponse{}, ValidationError{Message: page.Errors[0].Message}
	}
	return page, nil
}

func doGraphQLRequest(ctx context.Context, endpoint, token string, payload any) ([]byte, error) {
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
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ValidationError{Message: "GraphQL request failed: " + string(respBody)}
	}
	return respBody, nil
}

func addThreadsToCommentMap(threadMap map[string]threadInfo, page graphQLReviewThreadsResponse) {
	for _, thread := range page.Data.Repository.PullRequest.ReviewThreads.Nodes {
		if thread.ID == "" {
			continue
		}
		for _, comment := range thread.Comments.Nodes {
			if comment.ID == "" {
				continue
			}
			threadMap[comment.ID] = threadInfo{
				ThreadID:   thread.ID,
				IsResolved: thread.IsResolved,
			}
		}
	}
}

func nextReviewThreadCursor(page graphQLReviewThreadsResponse) (string, bool) {
	pageInfo := page.Data.Repository.PullRequest.ReviewThreads.PageInfo
	if !pageInfo.HasNextPage {
		return "", false
	}
	next := strings.TrimSpace(pageInfo.EndCursor)
	if next == "" {
		return "", false
	}
	return next, true
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
