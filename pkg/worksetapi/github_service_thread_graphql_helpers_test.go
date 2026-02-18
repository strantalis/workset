package worksetapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGraphQLResolveThreadResolveMutation(t *testing.T) {
	var receivedQuery string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/graphql" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("unexpected authorization header: %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var payload struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		receivedQuery = payload.Query

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"resolveReviewThread":{"thread":{"isResolved":true}}}}`))
	}))
	defer server.Close()

	originalClient := http.DefaultClient
	http.DefaultClient = server.Client()
	t.Cleanup(func() {
		http.DefaultClient = originalClient
	})

	host := strings.TrimPrefix(server.URL, "https://")
	resolved, err := graphQLResolveThread(context.Background(), "test-token", host, "THREAD_ID", true)
	if err != nil {
		t.Fatalf("graphQLResolveThread: %v", err)
	}
	if !resolved {
		t.Fatalf("expected resolved=true")
	}
	if !strings.Contains(receivedQuery, "resolveReviewThread") {
		t.Fatalf("expected resolve mutation, got: %q", receivedQuery)
	}
	if !strings.Contains(receivedQuery, "THREAD_ID") {
		t.Fatalf("expected thread id in query, got: %q", receivedQuery)
	}
}

func TestGraphQLResolveThreadReturnsValidationErrorForGraphQLErrors(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"mutation failed"}]}`))
	}))
	defer server.Close()

	originalClient := http.DefaultClient
	http.DefaultClient = server.Client()
	t.Cleanup(func() {
		http.DefaultClient = originalClient
	})

	host := strings.TrimPrefix(server.URL, "https://")
	_, err := graphQLResolveThread(context.Background(), "token", host, "THREAD_ID", false)
	if err == nil {
		t.Fatalf("expected error")
	}
	validationErr := requireErrorType[ValidationError](t, err)
	if validationErr.Message != "mutation failed" {
		t.Fatalf("unexpected message: %q", validationErr.Message)
	}
}

func TestGraphQLGetThreadIDReturnsThreadForComment(t *testing.T) {
	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var payload struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		receivedQuery = payload.Query

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"node": {
					"id": "COMMENT_NODE",
					"pullRequest": {
						"reviewThreads": {
							"nodes": [
								{"id":"THREAD_A","comments":{"nodes":[{"id":"OTHER_COMMENT"}]}},
								{"id":"THREAD_B","comments":{"nodes":[{"id":"COMMENT_NODE"}]}}
							]
						}
					}
				}
			}
		}`))
	}))
	defer server.Close()

	threadID, err := graphQLGetThreadID(context.Background(), server.URL, "token-123", "COMMENT_NODE")
	if err != nil {
		t.Fatalf("graphQLGetThreadID: %v", err)
	}
	if threadID != "THREAD_B" {
		t.Fatalf("unexpected thread id: %q", threadID)
	}
	if !strings.Contains(receivedQuery, "COMMENT_NODE") {
		t.Fatalf("expected comment node in query, got: %q", receivedQuery)
	}
}

func TestGraphQLGetThreadIDReturnsValidationErrorWhenThreadMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"node": {
					"id": "COMMENT_NODE",
					"pullRequest": {
						"reviewThreads": {
							"nodes": [
								{"id":"THREAD_A","comments":{"nodes":[{"id":"OTHER_COMMENT"}]}}
							]
						}
					}
				}
			}
		}`))
	}))
	defer server.Close()

	_, err := graphQLGetThreadID(context.Background(), server.URL, "token-123", "COMMENT_NODE")
	if err == nil {
		t.Fatalf("expected error")
	}
	validationErr := requireErrorType[ValidationError](t, err)
	if validationErr.Message != "could not find thread for comment" {
		t.Fatalf("unexpected message: %q", validationErr.Message)
	}
}

func TestGraphQLReviewThreadMapPaginatesAndBuildsCommentMap(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/graphql" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("unexpected authorization header: %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var payload struct {
			Variables struct {
				After any `json:"after"`
			} `json:"variables"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			_, _ = w.Write([]byte(`{
				"data": {
					"repository": {
						"pullRequest": {
							"reviewThreads": {
								"pageInfo": {"hasNextPage": true, "endCursor": "CURSOR_2"},
								"nodes": [
									{
										"id":"THREAD_A",
										"isResolved": false,
										"comments":{"nodes":[{"id":"COMMENT_A"}]}
									}
								]
							}
						}
					}
				}
			}`))
			return
		}

		if payload.Variables.After != "CURSOR_2" {
			t.Fatalf("unexpected cursor on second page: %#v", payload.Variables.After)
		}
		_, _ = w.Write([]byte(`{
			"data": {
				"repository": {
					"pullRequest": {
						"reviewThreads": {
							"pageInfo": {"hasNextPage": false, "endCursor": ""},
							"nodes": [
								{
									"id":"THREAD_B",
									"isResolved": true,
									"comments":{"nodes":[{"id":"COMMENT_B"}]}
								}
							]
						}
					}
				}
			}
		}`))
	}))
	defer server.Close()

	originalClient := http.DefaultClient
	http.DefaultClient = server.Client()
	t.Cleanup(func() {
		http.DefaultClient = originalClient
	})

	host := strings.TrimPrefix(server.URL, "https://")
	got, err := graphQLReviewThreadMap(context.Background(), "token-123", host, "owner", "repo", 7)
	if err != nil {
		t.Fatalf("graphQLReviewThreadMap: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected 2 requests, got %d", requests)
	}
	if got["COMMENT_A"] != (threadInfo{ThreadID: "THREAD_A", IsResolved: false}) {
		t.Fatalf("unexpected comment map for COMMENT_A: %#v", got["COMMENT_A"])
	}
	if got["COMMENT_B"] != (threadInfo{ThreadID: "THREAD_B", IsResolved: true}) {
		t.Fatalf("unexpected comment map for COMMENT_B: %#v", got["COMMENT_B"])
	}
}

func TestGraphQLReviewThreadMapReturnsValidationErrorForGraphQLErrors(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"query failed"}]}`))
	}))
	defer server.Close()

	originalClient := http.DefaultClient
	http.DefaultClient = server.Client()
	t.Cleanup(func() {
		http.DefaultClient = originalClient
	})

	host := strings.TrimPrefix(server.URL, "https://")
	_, err := graphQLReviewThreadMap(context.Background(), "token", host, "owner", "repo", 7)
	if err == nil {
		t.Fatalf("expected error")
	}
	validationErr := requireErrorType[ValidationError](t, err)
	if validationErr.Message != "query failed" {
		t.Fatalf("unexpected message: %q", validationErr.Message)
	}
}

func TestGraphQLReviewThreadMapStopsWhenCursorMissing(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"repository": {
					"pullRequest": {
						"reviewThreads": {
							"pageInfo": {"hasNextPage": true, "endCursor": "   "},
							"nodes": [
								{
									"id":"THREAD_A",
									"isResolved": false,
									"comments":{"nodes":[{"id":"COMMENT_A"}]}
								}
							]
						}
					}
				}
			}
		}`))
	}))
	defer server.Close()

	originalClient := http.DefaultClient
	http.DefaultClient = server.Client()
	t.Cleanup(func() {
		http.DefaultClient = originalClient
	})

	host := strings.TrimPrefix(server.URL, "https://")
	got, err := graphQLReviewThreadMap(context.Background(), "token", host, "owner", "repo", 9)
	if err != nil {
		t.Fatalf("graphQLReviewThreadMap: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected 1 request, got %d", requests)
	}
	if got["COMMENT_A"] != (threadInfo{ThreadID: "THREAD_A", IsResolved: false}) {
		t.Fatalf("unexpected comment map: %#v", got["COMMENT_A"])
	}
}
